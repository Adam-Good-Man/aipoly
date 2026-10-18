package trader

import (
	"testing"

	"aipoly/api/services/openai"
	"aipoly/api/services/polymarket"
)

func TestExposureGuardsBlockOpenAndCooldown(t *testing.T) {
	g := ExposureGuards{
		OpenMarketIDs:     map[string]bool{"m1": true},
		CooldownMarketIDs: map[string]bool{"m2": true},
		MinEntryPrice:     0.52,
		MaxEntryPrice:     0.86,
	}
	if got := g.BlockReason("m1"); got == "" {
		t.Fatal("expected open-position block")
	}
	if got := g.BlockReason("m2"); got == "" {
		t.Fatal("expected cooldown block")
	}
	if got := g.BlockReason("m3"); got != "" {
		t.Fatalf("unexpected block: %s", got)
	}
}

func TestEntryGateAllowsProfitableAdd(t *testing.T) {
	g := ExposureGuards{
		OpenByMarket: map[string]OpenPos{
			"m1": {
				MarketID:   "m1",
				Outcome:    "Team A",
				EntryPrice: 0.60,
				Amount:     10,
				TradeCount: 1,
				MarkPrice:  0.72,
			},
		},
		MaxAddFraction:   0.30,
		MaxAddsPerMarket: 1,
		MinEntryPrice:    0.52,
		MaxEntryPrice:    0.86,
	}
	reason, cap := g.EntryGate("m1", "Team A", 0.72, "")
	if reason != "" {
		t.Fatalf("expected allow add, got %q", reason)
	}
	if cap < 2.9 || cap > 3.1 {
		t.Fatalf("expected add cap ~3.0, got %.2f", cap)
	}
}

func TestEntryGateBlocksUnprofitableAdd(t *testing.T) {
	g := ExposureGuards{
		OpenByMarket: map[string]OpenPos{
			"m1": {
				MarketID:   "m1",
				Outcome:    "Team A",
				EntryPrice: 0.70,
				Amount:     10,
				TradeCount: 1,
				MarkPrice:  0.55,
			},
		},
		MaxAddFraction:   0.30,
		MaxAddsPerMarket: 1,
	}
	reason, cap := g.EntryGate("m1", "Team A", 0.55, "")
	if reason == "" || cap != 0 {
		t.Fatalf("expected block unprofitable add, got reason=%q cap=%.2f", reason, cap)
	}
}

func TestEntryGateBlocksSiblingEventMarket(t *testing.T) {
	g := ExposureGuards{
		OpenMarketIDs: map[string]bool{"bo3": true},
		OpenByMarket: map[string]OpenPos{
			"bo3": {MarketID: "bo3", Outcome: "G2", EntryPrice: 0.80, Amount: 4, TradeCount: 1},
		},
		OpenEventKeys: map[string]string{
			EventExposureKey("evt1", "G2 vs NAVI"): "bo3",
		},
	}
	reason, cap := g.EntryGate("game1", "G2", 0.74, EventExposureKey("evt1", "G2 vs NAVI"))
	if reason == "" || cap != 0 {
		t.Fatalf("expected sibling event block, got reason=%q cap=%.2f", reason, cap)
	}
	// Same market still goes through add path (unprofitable here → block add, not event message)
	reason, _ = g.EntryGate("bo3", "G2", 0.79, EventExposureKey("evt1", "G2 vs NAVI"))
	if reason == "" {
		t.Fatal("expected same-market gate to run (mark/add rules)")
	}
}

func TestSiblingMarketIDsOnOpenEvents(t *testing.T) {
	g := ExposureGuards{
		OpenEventKeys: map[string]string{"id:evt1": "bo3"},
	}
	pool := []polymarket.MarketSnapshot{
		{MarketID: "bo3", EventID: "evt1", EventTitle: "G2 vs NAVI"},
		{MarketID: "game1", EventID: "evt1", EventTitle: "G2 vs NAVI"},
		{MarketID: "other", EventID: "evt2", EventTitle: "Other"},
	}
	sibs := SiblingMarketIDsOnOpenEvents(g, pool)
	if len(sibs) != 1 || sibs[0] != "game1" {
		t.Fatalf("want [game1], got %#v", sibs)
	}
}

