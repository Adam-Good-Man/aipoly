package trader

import (
	"testing"

	"aipoly/api/services/polymarket"
)

func TestShouldStopOutAtThirty(t *testing.T) {
	pos := polymarket.AccountPosition{
		CurPrice: 0.42, AvgPrice: 0.64, Size: 5,
		CashPNL: -1.1, PercentPNL: -34.4,
	}
	ok, reason, loss, _ := shouldStopOut(pos, 30)
	if !ok || loss < 30 {
		t.Fatalf("expected stop at -30%%, got ok=%v reason=%s loss=%.1f", ok, reason, loss)
	}
}

func TestShouldStopOutUnderThreshold(t *testing.T) {
	pos := polymarket.AccountPosition{
		CurPrice: 0.55, AvgPrice: 0.64, Size: 5,
		CashPNL: -0.45, PercentPNL: -14,
	}
	ok, _, loss, _ := shouldStopOut(pos, 30)
	if ok {
		t.Fatalf("should not stop at %.1f%%", loss)
	}
}
