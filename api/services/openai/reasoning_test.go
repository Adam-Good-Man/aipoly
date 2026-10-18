package openai

import (
	"strings"
	"testing"
)

func TestFormatAndExtractReasonSections(t *testing.T) {
	d := TradeDecision{
		Outcome:         "Gen.G",
		MarketReason:    "本场为近期可结算电竞盘，流动性足够支持最低名义下单，且有相关赛前新闻可与赔率交叉验证。",
		OutcomeReason:   "RSSHub 头条显示 Gen.G 状态占优，赔率约 62¢ 尚未锁死，结合盘口倾向故选 Gen.G。",
		SourcesCited:    "RSSHub新闻1: Gen.G wins scrim；订单簿买一 61¢ 卖一 63¢；近成交偏买盘",
		AnalysisProcess: "①读新闻倾向 Gen.G→②对照赔率未充分定价→③看盘口确认→④给出 70% 胜率并下单",
	}
	text := FormatDecisionReason(d)
	m, o, s, p := ExtractReasonSections(text)
	if m == "" || o == "" || s == "" || p == "" {
		t.Fatalf("missing sections m=%q o=%q s=%q p=%q text=%q", m, o, s, p, text)
	}
	if err := ValidateReasonQuality(d); err != "" {
		t.Fatalf("validate: %s", err)
	}
}

func TestValidateReasonQualityRejectsBrief(t *testing.T) {
	d := TradeDecision{
		MarketReason:    "太短",
		OutcomeReason:   "也太短了不够字数限制测试用",
		SourcesCited:    "短",
		AnalysisProcess: "短",
	}
	if got := ValidateReasonQuality(d); got == "" {
		t.Fatal("expected reject")
	}
}

func TestAppendFillDiscrepancyNote(t *testing.T) {
	base := FormatDecisionReason(TradeDecision{
		Outcome:         "G2 Esports",
		MarketReason:    "本场为近期可结算电竞盘，流动性足够支持最低名义下单，且有相关赛前新闻可与赔率交叉验证。",
		OutcomeReason:   "RSSHub 头条显示 G2 状态占优，赔率约 84¢ 尚未锁死，结合盘口倾向故选 G2。",
		SourcesCited:    "RSSHub新闻1: Can TH upset G2；订单簿买一 84¢；近成交偏买盘",
		AnalysisProcess: "amount取默认3×（$12.67），可摊薄手续费。",
	})
	got := AppendFillDiscrepancyNote(base, 12.67, 3.74, "matched")
	if !strings.Contains(got, "【成交说明】") {
		t.Fatalf("missing fill section: %s", got)
	}
	if !strings.Contains(got, "12.67") || !strings.Contains(got, "3.74") {
		t.Fatalf("missing planned/filled amounts: %s", got)
	}
	if !strings.Contains(got, "卖单深度不足") {
		t.Fatalf("missing cause text: %s", got)
	}
	fill := ExtractFillNote(got)
	if fill == "" || !strings.Contains(fill, "FAK") && !strings.Contains(fill, "3.74") {
		t.Fatalf("ExtractFillNote failed: %q", fill)
	}
	// Tiny gap should not annotate.
	same := AppendFillDiscrepancyNote(base, 12.67, 12.60, "matched")
	if strings.Contains(same, "【成交说明】") {
		t.Fatalf("should ignore tiny gap: %s", same)
	}
}
