package trader

import (
	"testing"

	"aipoly/api/services/polymarket"
)

func TestBuildScreenBatchSkipsAndCapsCrypto(t *testing.T) {
	pool := []polymarket.MarketSnapshot{
		{MarketID: "s1", Question: "Team A vs Team B"},
		{MarketID: "s2", Question: "LoL: X vs Y"},
		{MarketID: "c1", Question: "Bitcoin Up or Down - 5 min"},
		{MarketID: "c2", Question: "Ethereum Up or Down - 15 min"},
		{MarketID: "c3", Question: "Solana Up or Down - 5 min"},
		{MarketID: "c4", Question: "BTC Up or Down - hourly"},
		{MarketID: "c5", Question: "XRP Up or Down - 5 min"},
		{MarketID: "s3", Question: "UFC fight night"},
	}
	skip := map[string]bool{"s1": true}
	batch := BuildScreenBatch(pool, skip, 6, 2)
	if len(batch) == 0 {
		t.Fatal("expected non-empty batch")
	}
	seen := map[string]bool{}
	crypto := 0
	for _, s := range batch {
		if s.MarketID == "s1" {
			t.Fatal("skipped id should not reappear")
		}
		if seen[s.MarketID] {
			t.Fatalf("duplicate %s", s.MarketID)
		}
		seen[s.MarketID] = true
		if IsCryptoDirectionMarket(s) {
			crypto++
		}
	}
	if crypto > 2 {
		t.Fatalf("crypto dir cap exceeded: %d", crypto)
	}
}

func TestForcedMinBetAmount(t *testing.T) {
	itemMin := 3.0
	got := ForcedMinBetAmount(itemMin, 100, 0, 100, true)
	want := itemMin * DefaultBetMultipleOfMin // 1× default
	if got != want {
		t.Fatalf("want default %.2f got %.2f", want, got)
	}
	if MaxBetByItemMin(itemMin) != 9.0 {
		t.Fatalf("want 3x min")
	}
	if FloorBetByItemMin(itemMin) != 3.0 {
		t.Fatalf("want 1x floor")
	}
	// add cap below floor → allow addCap for adds
	got = ForcedMinBetAmount(itemMin, 100, 2.5, 100, true)
	if got != 2.5 {
		t.Fatalf("want addCap 2.5 got %.2f", got)
	}
}
