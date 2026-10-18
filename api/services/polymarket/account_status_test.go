package polymarket

import "testing"

func TestDeriveSettlementStatus(t *testing.T) {
	cases := []struct {
		name string
		raw  rawPosition
		want string
	}{
		{
			name: "near zero stays open until settle or stop-loss",
			raw:  rawPosition{CurPrice: 0.0005, CashPNL: -4.45},
			want: "open",
		},
		{
			name: "high price without redeemable stays open",
			raw:  rawPosition{CurPrice: 0.99, CashPNL: 0},
			want: "open",
		},
		{
			name: "open position",
			raw:  rawPosition{CurPrice: 0.644, CashPNL: -0.15},
			want: "open",
		},
		{
			name: "redeemable mid price with profit",
			raw:  rawPosition{Redeemable: true, CurPrice: 0.92, CashPNL: 0.2},
			want: "redeemable",
		},
		{
			name: "redeemable winner",
			raw:  rawPosition{Redeemable: true, CurPrice: 1.0, CashPNL: 0.61},
			want: "redeemable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveSettlementStatus(tc.raw); got != tc.want {
				t.Fatalf("deriveSettlementStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDerivePositionResult(t *testing.T) {
	cases := []struct {
		status string
		raw    rawPosition
		result string
		known  bool
	}{
		{"open", rawPosition{CashPNL: -0.15}, "unknown", false},
		{"resolved_lost", rawPosition{CashPNL: -1.0}, "loss", true},
		{"redeemable", rawPosition{CashPNL: 0.61}, "profit", true},
		{"resolved_won", rawPosition{CashPNL: 0}, "breakeven", true},
		{"resolved_won", rawPosition{CashPNL: -0.002}, "breakeven", true},
		{"resolved_won", rawPosition{CashPNL: 0.61}, "profit", true},
	}
	for _, tc := range cases {
		result, known := derivePositionResult(tc.raw, tc.status)
		if result != tc.result || known != tc.known {
			t.Fatalf("derivePositionResult(%s) = (%q,%v), want (%q,%v)",
				tc.status, result, known, tc.result, tc.known)
		}
	}
}

func TestComputePositionPerformance(t *testing.T) {
	open := []AccountPosition{
		{Title: "Open A", Outcome: "Yes", Result: "unknown", ResultKnown: false, CashPNL: -0.5},
		{Title: "Won B", Outcome: "No", ConditionID: "0xabc", Result: "profit", ResultKnown: true, CashPNL: 0.61},
	}
	closed := []AccountClosedPosition{
		{Title: "France", Outcome: "Yes", ConditionID: "0xdef", RealizedPNL: 0.42, Result: "profit"},
		{Title: "Ronaldo", Outcome: "No", ConditionID: "0x123", RealizedPNL: -1.0, Result: "loss"},
	}
	perf := ComputePortfolioPerformance(open, closed)
	if perf.PositionCount != 4 || perf.OpenCount != 1 || perf.ClosedCount != 2 {
		t.Fatalf("unexpected counts: %+v", perf)
	}
	if perf.WonCount != 2 || perf.LostCount != 1 || perf.SettledCount != 3 {
		t.Fatalf("settled counts: %+v", perf)
	}
	if perf.WinRate < 66.6 || perf.WinRate > 66.7 {
		t.Fatalf("win rate = %.2f, want ~66.7", perf.WinRate)
	}
	wantPnL := 0.61 + 0.42 - 1.0 - 0.5
	if perf.TotalPnL < wantPnL-0.001 || perf.TotalPnL > wantPnL+0.001 {
		t.Fatalf("total pnl = %.4f, want %.4f", perf.TotalPnL, wantPnL)
	}
}

func TestComputePositionPerformanceOpenOnly(t *testing.T) {
	positions := []AccountPosition{
		{ConditionID: "0x1", Outcome: "Yes", Result: "profit", ResultKnown: true},
		{ConditionID: "0x2", Outcome: "Yes", Result: "breakeven", ResultKnown: true},
		{ConditionID: "0x3", Outcome: "Yes", Result: "loss", ResultKnown: true},
		{ConditionID: "0x4", Outcome: "Yes", Result: "unknown", ResultKnown: false},
	}
	perf := ComputePositionPerformance(positions)
	if perf.WonCount != 2 || perf.LostCount != 1 || perf.OpenCount != 1 || perf.SettledCount != 3 {
		t.Fatalf("unexpected counts: %+v", perf)
	}
}
