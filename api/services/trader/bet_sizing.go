package trader

const (
	defaultMinOrderShares = 5
	// MinBetMultipleOfMin is the hard floor for a new bet (1× = exchange minimum).
	MinBetMultipleOfMin = 1.0
	// DefaultBetMultipleOfMin is used when the AI omits amount or sends ≤0.
	DefaultBetMultipleOfMin = 1.0
	// MaxBetMultipleOfMin caps any single bet at N × item minimum.
	// Kept small so one wrong call cannot wipe a large cash fraction.
	MaxBetMultipleOfMin = 3.0
	// MaxBetFractionOfDeployable caps one ticket as a share of free cash.
	MaxBetFractionOfDeployable = 0.12
)

// MinDeployableToScan is the cash floor before calling the LLM.
// Needs enough for a typical 1× live order (≈5 shares near $1).
func MinDeployableToScan(liveTrading bool) float64 {
	min := ItemMinBetAmount(0.99, defaultMinOrderShares, liveTrading) * MinBetMultipleOfMin
	if liveTrading {
		min += 0.25 // fee / rounding buffer
	}
	if min < 1.0 {
		min = 1.0
	}
	return min
}

// ItemMinBetAmount returns the minimum USDC notional for one order on a market item.
func ItemMinBetAmount(price, minOrderSize float64, liveTrading bool) float64 {
	if price <= 0 {
		return 0
	}
	shares := minOrderSize
	if shares <= 0 {
		shares = defaultMinOrderShares
	}
	notional := price * shares
	if liveTrading {
		if notional < 1.0 {
			notional = 1.0
		}
	} else if notional < 0.5 {
		notional = 0.5
	}
	return notional
}

// FloorBetByItemMin is the hard floor for one new bet: MinBetMultipleOfMin × item minimum.
func FloorBetByItemMin(itemMinAmount float64) float64 {
	if itemMinAmount <= 0 {
		return 0
	}
	return itemMinAmount * MinBetMultipleOfMin
}

// DefaultBetByItemMin is the fallback size when AI amount is missing.
func DefaultBetByItemMin(itemMinAmount float64) float64 {
	if itemMinAmount <= 0 {
		return 0
	}
	return itemMinAmount * DefaultBetMultipleOfMin
}

// MaxBetByItemMin is the hard ceiling for one bet: MaxBetMultipleOfMin × item minimum.
func MaxBetByItemMin(itemMinAmount float64) float64 {
	if itemMinAmount <= 0 {
		return 0
	}
	return itemMinAmount * MaxBetMultipleOfMin
}

// fractionCapOfDeployable returns the cash-fraction ceiling for one ticket.
func fractionCapOfDeployable(deployable, itemMin float64) float64 {
	if deployable <= 0 || MaxBetFractionOfDeployable <= 0 {
		return 0
	}
	cap := deployable * MaxBetFractionOfDeployable
	// Only bind when the fraction still clears the exchange minimum.
	if cap+1e-9 < itemMin {
		return 0
	}
	return cap
}

// ClampAIBetAmount sizes a bet from the AI amount into [1×itemMin, 3×itemMin],
// further capped at MaxBetFractionOfDeployable of free cash.
// Amount ≤ 0 falls back to 1×itemMin. Adds may size below floor when addCap is smaller.
func ClampAIBetAmount(aiAmount, itemMin, deployable, addCap, clobBalance float64, liveTrading bool) float64 {
	if itemMin <= 0 {
		return 0
	}
	floor := FloorBetByItemMin(itemMin)
	maxAllowed := MaxBetByItemMin(itemMin)
	if deployable < floor && addCap <= 0 {
		// New entries need at least 1×; adds may use a smaller addCap below.
		if deployable < itemMin {
			return 0
		}
	}
	if deployable < itemMin {
		return 0
	}

	amount := aiAmount
	if amount <= 0 {
		amount = DefaultBetByItemMin(itemMin)
	}
	if addCap <= 0 {
		// New entry: enforce 1×–3× band.
		if amount < floor {
			amount = floor
		}
	} else {
		// Add: allow down to item min, but still prefer not going below 1×.
		if amount < itemMin {
			amount = itemMin
		}
	}
	if amount > maxAllowed {
		amount = maxAllowed
	}
	if frac := fractionCapOfDeployable(deployable, itemMin); frac > 0 && amount > frac {
		amount = frac
	}
	if addCap > 0 && amount > addCap {
		amount = addCap
	}
	if amount > deployable {
		amount = deployable
	}
	if liveTrading && clobBalance > 0 {
		feeBuffer := liveFeeBuffer(clobBalance)
		spendable := clobBalance - feeBuffer
		if spendable < 0 {
			spendable = 0
		}
		if amount > spendable {
			amount = spendable
		}
	}
	minAccept := floor
	if addCap > 0 {
		minAccept = 1.0
		if !liveTrading {
			minAccept = 0.5
		}
		if itemMin < minAccept {
			minAccept = itemMin
		}
	}
	if amount < minAccept {
		return 0
	}
	if amount > maxAllowed {
		amount = maxAllowed
	}
	return amount
}

// ForcedMinBetAmount sizes at the default multiple (1×) when AI amount is ignored.
func ForcedMinBetAmount(itemMin, deployable, addCap, clobBalance float64, liveTrading bool) float64 {
	return ClampAIBetAmount(0, itemMin, deployable, addCap, clobBalance, liveTrading)
}

// MaxSingleBetByDeployable reports headroom up to 3× item min (or deployable / cash fraction if smaller).
func MaxSingleBetByDeployable(deployable, itemMinAmount float64) float64 {
	if deployable <= 0 || itemMinAmount <= 0 {
		return 0
	}
	floor := FloorBetByItemMin(itemMinAmount)
	if deployable < floor {
		return 0
	}
	cap := MaxBetByItemMin(itemMinAmount)
	if frac := fractionCapOfDeployable(deployable, itemMinAmount); frac > 0 && frac < cap {
		cap = frac
	}
	if deployable < cap {
		return deployable
	}
	return cap
}
