package polymarket

// Buyable entry band aligned with trader MaxEntry/MinEntry (~52–86¢).
// Markets with no outcome in this band are thin favorites / lockouts the engine will reject.
const (
	BuyableMinPrice = 0.52
	BuyableMaxPrice = 0.86
)

// HasBuyableEntryBand reports whether any outcome price sits in the engine's buyable band.
func HasBuyableEntryBand(s MarketSnapshot) bool {
	for _, p := range s.Prices {
		if p >= BuyableMinPrice && p <= BuyableMaxPrice {
			return true
		}
	}
	return false
}
