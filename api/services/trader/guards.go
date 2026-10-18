package trader

import (
	"fmt"
	"strings"
	"time"

	"aipoly/api/models"
	"aipoly/api/services/polymarket"

	"gorm.io/gorm"
)

const (
	defaultLossCooldownHours = 24
	defaultMaxDailyLossPct   = 12 // pause new bets if today's realized losses exceed this % of equity
	defaultMinEntryPrice = 0.52
	// Cap favorites: above ~86¢ max upside is thin and taker fees eat most of the win.
	defaultMaxEntryPrice = 0.86
	defaultMaxAddFraction    = 0.30
	defaultMaxAddsPerMarket  = 1
)

// OpenPos aggregates open trades on one market.
type OpenPos struct {
	MarketID   string
	Outcome    string
	EntryPrice float64 // weighted average entry
	Amount     float64 // total cost basis
	Shares     float64
	TradeCount int
	MarkPrice  float64 // live mid; 0 if unknown
}

// ExposureGuards blocks rebet / tilt and circuit-breaks after bad days.
type ExposureGuards struct {
	OpenMarketIDs     map[string]bool
	OpenByMarket      map[string]OpenPos
	OpenEventKeys     map[string]string // event key → market_id that owns the open exposure
	CooldownMarketIDs map[string]bool
	DailyRealizedPnL  float64
	Equity            float64
	MaxDailyLossPct   float64
	MinEntryPrice     float64
	MaxEntryPrice     float64
	MaxAddFraction    float64
	MaxAddsPerMarket  int
	TradingPaused     bool
	PauseReason       string
}

// EventExposureKey groups sibling markets (BO3 / Game1 / handicap) under one Polymarket event.
// Prefers EventID; falls back to normalized EventTitle for legacy rows without EventID.
func EventExposureKey(eventID, eventTitle string) string {
	if id := strings.TrimSpace(eventID); id != "" {
		return "id:" + id
	}
	t := strings.ToLower(strings.TrimSpace(eventTitle))
	if t == "" {
		return ""
	}
	return "title:" + t
}

// LoadExposureGuards reads open positions, recent losses, and today's PnL from DB.
func LoadExposureGuards(db *gorm.DB, equity float64, cooldownHours int, maxDailyLossPct float64) ExposureGuards {
	if cooldownHours <= 0 {
		cooldownHours = defaultLossCooldownHours
	}
	if maxDailyLossPct <= 0 {
		maxDailyLossPct = defaultMaxDailyLossPct
	}

	g := ExposureGuards{
		OpenMarketIDs:     make(map[string]bool),
		OpenByMarket:      make(map[string]OpenPos),
		OpenEventKeys:     make(map[string]string),
		CooldownMarketIDs: make(map[string]bool),
		Equity:            equity,
		MaxDailyLossPct:   maxDailyLossPct,
		MinEntryPrice:     defaultMinEntryPrice,
		MaxEntryPrice:     defaultMaxEntryPrice,
		MaxAddFraction:    defaultMaxAddFraction,
		MaxAddsPerMarket:  defaultMaxAddsPerMarket,
	}

	var openTrades []models.Trade
	db.Where("status = ?", models.TradeStatusOpen).Find(&openTrades)
	for _, t := range openTrades {
		if t.MarketID == "" {
			continue
		}
		g.OpenMarketIDs[t.MarketID] = true
		pos := g.OpenByMarket[t.MarketID]
		pos.MarketID = t.MarketID
		if pos.Outcome == "" {
			pos.Outcome = t.Outcome
		}
		prevAmt := pos.Amount
		pos.Amount += t.Amount
		pos.Shares += t.Shares
		pos.TradeCount++
		if prevAmt <= 0 {
			pos.EntryPrice = t.Price
		} else if pos.Amount > 0 {
			pos.EntryPrice = (prevAmt*pos.EntryPrice + t.Amount*t.Price) / pos.Amount
		}
		g.OpenByMarket[t.MarketID] = pos
		if key := EventExposureKey(t.EventID, t.EventTitle); key != "" {
			if _, ok := g.OpenEventKeys[key]; !ok {
				g.OpenEventKeys[key] = t.MarketID
			}
		}
	}

	since := time.Now().Add(-time.Duration(cooldownHours) * time.Hour)
	var lostIDs []string
	db.Model(&models.Trade{}).
		Where("status = ? AND created_at >= ?", models.TradeStatusLost, since).
		Distinct("market_id").
		Pluck("market_id", &lostIDs)
	for _, id := range lostIDs {
		if id != "" {
			g.CooldownMarketIDs[id] = true
		}
	}

	dayStart := time.Now().Truncate(24 * time.Hour)
	db.Model(&models.Trade{}).
		Where("status IN ? AND COALESCE(resolved_at, created_at) >= ?",
			[]models.TradeStatus{models.TradeStatusWon, models.TradeStatusLost}, dayStart).
		Select("COALESCE(SUM(pnl), 0)").
		Scan(&g.DailyRealizedPnL)

	return g
}

