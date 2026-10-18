package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"aipoly/api/services/polymarket"
)

// ScreenPick is a light-pass shortlist entry (no sizing, no deep reasoning).
type ScreenPick struct {
	MarketID string `json:"market_id"`
	Why      string `json:"why"`
}

// ScreenResult is the light screening response.
type ScreenResult struct {
	Summary  string       `json:"summary"`
	Selected []ScreenPick `json:"selected"`
}

// ScreenMarkets asks the model to shortlist suitable markets from a small batch.
// Prompt is intentionally minimal — selection only.
func (s *Service) ScreenMarkets(
	batch []polymarket.MarketSnapshot,
	pctx MarketPromptContext,
	need int,
) (*ScreenResult, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY / MOONSHOT_API_KEY not configured")
	}
	if need <= 0 {
		need = 1
	}
	if need > len(batch) {
		need = len(batch)
	}

	system := fmt.Sprintf(`你是 Polymarket 初筛员：从本批（通常 30 个）市场中选出最多 %d 个进入深度分析，不做下注金额、不做详尽研报。

只输出 JSON:
{"summary":"中文简述","selected":[{"market_id":"id","why":"中文<=40字"}]}

硬性:
- 本批共 %d 个市场；最多选出 %d 个（尽量凑满 %d 个合格盘；可以更少或 0 个）
- 只选列表中的 market_id；禁止编造
- 合格：你认为诚实获胜概率大致 ≥ %.0f%%，且有可交易性（非锁盘 ≥97¢/≤3¢）
- 价格带优先：至少有一侧买入价约在 52–85¢（有费后利润空间）。≥86¢ 薄利大热不要选进短名单（引擎会拒）
- 体育/电竞优先；加密 Up/Down 可以选但要更挑剔
- blocked 中的 id 一律不选
- 信息很少：只根据题面+赔率做粗筛；选「最值得深挖、更可能有正期望」的盘，不是只选最高胜率数字
- selected 按你认为更值得深挖 / 更可能有 edge 的顺序排序`,
		need, len(batch), need, need, pctx.MinWinRate)

	user := buildScreenPrompt(batch, pctx, need)
	body, _ := json.Marshal(chatRequest{
		Model: s.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		ResponseFormat: &responseFormat{Type: "json_object"},
		Temperature:    modelTemperature(s.model),
		ReasoningEffort: modelReasoningEffort(s.model),
	})

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		raw, err := s.postChatCompletionRaw(body)
		if err == nil {
			raw = extractJSONObject(raw)
			var result ScreenResult
			if err := json.Unmarshal([]byte(raw), &result); err != nil {
				lastErr = fmt.Errorf("parse screen JSON: %w", err)
				if !isRetryableOpenAIErr(lastErr) || attempt == 2 {
					return nil, lastErr
				}
				continue
			}
			result.Selected = filterScreenPicks(result.Selected, batch, need)
			return &result, nil
		}
		lastErr = err
		if !isRetryableOpenAIErr(err) {
			break
		}
	}
	return nil, lastErr
}

func buildScreenPrompt(batch []polymarket.MarketSnapshot, pctx MarketPromptContext, need int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Need up to %d shortlist picks. Resolve within %d days. Min win≈%.0f%%. Prefer markets with a side in ~52–85¢ (skip thin ≥86¢-only favorites).\n",
		need, pctx.MaxMarketDays, pctx.MinWinRate))
	if len(pctx.BlockedMarketIDs) > 0 {
		blocked := pctx.BlockedMarketIDs
		if len(blocked) > 12 {
			blocked = blocked[:12]
		}
		sb.WriteString("Blocked: " + strings.Join(blocked, ", ") + "\n")
	}
	sb.WriteString("Batch:\n")
	for i, s := range batch {
		days := ""
		if s.DaysToEnd >= 0 {
			days = fmt.Sprintf(" ~%dd", s.DaysToEnd)
		}
		sb.WriteString(fmt.Sprintf("%d) id=%s%s\n", i+1, s.MarketID, days))
		sb.WriteString(fmt.Sprintf("Q: %s\n", truncateRunes(s.Question, 90)))
		for j, outcome := range s.Outcomes {
			price := 0.0
			if j < len(s.Prices) {
				price = s.Prices[j]
			}
			sb.WriteString(fmt.Sprintf("  %s %.0f¢\n", truncateRunes(outcome, 28), price*100))
		}
	}
	sb.WriteString("\nReturn selected market_ids only from this batch.")
	return sb.String()
}

func filterScreenPicks(picks []ScreenPick, batch []polymarket.MarketSnapshot, need int) []ScreenPick {
	allowed := make(map[string]polymarket.MarketSnapshot, len(batch))
	for _, s := range batch {
		allowed[s.MarketID] = s
	}
	out := make([]ScreenPick, 0, len(picks))
	seen := make(map[string]struct{})
	for _, p := range picks {
		id := strings.TrimSpace(p.MarketID)
		snap, ok := allowed[id]
		if id == "" || !ok {
			continue
		}
		// Drop thin-only favorites so screen rounds keep searching for mid-price edges.
		if !polymarket.HasBuyableEntryBand(snap) {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, ScreenPick{MarketID: id, Why: strings.TrimSpace(p.Why)})
		if need > 0 && len(out) >= need {
			break
		}
	}
	return out
}
