package openai

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	winRatePctRe = regexp.MustCompile(`(?i)(?:self_win_rate|胜率|赢面)[^0-9%]{0,12}(\d{2,3})\s*%?`)
	winRateBare  = regexp.MustCompile(`(\d{2,3})\s*%`)
)

// NormalizeDecisionWinRates fixes common model mistakes:
// - decimal 0.72 instead of 72
// - omitted field (0) while the prose still states a percent
func NormalizeDecisionWinRates(decisions []TradeDecision) {
	for i := range decisions {
		d := &decisions[i]
		if !d.ShouldTrade {
			continue
		}
		d.SelfWinRate = normalizeWinRateValue(d.SelfWinRate)
		if d.SelfWinRate > 0 {
			continue
		}
		if v := extractWinRateFromText(d.AnalysisProcess, d.OutcomeReason, d.MarketReason, d.Reasoning); v > 0 {
			d.SelfWinRate = v
		}
	}
}

// HasSubstantiveAnalysis reports whether the model wrote real analysis text.
// Shortlisted + analyzed decisions must never keep a numeric win rate of 0.
func HasSubstantiveAnalysis(d TradeDecision) bool {
	for _, s := range []string{d.MarketReason, d.OutcomeReason, d.AnalysisProcess, d.SourcesCited, d.Reasoning} {
		if utf8.RuneCountInString(strings.TrimSpace(s)) >= 18 {
			return true
		}
	}
	return false
}

// EnsureAnalyzedWinRate guarantees a non-zero self_win_rate when the model
// already wrote analysis (and should_trade). Priority:
// 1) existing / normalized numeric field
// 2) percent extracted from prose
// 3) market implied odds (price×100), floored at minWinRate
func EnsureAnalyzedWinRate(d *TradeDecision, outcomePrice, minWinRate float64) (filled bool) {
	if d == nil || !d.ShouldTrade {
		return false
	}
	d.SelfWinRate = normalizeWinRateValue(d.SelfWinRate)
	if d.SelfWinRate > 0 {
		return false
	}
	if !HasSubstantiveAnalysis(*d) {
		return false
	}
	if v := extractWinRateFromText(d.AnalysisProcess, d.OutcomeReason, d.MarketReason, d.Reasoning, d.SourcesCited); v > 0 {
		d.SelfWinRate = v
		return true
	}
	d.SelfWinRate = inferredWinRateFromPrice(outcomePrice, minWinRate)
	return true
}

func inferredWinRateFromPrice(price, minWinRate float64) float64 {
	implied := price * 100
	if minWinRate <= 0 {
		minWinRate = 62
	}
	if implied < minWinRate {
		// Analysis exists + shortlisted → never reject solely for a missing field.
		return minWinRate
	}
	if implied > 95 {
		return 95
	}
	return implied
}

func normalizeWinRateValue(v float64) float64 {
	if v <= 0 {
		return 0
	}
	// Model sometimes returns 0.68 meaning 68%.
	if v > 0 && v <= 1.0 {
		v *= 100
	}
	if v > 100 {
		v = 100
	}
	return v
}

func extractWinRateFromText(parts ...string) float64 {
	text := strings.Join(parts, "\n")
	if text == "" {
		return 0
	}
	if m := winRatePctRe.FindStringSubmatch(text); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n >= 50 && n <= 99 {
			return float64(n)
		}
	}
	// Prefer the last xx% in analysis process (often "自评 70%").
	all := winRateBare.FindAllStringSubmatch(text, -1)
	for i := len(all) - 1; i >= 0; i-- {
		n, err := strconv.Atoi(all[i][1])
		if err == nil && n >= 55 && n <= 95 {
			return float64(n)
		}
	}
	return 0
}