// SetOpenMark sets the live mark for a market (from midpoint or snapshot).
func (g *ExposureGuards) SetOpenMark(marketID string, mark float64) {
	if g == nil || marketID == "" || mark <= 0 {
		return
	}
	pos, ok := g.OpenByMarket[marketID]
	if !ok {
		return
	}
	pos.MarkPrice = mark
	g.OpenByMarket[marketID] = pos
}

// BlockReason keeps pause/cooldown checks (legacy callers without outcome/price).
func (g ExposureGuards) BlockReason(marketID string) string {
	if g.TradingPaused {
		if g.PauseReason != "" {
			return g.PauseReason
		}
		return "trading paused"
	}
	if marketID == "" {
		return ""
	}
	if g.CooldownMarketIDs[marketID] {
		return "market in loss cooldown"
	}
	if pos, ok := g.OpenByMarket[marketID]; ok {
		if !pos.CanAdd(g.MaxAddsPerMarket) {
			return "already have open position on this market"
		}
		return "already have open position on this market"
	}
	if g.OpenMarketIDs[marketID] {
		return "already have open position on this market"
	}
	return ""
}

// EntryGate decides whether a new buy or capped add is allowed.
// addCap > 0 means this is an add-to-winner; size must not exceed addCap.
// eventKey groups sibling markets under one event; a different market on an open event is blocked.
func (g ExposureGuards) EntryGate(marketID, outcome string, currentPrice float64, eventKey string) (skipReason string, addCap float64) {
	if g.TradingPaused {
		if g.PauseReason != "" {
			return g.PauseReason, 0
		}
		return "trading paused", 0
	}
	if marketID == "" {
		return "", 0
	}
	if g.CooldownMarketIDs[marketID] {
		return "market in loss cooldown", 0
	}
	pos, open := g.OpenByMarket[marketID]
	if !open {
		if g.OpenMarketIDs[marketID] {
			return "already have open position on this market", 0
		}
		if eventKey != "" {
			if owner, ok := g.OpenEventKeys[eventKey]; ok && owner != "" && owner != marketID {
				return "already have open position on this event", 0
			}
		}
		return "", 0
	}

	if !sameOutcome(pos.Outcome, outcome) {
		return "already have open position on different outcome", 0
	}
	if !pos.CanAdd(g.MaxAddsPerMarket) {
		return "max adds reached for this market", 0
	}

	mark := currentPrice
	if pos.MarkPrice > 0 {
		mark = pos.MarkPrice
	}
	if mark <= 0 {
		return "open position mark unknown; no add", 0
	}
	if mark <= pos.EntryPrice {
		return "open position not profitable; no add", 0
	}

	frac := g.MaxAddFraction
	if frac <= 0 {
		frac = defaultMaxAddFraction
	}
	addCap = pos.Amount * frac
	if addCap < 1.0 {
		return "add size too small", 0
	}
	return "", addCap
}

func (p OpenPos) CanAdd(maxAdds int) bool {
	if maxAdds <= 0 {
		maxAdds = defaultMaxAddsPerMarket
	}
	// TradeCount includes the original; allow up to maxAdds extras.
	return p.TradeCount <= maxAdds
}

func (p OpenPos) IsProfitable() bool {
	return p.MarkPrice > 0 && p.EntryPrice > 0 && p.MarkPrice > p.EntryPrice
}

