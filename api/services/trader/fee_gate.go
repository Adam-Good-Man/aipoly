package trader

// Conservative taker-fee assumption for Polymarket CLOB fills (sports ~lower, crypto ~higher).
const estimatedTakerFeeRate = 0.025

// Gross win profit must be at least this multiple of estimated fees.
// 2× = cover fees with clear headroom, without starving mid-priced favorites.
const minProfitOverFeeMultiple = 2.0

// RequiredEdgePercent returns how many points AI self_win_rate must beat implied odds.
// Mid favorites use the same base gap (no extra starve band).
func RequiredEdgePercent(price, baseMinEdge float64) float64 {
	_ = price
	if baseMinEdge < 0 {
		return 0
	}
	return baseMinEdge
}

// MaxGrossWinProfit is USDC profit if the BUY resolves in your favor (before fees).
// stake at price p → shares=stake/p → payout=shares → profit=stake*(1-p)/p.
func MaxGrossWinProfit(stake, price float64) float64 {
	if stake <= 0 || price <= 0 || price >= 1 {
		return 0
	}
	return stake * (1 - price) / price
}

// EstimatedTakerFee approximates round-trip / taker fee drag on a stake.
func EstimatedTakerFee(stake float64) float64 {
	if stake <= 0 {
		return 0
	}
	return stake * estimatedTakerFeeRate
}

// ThinUpsideAfterFees rejects buy prices where max win ROI cannot cover fees×multiple.
// For proportional fees this is independent of stake size.
func ThinUpsideAfterFees(price float64) string {
	if price <= 0 || price >= 1 {
		return "invalid price for fee check"
	}
	maxROI := (1 - price) / price
	need := estimatedTakerFeeRate * minProfitOverFeeMultiple
	if maxROI+1e-12 < need {
		return "win profit too thin vs fees"
	}
	return ""
}

// InsufficientFeeCoverage rejects tickets whose absolute win profit < fee×multiple.
func InsufficientFeeCoverage(stake, price float64) string {
	if stake <= 0 || price <= 0 || price >= 1 {
		return "invalid stake/price for fee check"
	}
	profit := MaxGrossWinProfit(stake, price)
	need := EstimatedTakerFee(stake) * minProfitOverFeeMultiple
	if profit+1e-9 < need {
		return "win profit cannot cover fees"
	}
	return ""
}
