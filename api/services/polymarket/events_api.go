package polymarket

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const eventsFetchCap = 100

func EventsFetchCap() int {
	return eventsFetchCap
}

var eventCategoryOrder = []string{"体育", "政治", "加密货币", "电竞", "娱乐", "其他"}

func FetchTrendingEventsPaged(c *Client, limit, offset int) ([]GammaEvent, error) {
	if limit < 1 {
		limit = 10
	}
	params := url.Values{}
	params.Set("active", "true")
	params.Set("closed", "false")
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("offset", fmt.Sprintf("%d", offset))
	params.Set("order", "volume24hr")
	params.Set("ascending", "false")

	var events []GammaEvent
	if err := c.getJSON(gammaBase+"/events?"+params.Encode(), &events); err != nil {
		return nil, err
	}
	return events, nil
}

func FetchEventByID(c *Client, id string) (*GammaEvent, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("event id required")
	}
	var event GammaEvent
	if err := c.getJSON(gammaBase+"/events/"+url.PathEscape(id), &event); err != nil {
		return nil, err
	}
	if event.ID == "" {
		return nil, fmt.Errorf("event %s not found", id)
	}
	return &event, nil
}

func EventCategory(e GammaEvent) string {
	if c := strings.TrimSpace(e.Category); c != "" {
		return normalizeCategoryName(c)
	}
	for _, tag := range e.Tags {
		if cat := categoryFromTag(tag.Slug, tag.Label); cat != "" {
			return cat
		}
	}
	return inferCategoryFromTitle(e.Title)
}

func normalizeCategoryName(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(lower, "sport"), strings.Contains(lower, "体育"):
		return "体育"
	case strings.Contains(lower, "politic"), strings.Contains(lower, "政治"), strings.Contains(lower, "election"):
		return "政治"
	case strings.Contains(lower, "crypto"), strings.Contains(lower, "加密"), strings.Contains(lower, "bitcoin"):
		return "加密货币"
	case strings.Contains(lower, "esport"), strings.Contains(lower, "game"), strings.Contains(lower, "电竞"), strings.Contains(lower, "lol"), strings.Contains(lower, "cs"):
		return "电竞"
	case strings.Contains(lower, "entertain"), strings.Contains(lower, "娱乐"), strings.Contains(lower, "pop"):
		return "娱乐"
	default:
		if raw == "其他" {
			return "其他"
		}
		return "其他"
	}
}

func categoryFromTag(slug, label string) string {
	s := strings.ToLower(slug + " " + label)
	switch {
	case strings.Contains(s, "sport"), strings.Contains(s, "soccer"), strings.Contains(s, "fifa"),
		strings.Contains(s, "nba"), strings.Contains(s, "nfl"), strings.Contains(s, "ufc"),
		strings.Contains(s, "world-cup"), strings.Contains(s, "tennis"), strings.Contains(s, "mlb"):
		return "体育"
	case strings.Contains(s, "politic"), strings.Contains(s, "election"), strings.Contains(s, "trump"),
		strings.Contains(s, "congress"), strings.Contains(s, "geopolit"):
		return "政治"
	case strings.Contains(s, "crypto"), strings.Contains(s, "bitcoin"), strings.Contains(s, "ethereum"), strings.Contains(s, "defi"):
		return "加密货币"
	case strings.Contains(s, "esport"), strings.Contains(s, "lol"), strings.Contains(s, "counter-strike"),
		strings.Contains(s, "dota"), strings.Contains(s, "valorant"), strings.Contains(s, "msi"):
		return "电竞"
	case strings.Contains(s, "entertain"), strings.Contains(s, "movie"), strings.Contains(s, "music"), strings.Contains(s, "celebr"):
		return "娱乐"
	default:
		return ""
	}
}

