package openai

import "sort"

// SelectTradeDecisions keeps high-intent entries, dedupes by market, ranks by
// likelihood to win (self_win_rate), then caps at max (0..max).
// Requires Amount > 0 (legacy single-pass flow).
func SelectTradeDecisions(decisions []TradeDecision, max int) []TradeDecision {
	return selectTradeDecisions(decisions, max, false)
}

// SelectTradeDecisionsAllowZeroAmount keeps deep-pass decisions even if amount
// is 0; the engine clamps into [itemMin, 3×itemMin] (missing → itemMin).
func SelectTradeDecisionsAllowZeroAmount(decisions []TradeDecision, max int) []TradeDecision {
	return selectTradeDecisions(decisions, max, true)
}

func selectTradeDecisions(decisions []TradeDecision, max int, allowZeroAmount bool) []TradeDecision {
	if max <= 0 {
		max = 3
	}
	cands := make([]TradeDecision, 0, len(decisions))
	seen := make(map[string]struct{})
	for _, d := range decisions {
		if !d.ShouldTrade || d.MarketID == "" {
			continue
		}
		if !allowZeroAmount && d.Amount <= 0 {
			continue
		}
		if _, dup := seen[d.MarketID]; dup {
			continue
		}
		seen[d.MarketID] = struct{}{}
		cands = append(cands, d)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].SelfWinRate != cands[j].SelfWinRate {
			return cands[i].SelfWinRate > cands[j].SelfWinRate
		}
		return cands[i].Amount > cands[j].Amount
	})
	if len(cands) > max {
		cands = cands[:max]
	}
	return cands
}
