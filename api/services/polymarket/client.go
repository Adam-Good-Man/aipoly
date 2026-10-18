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

const (
	gammaBase = "https://gamma-api.polymarket.com"
	clobBase  = "https://clob.polymarket.com"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
	}
}

type GammaEvent struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Slug        string        `json:"slug"`
	Volume      float64       `json:"volume"`
	Liquidity   float64       `json:"liquidity"`
	Category    string        `json:"category"`
	EndDate     string        `json:"endDate"`
	Markets     []GammaMarket `json:"markets"`
	Tags        []GammaTag    `json:"tags"`
	Active      bool          `json:"active"`
	Closed      bool          `json:"closed"`
}

type GammaTag struct {
	Label string `json:"label"`
	Slug  string `json:"slug"`
}

type GammaMarket struct {
	ID              string  `json:"id"`
	Question        string  `json:"question"`
	ConditionID     string  `json:"conditionId"`
	Slug            string  `json:"slug"`
	OutcomePrices   string  `json:"outcomePrices"`
	Outcomes        string  `json:"outcomes"`
	ClobTokenIds    string  `json:"clobTokenIds"`
	Volume          float64 `json:"volumeNum"`
	Liquidity       float64 `json:"liquidityNum"`
	EndDate         string  `json:"endDate"`
	Active          bool    `json:"active"`
	Closed          bool    `json:"closed"`
	BestBid         float64 `json:"bestBid"`
	BestAsk         float64 `json:"bestAsk"`
	LastTradePrice  float64 `json:"lastTradePrice"`
	OneDayPriceChange float64 `json:"oneDayPriceChange"`
	OrderMinSize      float64 `json:"orderMinSize"`
}

type MarketSnapshot struct {
	EventID     string
	EventTitle  string
	MarketID    string
	Question    string
	ConditionID string
	Outcomes    []string
	Prices      []float64
	TokenIDs    []string
	Volume      float64
	Liquidity   float64
	EndDate     string
	Category    string
	Description string
	IsFast      bool
	PriceChange float64
	DaysToEnd   int
	OrderMinSize float64
}

func (c *Client) FetchTrendingEvents(limit int) ([]GammaEvent, error) {
	return c.fetchEventsOrdered(limit, "volume24hr", false, 0)
}

// FetchShortTermEvents returns high-volume events settling within maxDays.
func (c *Client) FetchShortTermEvents(limit, maxDays int) ([]GammaEvent, error) {
	if maxDays <= 0 {
		maxDays = 2
	}
	return c.fetchEventsOrdered(limit, "volume24hr", false, maxDays)
}

func (c *Client) fetchEventsOrdered(limit int, order string, ascending bool, maxDays int) ([]GammaEvent, error) {
	params := url.Values{}
	params.Set("active", "true")
	params.Set("closed", "false")
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("order", order)
	params.Set("ascending", fmt.Sprintf("%t", ascending))
	if maxDays > 0 {
		endMax := time.Now().Add(time.Duration(maxDays) * 24 * time.Hour).Format(time.RFC3339)
		params.Set("end_date_max", endMax)
	}

	var events []GammaEvent
	if err := c.getJSON(gammaBase+"/events?"+params.Encode(), &events); err != nil {
		return nil, err
	}
	return events, nil
}

func (c *Client) FetchFastEvents(limit int) ([]GammaEvent, error) {
	params := url.Values{}
	params.Set("active", "true")
	params.Set("closed", "false")
	params.Set("limit", fmt.Sprintf("%d", limit*3))
	params.Set("order", "startDate")
	params.Set("ascending", "false")

	var events []GammaEvent
	if err := c.getJSON(gammaBase+"/events?"+params.Encode(), &events); err != nil {
		return nil, err
	}

	now := time.Now()
	var fast []GammaEvent
	for _, e := range events {
		if IsFastEvent(e, now) {
			fast = append(fast, e)
		}
		if len(fast) >= limit {
			break
		}
	}
	return fast, nil
}

func IsFastEvent(e GammaEvent, now time.Time) bool {
	for _, tag := range e.Tags {
		slug := strings.ToLower(tag.Slug)
		if strings.Contains(slug, "breaking") || strings.Contains(slug, "news") ||
			strings.Contains(slug, "politics") || strings.Contains(slug, "crypto") {
			return true
		}
	}
	if e.EndDate != "" {
		if t, err := time.Parse(time.RFC3339, e.EndDate); err == nil {
			if t.Sub(now) < 7*24*time.Hour {
				return true
			}
		}
	}
	for _, m := range e.Markets {
		if m.OneDayPriceChange != 0 && (m.OneDayPriceChange > 0.05 || m.OneDayPriceChange < -0.05) {
			return true
		}
	}
	return e.Volume > 100000
}

