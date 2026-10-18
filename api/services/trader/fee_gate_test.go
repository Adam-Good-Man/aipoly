package trader

import (
	"testing"

	"aipoly/api/services/openai"
	"aipoly/api/services/polymarket"
)

func TestThinUpsideAfterFees(t *testing.T) {
	// At 96¢ max ROI ≈ 4.2% < 5% (2.5%×2) → reject
	if got := ThinUpsideAfterFees(0.96); got == "" {
		t.Fatal("expected reject 96¢ thin favorite")
	}
	// At 86¢ max ROI ≈ 16.3% > 5% → ok on fee ratio alone
	if got := ThinUpsideAfterFees(0.86); got != "" {
		t.Fatalf("unexpected reject at 86¢: %s", got)
	}
	// At 70¢ plenty of upside
	if got := ThinUpsideAfterFees(0.70); got != "" {
		t.Fatalf("unexpected reject at 70¢: %s", got)
	}
}

func TestInsufficientFeeCoverage(t *testing.T) {
	// $5 at 96¢ → profit ≈ $0.21, fee×2 ≈ $0.25 → reject
	if got := InsufficientFeeCoverage(5, 0.96); got == "" {
		t.Fatal("expected fee coverage reject")
	}
	// $5 at 70¢ → profit ≈ $2.14, fee×2 ≈ $0.25 → ok
	if got := InsufficientFeeCoverage(5, 0.70); got != "" {
		t.Fatalf("unexpected: %s", got)
	}
}

func TestValidateAndSizeDecisionSkipsThinFavorite(t *testing.T) {
	snap := polymarketSnap("m-thin", "Favorite lock?", []string{"Yes", "No"}, []float64{0.90, 0.10})
	ctx := TradingContext{
		Balance:        100,
		Deployable:     70,
		MinWinRate:     68,
		MinEdgePercent: 5,
		Guards: ExposureGuards{
			MinEntryPrice: 0.52,
			MaxEntryPrice: 0.86,
		},
	}
	d := openaiDecision("m-thin", "Yes", 5, 96)
	amount, reason := ValidateAndSizeDecision(d, snap, ctx, 70, true, 0)
	if amount != 0 || reason == "" {
		t.Fatalf("want skip thin favorite, got amount=%.2f reason=%q", amount, reason)
	}
}

func TestValidateAndSizeDecisionSkipsLowEdge(t *testing.T) {
	// 80¢ implied → need win ≥ 85 with MinEdge=5; AI says 83 → skip
	snap := polymarketSnap("m-edge", "Close favorite", []string{"Yes", "No"}, []float64{0.80, 0.20})
	ctx := TradingContext{
		Balance:        100,
		Deployable:     70,
		MinWinRate:     68,
		MinEdgePercent: 5, // tests keep stricter edge; runtime default is 2
		Guards: ExposureGuards{
			MinEntryPrice: 0.52,
			MaxEntryPrice: 0.86,
		},
	}
	d := openaiDecision("m-edge", "Yes", 5, 83)
	amount, reason := ValidateAndSizeDecision(d, snap, ctx, 70, true, 0)
	if amount != 0 || reason == "" {
		t.Fatalf("want skip low edge, got amount=%.2f reason=%q", amount, reason)
	}
}

func TestValidateAndSizeDecisionAllowsFeeAwareFavorite(t *testing.T) {
	// 70¢ + 76% win → edge 6 ≥ 5; upside healthy; clears MinWinRate 68
	snap := polymarketSnap("m-ok", "Solid edge", []string{"Yes", "No"}, []float64{0.70, 0.30})
	ctx := TradingContext{
		Balance:        100,
		Deployable:     70,
		MinWinRate:     68,
		MinEdgePercent: 5,
		Guards: ExposureGuards{
			MinEntryPrice: 0.52,
			MaxEntryPrice: 0.86,
		},
	}
	d := openaiDecision("m-ok", "Yes", 5, 76)
	amount, reason := ValidateAndSizeDecision(d, snap, ctx, 70, true, 0)
	if reason != "" || amount <= 0 {
		t.Fatalf("want allow, got amount=%.2f reason=%q", amount, reason)
	}
}

func polymarketSnap(id, q string, outcomes []string, prices []float64) polymarket.MarketSnapshot {
	return polymarket.MarketSnapshot{
		MarketID: id,
		Question: q,
		Outcomes: outcomes,
		Prices:   prices,
		EndDate:  "2099-01-01T00:00:00Z",
	}
}

func openaiDecision(marketID, outcome string, amount, win float64) openai.TradeDecision {
	return openai.TradeDecision{
		ShouldTrade:     true,
		MarketID:        marketID,
		Outcome:         outcome,
		Amount:          amount,
		SelfWinRate:     win,
		MarketReason:    "本场今日内可结算、成交与流动性足够支持最低名义下单，且题面信息完整便于用新闻交叉验证，因此纳入候选。",
		OutcomeReason:   "根据赛程与伤病信息判断该方向仍有明确优势可验证，当前赔率尚未充分反映该信息，因此选择该方向下注。",
		SourcesCited:    "无RSSHub新闻，仅依据盘口与题面；流动性足够完成最低名义下单",
		AnalysisProcess: "①阅读题面与双方赔率②对照最低胜率与费后利润门槛③确认方向清晰后按仓位区间下注。",
	}
}
