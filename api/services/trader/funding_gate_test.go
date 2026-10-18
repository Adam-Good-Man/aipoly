package trader

import "testing"

func TestAIFundingGatePauseAfterTwo(t *testing.T) {
	var g AIFundingGate
	min := 5.0

	ok, reason := g.ObserveDeployable(1.0, min)
	if ok || g.Paused || g.Streak != 1 {
		t.Fatalf("round1: ok=%v paused=%v streak=%d reason=%s", ok, g.Paused, g.Streak, reason)
	}
	ok, reason = g.ObserveDeployable(1.0, min)
	if ok || !g.Paused || g.Streak != 2 {
		t.Fatalf("round2 should pause: ok=%v paused=%v streak=%d reason=%s", ok, g.Paused, g.Streak, reason)
	}
	ok, reason = g.ObserveDeployable(1.0, min)
	if ok || !g.Paused {
		t.Fatalf("while paused: ok=%v reason=%s", ok, reason)
	}
	ok, reason = g.ObserveDeployable(6.0, min)
	if !ok || g.Paused || g.Streak != 0 {
		t.Fatalf("resume: ok=%v paused=%v streak=%d reason=%s", ok, g.Paused, g.Streak, reason)
	}
}

func TestMinDeployableToScan(t *testing.T) {
	got := MinDeployableToScan(true)
	if got < 5.0 {
		t.Fatalf("live min deployable = %.2f, want >= 5", got)
	}
}