func inferCategoryFromTitle(title string) string {
	t := strings.ToLower(title)
	switch {
	case strings.Contains(t, " vs "), strings.Contains(t, " vs. "), strings.Contains(t, "world cup"),
		strings.Contains(t, "ufc"), strings.Contains(t, "nba"), strings.Contains(t, "nfl"),
		strings.Contains(t, "o/u"), strings.Contains(t, "spread"):
		return "体育"
	case strings.Contains(t, "lol:"), strings.Contains(t, "counter-strike"), strings.Contains(t, "dota"),
		strings.Contains(t, "valorant"), strings.Contains(t, "esports"):
		return "电竞"
	case strings.Contains(t, "bitcoin"), strings.Contains(t, "ethereum"), strings.Contains(t, "crypto"):
		return "加密货币"
	case strings.Contains(t, "president"), strings.Contains(t, "election"), strings.Contains(t, "prime minister"):
		return "政治"
	default:
		return "其他"
	}
}

func CountActiveMarkets(e GammaEvent) int {
	n := 0
	for _, m := range e.Markets {
		if m.Active && !m.Closed {
			n++
		}
	}
	return n
}

func OneLineSummary(text string, max int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "…"
}

func EventListItem(e GammaEvent, now time.Time) map[string]any {
	tags := make([]string, 0, len(e.Tags))
	for _, t := range e.Tags {
		if t.Label != "" {
			tags = append(tags, t.Label)
		}
	}
	return map[string]any{
		"id":           e.ID,
		"event_id":     e.ID,
		"title":        e.Title,
		"summary":      OneLineSummary(e.Description, 120),
		"description":  e.Description,
		"category":     EventCategory(e),
		"volume":       e.Volume,
		"liquidity":    e.Liquidity,
		"is_fast":      IsFastEvent(e, now),
		"end_date":     e.EndDate,
		"slug":         e.Slug,
		"market_count": CountActiveMarkets(e),
		"tags":         tags,
		"fetched_at":   now,
	}
}

func MarketListItem(m GammaMarket) map[string]any {
	outcomes := parseStringArray(m.Outcomes)
	prices := parseFloatArray(m.OutcomePrices)
	items := make([]map[string]any, 0, len(outcomes))
	for i, name := range outcomes {
		price := 0.0
		if i < len(prices) {
			price = prices[i]
		}
		items = append(items, map[string]any{
			"outcome": name,
			"price":   price,
		})
	}
	return map[string]any{
		"id":                 m.ID,
		"question":           m.Question,
		"condition_id":       m.ConditionID,
		"slug":               m.Slug,
		"volume":             m.Volume,
		"liquidity":          m.Liquidity,
		"end_date":           m.EndDate,
		"active":             m.Active,
		"closed":             m.Closed,
		"outcomes":           items,
		"last_trade_price":   m.LastTradePrice,
		"one_day_price_change": m.OneDayPriceChange,
	}
}

func EventDetailItem(e GammaEvent, now time.Time) map[string]any {
	item := EventListItem(e, now)
	markets := make([]map[string]any, 0, len(e.Markets))
	for _, m := range e.Markets {
		if !m.Active || m.Closed {
			continue
		}
		markets = append(markets, MarketListItem(m))
	}
	item["markets"] = markets
	item["polymarket_url"] = polymarketEventURL(e.Slug)
	return item
}

func polymarketEventURL(slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "https://polymarket.com"
	}
	return "https://polymarket.com/event/" + slug
}

func BuildCategoryCounts(events []GammaEvent) []map[string]any {
	counts := map[string]int{"全部": len(events)}
	for _, e := range events {
		counts[EventCategory(e)]++
	}
	out := []map[string]any{{"key": "", "label": "全部", "count": counts["全部"]}}
	for _, name := range eventCategoryOrder {
		out = append(out, map[string]any{
			"key":   name,
			"label": name,
			"count": counts[name],
		})
	}
	return out
}

func FilterEventsByCategory(events []GammaEvent, category string) []GammaEvent {
	category = strings.TrimSpace(category)
	if category == "" {
		return events
	}
	out := make([]GammaEvent, 0, len(events))
	for _, e := range events {
		if EventCategory(e) == category {
			out = append(out, e)
		}
	}
	return out
}

func PaginateEvents(events []GammaEvent, page, limit int) ([]GammaEvent, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	total := len(events)
	start := (page - 1) * limit
	if start >= total {
		return nil, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return events[start:end], total
}