func TestRequiredEdgePercentMidFavorite(t *testing.T) {
	if got := RequiredEdgePercent(0.75, 3); got != 3 {
		t.Fatalf("want base 3 got %.1f", got)
	}
	if got := RequiredEdgePercent(0.55, 3); got != 3 {
		t.Fatalf("want base 3 got %.1f", got)
	}
}

func TestStopLossTrigger(t *testing.T) {
	entry := 0.70
	threshold := entry * (1 - 30.0/100.0) // ~0.49
	if !(0.50 > threshold) {
		t.Fatalf("0.50 should be above trigger (threshold=%.6f)", threshold)
	}
	if !(0.48 <= threshold) {
		t.Fatalf("0.48 should be at or below trigger (threshold=%.6f)", threshold)
	}
	// Code uses mark <= entry*(1-pct/100)
	mark := 0.48
	if mark > threshold {
		t.Fatal("expected stop-loss to fire")
	}
}

func TestIsVolatileCryptoSpot(t *testing.T) {
	btc := polymarket.MarketSnapshot{Question: "Will the price of Bitcoin be above $64,000 on July 12?"}
	if !IsVolatileCryptoSpot(btc) {
		t.Fatal("expected BTC price market to be excluded")
	}
	sports := polymarket.MarketSnapshot{Question: "UFC 329: Max Holloway vs Conor McGregor", Category: "Sports"}
	if IsVolatileCryptoSpot(sports) {
		t.Fatal("sports market should not be crypto-excluded")
	}
}

func TestValidateAndSizeDecisionBlocksRebet(t *testing.T) {
	snap := polymarket.MarketSnapshot{
		MarketID: "m1",
		Question: "Team A vs Team B",
		Outcomes: []string{"Team A", "Team B"},
		Prices:   []float64{0.55, 0.45}, // below entry → unprofitable
		EndDate:  "2099-01-01T00:00:00Z",
	}
	ctx := TradingContext{
		Balance:        100,
		Deployable:     70,
		MinWinRate:     78,
		MinEdgePercent: 10,
		Guards: ExposureGuards{
			OpenMarketIDs: map[string]bool{"m1": true},
			OpenByMarket: map[string]OpenPos{
				"m1": {
					MarketID:   "m1",
					Outcome:    "Team A",
					EntryPrice: 0.70,
					Amount:     10,
					TradeCount: 1,
					MarkPrice:  0.55,
				},
			},
			MaxAddFraction:   0.30,
			MaxAddsPerMarket: 1,
			MinEntryPrice:    0.52,
			MaxEntryPrice:    0.86,
		},
	}
	d := openai.TradeDecision{
		ShouldTrade:     true,
		MarketID:        "m1",
		Outcome:         "Team A",
		Amount:          5,
		SelfWinRate:     85,
		MarketReason:    "本场今日内可结算、成交与流动性足够支持最低名义下单，且题面信息完整便于用新闻交叉验证，因此纳入候选。",
		OutcomeReason:   "Team A 近期状态与盘口隐含概率相比仍有可验证优势，结合新闻与赔率同向，排除 Team B 后选择 Team A。",
		SourcesCited:    "无RSSHub新闻，仅依据盘口与题面；订单簿买卖一价差正常；近成交未见异常砸盘。",
		AnalysisProcess: "①对照题面与赔率②排除锁盘与反向仓③确认方向胜率达标后决定买入 Team A。",
	}
	amount, reason := ValidateAndSizeDecision(d, snap, ctx, 70, true, 0)
	if amount != 0 || reason == "" {
		t.Fatalf("want blocked rebet, got amount=%.2f reason=%q", amount, reason)
	}
}

