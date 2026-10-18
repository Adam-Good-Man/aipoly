package trader

import (
	"sort"
	"strings"
	"time"

	"aipoly/api/services/openai"
	"aipoly/api/services/polymarket"
)

// TradingContext captures deployable capital and exposure for sizing decisions.
type TradingContext struct {
	Balance           float64
	Deployable        float64
	ReserveAmount     float64
	OpenExposure      float64
	OpenPositions     int
	MaxBetPercent     float64
	MinCashReservePct float64
	MinWinRate        float64
	MinEdgePercent    float64
	Guards            ExposureGuards
}

// BuildTradingContext computes deployable capital from free CLOB cash.
// By default (reservePct=0) all free CLOB cash is deployable — open positions do not
// reduce it (that USDC already left the CLOB when those bets were placed).
// If reservePct > 0, keep that % of total equity undeployed (optional cash buffer).
func BuildTradingContext(clobCash, openExposure float64, openPositions int, maxBetPct, reservePct, minWinRate, minEdge float64) TradingContext {
	totalEquity := clobCash + openExposure
	var reserve float64
	deployable := clobCash
	if reservePct > 0 {
		reserve = totalEquity * reservePct / 100
		deployable = clobCash - reserve
		if deployable < 0 {
			deployable = 0
		}
	}

	return TradingContext{
		Balance:           totalEquity,
		Deployable:        deployable,
		ReserveAmount:     reserve,
		OpenExposure:      openExposure,
		OpenPositions:     openPositions,
		MaxBetPercent:     maxBetPct,
		MinCashReservePct: reservePct,
		MinWinRate:        minWinRate,
		MinEdgePercent:    minEdge,
	}
}

func daysUntilEnd(snap polymarket.MarketSnapshot) int {
	days, known := settlementDaysRemaining(snap.EndDate, time.Now())
	if !known {
		return -1
	}
	return days
}

// marketWithinMaxDays requires a known end date within maxDays (strict 24h windows).
func marketWithinMaxDays(snap polymarket.MarketSnapshot, maxDays int, now time.Time) bool {
	if maxDays <= 0 {
		return true
	}
	end := parseEndDate(snap.EndDate)
	if end == nil {
		return false
	}
	if !end.After(now) {
		return false
	}
	cutoff := now.Add(time.Duration(maxDays) * 24 * time.Hour)
	return !end.After(cutoff)
}

func settlementDaysRemaining(endDate string, now time.Time) (days int, known bool) {
	end := parseEndDate(endDate)
	if end == nil {
		return -1, false
	}
	if !end.After(now) {
		return 0, true
	}
	return int(end.Sub(now).Hours() / 24), true
}

// ValidateAndSizeDecision applies risk rules and returns final bet size (0 = skip).
// AI amount is clamped into [1×itemMin, 3×itemMin] (default 1×; deployable / cash-fraction / addCap / fee buffer also apply).
func ValidateAndSizeDecision(
	decision openai.TradeDecision,
	snap polymarket.MarketSnapshot,
	ctx TradingContext,
	clobBalance float64,
	liveTrading bool,
	maxMarketDays int,
) (amount float64, skipReason string) {
	if !decision.ShouldTrade {
		return 0, "not trading"
	}
	if skip := openai.ValidateReasonQuality(decision); skip != "" {
		return 0, skip
	}

	price, _ := findOutcomePrice(snap, decision.Outcome)
	if price <= 0 {
		return 0, "invalid outcome price"
	}

	skipReason, addCap := ctx.Guards.EntryGate(
		decision.MarketID,
		decision.Outcome,
		price,
		EventExposureKey(snap.EventID, snap.EventTitle),
	)
	if skipReason != "" {
		return 0, skipReason
	}

	// Spot price crypto stays excluded; short UP/Down may trade after deep pass.
	if IsVolatileCryptoSpot(snap) {
		return 0, "crypto spot price market excluded"
	}
	if maxMarketDays > 0 {
		if !marketWithinMaxDays(snap, maxMarketDays, time.Now()) {
			if parseEndDate(snap.EndDate) == nil {
				return 0, "unknown resolution date"
			}
			return 0, "market resolves too far in the future"
		}
	}
	if decision.SelfWinRate < ctx.MinWinRate {
		return 0, "win rate below minimum"
	}

	if ctx.Deployable <= 0 {
		return 0, "deployable balance too low"
	}

	// New entries: skip thin favorites where upside is fee-eaten.
	if addCap <= 0 {
		maxEntry := ctx.Guards.MaxEntryPrice
		if maxEntry <= 0 {
			maxEntry = defaultMaxEntryPrice
		}
		if price > maxEntry {
			return 0, "entry price too high (thin upside / fee drag)"
		}
		minEntry := ctx.Guards.MinEntryPrice
		if minEntry > 0 && price < minEntry {
			return 0, "entry price too low"
		}
		if reason := ThinUpsideAfterFees(price); reason != "" {
			return 0, reason
		}
		// Require AI win rate to beat market implied odds by RequiredEdgePercent.
		// Mid favorites (68–82¢) need a wider gap — historical AI calibration was soft here.
		needEdge := RequiredEdgePercent(price, ctx.MinEdgePercent)
		if needEdge > 0 {
			implied := price * 100
			if decision.SelfWinRate+1e-9 < implied+needEdge {
				return 0, "edge below minimum (win rate barely above price)"
			}
		} else if decision.SelfWinRate+1e-9 < price*100 {
			return 0, "edge below minimum (win rate below price)"
		}
	}

	itemMin := ItemMinBetAmount(price, snap.OrderMinSize, liveTrading)
	amount = ClampAIBetAmount(decision.Amount, itemMin, ctx.Deployable, addCap, clobBalance, liveTrading)
	if amount <= 0 {
		if ctx.Deployable < itemMin {
			return 0, "deployable below item minimum"
		}
		return 0, "below item minimum bet size"
	}
	if amount > ctx.Balance {
		amount = ctx.Balance
	}

	// Final absolute check: if this ticket wins, profit must cover fees with headroom.
	if addCap <= 0 && liveTrading {
		if reason := InsufficientFeeCoverage(amount, price); reason != "" {
			return 0, reason
		}
	}
	return amount, ""
}

