package news

import (
	"strings"
	"testing"
	"time"

	"aipoly/api/services/polymarket"
)

func TestBuildQueriesVs(t *testing.T) {
	snap := polymarket.MarketSnapshot{
		EventTitle: "LoL: Gen.G vs T1",
		Question:   "LoL: Gen.G vs T1 (BO3) - LCK",
	}
	qs := buildQueries(snap)
	if len(qs) == 0 {
		t.Fatal("expected queries")
	}
	joined := strings.ToLower(strings.Join(qs, " "))
	if !strings.Contains(joined, "gen") && !strings.Contains(joined, "t1") {
		t.Fatalf("expected team names in %v", qs)
	}
}

func TestParseRSS(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<rss version="2.0"><channel>
<item><title>Gen.G wins LCK</title><link>https://x.test/1</link><description>Team &lt;b&gt;won&lt;/b&gt;</description><pubDate>Mon, 01 Jan 2026 00:00:00 GMT</pubDate></item>
</channel></rss>`)
	items, err := parseRSS(raw, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Gen.G wins LCK" {
		t.Fatalf("got %+v", items)
	}
	if !strings.Contains(items[0].Summary, "won") {
		t.Fatalf("summary clean failed: %q", items[0].Summary)
	}
}

func TestScoreHeadline(t *testing.T) {
	it := Item{Title: "Trump delays attorney general nomination", Summary: "Senate dispute"}
	score := scoreHeadline(it, []string{"trump", "senate", "bitcoin"})
	if score < 2 {
		t.Fatalf("expected trump+senate hits, got %d", score)
	}
	bad := scoreHeadline(Item{Title: "EU hits Google with fine"}, []string{"hit", "bitcoin"})
	if bad != 0 {
		t.Fatalf("expected 0 for substring false positive, got %d", bad)
	}
}

func TestClassifyAndRoutes(t *testing.T) {
	cases := []struct {
		q    string
		kind MarketKind
		need string
	}{
		{"Counter-Strike: 3DMAX vs MOUZ (BO3) - BLAST", KindEsportsCS, "cs2news"},
		{"LoL: Anyone's Legend vs EDward Gaming", KindEsportsLoL, "LoLEsports"},
		{"Valorant: KRÜ vs G2", KindEsportsValorant, "VALORANT"},
		{"Will Bitcoin be above $100k?", KindCrypto, "coindesk"},
		{"Will Trump win the midterms?", KindPolitics, "politics"},
		{"Will the Fed cut rates?", KindEconomy, "jin10"},
	}
	for _, tc := range cases {
		snap := polymarket.MarketSnapshot{Question: tc.q}
		got := ClassifyMarket(snap)
		if got != tc.kind {
			t.Fatalf("%q: kind=%s want %s", tc.q, got, tc.kind)
		}
		routes := topicRoutesForKind(got, snap)
		joined := strings.Join(routes, " ")
		if !strings.Contains(joined, tc.need) {
			t.Fatalf("%q routes missing %q: %v", tc.q, tc.need, routes)
		}
	}
}

func TestSelectFreshRelevantNewestFirst(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	pool := []Item{
		{Title: "old MOUZ", Score: 4, PublishedAt: now.Add(-10 * 24 * time.Hour)},
		{Title: "new MOUZ", Score: 3, PublishedAt: now.Add(-2 * time.Hour)},
		{Title: "mid MOUZ", Score: 5, PublishedAt: now.Add(-30 * time.Hour)},
		{Title: "weak", Score: 1, PublishedAt: now.Add(-1 * time.Hour)},
	}
	out := selectFreshRelevant(pool, 5*24*time.Hour, now, 5)
	if len(out) != 2 {
		// old excluded by age; weak by score
		t.Fatalf("len=%d want 2: %+v", len(out), out)
	}
	if out[0].Title != "new MOUZ" {
		t.Fatalf("want newest first, got %q", out[0].Title)
	}
}

func TestLooksLikeRSSHubErrorPage(t *testing.T) {
	if !looksLikeRSSHubErrorPage([]byte(`<html><title>Welcome to RSSHub!</title></html>`)) {
		t.Fatal("expected error page detect")
	}
	if looksLikeRSSHubErrorPage([]byte(`<?xml version="1.0"?><rss><channel><item><title>x</title></item></channel></rss>`)) {
		t.Fatal("valid rss should not look like error")
	}
}

func TestFormatItemsShowsAge(t *testing.T) {
	out := FormatItemsForPrompt([]Item{{
		Title: "MOUZ wins", Score: 4, SourceKind: "esports_cs", AgeHours: 3,
	}}, 3)
	if !strings.Contains(out, "3h ago") || !strings.Contains(out, "esports_cs") {
		t.Fatalf("unexpected format: %s", out)
	}
}
