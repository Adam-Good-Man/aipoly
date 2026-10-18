package trader

import "fmt"

// AIFundingGate pauses AI screening after repeated insufficient-cash cycles
// so we do not burn LLM tokens when no order can be sized.
type AIFundingGate struct {
	Streak int
	Paused bool
}

const lowBalancePauseAfter = 2

// ObserveDeployable updates pause state from the latest free cash.
// Returns whether AI scanning should run this cycle.
func (g *AIFundingGate) ObserveDeployable(deployable, minNeeded float64) (allowAI bool, skipReason string) {
	if g == nil {
		return true, ""
	}
	if minNeeded <= 0 {
		minNeeded = 1
	}
	if deployable >= minNeeded {
		wasPaused := g.Paused
		g.Paused = false
		g.Streak = 0
		if wasPaused {
			return true, ""
		}
		return true, ""
	}

	// Insufficient funds.
	if g.Paused {
		return false, fmt.Sprintf(
			"AI paused: deployable $%.2f < min $%.2f — waiting for settle/stop-loss/redeem",
			deployable, minNeeded)
	}
	g.Streak++
	if g.Streak >= lowBalancePauseAfter {
		g.Paused = true
		return false, fmt.Sprintf(
			"AI paused after %d low-balance cycles: deployable $%.2f < min $%.2f — waiting for settle/stop-loss/redeem",
			lowBalancePauseAfter, deployable, minNeeded)
	}
	return false, fmt.Sprintf(
		"deployable $%.2f < min $%.2f (streak %d/%d) — skip AI",
		deployable, minNeeded, g.Streak, lowBalancePauseAfter)
}
