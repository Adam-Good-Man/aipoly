package news

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"aipoly/api/services/polymarket"
)

// Item is one RSS headline used for direction judgment.
type Item struct {
	Title       string
	Link        string
	Summary     string
	Published   string
	PublishedAt time.Time // zero if unknown
	SourceRoute string
	SourceKind  string // esports|crypto|politics|sports|economy|tech|general
	Score       int
	AgeHours    float64 // computed at fetch time; -1 if unknown
}

// Client fetches news via an RSSHub instance.
type Client struct {
	baseURL string
	http    *http.Client
	enabled bool
	now     func() time.Time // injectable for tests
}

// NewClient builds an RSSHub client. Empty baseURL disables fetching.
func NewClient(baseURL string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &Client{
		baseURL: baseURL,
		enabled: baseURL != "",
		now:     time.Now,
		http: &http.Client{
			Timeout: 25 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.enabled
}

func (c *Client) clock() time.Time {
	if c != nil && c.now != nil {
		return c.now()
	}
	return time.Now()
}

// MarketKind classifies a Polymarket item for specialty news routing.
type MarketKind string

const (
	KindEsportsCS       MarketKind = "esports_cs"
	KindEsportsLoL      MarketKind = "esports_lol"
	KindEsportsValorant MarketKind = "esports_valorant"
	KindEsportsOther    MarketKind = "esports"
	KindCrypto          MarketKind = "crypto"
	KindPolitics        MarketKind = "politics"
	KindSports          MarketKind = "sports"
	KindEconomy         MarketKind = "economy"
	KindTech            MarketKind = "tech"
	KindGeneral         MarketKind = "general"
)

// FetchForMarket pulls the newest relevant headlines from specialty sources
// for this market kind (esports → esports wires, crypto → crypto wires, etc.).
func (c *Client) FetchForMarket(snap polymarket.MarketSnapshot, limit int) ([]Item, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("rsshub disabled")
	}
	if limit <= 0 {
		limit = 8
	}

	kind := ClassifyMarket(snap)
	maxAge := maxAgeForKind(kind)
	keywords := relevanceKeywords(snap)
	seen := make(map[string]bool)
	var pool []Item
	now := c.clock()

	addItems := func(items []Item, route string) {
		for _, it := range items {
			key := strings.ToLower(strings.TrimSpace(it.Title))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			it.SourceRoute = route
			it.SourceKind = string(kind)
			it.Score = scoreHeadline(it, keywords)
			it.PublishedAt = parsePublished(it.Published)
			if !it.PublishedAt.IsZero() {
				it.AgeHours = now.Sub(it.PublishedAt).Hours()
			} else {
				it.AgeHours = -1
			}
			pool = append(pool, it)
		}
	}

	// 1) Specialty / category feeds first (domain-correct + usually fresher).
	for _, route := range topicRoutesForKind(kind, snap) {
		items, err := c.fetchRoute(route, 50)
		if err != nil {
			log.Printf("rsshub %s %s: %v", kind, route, err)
			continue
		}
		addItems(items, route)
	}

	// 2) Optional Google News keyword search (often 503 on self-host — soft-fail).
	for _, q := range buildQueries(snap) {
		if matchedFresh(pool, maxAge, now) >= limit {
			break
		}
		route := "/google/news/" + url.PathEscape(q) + "/en/US/US:en"
		items, err := c.fetchRoute(route, limit*2)
		if err != nil {
			log.Printf("rsshub google %q: %v", q, err)
			continue
		}
		addItems(items, route)
	}

	if len(pool) == 0 {
		return nil, fmt.Errorf("no rsshub headlines for market kind=%s", kind)
	}

	// Relevance + recency: keep keyword hits within the freshness window.
	out := selectFreshRelevant(pool, maxAge, now, limit)
	if len(out) == 0 {
		// One-time widen (still require keyword match) for slow news markets.
		wide := maxAge * 2
		if wide < 72*time.Hour {
			wide = 72 * time.Hour
		}
		out = selectFreshRelevant(pool, wide, now, limit)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no fresh keyword-matched headlines kind=%s window=%s (scanned %d)",
			kind, maxAge, len(pool))
	}

	log.Printf("rsshub kind=%s window=%s got %d fresh headlines (scanned %d)",
		kind, maxAge, len(out), len(pool))
	return out, nil
}

