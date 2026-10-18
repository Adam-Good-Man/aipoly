package polymarket

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// OrderBookLevel is one bid/ask row.
type OrderBookLevel struct {
	Price float64
	Size  float64
}

// MarketDossier is context for direction judgment on a shortlisted market.
type MarketDossier struct {
	Snapshot        MarketSnapshot
	FullDescription string
	NewsHeadlines   []string // RSSHub / news lines already formatted
	BookBids        []OrderBookLevel
	BookAsks        []OrderBookLevel
	LastTradePrice  float64
	Spread          float64
	PriceHistory    []PricePoint
	RecentTrades    []RecentTrade
	Comments        []string
	OpenInterest    float64
	TopHolders      []string
	IntelNotes      []string // fetch errors / gaps for the model
}

// PricePoint is a simplified history sample.
type PricePoint struct {
	Ts    int64
	Price float64
}

// RecentTrade is a public tape print.
type RecentTrade struct {
	Side      string
	Price     float64
	Size      float64
	Timestamp int64
	Outcome   string
}

// BuildMarketDossier gathers order book, tape, comments, and related intel.
// Missing feeds are recorded in IntelNotes rather than failing the whole dossier.
func (c *Client) BuildMarketDossier(snap MarketSnapshot, fullDescription string) MarketDossier {
	d := MarketDossier{
		Snapshot:        snap,
		FullDescription: strings.TrimSpace(fullDescription),
	}
	if d.FullDescription == "" {
		d.FullDescription = strings.TrimSpace(snap.Description)
	}

	tokenID := primaryTokenID(snap)
	if tokenID != "" {
		if bids, asks, last, err := c.FetchOrderBook(tokenID); err != nil {
			d.IntelNotes = append(d.IntelNotes, "order_book: "+err.Error())
		} else {
			d.BookBids = bids
			d.BookAsks = asks
			d.LastTradePrice = last
			if len(bids) > 0 && len(asks) > 0 {
				d.Spread = asks[0].Price - bids[0].Price
			}
		}
		if hist, err := c.FetchPricesHistory(tokenID, "1d", 30); err != nil {
			d.IntelNotes = append(d.IntelNotes, "price_history: "+err.Error())
		} else {
			d.PriceHistory = hist
		}
	} else {
		d.IntelNotes = append(d.IntelNotes, "order_book: missing token_id")
	}

	if snap.ConditionID != "" {
		if trades, err := c.FetchMarketTrades(snap.ConditionID, 25); err != nil {
			d.IntelNotes = append(d.IntelNotes, "recent_trades: "+err.Error())
		} else {
			d.RecentTrades = trades
		}
		if comments, err := c.FetchMarketComments(snap.ConditionID, 15); err != nil {
			d.IntelNotes = append(d.IntelNotes, "comments: "+err.Error())
		} else {
			d.Comments = comments
		}
		if oi, holders, err := c.FetchHoldersSummary(snap.ConditionID, 5); err != nil {
			d.IntelNotes = append(d.IntelNotes, "holders: "+err.Error())
		} else {
			d.OpenInterest = oi
			d.TopHolders = holders
		}
	}

	return d
}

func primaryTokenID(snap MarketSnapshot) string {
	for _, t := range snap.TokenIDs {
		if strings.TrimSpace(t) != "" {
			return t
		}
	}
	return ""
}