func sameOutcome(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func (g *ExposureGuards) MarkOpened(marketID string) {
	if g == nil || marketID == "" {
		return
	}
	if g.OpenMarketIDs == nil {
		g.OpenMarketIDs = make(map[string]bool)
	}
	g.OpenMarketIDs[marketID] = true
	pos := g.OpenByMarket[marketID]
	pos.MarketID = marketID
	pos.TradeCount++
	if g.OpenByMarket == nil {
		g.OpenByMarket = make(map[string]OpenPos)
	}
	g.OpenByMarket[marketID] = pos
}

// MarkOpenedTrade updates open aggregation after a successful place.
func (g *ExposureGuards) MarkOpenedTrade(marketID, outcome string, amount, price float64, eventKey string) {
	if g == nil || marketID == "" {
		return
	}
	if g.OpenMarketIDs == nil {
		g.OpenMarketIDs = make(map[string]bool)
	}
	if g.OpenByMarket == nil {
		g.OpenByMarket = make(map[string]OpenPos)
	}
	if g.OpenEventKeys == nil {
		g.OpenEventKeys = make(map[string]string)
	}
	g.OpenMarketIDs[marketID] = true
	pos := g.OpenByMarket[marketID]
	pos.MarketID = marketID
	if pos.Outcome == "" {
		pos.Outcome = outcome
	}
	prevAmt := pos.Amount
	pos.Amount += amount
	if price > 0 {
		pos.Shares += amount / price
		if pos.Amount > 0 {
			if prevAmt == 0 {
				pos.EntryPrice = price
			} else {
				pos.EntryPrice = (prevAmt*pos.EntryPrice + amount*price) / pos.Amount
			}
		}
	}
	pos.TradeCount++
	g.OpenByMarket[marketID] = pos
	if eventKey != "" {
		if _, ok := g.OpenEventKeys[eventKey]; !ok {
			g.OpenEventKeys[eventKey] = marketID
		}
	}
}

// HardBlockedMarketIDs are markets AI must not touch (cooldown or non-addable opens).
func HardBlockedMarketIDs(g ExposureGuards) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(g.CooldownMarketIDs)+len(g.OpenByMarket))
	for id := range g.CooldownMarketIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	for id, pos := range g.OpenByMarket {
		if id == "" || seen[id] {
			continue
		}
		if pos.IsProfitable() && pos.CanAdd(g.MaxAddsPerMarket) {
			continue // addable — listed separately
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// SiblingMarketIDsOnOpenEvents returns pool markets that share an event with an existing open
// position but are not that open market (e.g. Game1 when BO3 is already open).
func SiblingMarketIDsOnOpenEvents(g ExposureGuards, pool []polymarket.MarketSnapshot) []string {
	if len(g.OpenEventKeys) == 0 || len(pool) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, s := range pool {
		if s.MarketID == "" || seen[s.MarketID] {
			continue
		}
		key := EventExposureKey(s.EventID, s.EventTitle)
		if key == "" {
			continue
		}
		owner, ok := g.OpenEventKeys[key]
		if !ok || owner == "" || owner == s.MarketID {
			continue
		}
		seen[s.MarketID] = true
		out = append(out, s.MarketID)
	}
	return out
}

// AddablePositions lists profitable opens that may receive a small add.
func AddablePositions(g ExposureGuards) []OpenPos {
	out := make([]OpenPos, 0)
	frac := g.MaxAddFraction
	if frac <= 0 {
		frac = defaultMaxAddFraction
	}
	for _, pos := range g.OpenByMarket {
		if !pos.IsProfitable() || !pos.CanAdd(g.MaxAddsPerMarket) {
			continue
		}
		out = append(out, pos)
	}
	_ = frac
	return out
}

func formatAddableHint(pos OpenPos, maxAddFrac float64) string {
	if maxAddFrac <= 0 {
		maxAddFrac = defaultMaxAddFraction
	}
	capAmt := pos.Amount * maxAddFrac
	return fmt.Sprintf("%s outcome=%s entry=%.0f¢ mark=%.0f¢ max_add=$%.2f",
		pos.MarketID, pos.Outcome, pos.EntryPrice*100, pos.MarkPrice*100, capAmt)
}

// IsVolatileCryptoSpot skips near-settlement crypto spot price bets (historically poor EV).
func IsVolatileCryptoSpot(snap polymarket.MarketSnapshot) bool {
	q := marketTextLower(snap)
	crypto := isCryptoMarketText(q)
	if !crypto {
		return false
	}
	priceish := strings.Contains(q, "price") ||
		strings.Contains(q, "above") ||
		strings.Contains(q, "below") ||
		strings.Contains(q, "reach") ||
		strings.Contains(q, "$")
	return priceish
}

// IsCryptoDirectionMarket skips short-window crypto Up/Down roulette (dominates fast feeds, negative EV).
func IsCryptoDirectionMarket(snap polymarket.MarketSnapshot) bool {
	q := marketTextLower(snap)
	if !isCryptoMarketText(q) {
		return false
	}
	direction := strings.Contains(q, "up or down") ||
		strings.Contains(q, "up/down") ||
		strings.Contains(q, " higher ") ||
		strings.Contains(q, "chainlink")
	window := strings.Contains(q, "5 min") ||
		strings.Contains(q, "5-min") ||
		strings.Contains(q, "15 min") ||
		strings.Contains(q, "15-min") ||
		strings.Contains(q, "1 hour") ||
		strings.Contains(q, "hourly")
	return direction || (window && (strings.Contains(q, "up") || strings.Contains(q, "down")))
}

func marketTextLower(snap polymarket.MarketSnapshot) string {
	return strings.ToLower(snap.Question + " " + snap.EventTitle + " " + snap.Category)
}

func isCryptoMarketText(q string) bool {
	return strings.Contains(q, "bitcoin") ||
		strings.Contains(q, "btc") ||
		strings.Contains(q, "ethereum") ||
		strings.Contains(q, "eth ") ||
		strings.Contains(q, "solana") ||
		strings.Contains(q, "xrp") ||
		strings.Contains(q, "doge") ||
		strings.Contains(q, "crypto")
}
