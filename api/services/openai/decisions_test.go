package openai

import "testing"

func TestSelectTradeDecisions(t *testing.T) {
	in := []TradeDecision{
		{ShouldTrade: false, MarketID: "1", Amount: 1, SelfWinRate: 90},
		{ShouldTrade: true, MarketID: "2", Amount: 1, SelfWinRate: 74},
		{ShouldTrade: true, MarketID: "2", Amount: 2, SelfWinRate: 80}, // dup market — first kept before sort; seen skips second
		{ShouldTrade: true, MarketID: "3", Amount: 1, SelfWinRate: 82},
		{ShouldTrade: true, MarketID: "4", Amount: 1, SelfWinRate: 78},
		{ShouldTrade: true, MarketID: "5", Amount: 1, SelfWinRate: 70},
	}
	out := SelectTradeDecisions(in, 3)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	// Ranked by self_win_rate: 3(82), 4(78), 2(74) — market 5 dropped
	if out[0].MarketID != "3" || out[1].MarketID != "4" || out[2].MarketID != "2" {
		t.Fatalf("unexpected selection order: %+v", out)
	}
}

func TestSelectTradeDecisionsZeroWhenNone(t *testing.T) {
	out := SelectTradeDecisions([]TradeDecision{
		{ShouldTrade: false, MarketID: "1", Amount: 1},
	}, 3)
	if len(out) != 0 {
		t.Fatalf("len = %d, want 0", len(out))
	}
}

func TestSelectTradeDecisionsCapsAtMax(t *testing.T) {
	in := []TradeDecision{
		{ShouldTrade: true, MarketID: "a", Amount: 1, SelfWinRate: 90},
		{ShouldTrade: true, MarketID: "b", Amount: 1, SelfWinRate: 88},
		{ShouldTrade: true, MarketID: "c", Amount: 1, SelfWinRate: 85},
		{ShouldTrade: true, MarketID: "d", Amount: 1, SelfWinRate: 84},
	}
	out := SelectTradeDecisions(in, 3)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	if out[0].MarketID != "a" || out[2].MarketID != "c" {
		t.Fatalf("want top-3 by win rate, got %+v", out)
	}
}