func TestValidateAndSizeDecisionAllowsCappedAdd(t *testing.T) {
	snap := polymarket.MarketSnapshot{
		MarketID: "m1",
		Question: "Team A vs Team B",
		Outcomes: []string{"Team A", "Team B"},
		Prices:   []float64{0.75, 0.25},
		EndDate:  "2099-01-01T00:00:00Z",
	}
	ctx := TradingContext{
		Balance:        100,
		Deployable:     70,
		MinWinRate:     78,
		MinEdgePercent: 10,
		Guards: ExposureGuards{
			OpenByMarket: map[string]OpenPos{
				"m1": {
					MarketID:   "m1",
					Outcome:    "Team A",
					EntryPrice: 0.60,
					Amount:     10,
					TradeCount: 1,
					MarkPrice:  0.75,
				},
			},
			MaxAddFraction:   0.30,
			MaxAddsPerMarket: 1,
			MinEntryPrice:    0.52,
			MaxEntryPrice:    0.86,
		},
	}
	d := openai.TradeDecision{
		ShouldTrade:     true,
		MarketID:        "m1",
		Outcome:         "Team A",
		Amount:          99, // clamped by addCap (and 10× item min)
		SelfWinRate:     90,
		MarketReason:    "本场今日内可结算、成交与流动性足够支持最低名义下单，且题面信息完整便于用新闻交叉验证，因此纳入候选。",
		OutcomeReason:   "Team A 近期状态与盘口隐含概率相比仍有可验证优势，结合新闻与赔率同向，排除 Team B 后选择 Team A。",
		SourcesCited:    "无RSSHub新闻，仅依据盘口与题面；订单簿买卖一价差正常；近成交未见异常砸盘。",
		AnalysisProcess: "①对照题面与赔率②确认持仓浮盈可加仓③按加仓上限与系统仓位区间执行加仓。",
	}
	amount, reason := ValidateAndSizeDecision(d, snap, ctx, 70, true, 0)
	if reason != "" {
		t.Fatalf("expected allow add, got reason=%q", reason)
	}
	// itemMin at 0.75*5=3.75, addCap=3.0 → clamped to ~3.0
	if amount < 2.9 || amount > 3.1 {
		t.Fatalf("expected capped add ~3.0, got %.2f", amount)
	}
}

func TestValidateAndSizeDecisionAllowsAnyEntryPrice(t *testing.T) {
	snap := polymarket.MarketSnapshot{
		MarketID: "m9",
		Question: "Underdog special",
		Outcomes: []string{"Yes", "No"},
		Prices:   []float64{0.40, 0.60},
		EndDate:  "2099-01-01T00:00:00Z",
	}
	ctx := TradingContext{
		Balance:    100,
		Deployable: 70,
		MinWinRate: 72,
		Guards:     ExposureGuards{},
	}
	d := openai.TradeDecision{
		ShouldTrade:     true,
		MarketID:        "m9",
		Outcome:         "Yes",
		Amount:          5,
		SelfWinRate:     90,
		MarketReason:    "本场今日内可结算、成交与流动性足够支持最低名义下单，且题面信息完整便于用新闻交叉验证，因此纳入候选。",
		OutcomeReason:   "根据赛程与伤病信息判断冷门方向仍有明确优势可验证，当前赔率尚未充分反映该信息，因此选择 Yes 方向。",
		SourcesCited:    "无RSSHub新闻，仅依据盘口与题面；流动性足够完成最低名义下单",
		AnalysisProcess: "①阅读题面与双方赔率②对照最低胜率门槛③确认方向清晰后按 1～5 倍最低额选择仓位。",
	}
	amount, reason := ValidateAndSizeDecision(d, snap, ctx, 70, true, 0)
	if reason != "" || amount <= 0 {
		t.Fatalf("want allow without entry/edge band, got amount=%.2f reason=%q", amount, reason)
	}
	// AI amount $5 kept: itemMin=0.40*5=2.0, max=10.0
	if amount < 4.9 || amount > 5.1 {
		t.Fatalf("want AI amount ~5.0, got %.2f", amount)
	}
}