// FetchOrderBook returns top-of-book levels for a CLOB token.
func (c *Client) FetchOrderBook(tokenID string) (bids, asks []OrderBookLevel, last float64, err error) {
	var resp struct {
		Bids []struct {
			Price string `json:"price"`
			Size  string `json:"size"`
		} `json:"bids"`
		Asks []struct {
			Price string `json:"price"`
			Size  string `json:"size"`
		} `json:"asks"`
		LastTradePrice string `json:"last_trade_price"`
	}
	if err = c.getJSON(clobBase+"/book?token_id="+url.QueryEscape(tokenID), &resp); err != nil {
		return nil, nil, 0, err
	}
	bids = parseBookSide(resp.Bids, 8)
	asks = parseBookSide(resp.Asks, 8)
	sort.Slice(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	fmt.Sscanf(resp.LastTradePrice, "%f", &last)
	return bids, asks, last, nil
}

func parseBookSide(rows []struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}, limit int) []OrderBookLevel {
	out := make([]OrderBookLevel, 0, limit)
	for _, r := range rows {
		var p, s float64
		fmt.Sscanf(r.Price, "%f", &p)
		fmt.Sscanf(r.Size, "%f", &s)
		if p <= 0 {
			continue
		}
		out = append(out, OrderBookLevel{Price: p, Size: s})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// FetchPricesHistory pulls recent midpoint history for a token.
func (c *Client) FetchPricesHistory(tokenID, interval string, fidelity int) ([]PricePoint, error) {
	if interval == "" {
		interval = "1d"
	}
	if fidelity <= 0 {
		fidelity = 60
	}
	params := url.Values{}
	params.Set("market", tokenID)
	params.Set("interval", interval)
	params.Set("fidelity", fmt.Sprintf("%d", fidelity))
	rawURL := clobBase + "/prices-history?" + params.Encode()

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "aiPoly/1.0")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("prices-history HTTP %d", resp.StatusCode)
	}

	var points []PricePoint
	var asObj struct {
		History []struct {
			T int64   `json:"t"`
			P float64 `json:"p"`
		} `json:"history"`
	}
	if err := json.Unmarshal(body, &asObj); err == nil && len(asObj.History) > 0 {
		for _, h := range asObj.History {
			points = append(points, PricePoint{Ts: h.T, Price: h.P})
		}
	} else {
		var arr []struct {
			T int64   `json:"t"`
			P float64 `json:"p"`
		}
		if err := json.Unmarshal(body, &arr); err != nil {
			return nil, fmt.Errorf("parse prices-history: %w", err)
		}
		for _, h := range arr {
			points = append(points, PricePoint{Ts: h.T, Price: h.P})
		}
	}
	return downsamplePoints(points, 24), nil
}

func downsamplePoints(in []PricePoint, max int) []PricePoint {
	if max <= 0 || len(in) <= max {
		return in
	}
	step := len(in) / max
	if step < 1 {
		step = 1
	}
	out := make([]PricePoint, 0, max)
	for i := 0; i < len(in) && len(out) < max; i += step {
		out = append(out, in[i])
	}
	if len(out) == 0 || out[len(out)-1].Ts != in[len(in)-1].Ts {
		out = append(out, in[len(in)-1])
	}
	return out
}

// FetchMarketTrades returns recent public trades for a condition.
func (c *Client) FetchMarketTrades(conditionID string, limit int) ([]RecentTrade, error) {
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("market", conditionID)
	params.Set("limit", fmt.Sprintf("%d", limit))
	var rows []struct {
		Side      string  `json:"side"`
		Price     float64 `json:"price"`
		Size      float64 `json:"size"`
		Timestamp int64   `json:"timestamp"`
		Outcome   string  `json:"outcome"`
	}
	if err := c.getJSON(dataAPIBase+"/trades?"+params.Encode(), &rows); err != nil {
		return nil, err
	}
	out := make([]RecentTrade, 0, len(rows))
	for _, r := range rows {
		out = append(out, RecentTrade{
			Side:      r.Side,
			Price:     r.Price,
			Size:      r.Size,
			Timestamp: r.Timestamp,
			Outcome:   r.Outcome,
		})
	}
	return out, nil
}

// FetchMarketComments returns recent Gamma comment bodies (sentiment proxy).
func (c *Client) FetchMarketComments(conditionID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 12
	}
	params := url.Values{}
	params.Set("condition_id", conditionID)
	params.Set("limit", fmt.Sprintf("%d", limit))
	var rows []struct {
		Body      string `json:"body"`
		CreatedAt string `json:"createdAt"`
	}
	if err := c.getJSON(gammaBase+"/comments?"+params.Encode(), &rows); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		b := strings.TrimSpace(r.Body)
		if b == "" {
			continue
		}
		if len([]rune(b)) > 180 {
			rns := []rune(b)
			b = string(rns[:180]) + "…"
		}
		out = append(out, b)
	}
	return out, nil
}

// FetchHoldersSummary returns a rough open-interest proxy and top holder labels.
func (c *Client) FetchHoldersSummary(conditionID string, limit int) (oi float64, holders []string, err error) {
	if limit <= 0 {
		limit = 5
	}
	params := url.Values{}
	params.Set("market", conditionID)
	params.Set("limit", fmt.Sprintf("%d", limit))
	var rows []struct {
		ProxyWallet string  `json:"proxyWallet"`
		Name        string  `json:"name"`
		Amount      float64 `json:"amount"`
		Outcome     string  `json:"outcome"`
	}
	// data-api holders endpoint shape varies; tolerate failure.
	if err = c.getJSON(dataAPIBase+"/holders?"+params.Encode(), &rows); err != nil {
		// Fallback: open-interest endpoint
		var oiResp []struct {
			ConditionID string  `json:"conditionId"`
			Value       float64 `json:"value"`
		}
		params2 := url.Values{}
		params2.Set("market", conditionID)
		if err2 := c.getJSON(dataAPIBase+"/oi?"+params2.Encode(), &oiResp); err2 != nil {
			return 0, nil, err
		}
		for _, r := range oiResp {
			oi += r.Value
		}
		return oi, nil, nil
	}
	for _, r := range rows {
		oi += r.Amount
		label := strings.TrimSpace(r.Name)
		if label == "" {
			label = shortenAddr(r.ProxyWallet)
		}
		holders = append(holders, fmt.Sprintf("%s %s $%.0f", label, r.Outcome, r.Amount))
	}
	return oi, holders, nil
}

