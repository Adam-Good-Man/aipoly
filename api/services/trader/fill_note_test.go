package trader

import (
	"strings"
	"testing"

	"aipoly/api/models"
	"aipoly/api/services/polymarket"
)

func TestApplyLiveBuyFillPartial(t *testing.T) {
	trade := &models.Trade{
		Amount:    12.67,
		Price:     0.845,
		Shares:    12.67 / 0.845,
		Reasoning: "【为何选此市场】本场为近期可结算电竞盘，流动性足够支持最低名义下单，且有相关赛前新闻可与赔率交叉验证。\n【为何选 G2】RSSHub 头条显示 G2 状态占优，赔率约 84¢ 尚未锁死，结合盘口倾向故选 G2。\n【查阅资料】RSSHub新闻1: Can TH upset G2；订单簿买一 84¢；近成交偏买盘\n【分析过程】amount取默认3×（$12.67），可摊薄手续费。",
	}
	applyLiveBuyFill(trade, 12.67, 0.845, &polymarket.LiveOrderResult{
		Status:       "matched",
		FilledUSDC:   3.74,
		FilledShares: 4.43,
	})
	if trade.Amount < 3.7 || trade.Amount > 3.8 {
		t.Fatalf("want filled amount ~3.74, got %.2f", trade.Amount)
	}
	if !strings.Contains(trade.Reasoning, "【成交说明】") {
		t.Fatalf("missing fill note: %s", trade.Reasoning)
	}
	if !strings.Contains(trade.Reasoning, "12.67") || !strings.Contains(trade.Reasoning, "3.74") {
		t.Fatalf("note should cite planned vs filled: %s", trade.Reasoning)
	}
}

func TestApplyLiveBuyFillFullKeepsPlan(t *testing.T) {
	trade := &models.Trade{
		Amount:    10,
		Price:     0.7,
		Shares:    10 / 0.7,
		Reasoning: "【分析过程】计划 3× 下单。",
	}
	applyLiveBuyFill(trade, 10, 0.7, &polymarket.LiveOrderResult{
		Status:     "matched",
		FilledUSDC: 10,
	})
	if trade.Amount != 10 {
		t.Fatalf("want full fill 10, got %.2f", trade.Amount)
	}
	if strings.Contains(trade.Reasoning, "【成交说明】") {
		t.Fatalf("full fill should not add discrepancy note")
	}
}