func liveFeeBuffer(clobBalance float64) float64 {
	// ~2% taker fee + rounding; at least $0.03 on small tickets.
	buf := clobBalance * 0.025
	if buf < 0.03 {
		buf = 0.03
	}
	if buf > 0.25 {
		buf = 0.25
	}
	return buf
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func parseEndDate(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", raw); err == nil {
		return &t
	}
	return nil
}

// FilterShortTermSnapshots keeps markets with a known end date within maxDays.
// High-volume / "fast" tags alone do not bypass the settlement window.
func FilterShortTermSnapshots(snapshots []polymarket.MarketSnapshot, maxDays int) []polymarket.MarketSnapshot {
	if maxDays <= 0 {
		return snapshots
	}
	now := time.Now()
	out := make([]polymarket.MarketSnapshot, 0, len(snapshots))
	for _, s := range snapshots {
		if !marketWithinMaxDays(s, maxDays, now) {
			continue
		}
		s.DaysToEnd = daysUntilEnd(s)
		out = append(out, s)
	}
	sortShortTermSnapshots(out)
	return out
}

// FilterProfitFriendlySnapshots drops historically poor EV crypto spot price markets.
// Short crypto UP/Down remains eligible but is demoted in scoring and batch-capped.
func FilterProfitFriendlySnapshots(snapshots []polymarket.MarketSnapshot) []polymarket.MarketSnapshot {
	out := make([]polymarket.MarketSnapshot, 0, len(snapshots))
	for _, s := range snapshots {
		if IsVolatileCryptoSpot(s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// PrioritizeTradableSnapshots puts sports/esports and liquid two-sided markets first for the AI prompt.
func PrioritizeTradableSnapshots(snapshots []polymarket.MarketSnapshot) []polymarket.MarketSnapshot {
	out := append([]polymarket.MarketSnapshot(nil), snapshots...)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := snapshotTradabilityScore(out[i]), snapshotTradabilityScore(out[j])
		if si != sj {
			return si > sj
		}
		if out[i].Volume != out[j].Volume {
			return out[i].Volume > out[j].Volume
		}
		di, dj := out[i].DaysToEnd, out[j].DaysToEnd
		if di >= 0 && dj >= 0 && di != dj {
			return di < dj
		}
		return false
	})
	return out
}

func snapshotTradabilityScore(s polymarket.MarketSnapshot) int {
	score := 0
	q := marketTextLower(s)
	if isSportsLikeMarket(q) {
		score += 40
	}
	if isEsportsLikeMarket(q) {
		score += 35
	}
	if s.Volume >= 50_000 {
		score += 15
	} else if s.Volume >= 10_000 {
		score += 8
	}
	if s.Liquidity >= 20_000 {
		score += 10
	}
	if s.DaysToEnd >= 0 && s.DaysToEnd <= 2 {
		score += 12
	}
	if polymarket.HasBuyableEntryBand(s) {
		score += 25 // mid-price inventory the engine can actually buy
	} else {
		score -= 40 // thin ≥86¢ favorites fill shortlists and yield overnight zero fills
	}
	if isLockedMarket(s) {
		score -= 50
	}
	if IsCryptoDirectionMarket(s) {
		score -= 45 // keep UP/Down available but low priority
	} else if isCryptoMarketText(q) {
		score -= 30
	}
	return score
}

func isSportsLikeMarket(q string) bool {
	return strings.Contains(q, " vs ") ||
		strings.Contains(q, " vs. ") ||
		strings.Contains(q, "o/u") ||
		strings.Contains(q, "spread") ||
		strings.Contains(q, "ufc") ||
		strings.Contains(q, "fifa") ||
		strings.Contains(q, "world cup") ||
		strings.Contains(q, "nba") ||
		strings.Contains(q, "nfl")
}

func isEsportsLikeMarket(q string) bool {
	return strings.Contains(q, "lol:") ||
		strings.Contains(q, "counter-strike") ||
		strings.Contains(q, "dota") ||
		strings.Contains(q, "valorant") ||
		strings.Contains(q, "esports")
}

func isLockedMarket(s polymarket.MarketSnapshot) bool {
	if len(s.Prices) == 0 {
		return false
	}
	maxP, minP := 0.0, 1.0
	for _, p := range s.Prices {
		if p > maxP {
			maxP = p
		}
		if p > 0 && p < minP {
			minP = p
		}
	}
	return maxP >= 0.97 || minP <= 0.03
}

func sortShortTermSnapshots(snapshots []polymarket.MarketSnapshot) {
	sort.Slice(snapshots, func(i, j int) bool {
		si, sj := snapshotTradabilityScore(snapshots[i]), snapshotTradabilityScore(snapshots[j])
		if si != sj {
			return si > sj
		}
		di, dj := snapshots[i].DaysToEnd, snapshots[j].DaysToEnd
		if di >= 0 && dj >= 0 && di != dj {
			return di < dj
		}
		return snapshots[i].Volume > snapshots[j].Volume
	})
}