func (c *Client) BuildSnapshots(events []GammaEvent) []MarketSnapshot {
	var snapshots []MarketSnapshot
	now := time.Now()

	for _, e := range events {
		for _, m := range e.Markets {
			if !m.Active || m.Closed {
				continue
			}
			outcomes := parseStringArray(m.Outcomes)
			prices := parseFloatArray(m.OutcomePrices)
			tokens := parseStringArray(m.ClobTokenIds)
			if len(outcomes) == 0 || len(prices) == 0 {
				continue
			}
			endDate := m.EndDate
			if endDate == "" {
				endDate = e.EndDate
			}
			snapshots = append(snapshots, MarketSnapshot{
				EventID:     e.ID,
				EventTitle:  e.Title,
				MarketID:    m.ID,
				Question:    m.Question,
				ConditionID: m.ConditionID,
				Outcomes:    outcomes,
				Prices:      prices,
				TokenIDs:    tokens,
				Volume:      m.Volume,
				Liquidity:   m.Liquidity,
				EndDate:     endDate,
				Category:    e.Category,
				Description: truncateText(e.Description, 600),
				IsFast:      IsFastEvent(e, now),
				PriceChange: m.OneDayPriceChange,
				DaysToEnd:   daysUntilSnapshotEnd(endDate, now),
				OrderMinSize: m.OrderMinSize,
			})
		}
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].IsFast != snapshots[j].IsFast {
			return snapshots[i].IsFast
		}
		di, dj := snapshots[i].DaysToEnd, snapshots[j].DaysToEnd
		if di >= 0 && dj >= 0 && di != dj {
			return di < dj
		}
		return snapshots[i].Volume > snapshots[j].Volume
	})

	return snapshots
}

func daysUntilSnapshotEnd(endDate string, now time.Time) int {
	endDate = strings.TrimSpace(endDate)
	if endDate == "" {
		return -1
	}
	t, err := time.Parse(time.RFC3339, endDate)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", endDate)
	}
	if err != nil {
		return -1
	}
	d := t.Sub(now)
	if d < 0 {
		return 0
	}
	return int(d.Hours()/24) + 1
}

func truncateText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func parseStringArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return strings.Split(raw, ",")
	}
	return arr
}

func parseFloatArray(raw string) []float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var strs []string
	if err := json.Unmarshal([]byte(raw), &strs); err != nil {
		return nil
	}
	var floats []float64
	for _, s := range strs {
		var f float64
		fmt.Sscanf(s, "%f", &f)
		floats = append(floats, f)
	}
	return floats
}

func (c *Client) GetMidpoint(tokenID string) (float64, error) {
	var resp struct {
		Mid string `json:"mid"`
	}
	err := c.getJSON(clobBase+"/midpoint?token_id="+url.QueryEscape(tokenID), &resp)
	if err != nil {
		return 0, err
	}
	var mid float64
	fmt.Sscanf(resp.Mid, "%f", &mid)
	return mid, nil
}

func (c *Client) FetchMarketByID(marketID string) (*GammaMarket, error) {
	var one GammaMarket
	if err := c.getJSON(gammaBase+"/markets/"+url.QueryEscape(marketID), &one); err != nil {
		return nil, err
	}
	if one.ID == "" {
		return nil, fmt.Errorf("market %s not found", marketID)
	}
	return &one, nil
}

func (c *Client) FetchMarketByConditionID(conditionID string) (*GammaMarket, error) {
	params := url.Values{}
	params.Set("condition_id", conditionID)
	var markets []GammaMarket
	if err := c.getJSON(gammaBase+"/markets?"+params.Encode(), &markets); err != nil {
		return nil, err
	}
	if len(markets) == 0 {
		return nil, fmt.Errorf("no market for condition %s", conditionID)
	}
	return &markets[0], nil
}

func MarketSnapshotFromGamma(m GammaMarket, event GammaEvent) MarketSnapshot {
	outcomes := parseStringArray(m.Outcomes)
	prices := parseFloatArray(m.OutcomePrices)
	tokens := parseStringArray(m.ClobTokenIds)
	return MarketSnapshot{
		EventID:     event.ID,
		EventTitle:  event.Title,
		MarketID:    m.ID,
		Question:    m.Question,
		ConditionID: m.ConditionID,
		Outcomes:    outcomes,
		Prices:      prices,
		TokenIDs:    tokens,
		Volume:      m.Volume,
		Liquidity:   m.Liquidity,
		EndDate:     m.EndDate,
		Category:    event.Category,
		IsFast:      IsFastEvent(event, time.Now()),
		PriceChange: m.OneDayPriceChange,
		OrderMinSize: m.OrderMinSize,
	}
}

func (c *Client) getJSON(url string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "aiPoly/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, dest)
}