func shortenAddr(a string) string {
	a = strings.TrimSpace(a)
	if len(a) < 10 {
		return a
	}
	return a[:6] + "…" + a[len(a)-4:]
}

// FormatDossierForPrompt renders a compact but rich text block for the deep AI pass.
func FormatDossierForPrompt(d MarketDossier) string {
	s := d.Snapshot
	var b strings.Builder
	b.WriteString(fmt.Sprintf("id=%s days~%d vol=$%.0f liq=$%.0f\n", s.MarketID, s.DaysToEnd, s.Volume, s.Liquidity))
	if s.EventTitle != "" {
		b.WriteString("Event: " + truncateRunes(s.EventTitle, 100) + "\n")
	}
	b.WriteString("Q: " + truncateRunes(s.Question, 160) + "\n")
	if len(d.NewsHeadlines) > 0 {
		b.WriteString("News(RSSHub) — PRIMARY signal (specialty sources, newest-first; cite recent headlines):\n")
		for _, line := range d.NewsHeadlines {
			// Already a formatted multi-line block from news.FormatItemsForPrompt.
			b.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				b.WriteString("\n")
			}
		}
	} else {
		b.WriteString("News(RSSHub): (none fetched) — rely on odds/book/tape only; lower confidence\n")
	}
	if d.FullDescription != "" {
		b.WriteString("FullCtx: " + truncateRunes(d.FullDescription, 500) + "\n")
	}
	for i, o := range s.Outcomes {
		p := 0.0
		if i < len(s.Prices) {
			p = s.Prices[i]
		}
		b.WriteString(fmt.Sprintf("  %s %.1f¢\n", truncateRunes(o, 40), p*100))
	}
	if d.LastTradePrice > 0 || d.Spread != 0 {
		b.WriteString(fmt.Sprintf("Tape: last=%.1f¢ spread=%.1f¢\n", d.LastTradePrice*100, d.Spread*100))
	}
	if len(d.BookBids) > 0 || len(d.BookAsks) > 0 {
		b.WriteString("OrderBook top:\n")
		for i, a := range d.BookAsks {
			if i >= 4 {
				break
			}
			b.WriteString(fmt.Sprintf("  ASK %.1f¢ x%.0f\n", a.Price*100, a.Size))
		}
		for i, bid := range d.BookBids {
			if i >= 4 {
				break
			}
			b.WriteString(fmt.Sprintf("  BID %.1f¢ x%.0f\n", bid.Price*100, bid.Size))
		}
	}
	if len(d.PriceHistory) > 0 {
		b.WriteString("PriceHistory(1d samples): ")
		for i, p := range d.PriceHistory {
			if i > 0 {
				b.WriteString(", ")
			}
			ts := time.Unix(p.Ts, 0).UTC().Format("01-02 15:04")
			b.WriteString(fmt.Sprintf("%s=%.0f¢", ts, p.Price*100))
		}
		b.WriteString("\n")
	}
	if len(d.RecentTrades) > 0 {
		b.WriteString("RecentTrades:\n")
		for i, t := range d.RecentTrades {
			if i >= 12 {
				break
			}
			b.WriteString(fmt.Sprintf("  %s %s @%.1f¢ sz=%.1f\n", t.Side, truncateRunes(t.Outcome, 24), t.Price*100, t.Size))
		}
	}
	if d.OpenInterest > 0 || len(d.TopHolders) > 0 {
		b.WriteString(fmt.Sprintf("Holders/OI proxy: oi~%.0f\n", d.OpenInterest))
		for i, h := range d.TopHolders {
			if i >= 5 {
				break
			}
			b.WriteString("  " + h + "\n")
		}
	}
	if len(d.Comments) > 0 {
		b.WriteString("MarketComments(sentiment):\n")
		for i, cmt := range d.Comments {
			if i >= 8 {
				break
			}
			b.WriteString("  - " + cmt + "\n")
		}
	}
	if len(d.IntelNotes) > 0 {
		b.WriteString("IntelGaps: " + strings.Join(d.IntelNotes, "; ") + "\n")
	}
	return b.String()
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
