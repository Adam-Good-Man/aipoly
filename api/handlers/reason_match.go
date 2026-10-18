package handlers

import (
	"strings"
	"time"

	"aipoly/api/models"
	"aipoly/api/services/openai"
	"aipoly/api/services/polymarket"
)

type aiReasonEntry struct {
	Reasoning     string
	MarketReason  string
	OutcomeReason string
	SelfWinRate   float64
	ConditionID   string
	TokenID       string
	ClobOrderID   string
	MarketTitle   string
	Outcome       string
	Amount        float64
	CreatedAt     time.Time
}

func (h *Handler) buildAIReasonEntries() []aiReasonEntry {
	var trades []models.Trade
	q := h.db.Where("reasoning <> '' AND status <> ?", models.TradeStatusCanceled).
		Order("created_at DESC").Limit(100)
	if !h.cfg.PaperTrading {
		q = q.Where("is_paper = ?", false)
	}
	q.Find(&trades)

	out := make([]aiReasonEntry, 0, len(trades))
	for _, t := range trades {
		if strings.TrimSpace(t.Reasoning) == "" && strings.TrimSpace(t.MarketReason) == "" {
			continue
		}
		mr := strings.TrimSpace(t.MarketReason)
		or := strings.TrimSpace(t.OutcomeReason)
		if mr == "" && or == "" {
			mr, or = openai.ParseDecisionReason(t.Reasoning)
		}
		if mr == "" && or == "" {
			or = t.Reasoning
		}
		out = append(out, aiReasonEntry{
			Reasoning:     t.Reasoning,
			MarketReason:  mr,
			OutcomeReason: or,
			SelfWinRate:   t.SelfWinRate,
			ConditionID:   t.ConditionID,
			TokenID:       t.TokenID,
			ClobOrderID:   t.ClobOrderID,
			MarketTitle:   t.MarketTitle,
			Outcome:       t.Outcome,
			Amount:        t.Amount,
			CreatedAt:     t.CreatedAt,
		})
	}
	return out
}

func (h *Handler) attachAIReasoning(items []polymarket.AccountActivityItem) []polymarket.AccountActivityItem {
	entries := h.buildAIReasonEntries()
	used := make(map[int]bool)
	for i := range items {
		if e, idx := findBestReasonMatch(items[i], entries, used); idx >= 0 {
			used[idx] = true
			items[i].Reasoning = e.Reasoning
			items[i].MarketReason = e.MarketReason
			items[i].OutcomeReason = e.OutcomeReason
			items[i].IsAITrade = true
			items[i].SelfWinRate = e.SelfWinRate
		}
	}
	// 只展示 Polymarket 链上 activity，不把本地 DB 未上链记录追加进来
	return items
}

func findBestReasonMatch(item polymarket.AccountActivityItem, entries []aiReasonEntry, used map[int]bool) (aiReasonEntry, int) {
	itemOutcome := normalizeOutcome(item.Outcome)
	itemTitle := normalizeTitle(item.Title)
	itemCond := strings.ToLower(strings.TrimSpace(item.ConditionID))
	itemAsset := strings.ToLower(strings.TrimSpace(item.Asset))

	bestScore := 0
	bestIdx := -1
	var best aiReasonEntry

	for j, e := range entries {
		if used[j] {
			continue
		}
		score := scoreReasonMatch(itemOutcome, itemTitle, itemCond, itemAsset, item.USDCSize, e)
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

func scoreReasonMatch(itemOutcome, itemTitle, itemCond, itemAsset string, usdcSize float64, e aiReasonEntry) int {
	score := 0
	eOutcome := normalizeOutcome(e.Outcome)
	eTitle := normalizeTitle(e.MarketTitle)
	eCond := strings.ToLower(strings.TrimSpace(e.ConditionID))
	eAsset := strings.ToLower(strings.TrimSpace(e.TokenID))

	if itemCond != "" && eCond != "" && itemCond == eCond {
		score += 50
	}
	if itemAsset != "" && eAsset != "" && itemAsset == eAsset {
		score += 40
	}
	if itemTitle != "" && eTitle != "" {
		if itemTitle == eTitle {
			score += 30
		} else if strings.Contains(itemTitle, eTitle) || strings.Contains(eTitle, itemTitle) {
			score += 20
		}
	}
	if itemOutcome != "" && eOutcome != "" && itemOutcome == eOutcome {
		score += 15
	}
	if usdcSize > 0 && e.Amount > 0 {
		diff := usdcSize - e.Amount
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

func normalizeOutcome(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "yes", "y", "true":
		return "yes"
	case "no", "n", "false":
		return "no"
	default:
		return s
	}
}

func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), " ")
}
