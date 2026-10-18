package openai

import "testing"

func TestNormalizeDecisionWinRates(t *testing.T) {
	ds := []TradeDecision{
		{ShouldTrade: true, SelfWinRate: 0.72},
		{ShouldTrade: true, SelfWinRate: 0, AnalysisProcess: "①读盘②对照赔率③自评胜率约68%后下单"},
		{ShouldTrade: true, SelfWinRate: 80},
		{ShouldTrade: false, SelfWinRate: 0},
	}
	NormalizeDecisionWinRates(ds)
	if ds[0].SelfWinRate < 71 || ds[0].SelfWinRate > 73 {
		t.Fatalf("decimal: got %.1f", ds[0].SelfWinRate)
	}
	if ds[1].SelfWinRate != 68 {
		t.Fatalf("extract: got %.1f", ds[1].SelfWinRate)
	}
	if ds[2].SelfWinRate != 80 {
		t.Fatalf("keep: got %.1f", ds[2].SelfWinRate)
	}
	if ds[3].SelfWinRate != 0 {
		t.Fatalf("non-trade should stay 0")
	}
}

func TestEnsureAnalyzedWinRateNeverZeroWithText(t *testing.T) {
	d := TradeDecision{
		ShouldTrade:   true,
		SelfWinRate:   0,
		MarketReason:  "本场电竞盘结算快流动性足够，且初筛已从二十个候选中选出。",
		OutcomeReason: "根据盘口走势与成交方向选择热门侧，理由充分可下。",
		SourcesCited:  "无RSSHub新闻，仅依据盘口与题面及近期成交。",
		AnalysisProcess: "①初筛入选②读盘口③确认方向后准备下单",
	}
	if !EnsureAnalyzedWinRate(&d, 0.755, 62) {
		t.Fatal("expected fill")
	}
	if d.SelfWinRate < 62 {
		t.Fatalf("want >= min win, got %.1f", d.SelfWinRate)
	}
	// price 75.5¢ → implied 75.5
	if d.SelfWinRate < 75 || d.SelfWinRate > 76 {
		t.Fatalf("want ~75.5 from price, got %.1f", d.SelfWinRate)
	}
}
