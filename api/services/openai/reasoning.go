package openai

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	minMarketReasonRunes    = 40
	minOutcomeReasonRunes   = 40
	minAnalysisProcessRunes = 30
	minSourcesCitedRunes    = 18
)

// FormatDecisionReason builds a detailed Chinese explanation for UI / DB.
func FormatDecisionReason(d TradeDecision) string {
	mr := strings.TrimSpace(d.MarketReason)
	or := strings.TrimSpace(d.OutcomeReason)
	ap := strings.TrimSpace(d.AnalysisProcess)
	sc := strings.TrimSpace(d.SourcesCited)
	fallback := strings.TrimSpace(d.Reasoning)

	if mr == "" && or == "" && ap == "" && sc == "" {
		if fallback != "" {
			return fallback
		}
		return fmt.Sprintf("【为何选此市场】AI 推荐参与该市场\n【为何选 %s】基于概率与赔率判断", d.Outcome)
	}
	if mr == "" {
		mr = fallback
	}
	if or == "" {
		or = fallback
	}
	outcome := strings.TrimSpace(d.Outcome)
	if outcome == "" {
		outcome = "该方向"
	}

	var b strings.Builder
	b.WriteString("【为何选此市场】")
	b.WriteString(mr)
	b.WriteString("\n【为何选 ")
	b.WriteString(outcome)
	b.WriteString("】")
	b.WriteString(or)
	if sc != "" {
		b.WriteString("\n【查阅资料】")
		b.WriteString(sc)
	}
	if ap != "" {
		b.WriteString("\n【分析过程】")
		b.WriteString(ap)
	}
	return b.String()
}

// AppendFillDiscrepancyNote documents planned vs actual FAK fill on the reasoning blob.
func AppendFillDiscrepancyNote(reasoning string, plannedUSDC, filledUSDC float64, orderStatus string) string {
	if plannedUSDC <= 0 || filledUSDC <= 0 {
		return reasoning
	}
	diff := plannedUSDC - filledUSDC
	if diff < 0 {
		diff = -diff
	}
	// Ignore tiny rounding gaps.
	if diff < 0.25 && filledUSDC/plannedUSDC >= 0.92 {
		return reasoning
	}
	pct := filledUSDC / plannedUSDC * 100
	status := strings.TrimSpace(orderStatus)
	if status == "" {
		status = "FAK"
	}
	note := fmt.Sprintf(
		"计划市价买入 $%.2f，实际成交 $%.2f（约 %.0f%%）。原因：CLOB 使用 FAK（能成交多少算多少，剩余自动取消），当时对手盘卖单深度不足，未能按计划额全部成交。界面「下注」金额以实际成交为准；上文仓位倍数/金额为计划额。订单状态：%s。",
		plannedUSDC, filledUSDC, pct, status,
	)

	reasoning = strings.TrimSpace(reasoning)
	const fillTag = "【成交说明】"
	const procTag = "【分析过程】"
	if strings.Contains(reasoning, fillTag) {
		return reasoning
	}
	if i := strings.Index(reasoning, procTag); i >= 0 {
		// Keep plan text, then append fill explanation inside 分析过程.
		body := strings.TrimSpace(reasoning[i+len(procTag):])
		prefix := reasoning[:i+len(procTag)]
		reasoning = prefix + "\n" + body + "\n（成交复核）" + note
	}
	if reasoning == "" {
		return fillTag + note
	}
	return reasoning + "\n" + fillTag + note
}

// ParseDecisionReason splits stored reasoning into market / outcome / sources / process.
func ParseDecisionReason(text string) (marketReason, outcomeReason string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	const mk = "【为何选此市场】"
	const ok = "【为何选 "
	if !strings.Contains(text, mk) {
		return "", text
	}
	rest := strings.TrimPrefix(text, mk)
	if idx := strings.Index(rest, "\n"+ok); idx >= 0 {
		marketReason = strings.TrimSpace(rest[:idx])
		after := rest[idx+1:]
		if j := strings.Index(after, "】"); j >= 0 {
			outcomeReason = strings.TrimSpace(after[j+len("】"):])
			// Strip trailing sections so outcomeReason stays clean when parsed.
			for _, cut := range []string{"\n【查阅资料】", "\n【分析过程】"} {
				if k := strings.Index(outcomeReason, cut); k >= 0 {
					outcomeReason = strings.TrimSpace(outcomeReason[:k])
				}
			}
		}
		return marketReason, outcomeReason
	}
	return strings.TrimSpace(rest), ""
}

// ExtractReasonSections pulls all structured sections from a stored reasoning blob.
func ExtractReasonSections(text string) (market, outcome, sources, process string) {
	market, outcome = ParseDecisionReason(text)
	text = strings.TrimSpace(text)
	if i := strings.Index(text, "【查阅资料】"); i >= 0 {
		rest := text[i+len("【查阅资料】"):]
		if j := strings.Index(rest, "\n【"); j >= 0 {
			sources = strings.TrimSpace(rest[:j])
		} else {
			sources = strings.TrimSpace(rest)
		}
	}
	if i := strings.Index(text, "【分析过程】"); i >= 0 {
		process = strings.TrimSpace(text[i+len("【分析过程】"):])
		for _, cut := range []string{"\n【成交说明】", "\n【查阅资料】"} {
			if k := strings.Index(process, cut); k >= 0 {
				process = strings.TrimSpace(process[:k])
			}
		}
	}
	return market, outcome, sources, process
}

// ExtractFillNote returns the 【成交说明】 section when present.
func ExtractFillNote(text string) string {
	const tag = "【成交说明】"
	i := strings.Index(text, tag)
	if i < 0 {
		return ""
	}
	rest := strings.TrimSpace(text[i+len(tag):])
	if j := strings.Index(rest, "\n【"); j >= 0 {
		rest = strings.TrimSpace(rest[:j])
	}
	return rest
}

// ValidateReasonQuality rejects decisions without substantive Chinese reasoning.
func ValidateReasonQuality(d TradeDecision) string {
	mr := strings.TrimSpace(d.MarketReason)
	or := strings.TrimSpace(d.OutcomeReason)
	ap := strings.TrimSpace(d.AnalysisProcess)
	sc := strings.TrimSpace(d.SourcesCited)
	fallback := strings.TrimSpace(d.Reasoning)

	if mr == "" && or == "" && fallback == "" {
		return "missing reasoning"
	}
	if mr == "" {
		mr = fallback
	}
	if or == "" {
		or = fallback
	}
	if utf8.RuneCountInString(mr) < minMarketReasonRunes {
		return "market_reason too brief"
	}
	if utf8.RuneCountInString(or) < minOutcomeReasonRunes {
		return "outcome_reason too brief"
	}
	if utf8.RuneCountInString(ap) < minAnalysisProcessRunes {
		return "analysis_process too brief"
	}
	if utf8.RuneCountInString(sc) < minSourcesCitedRunes {
		return "sources_cited too brief"
	}
	return ""
}
