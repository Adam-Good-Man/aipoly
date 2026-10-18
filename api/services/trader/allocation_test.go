package trader

import (
	"testing"
	"time"

	"aipoly/api/services/polymarket"
)

func TestBuildTradingContextAllCashDeployable(t *testing.T) {
	// reserve=0: open exposure must not shrink deployable; all free CLOB cash is usable.
	ctx := BuildTradingContext(17.13, 12.80, 10, 10, 0, 62, 5)
	if ctx.Deployable < 17.12 || ctx.Deployable > 17.14 {
		t.Fatalf("deployable = %.2f, want full cash 17.13", ctx.Deployable)
	}
	if ctx.ReserveAmount != 0 {
		t.Fatalf("reserve = %.2f, want 0", ctx.ReserveAmount)
	}
}

func TestBuildTradingContextOptionalReserve(t *testing.T) {
	ctx := BuildTradingContext(100, 0, 0, 10, 30, 62, 5)
	// 30% of equity 100 = 30 reserved → deployable 70
	if ctx.Deployable < 69.99 || ctx.Deployable > 70.01 {
		t.Fatalf("deployable = %.2f, want 70", ctx.Deployable)
	}
}

func TestBuildTradingContextEmptyOpenPositions(t *testing.T) {
	ctx := BuildTradingContext(100, 0, 0, 10, 0, 62, 5)
	if ctx.Deployable < 99.99 || ctx.Deployable > 100.01 {
		t.Fatalf("deployable = %.2f, want ~100 (full CLOB cash)", ctx.Deployable)
	}
}

func TestFilterShortTermSnapshotsStrict(t *testing.T) {
	now := time.Now()
	snapshots := []polymarket.MarketSnapshot{
		{Question: "near", EndDate: now.Add(36 * time.Hour).Format(time.RFC3339)},
		{Question: "far", EndDate: now.Add(10 * 24 * time.Hour).Format(time.RFC3339)},
		{Question: "fast-no-date", IsFast: true},
		{Question: "world cup", EndDate: now.Add(30 * 24 * time.Hour).Format(time.RFC3339)},
	}
	out := FilterShortTermSnapshots(snapshots, 2)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 short-term market", len(out))
	}
	if out[0].Question != "near" {
		t.Fatalf("got %q, want near", out[0].Question)
	}
}

func TestMarketWithinMaxDaysUnknownEnd(t *testing.T) {
	snap := polymarket.MarketSnapshot{IsFast: true, Volume: 1_000_000}
	if marketWithinMaxDays(snap, 2, time.Now()) {
		t.Fatal("expected unknown end date to be rejected")
	}
}
