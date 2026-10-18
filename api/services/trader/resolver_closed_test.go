package trader

import (
	"testing"

	"aipoly/api/models"
	"aipoly/api/services/polymarket"
)

func TestMatchClosedPosition(t *testing.T) {
	trade := &models.Trade{
		MarketTitle: "LoL: Conviction vs Dorado Gaming (BO3) - North American Challengers League Group Stage",
		Outcome:     "Conviction",
		ConditionID: "0xabc",
	}
	closed := []polymarket.AccountClosedPosition{
		{Title: "Counter-Strike: Liquid vs Spirit", Outcome: "Spirit", ConditionID: "0xother", RealizedPNL: 1.2, Result: "profit"},
		{Title: "LoL: Conviction vs Dorado Gaming (BO3) - North American Challengers League Group Stage", Outcome: "Conviction", ConditionID: "0xabc", RealizedPNL: -4.45, Result: "loss"},
	}
	cp, ok := matchClosedPosition(trade, closed)
	if !ok {
		t.Fatal("expected match")
	}
	if cp.RealizedPNL != -4.45 || cp.Result != "loss" {
		t.Fatalf("got %+v", cp)
	}
}

func TestMatchClosedPositionByTitle(t *testing.T) {
	trade := &models.Trade{
		MarketTitle: "LoL: Conviction vs Dorado Gaming (BO3) - North American",
		Outcome:     "Conviction",
	}
	closed := []polymarket.AccountClosedPosition{
		{Title: "LoL: Conviction vs Dorado Gaming (BO3) - North American Challengers League Group Stage", Outcome: "Conviction", RealizedPNL: -4.45, Result: "loss"},
	}
	if _, ok := matchClosedPosition(trade, closed); !ok {
		t.Fatal("expected title fallback match")
	}
}
