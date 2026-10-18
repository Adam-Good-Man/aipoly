package polymarket

import "testing"

func TestHasBuyableEntryBand(t *testing.T) {
	thin := MarketSnapshot{Prices: []float64{0.92, 0.08}}
	if HasBuyableEntryBand(thin) {
		t.Fatal("92¢ favorite should not count as buyable band")
	}
	mid := MarketSnapshot{Prices: []float64{0.72, 0.28}}
	if !HasBuyableEntryBand(mid) {
		t.Fatal("72¢ should be buyable")
	}
	edge := MarketSnapshot{Prices: []float64{0.86, 0.14}}
	if !HasBuyableEntryBand(edge) {
		t.Fatal("86¢ should still be buyable (inclusive max)")
	}
}
