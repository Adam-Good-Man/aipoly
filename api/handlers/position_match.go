package handlers

import (
	"context"
	"strings"

	"aipoly/api/services/polymarket"
)

func (h *Handler) enrichPositions(ctx context.Context, addr string, items []polymarket.AccountPosition) ([]polymarket.AccountPosition, int) {
	svc := h.accountService()
	activities, err := svc.GetActivity(ctx, addr, 100, 0)
	activityCount := 0
	if err == nil {
		activityCount = len(activities)
		activities = h.attachAIReasoning(activities)
	}

	entries := h.buildAIReasonEntries()
	usedAct := make(map[int]bool)
	usedEntry := make(map[int]bool)

	for i := range items {
		pos := &items[i]
		if idx, score := findBestActivityForPosition(*pos, activities, usedAct); idx >= 0 && score >= 25 {
			act := activities[idx]
			usedAct[idx] = true
			pos.EntryAmount = act.USDCSize
			pos.EntryTimestamp = act.Timestamp
			applyActivityAI(pos, act)
		}
		if !pos.IsAITrade {
			if e, idx := findBestEntryForPosition(*pos, entries, usedEntry); idx >= 0 {
				usedEntry[idx] = true
				applyEntryAI(pos, e)
				if pos.EntryAmount == 0 && e.Amount > 0 {
					pos.EntryAmount = e.Amount
				}
				if pos.EntryTimestamp == 0 && !e.CreatedAt.IsZero() {
					pos.EntryTimestamp = e.CreatedAt.Unix()
				}
			}
		}
	}
	return items, activityCount
}

func applyActivityAI(pos *polymarket.AccountPosition, act polymarket.AccountActivityItem) {
	if !act.IsAITrade {
		return
	}
	pos.IsAITrade = true
	pos.Reasoning = act.Reasoning
	pos.MarketReason = act.MarketReason
	pos.OutcomeReason = act.OutcomeReason
	pos.SelfWinRate = act.SelfWinRate
}

func applyEntryAI(pos *polymarket.AccountPosition, e aiReasonEntry) {
	pos.IsAITrade = true
	pos.Reasoning = e.Reasoning
	pos.MarketReason = e.MarketReason
	pos.OutcomeReason = e.OutcomeReason
	pos.SelfWinRate = e.SelfWinRate
}

func findBestActivityForPosition(pos polymarket.AccountPosition, activities []polymarket.AccountActivityItem, used map[int]bool) (int, int) {
	posOutcome := normalizeOutcome(pos.Outcome)
	posTitle := normalizeTitle(pos.Title)
	posCond := strings.ToLower(strings.TrimSpace(pos.ConditionID))

	bestScore := 0
	bestIdx := -1
	for j, act := range activities {
		if used[j] {
			continue
		}
		if side := strings.ToUpper(strings.TrimSpace(act.Side)); side != "" && side != "BUY" {
			continue
		}
		score := scorePositionMatch(posOutcome, posTitle, posCond, act.Outcome, act.Title, act.ConditionID, act.USDCSize, 0)
		if score > bestScore {
			bestScore = score
			bestIdx = j
		}
	}
	return bestIdx, bestScore
}

func findBestEntryForPosition(pos polymarket.AccountPosition, entries []aiReasonEntry, used map[int]bool) (aiReasonEntry, int) {
	posOutcome := normalizeOutcome(pos.Outcome)
	posTitle := normalizeTitle(pos.Title)
	posCond := strings.ToLower(strings.TrimSpace(pos.ConditionID))

	bestScore := 0
	bestIdx := -1
	var best aiReasonEntry
	for j, e := range entries {
		if used[j] {
			continue
		}
		score := scorePositionMatch(posOutcome, posTitle, posCond, e.Outcome, e.MarketTitle, e.ConditionID, 0, e.Amount)
		if score > bestScore {
			bestScore = score
			best = e
			bestIdx = j
		}
	}
	if bestScore < 25 {
		return aiReasonEntry{}, -1
	}
	return best, bestIdx
}

func scorePositionMatch(posOutcome, posTitle, posCond, otherOutcome, otherTitle, otherCond string, usdcSize, dbAmount float64) int {
	score := 0
	eOutcome := normalizeOutcome(otherOutcome)
	eTitle := normalizeTitle(otherTitle)
	eCond := strings.ToLower(strings.TrimSpace(otherCond))

	if posCond != "" && eCond != "" && posCond == eCond {
		score += 50
	}
	if posTitle != "" && eTitle != "" {
		if posTitle == eTitle {
			score += 30
		} else if strings.Contains(posTitle, eTitle) || strings.Contains(eTitle, posTitle) {
			score += 20
		}
	}
	if posOutcome != "" && eOutcome != "" && posOutcome == eOutcome {
		score += 15
	}
	if usdcSize > 0 && dbAmount > 0 {
		diff := usdcSize - dbAmount
		if diff < 0 {
			diff = -diff
		}
		if diff <= 0.15 {
			score += 10
		} else if diff <= 0.5 {
			score += 5
		}
	}
	return score
}