func matchedFresh(items []Item, maxAge time.Duration, now time.Time) int {
	n := 0
	for _, it := range items {
		if it.Score >= 2 && isFresh(it, maxAge, now) {
			n++
		}
	}
	return n
}

func selectFreshRelevant(pool []Item, maxAge time.Duration, now time.Time, limit int) []Item {
	matched := make([]Item, 0, len(pool))
	for _, it := range pool {
		if it.Score < 2 {
			continue
		}
		if !isFresh(it, maxAge, now) {
			continue
		}
		matched = append(matched, it)
	}
	sort.SliceStable(matched, func(i, j int) bool {
		// Newest first, then higher relevance.
		ti, tj := matched[i].PublishedAt, matched[j].PublishedAt
		if !ti.IsZero() && !tj.IsZero() && !ti.Equal(tj) {
			return ti.After(tj)
		}
		if ti.IsZero() != tj.IsZero() {
			return !ti.IsZero() // prefer dated items
		}
		if matched[i].Score != matched[j].Score {
			return matched[i].Score > matched[j].Score
		}
		return matched[i].Published > matched[j].Published
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched
}

func isFresh(it Item, maxAge time.Duration, now time.Time) bool {
	if it.PublishedAt.IsZero() {
		// Undated AP items: keep only if highly relevant (score≥4), treat as "maybe recent".
		return it.Score >= 4
	}
	age := now.Sub(it.PublishedAt)
	if age < -time.Hour {
		// Clock skew / future dates — allow small skew.
		return true
	}
	return age <= maxAge
}

func maxAgeForKind(kind MarketKind) time.Duration {
	switch kind {
	case KindCrypto:
		return 48 * time.Hour
	case KindEsportsCS, KindEsportsLoL, KindEsportsValorant, KindEsportsOther:
		return 5 * 24 * time.Hour
	case KindSports:
		return 3 * 24 * time.Hour
	case KindEconomy, KindTech:
		return 4 * 24 * time.Hour
	case KindPolitics:
		return 7 * 24 * time.Hour
	default:
		return 5 * 24 * time.Hour
	}
}

// ClassifyMarket maps a snapshot to a specialty news domain.
func ClassifyMarket(snap polymarket.MarketSnapshot) MarketKind {
	q := strings.ToLower(snap.Question + " " + snap.EventTitle + " " + snap.Category)
	switch {
	case strings.Contains(q, "bitcoin") || strings.Contains(q, "ethereum") || strings.Contains(q, "crypto") ||
		strings.Contains(q, "btc") || strings.Contains(q, " eth") || strings.HasPrefix(q, "eth ") ||
		strings.Contains(q, "solana") || strings.Contains(q, " sol ") || strings.Contains(q, "xrp") ||
		strings.Contains(q, "dogecoin") || strings.Contains(q, "up or down") || strings.Contains(q, "updown"):
		return KindCrypto
	case strings.Contains(q, "valorant"):
		return KindEsportsValorant
	case strings.Contains(q, "lol") || strings.Contains(q, "league of legends") || strings.Contains(q, "lpl") ||
		strings.Contains(q, "lck") || strings.Contains(q, "lcs") || strings.Contains(q, "lec") ||
		strings.Contains(q, "worlds"):
		return KindEsportsLoL
	case strings.Contains(q, "counter-strike") || strings.Contains(q, "cs2") || strings.Contains(q, "cs:") ||
		strings.Contains(q, "blast") || strings.Contains(q, "hltv") || strings.Contains(q, "iem "):
		return KindEsportsCS
	case strings.Contains(q, "dota") || strings.Contains(q, "esport"):
		return KindEsportsOther
	case strings.Contains(q, "trump") || strings.Contains(q, "election") || strings.Contains(q, "senate") ||
		strings.Contains(q, "congress") || strings.Contains(q, "president") || strings.Contains(q, "governor") ||
		strings.Contains(q, "democrat") || strings.Contains(q, "republican") || strings.Contains(q, "white house") ||
		strings.Contains(q, "iran") || strings.Contains(q, "gaza") || strings.Contains(q, "ukraine"):
		return KindPolitics
	case strings.Contains(q, "fed") || strings.Contains(q, "inflation") || strings.Contains(q, "gdp") ||
		strings.Contains(q, "unemployment") || strings.Contains(q, "cpi") || strings.Contains(q, "rate cut") ||
		strings.Contains(q, "interest rate"):
		return KindEconomy
	case strings.Contains(q, "nba") || strings.Contains(q, "nfl") || strings.Contains(q, "mlb") ||
		strings.Contains(q, "nhl") || strings.Contains(q, "ufc") || strings.Contains(q, "fifa") ||
		strings.Contains(q, "soccer") || strings.Contains(q, "tennis") || strings.Contains(q, "golf"):
		return KindSports
	case strings.Contains(q, "openai") || strings.Contains(q, "nvidia") || strings.Contains(q, "microsoft") ||
		strings.Contains(q, "apple") || strings.Contains(q, "google") ||
		(strings.Contains(q, " ai ") || strings.HasPrefix(q, "ai ")):
		return KindTech
	case strings.Contains(q, " vs "):
		// Ambiguous matchup without sport tags — treat as sports wire.
		return KindSports
	default:
		return KindGeneral
	}
}

func topicRoutesForKind(kind MarketKind, snap polymarket.MarketSnapshot) []string {
	q := strings.ToLower(snap.Question + " " + snap.EventTitle)
	routes := make([]string, 0, 8)
	seen := map[string]bool{}
	add := func(r string) {
		if r == "" || seen[r] {
			return
		}
		seen[r] = true
		routes = append(routes, r)
	}

	switch kind {
	case KindCrypto:
		// Fast crypto wires — CoinDesk first (English coin names match Polymarket titles).
		add("/coindesk/news")
		add("/jin10")
		add("/wallstreetcn/live/global")
		add("/wallstreetcn/news/global")
		add("/36kr/newsflashes")
		add("/apnews/topics/business")
		add("/apnews/topics/technology")
	case KindEsportsCS:
		add("/telegram/channel/cs2news")
		add("/youtube/community/@HLTVorg")
		add("/youtube/community/@BLASTPremier")
		if strings.Contains(q, "blast") {
			add("/telegram/channel/blastpremier")
		}
	case KindEsportsLoL:
		add("/youtube/community/@LoLEsports")
		add("/youtube/community/@LCK")
		add("/apnews/topics/entertainment")
	case KindEsportsValorant:
		add("/youtube/community/@VALORANTEsports")
		add("/youtube/community/@LoLEsports")
		add("/apnews/topics/entertainment")
	case KindEsportsOther:
		add("/telegram/channel/cs2news")
		add("/youtube/community/@HLTVorg")
		add("/youtube/community/@LoLEsports")
		add("/youtube/community/@VALORANTEsports")
		add("/youtube/community/@BLASTPremier")
	case KindPolitics:
		add("/apnews/topics/politics")
		add("/apnews/topics/apf-topnews")
		add("/npr/1003")
		add("/npr/1001")
	case KindSports:
		add("/apnews/topics/sports")
		add("/apnews/topics/apf-topnews")
	case KindEconomy:
		add("/jin10")
		add("/wallstreetcn/live/global")
		add("/wallstreetcn/news/global")
		add("/apnews/topics/business")
		add("/npr/1001")
	case KindTech:
		add("/36kr/newsflashes")
		add("/wallstreetcn/news/global")
		add("/apnews/topics/technology")
		add("/apnews/topics/business")
	default:
		add("/apnews/topics/apf-topnews")
		add("/apnews/topics/politics")
		add("/apnews/topics/business")
		add("/npr/1001")
		add("/wallstreetcn/news/global")
	}
	return routes
}

// topicRoutes kept for tests / callers that only pass a snapshot.
func topicRoutes(snap polymarket.MarketSnapshot) []string {
	return topicRoutesForKind(ClassifyMarket(snap), snap)
}

func (c *Client) fetchRoute(route string, limit int) ([]Item, error) {
	u := c.baseURL + route
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, */*")
	req.Header.Set("User-Agent", "aiPoly/1.0 (+rsshub)")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, route)
	}
	if looksLikeRSSHubErrorPage(body) {
		return nil, fmt.Errorf("RSSHub error page from %s", route)
	}
	return parseRSS(body, limit)
}

func looksLikeRSSHubErrorPage(body []byte) bool {
	s := string(body)
	if strings.Contains(s, "<rss") || strings.Contains(s, "<feed") || strings.Contains(s, "<item>") {
		return false
	}
	return strings.Contains(s, "Welcome to RSSHub") || strings.Contains(s, "Something went wrong")
}

type rssFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
	Entries []atomEntry `xml:"entry"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type atomEntry struct {
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Content string `xml:"content"`
	Updated string `xml:"updated"`
	Link    struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

func parseRSS(body []byte, limit int) ([]Item, error) {
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}
	out := make([]Item, 0, limit)
	for _, it := range feed.Channel.Items {
		title := cleanText(it.Title)
		if title == "" {
			continue
		}
		out = append(out, Item{
			Title:     title,
			Link:      strings.TrimSpace(it.Link),
			Summary:   cleanText(it.Description),
			Published: strings.TrimSpace(it.PubDate),
		})
		if len(out) >= limit {
			return out, nil
		}
	}
	for _, it := range feed.Entries {
		title := cleanText(it.Title)
		if title == "" {
			continue
		}
		sum := cleanText(it.Summary)
		if sum == "" {
			sum = cleanText(it.Content)
		}
		out = append(out, Item{
			Title:     title,
			Link:      strings.TrimSpace(it.Link.Href),
			Summary:   sum,
			Published: strings.TrimSpace(it.Updated),
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func parsePublished(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

var tagRe = regexp.MustCompile(`(?s)<[^>]*>`)

func cleanText(s string) string {
	s = html.UnescapeString(s)
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > 280 {
		s = string([]rune(s)[:280]) + "…"
	}
	return strings.TrimSpace(s)
}

func buildQueries(snap polymarket.MarketSnapshot) []string {
	text := strings.TrimSpace(snap.EventTitle + " " + snap.Question)
	queries := make([]string, 0, 3)
	seen := map[string]bool{}

	add := func(q string) {
		q = strings.TrimSpace(q)
		q = strings.Trim(q, `"'`)
		if len(q) < 3 {
			return
		}
		key := strings.ToLower(q)
		if seen[key] {
			return
		}
		seen[key] = true
		queries = append(queries, q)
	}

	lower := strings.ToLower(text)
	if i := strings.Index(lower, " vs "); i >= 0 {
		left := trimSide(text[:i])
		right := trimSide(text[i+4:])
		if left != "" && right != "" {
			add(left + " " + right)
			add(left)
			add(right)
		}
	} else if i := strings.Index(lower, " vs. "); i >= 0 {
		left := trimSide(text[:i])
		right := trimSide(text[i+5:])
		if left != "" && right != "" {
			add(left + " " + right)
		}
	}

	if len(queries) == 0 {
		add(compactQuery(text, 6))
	}
	if q := compactQuery(snap.EventTitle, 5); q != "" {
		add(q)
	}
	return queries
}

func trimSide(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ":"); i > 0 && i < 18 {
		s = strings.TrimSpace(s[i+1:])
	}
	for _, sep := range []string{" (", " - ", " | ", " BO", " Map "} {
		if i := strings.Index(s, sep); i > 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	return compactQuery(s, 4)
}

func compactQuery(s string, maxTokens int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == '/' || r == '|'
	})
	stop := queryStopWords()
	kept := make([]string, 0, maxTokens)
	for _, p := range parts {
		p = strings.Trim(p, "()[]{}\"'")
		if len(p) < 2 {
			continue
		}
		if stop[strings.ToLower(p)] {
			continue
		}
		kept = append(kept, p)
		if len(kept) >= maxTokens {
			break
		}
	}
	return strings.Join(kept, " ")
}

func queryStopWords() map[string]bool {
	return map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "of": true,
		"will": true, "be": true, "on": true, "in": true, "to": true, "for": true,
		"win": true, "winner": true, "game": true, "map": true, "match": true,
		"vs": true, "vs.": true, "bo1": true, "bo3": true, "bo5": true,
		"by": true, "at": true, "from": true, "with": true, "before": true,
		"after": true, "end": true, "ends": true, "price": true, "above": true,
		"below": true, "than": true, "more": true, "less": true, "yes": true,
		"no": true, "up": true, "down": true, "july": true, "august": true,
		"september": true, "october": true, "november": true, "december": true,
		"january": true, "february": true, "march": true, "april": true, "june": true,
		"2024": true, "2025": true, "2026": true, "2027": true,
		"hit": true, "hits": true, "reach": true, "reaches": true, "pull": true, "pulls": true,
		"say": true, "says": true, "may": true, "might": true, "can": true, "could": true,
		"does": true, "did": true, "has": true, "have": true, "had": true, "is": true, "are": true,
		"was": true, "were": true, "this": true, "that": true, "these": true, "those": true,
		"what": true, "when": true, "where": true, "which": true, "who": true, "how": true,
		"new": true, "next": true, "first": true, "last": true, "over": true, "under": true,
		"into": true, "about": true, "against": true, "between": true, "during": true,
		"market": true, "markets": true, "odds": true, "polymarket": true, "event": true,
		"legend": true, "gaming": true, "esports": true, "team": true, "university": true,
	}
}

func relevanceKeywords(snap polymarket.MarketSnapshot) []string {
	raw := snap.EventTitle + " " + snap.Question
	for _, o := range snap.Outcomes {
		raw += " " + o
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == '/' || r == '|' || r == '-' || r == '?' || r == '!'
	})
	stop := queryStopWords()
	seen := map[string]bool{}
	out := make([]string, 0, 16)
	add := func(low string) {
		low = strings.TrimSpace(low)
		if low == "" || seen[low] {
			return
		}
		// Allow CJK tokens shorter than 3 runes (e.g. 以太).
		runes := []rune(low)
		minLen := 3
		if hasCJK(low) {
			minLen = 2
		}
		if len(runes) < minLen || stop[low] {
			return
		}
		if isMostlyDigits(low) {
			return
		}
		seen[low] = true
		out = append(out, low)
	}
	for _, p := range parts {
		p = strings.Trim(p, "()[]{}\"'“”‘’.:;$")
		add(strings.ToLower(p))
		if len(out) >= 14 {
			break
		}
	}
	for _, o := range snap.Outcomes {
		o = strings.TrimSpace(o)
		add(strings.ToLower(o))
		// Common esports shorthand.
		low := strings.ToLower(o)
		switch {
		case strings.Contains(low, "edward") || low == "edg":
			add("edg")
			add("edward")
		case strings.Contains(low, "anyone") && strings.Contains(low, "legend"):
			add("anyone's legend")
		case strings.HasPrefix(low, "g2"):
			add("g2")
		case strings.Contains(low, "gen.g") || strings.Contains(low, "geng"):
			add("gen.g")
			add("geng")
		}
	}
	joined := strings.ToLower(raw)
	switch {
	case strings.Contains(joined, "bitcoin") || strings.Contains(joined, "btc"):
		add("bitcoin")
		add("btc")
		add("比特币")
		add("btc")
	case strings.Contains(joined, "ethereum") || strings.Contains(joined, "eth"):
		add("ethereum")
		add("eth")
		add("以太坊")
		add("以太")
	}
	if strings.Contains(joined, "solana") || containsToken(out, "sol") {
		add("solana")
		add("sol")
		add("索拉纳")
	}
	if strings.Contains(joined, "crypto") {
		add("crypto")
		add("加密")
		add("加密货币")
		add("数字货币")
	}
	if strings.Contains(joined, "federal reserve") || containsToken(out, "fed") {
		add("fed")
		add("federal")
		add("powell")
	}
	return out
}

func hasCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func containsToken(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func isMostlyDigits(s string) bool {
	digits := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits > 0 && digits*2 >= len([]rune(s))
}

func scoreHeadline(it Item, keywords []string) int {
	if len(keywords) == 0 {
		return 0
	}
	titleTokens := tokenizeLower(it.Title)
	allTokens := tokenizeLower(it.Title + " " + it.Summary)
	titleLow := strings.ToLower(it.Title + " " + it.Summary)
	score := 0
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		minLen := 3
		if hasCJK(kw) {
			minLen = 2
		}
		if len([]rune(kw)) < minLen {
			continue
		}
		if strings.Contains(kw, " ") || strings.Contains(kw, "'") || hasCJK(kw) {
			if strings.Contains(titleLow, kw) {
				score += 3
				if hasCJK(kw) || strings.Contains(kw, " ") {
					continue
				}
			}
			if hasCJK(kw) || strings.Contains(kw, " ") || strings.Contains(kw, "'") {
				continue
			}
		}
		if allTokens[kw] {
			score++
			if titleTokens[kw] {
				score++
			}
		}
	}
	return score
}

func tokenizeLower(s string) map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if p != "" {
			out[p] = true
		}
	}
	return out
}

// FormatItemsForPrompt renders headlines for the AI direction pass.
func FormatItemsForPrompt(items []Item, max int) string {
	if max <= 0 {
		max = 8
	}
	var b strings.Builder
	kind := ""
	if len(items) > 0 {
		kind = items[0].SourceKind
	}
	if kind != "" {
		b.WriteString(fmt.Sprintf("(specialty=%s; newest-first; use only recent headlines for direction)\n", kind))
	}
	b.WriteString(fmt.Sprintf("(keyword-matched headlines: %d)\n", len(items)))
	for i, it := range items {
		if i >= max {
			break
		}
		b.WriteString(fmt.Sprintf("%d) %s", i+1, it.Title))
		if it.Score > 0 {
			b.WriteString(fmt.Sprintf(" [rel=%d]", it.Score))
		}
		if it.AgeHours >= 0 {
			if it.AgeHours < 48 {
				b.WriteString(fmt.Sprintf(" [%.0fh ago]", it.AgeHours))
			} else {
				b.WriteString(fmt.Sprintf(" [%.1fd ago]", it.AgeHours/24))
			}
		} else if it.Published != "" {
			b.WriteString(" [" + it.Published + "]")
		}
		b.WriteString("\n")
		if it.Summary != "" {
			b.WriteString("   " + it.Summary + "\n")
		}
		if it.Link != "" {
			b.WriteString("   link: " + it.Link + "\n")
		}
		if it.SourceRoute != "" {
			b.WriteString("   via: " + it.SourceRoute + "\n")
		}
	}
	return b.String()
}
