package trader

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aipoly/api/config"
	"aipoly/api/database"
	"aipoly/api/models"
	"aipoly/api/services/news"
	"aipoly/api/services/openai"
	"aipoly/api/services/polymarket"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Engine struct {
	db         *gorm.DB
	cfg        *config.Config
	poly       *polymarket.Client
	clobTrader *polymarket.ClobTrader
	redeemer   *polymarket.PositionRedeemer
	openai     *openai.Service
	news       *news.Client
	running    bool
	lastCycle  CycleStatus
	cycleMu    sync.RWMutex
	settleBusy bool
	funding    AIFundingGate
}

type CycleStatus struct {
	At             time.Time `json:"at"`
	Summary        string    `json:"summary,omitempty"`
	TradesPlaced   int       `json:"trades_placed"`
	AICandidates   int       `json:"ai_candidates"`
	SkippedByRules int       `json:"skipped_by_rules"`
	Skipped        string    `json:"skipped,omitempty"`
	// Screening diagnostics (always serialized so the dashboard can show them)
	PoolSize      int `json:"pool_size"`
	ScreenRounds  int `json:"screen_rounds"`
	ItemsScreened int `json:"items_screened"`
	ShortlistSize int `json:"shortlist_size"`
	// Funding / AI pause
	AIPaused         bool    `json:"ai_paused"`
	LowBalanceStreak int     `json:"low_balance_streak"`
	Deployable       float64 `json:"deployable"`
	GeoblockBlocked  bool    `json:"geoblock_blocked,omitempty"`
	GeoblockDetail   string  `json:"geoblock_detail,omitempty"`
}

func NewEngine(db *gorm.DB, cfg *config.Config) *Engine {
	return &Engine{
		db:         db,
		cfg:        cfg,
		poly:       polymarket.NewClient(),
		clobTrader: polymarket.NewClobTrader(cfg),
		redeemer:   polymarket.NewPositionRedeemer(cfg),
		openai:     openai.NewService(cfg.OpenAIKey, cfg.OpenAIModel, cfg.OpenAIBaseURL),
		news:       news.NewClient(cfg.RSSHubBaseURL),
	}
}

func (e *Engine) Start() {
	go e.runLoop()
}

func (e *Engine) runLoop() {
	time.Sleep(5 * time.Second)
	e.RunCycle()

	ticker := time.NewTicker(time.Duration(e.cfg.TradingIntervalMins) * time.Minute)
	for range ticker.C {
		e.RunCycle()
	}
}

func (e *Engine) WarmClob() {
	if e.cfg.PaperTrading || e.clobTrader == nil {
		return
	}
	if e.clobTrader.Ready() {
		log.Println("CLOB live trading ready")
		e.SyncLiveBalance()
	} else {
		log.Printf("WARNING: CLOB live trading not ready: %v", e.clobTrader.InitError())
	}
}

func (e *Engine) ClobReady() bool {
	return e.clobTrader != nil && e.clobTrader.Ready()
}

func (e *Engine) ClobError() string {
	if e.clobTrader == nil {
		return "CLOB trader not initialized"
	}
	if err := e.clobTrader.InitError(); err != nil {
		return err.Error()
	}
	return ""
}

func (e *Engine) ClobBalance() float64 {
	if e.clobTrader == nil {
		return 0
	}
	return e.clobTrader.BalanceUSDC()
}

func (e *Engine) PaperTrading() bool {
	return e.cfg.PaperTrading
}

func (e *Engine) FetchLiveEvents(limit int, fastOnly bool) ([]polymarket.GammaEvent, error) {
	if fastOnly {
		return e.poly.FetchFastEvents(limit)
	}
	return e.poly.FetchTrendingEvents(limit)
}

func (e *Engine) FetchLiveEventsPaged(limit, offset int, fastOnly bool) ([]polymarket.GammaEvent, error) {
	if fastOnly {
		events, err := e.poly.FetchFastEvents(polymarket.EventsFetchCap())
		if err != nil {
			return nil, err
		}
		if offset >= len(events) {
			return nil, nil
		}
		end := offset + limit
		if end > len(events) {
			end = len(events)
		}
		return events[offset:end], nil
	}
	return polymarket.FetchTrendingEventsPaged(e.poly, limit, offset)
}

func (e *Engine) FetchEventByID(id string) (*polymarket.GammaEvent, error) {
	return polymarket.FetchEventByID(e.poly, id)
}

func (e *Engine) SyncLiveBalance() {
	if e.cfg.PaperTrading || e.clobTrader == nil || !e.clobTrader.Ready() {
		return
	}
	if bal, err := e.clobTrader.GetBalance(context.Background()); err == nil {
		log.Printf("CLOB USDC balance: %s", bal)
	}
	_ = database.SyncPortfolioBalance(e.db, e.clobTrader.BalanceUSDC(), true)
}

func (e *Engine) RunCycle() {
	e.runCycle(0)
}

// RunCycleMax runs analysis and places at most maxTrades orders (0 = no limit).
func (e *Engine) RunCycleMax(maxTrades int) {
	e.runCycle(maxTrades)
}

func (e *Engine) TradingIntervalMins() int {
	if e.cfg.TradingIntervalMins <= 0 {
		return 15
	}
	return e.cfg.TradingIntervalMins
}

func (e *Engine) LastCycleStatus() CycleStatus {
	e.cycleMu.RLock()
	defer e.cycleMu.RUnlock()
	return e.lastCycle
}

func (e *Engine) setCycleStatus(st CycleStatus) {
	e.cycleMu.Lock()
	e.lastCycle = st
	e.cycleMu.Unlock()
}

func (e *Engine) runCycle(maxTrades int) {
	if e.running {
		log.Println("trading cycle already running, skip")
		return
	}
	e.running = true
	defer func() { e.running = false }()

	log.Println("starting trading cycle...")

	if !e.cfg.PaperTrading {
		e.SyncLiveBalance()
		e.SettleAndSync(context.Background())
		if e.clobTrader != nil && e.clobTrader.Ready() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			geo := e.clobTrader.CheckGeoblock(ctx)
			cancel()
			if geo.Error != "" {
				log.Printf("geoblock check error: %s (continuing; orders may still fail)", geo.Error)
			} else if geo.Blocked {
				detail := fmt.Sprintf("Polymarket geoblock: ip=%s country=%s region=%s — switch HTTPS_PROXY exit until polymarket.com/api/geoblock returns blocked=false",
					geo.IP, geo.Country, geo.Region)
				log.Printf("skip AI cycle: %s", detail)
				e.setCycleStatus(CycleStatus{
					At:              time.Now(),
					Skipped:         detail,
					GeoblockBlocked: true,
					GeoblockDetail:  detail,
				})
				return
			} else {
				log.Printf("geoblock ok: ip=%s country=%s region=%s", geo.IP, geo.Country, geo.Region)
			}
		}
	}

	portfolio, err := database.GetPortfolio(e.db)
	if err != nil {
		log.Printf("get portfolio error: %v", err)
		e.setCycleStatus(CycleStatus{At: time.Now(), Skipped: "portfolio error"})
		return
	}

	var openAmount float64
	var openCount int64
	e.db.Model(&models.Trade{}).Where("status = ?", models.TradeStatusOpen).
		Select("COALESCE(SUM(amount), 0)").Scan(&openAmount)
	e.db.Model(&models.Trade{}).Where("status = ?", models.TradeStatusOpen).Count(&openCount)

	clobCash := portfolio.Balance
	if !e.cfg.PaperTrading && e.clobTrader != nil && e.clobTrader.Ready() {
		clobCash = e.clobTrader.BalanceUSDC()
	}

	tradeCtx := BuildTradingContext(
		clobCash, openAmount, int(openCount),
		e.cfg.MaxBetPercent, e.cfg.MinCashReservePercent,
		e.cfg.MinWinRate, e.cfg.MinEdgePercent,
	)
	tradeCtx.Guards = LoadExposureGuards(
		e.db, tradeCtx.Balance, e.cfg.LossCooldownHours, e.cfg.MaxDailyLossPercent,
	)
	if e.cfg.MaxAddFraction > 0 {
		tradeCtx.Guards.MaxAddFraction = e.cfg.MaxAddFraction
	}
	if e.cfg.MaxAddsPerMarket > 0 {
		tradeCtx.Guards.MaxAddsPerMarket = e.cfg.MaxAddsPerMarket
	}
	e.enrichGuardMarks(&tradeCtx.Guards)
	guards := tradeCtx.Guards

	minCash := MinDeployableToScan(!e.cfg.PaperTrading)
	wasPaused := e.funding.Paused
	allowAI, fundSkip := e.funding.ObserveDeployable(tradeCtx.Deployable, minCash)
	if !allowAI {
		log.Printf("skip AI cycle: %s (cash=$%.2f equity=$%.2f open=$%.2f paused=%v streak=%d)",
			fundSkip, clobCash, tradeCtx.Balance, tradeCtx.OpenExposure, e.funding.Paused, e.funding.Streak)
		e.setCycleStatus(CycleStatus{
			At:               time.Now(),
			Skipped:          fundSkip,
			AIPaused:         e.funding.Paused,
			LowBalanceStreak: e.funding.Streak,
			Deployable:       tradeCtx.Deployable,
		})
		return
	}
	if wasPaused {
		log.Printf("AI funding gate resumed: deployable=$%.2f min=$%.2f", tradeCtx.Deployable, minCash)
	}

	shortTerm, err := e.poly.FetchShortTermEvents(100, e.cfg.MaxMarketDays)
	if err != nil {
		log.Printf("fetch short-term events error: %v", err)
	}

	trending, err := e.poly.FetchTrendingEvents(80)
	if err != nil {
		log.Printf("fetch trending events error: %v", err)
	}

	fast, err := e.poly.FetchFastEvents(50)
	if err != nil {
		log.Printf("fetch fast events error: %v", err)
	}

	if len(shortTerm) == 0 && len(trending) == 0 && len(fast) == 0 {
		e.setCycleStatus(CycleStatus{At: time.Now(), Skipped: "no market data"})
		return
	}

	eventMap := make(map[string]polymarket.GammaEvent)
	for _, ev := range shortTerm {
		eventMap[ev.ID] = ev
	}
	for _, ev := range trending {
		eventMap[ev.ID] = ev
	}
	for _, ev := range fast {
		eventMap[ev.ID] = ev
	}

	var allEvents []polymarket.GammaEvent
	for _, ev := range eventMap {
		allEvents = append(allEvents, ev)
		e.cacheEvent(ev)
	}

	snapshots := e.poly.BuildSnapshots(allEvents)
	snapshots = FilterShortTermSnapshots(snapshots, e.cfg.MaxMarketDays)
	beforeFilter := len(snapshots)
	snapshots = FilterProfitFriendlySnapshots(snapshots)
	snapshots = PrioritizeTradableSnapshots(snapshots)
	e.applySnapshotMarks(&guards, snapshots)
	tradeCtx.Guards = guards
	log.Printf("market pool: %d events -> %d short-term -> %d after spot-crypto filter",
		len(allEvents), beforeFilter, len(snapshots))
	if len(snapshots) == 0 {
		log.Printf("no markets resolving within %d days", e.cfg.MaxMarketDays)
		e.setCycleStatus(CycleStatus{
			At:      time.Now(),
			Skipped: fmt.Sprintf("no markets within %d days", e.cfg.MaxMarketDays),
		})
		return
	}

	addable := AddablePositions(guards)
	blocked := HardBlockedMarketIDs(guards)
	blocked = append(blocked, SiblingMarketIDsOnOpenEvents(guards, snapshots)...)
	promptCtx := openai.MarketPromptContext{
		Balance:           tradeCtx.Balance,
		Deployable:        tradeCtx.Deployable,
		ReserveAmount:     tradeCtx.ReserveAmount,
		OpenExposure:      tradeCtx.OpenExposure,
		OpenPositions:     tradeCtx.OpenPositions,
		MaxBetPercent:     e.cfg.MaxBetPercent,
		MaxMarketDays:     e.cfg.MaxMarketDays,
		MinWinRate:        e.cfg.MinWinRate,
		MinEdgePercent:    e.cfg.MinEdgePercent,
		MaxTradesPerCycle: e.cfg.MaxTradesPerCycle,
		BlockedMarketIDs:  blocked,
		AddableHints:      formatAddableHints(addable, guards.MaxAddFraction),
		MaxSingleBet:      MaxSingleBetByDeployable(tradeCtx.Deployable, 1.0),
		LiveTrading:       !e.cfg.PaperTrading,
	}

	cycleLimit := maxTrades
	if cycleLimit <= 0 {
		cycleLimit = e.cfg.MaxTradesPerCycle
	}
	if cycleLimit <= 0 {
		cycleLimit = finalBetsPerCycle
	}
	// Screen aims for deep-analysis shortlist of 3; place top 0–2 that clear MinWinRate.
	shortlistNeed := screenShortlistSize
	finalBetLimit := finalBetsPerCycle
	if cycleLimit < finalBetLimit {
		finalBetLimit = cycleLimit
	}

	promptCtx.MaxTradesPerCycle = shortlistNeed // deep evaluates the full shortlist
	promptCtx.MaxPlacePerCycle = finalBetLimit  // engine places at most this many

	shortlist, screenSummary, screenStats, err := e.multiRoundScreen(snapshots, promptCtx, shortlistNeed)
	if err != nil {
		log.Printf("AI screen error: %v", err)
		e.setCycleStatus(CycleStatus{
			At: time.Now(), Skipped: "AI screen failed: " + err.Error(),
			PoolSize: screenStats.PoolSize, ScreenRounds: screenStats.Rounds, ItemsScreened: screenStats.ItemsScreened,
		})
		return
	}
	if len(shortlist) == 0 {
		log.Printf("screen produced 0 candidates: %s", screenSummary)
		e.setCycleStatus(CycleStatus{
			At:      time.Now(),
			Summary: screenSummary,
			Skipped: "screen shortlist empty",
			PoolSize: screenStats.PoolSize, ScreenRounds: screenStats.Rounds,
			ItemsScreened: screenStats.ItemsScreened, ShortlistSize: 0,
		})
		return
	}

	dossiers := e.buildDossiers(shortlist)
	analysis, err := e.openai.DeepAnalyzeMarkets(dossiers, promptCtx, shortlistNeed)
	if err != nil {
		log.Printf("AI deep analysis error: %v", err)
		e.setCycleStatus(CycleStatus{
			At: time.Now(), Skipped: "AI deep analysis failed: " + err.Error(),
			PoolSize: screenStats.PoolSize, ScreenRounds: screenStats.Rounds,
			ItemsScreened: screenStats.ItemsScreened, ShortlistSize: len(shortlist),
		})
		return
	}
	if analysis.Summary == "" {
		analysis.Summary = screenSummary
	} else if screenSummary != "" {
		analysis.Summary = screenSummary + " | " + analysis.Summary
	}

	snapshotMap := make(map[string]polymarket.MarketSnapshot)
	for _, s := range snapshots {
		snapshotMap[s.MarketID] = s
	}
	// Backfill win rates, then place top finalBetLimit among those that clear MinWinRate (0–2).
	for i := range analysis.Decisions {
		d := &analysis.Decisions[i]
		if !d.ShouldTrade {
			continue
		}
		snap, ok := snapshotMap[d.MarketID]
		if !ok {
			continue
		}
		price, _ := findOutcomePrice(snap, d.Outcome)
		if openai.EnsureAnalyzedWinRate(d, price, tradeCtx.MinWinRate) {
			log.Printf("backfilled self_win_rate=%.0f%% for %s (AI omitted numeric field)",
				d.SelfWinRate, truncateLog(snap.Question, 40))
		}
	}
	beforePick := len(analysis.Decisions)
	eligible := make([]openai.TradeDecision, 0, len(analysis.Decisions))
	for _, d := range analysis.Decisions {
		if !d.ShouldTrade || d.MarketID == "" {
			continue
		}
		if d.SelfWinRate < tradeCtx.MinWinRate {
			log.Printf("deep reject below gate: market=%s win=%.0f%% < %.0f%%",
				d.MarketID, d.SelfWinRate, tradeCtx.MinWinRate)
			continue
		}
		eligible = append(eligible, d)
	}
	analysis.Decisions = openai.SelectTradeDecisionsAllowZeroAmount(eligible, finalBetLimit)
	log.Printf("AI deep summary: %s (shortlist=%d deep_decisions=%d eligible≥%.0f%%=%d -> place_top=%d, deployable=$%.2f)",
		analysis.Summary, len(shortlist), beforePick, tradeCtx.MinWinRate, len(eligible), len(analysis.Decisions), tradeCtx.Deployable)
	if len(analysis.Decisions) == 0 {
		log.Printf("trading cycle: no deep candidates cleared %.0f%% win-rate gate — skip bets this round", tradeCtx.MinWinRate)
	}
	for i, d := range analysis.Decisions {
		log.Printf("AI place[%d]: trade=%v market=%s outcome=%s win=%.0f%% amount=$%.2f (clamp 1–3× item min, default 1×)",
			i, d.ShouldTrade, d.MarketID, d.Outcome, d.SelfWinRate, d.Amount)
	}

	tradesPlaced := 0
	skippedByRules := 0
	var skipNotes []string
	noteSkip := func(note string) {
		note = strings.TrimSpace(note)
		if note == "" {
			return
		}
		skipNotes = append(skipNotes, note)
		log.Printf("skip %s", note)
	}
	for _, decision := range analysis.Decisions {
		if !decision.ShouldTrade {
			noteSkip(fmt.Sprintf("market=%s: should_trade=false", decision.MarketID))
			continue
		}
		snap, ok := snapshotMap[decision.MarketID]
		if !ok {
			noteSkip(fmt.Sprintf("market=%s: not found in snapshots", decision.MarketID))
			continue
		}

		clobBal := 0.0
		if !e.cfg.PaperTrading && e.clobTrader != nil {
			clobBal = e.clobTrader.BalanceUSDC()
		}

		// Refresh deployable after prior trades in this cycle
		portfolio, _ = database.GetPortfolio(e.db)
		var cycleOpenAmount float64
		e.db.Model(&models.Trade{}).Where("status = ?", models.TradeStatusOpen).
			Select("COALESCE(SUM(amount), 0)").Scan(&cycleOpenAmount)
		cycleClobCash := clobBal
		if cycleClobCash <= 0 {
			cycleClobCash = clobCash
		}
		tradeCtx = BuildTradingContext(
			cycleClobCash, cycleOpenAmount, int(openCount)+tradesPlaced,
			e.cfg.MaxBetPercent, e.cfg.MinCashReservePercent,
			e.cfg.MinWinRate, e.cfg.MinEdgePercent,
		)
		tradeCtx.Guards = guards

		amount, skipReason := ValidateAndSizeDecision(decision, snap, tradeCtx, clobBal, !e.cfg.PaperTrading, e.cfg.MaxMarketDays)
		if amount <= 0 {
			skippedByRules++
			winNote := decision.SelfWinRate
			if skipReason == "win rate below minimum" && winNote <= 0 {
				skipReason = "win rate missing/zero from AI (need ≥" + fmt.Sprintf("%.0f", tradeCtx.MinWinRate) + ")"
			}
			noteSkip(fmt.Sprintf("%s: %s (win=%.0f%%)", truncateLog(snap.Question, 48), skipReason, winNote))
			continue
		}

		price, tokenID := findOutcomePrice(snap, decision.Outcome)
		if price <= 0 {
			noteSkip(fmt.Sprintf("%s: invalid price for outcome %s", truncateLog(snap.Question, 48), decision.Outcome))
			continue
		}
		price = roundPrice(price, 0.001)

		trade := models.Trade{
			ID:            uuid.New(),
			MarketID:      snap.MarketID,
			ConditionID:   snap.ConditionID,
			EventID:       snap.EventID,
			EventTitle:    snap.EventTitle,
			MarketTitle:   snap.Question,
			Outcome:       decision.Outcome,
			Side:          "BUY",
			Amount:        amount,
			Price:         price,
			Shares:        amount / price,
			Reasoning:     openai.FormatDecisionReason(decision),
			MarketReason:  strings.TrimSpace(decision.MarketReason),
			OutcomeReason: strings.TrimSpace(decision.OutcomeReason),
			SelfWinRate:   decision.SelfWinRate,
			Status:        models.TradeStatusOpen,
			IsPaper:       e.cfg.PaperTrading,
			TokenID:       tokenID,
		}

		if !e.cfg.PaperTrading {
			if tokenID == "" {
				noteSkip(fmt.Sprintf("%s: missing token ID", truncateLog(snap.Question, 48)))
				continue
			}
			if err := e.executeLiveOrder(&trade, tokenID, amount, price); err != nil {
				// Failed fills are not recorded — only successful opens count.
				noteSkip(fmt.Sprintf("%s: live order failed: %v", truncateLog(snap.Question, 48), err))
				continue
			}
			// Prefer actual filled notional after FAK (may be < planned).
			amount = trade.Amount
			price = trade.Price
		}

		if err := e.db.Create(&trade).Error; err != nil {
			noteSkip(fmt.Sprintf("%s: create trade error: %v", truncateLog(snap.Question, 48), err))
			continue
		}
		tradesPlaced++
		guards.MarkOpenedTrade(snap.MarketID, decision.Outcome, amount, price, EventExposureKey(snap.EventID, snap.EventTitle))
		log.Printf("trade placed: %s | %s @ %.2f | $%.2f | win rate: %.0f%% | days=%d",
			snap.Question, decision.Outcome, price, amount, decision.SelfWinRate, daysUntilEnd(snap))
		log.Printf("AI reasoning: %s", trade.Reasoning)

		// Deduct from deployable pool for subsequent trades this cycle
		tradeCtx.Deployable -= amount
		if tradeCtx.Deployable < 0 {
			tradeCtx.Deployable = 0
		}
		if !e.cfg.PaperTrading && e.clobTrader != nil {
			e.SyncLiveBalance()
		}

		if finalBetLimit > 0 && tradesPlaced >= finalBetLimit {
			log.Printf("reached final bet limit (%d) — stopping cycle", finalBetLimit)
			break
		}
	}

	e.db.Create(&models.AnalysisLog{
		ID:             uuid.New(),
		Summary:        sanitizeUTF8(analysis.Summary),
		EventsAnalyzed: len(allEvents),
		TradesPlaced:   tradesPlaced,
	})

	_ = database.UpdatePortfolioStats(e.db, !e.cfg.PaperTrading)
	skipMsg := ""
	if tradesPlaced == 0 {
		if len(analysis.Decisions) == 0 {
			skipMsg = "AI returned no should_trade decisions (summary-only)"
		} else if len(skipNotes) > 0 {
			skipMsg = strings.Join(skipNotes, "; ")
		} else {
			skipMsg = "AI decisions present but none actionable"
		}
		if skippedByRules > 0 {
			log.Printf("trading cycle complete: 0 trades (%d risk-skip, detail=%s)", skippedByRules, skipMsg)
		} else {
			log.Printf("trading cycle complete: 0 trades (%s)", skipMsg)
		}
	} else {
		log.Printf("trading cycle complete: %d trades placed", tradesPlaced)
	}
	e.setCycleStatus(CycleStatus{
		At:             time.Now(),
		Summary:        analysis.Summary,
		TradesPlaced:   tradesPlaced,
		AICandidates:   len(analysis.Decisions),
		SkippedByRules: skippedByRules,
		Skipped:        skipMsg,
		PoolSize:       screenStats.PoolSize,
		ScreenRounds:   screenStats.Rounds,
		ItemsScreened:  screenStats.ItemsScreened,
		ShortlistSize:  len(shortlist),
		AIPaused:       e.funding.Paused,
		Deployable:     tradeCtx.Deployable,
	})
}

func truncateLog(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// ScreenCycleStats summarizes multi-round light screening for the dashboard.
type ScreenCycleStats struct {
	PoolSize      int
	Rounds        int
	ItemsScreened int
}

// multiRoundScreen sends the ranked pool to AI in batches of 20.
// Markets already shown/rejected are never re-included; each round fills with fresh items.
func (e *Engine) multiRoundScreen(
	pool []polymarket.MarketSnapshot,
	promptCtx openai.MarketPromptContext,
	need int,
) ([]polymarket.MarketSnapshot, string, ScreenCycleStats, error) {
	stats := ScreenCycleStats{PoolSize: len(pool)}
	if need <= 0 {
		need = 1
	}
	skip := make(map[string]bool)
	for _, id := range promptCtx.BlockedMarketIDs {
		if id != "" {
			skip[id] = true
		}
	}

	byID := make(map[string]polymarket.MarketSnapshot, len(pool))
	for _, s := range pool {
		byID[s.MarketID] = s
	}

	shortlist := make([]polymarket.MarketSnapshot, 0, need)
	var summaries []string

	maxRounds := maxScreenRounds
	if n := (len(pool) + screenBatchSize - 1) / screenBatchSize; n > 0 && n < maxRounds {
		maxRounds = n
	}
	if maxRounds < 1 {
		maxRounds = 1
	}

	for round := 1; round <= maxRounds && len(shortlist) < need; round++ {
		remain := need - len(shortlist)
		batch := BuildScreenBatch(pool, skip, screenBatchSize, maxCryptoDirPerBatch)
		if len(batch) == 0 {
			log.Printf("screen round %d: no more markets to batch", round)
			break
		}
		for _, s := range batch {
			skip[s.MarketID] = true // shown this round — never re-send
		}
		stats.Rounds = round
		stats.ItemsScreened += len(batch)
		log.Printf("screen round %d: batch=%d need=%d shortlist=%d screened_total=%d",
			round, len(batch), remain, len(shortlist), stats.ItemsScreened)

		result, err := e.openai.ScreenMarkets(batch, promptCtx, remain)
		if err != nil {
			return shortlist, strings.Join(summaries, " | "), stats, err
		}
		if result.Summary != "" {
			summaries = append(summaries, fmt.Sprintf("R%d:%s", round, truncateLog(result.Summary, 80)))
		}
		picked := 0
		for _, p := range result.Selected {
			snap, ok := byID[p.MarketID]
			if !ok {
				continue
			}
			already := false
			for _, s := range shortlist {
				if s.MarketID == snap.MarketID {
					already = true
					break
				}
			}
			if already {
				continue
			}
			// One market per event in the shortlist (avoid BO3 + Game1 both deep-analyzed).
			eventKey := EventExposureKey(snap.EventID, snap.EventTitle)
			if eventKey != "" {
				eventDup := false
				for _, s := range shortlist {
					if EventExposureKey(s.EventID, s.EventTitle) == eventKey {
						eventDup = true
						break
					}
				}
				if eventDup {
					continue
				}
			}
			shortlist = append(shortlist, snap)
			picked++
			log.Printf("screen pick: %s (%s)", truncateLog(snap.Question, 48), truncateLog(p.Why, 40))
			if len(shortlist) >= need {
				break
			}
		}
		log.Printf("screen round %d: picked=%d total_shortlist=%d", round, picked, len(shortlist))
	}

	return shortlist, strings.Join(summaries, " | "), stats, nil
}

func (e *Engine) buildDossiers(snaps []polymarket.MarketSnapshot) []polymarket.MarketDossier {
	out := make([]polymarket.MarketDossier, 0, len(snaps))
	for _, snap := range snaps {
		full := e.eventDescription(snap.EventID)
		d := e.poly.BuildMarketDossier(snap, full)
		if e.news != nil && e.news.Enabled() {
			if items, err := e.news.FetchForMarket(snap, 5); err != nil {
				d.IntelNotes = append(d.IntelNotes, "rsshub: "+err.Error())
				log.Printf("rsshub news miss: %s: %v", truncateLog(snap.Question, 40), err)
			} else {
				// Preformatted block with relevance tags, links, and source routes.
				block := strings.TrimSpace(news.FormatItemsForPrompt(items, 5))
				d.NewsHeadlines = []string{block}
				matched := 0
				for _, it := range items {
					if it.Score > 0 {
						matched++
					}
				}
				log.Printf("rsshub news: %s got %d headlines (%d keyword-matched)",
					truncateLog(snap.Question, 40), len(items), matched)
			}
		} else {
			d.IntelNotes = append(d.IntelNotes, "rsshub: disabled (set RSSHUB_BASE_URL)")
		}
		out = append(out, d)
		log.Printf("dossier built: %s news=%d book=%d/%d trades=%d comments=%d gaps=%d",
			truncateLog(snap.Question, 40), len(d.NewsHeadlines), len(d.BookBids), len(d.BookAsks),
			len(d.RecentTrades), len(d.Comments), len(d.IntelNotes))
	}
	return out
}

func (e *Engine) eventDescription(eventID string) string {
	if eventID == "" || e.db == nil {
		return ""
	}
	var ev models.MarketEvent
	if err := e.db.Where("event_id = ?", eventID).Order("fetched_at desc").Limit(1).Find(&ev).Error; err != nil {
		return ""
	}
	return ev.Description
}

func (e *Engine) executeLiveOrder(trade *models.Trade, tokenID string, amount, price float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	result, err := e.clobTrader.PlaceBuy(ctx, tokenID, amount)
	if err != nil {
		// FOK: insufficient book depth fails here with no fill (preferred over FAK dust).
		return err
	}

	trade.ClobOrderID = result.OrderID
	trade.IsPaper = false
	applyLiveBuyFill(trade, amount, price, result)
	if trade.Amount < 0.5 {
		return fmt.Errorf("fill too small: planned=$%.2f filled=$%.2f status=%s",
			amount, trade.Amount, result.Status)
	}
	log.Printf("CLOB order placed: id=%s status=%s market=%s planned=$%.2f filled=$%.2f shares=%.4f",
		result.OrderID, result.Status, trade.MarketTitle, amount, trade.Amount, trade.Shares)
	return nil
}

// applyLiveBuyFill updates trade size from the CLOB response and annotates partial fills.
func applyLiveBuyFill(trade *models.Trade, planned, price float64, result *polymarket.LiveOrderResult) {
	if trade == nil || result == nil {
		return
	}
	filled := result.FilledUSDC
	shares := result.FilledShares
	if filled <= 0 {
		// API sometimes omits amounts on full match — keep planned.
		filled = planned
	}
	trade.Amount = filled
	if shares > 0 {
		trade.Shares = shares
		trade.Price = filled / shares
	} else if price > 0 {
		trade.Shares = filled / price
	}
	trade.Reasoning = openai.AppendFillDiscrepancyNote(trade.Reasoning, planned, filled, result.Status)
}

func (e *Engine) cacheEvent(ev polymarket.GammaEvent) {
	marketsJSON, _ := json.Marshal(ev.Markets)
	endDate := parseTime(ev.EndDate)
	now := time.Now()

	record := models.MarketEvent{
		ID:          uuid.New(),
		EventID:     ev.ID,
		Title:       ev.Title,
		Description: ev.Description,
		Category:    ev.Category,
		Volume:      ev.Volume,
		Liquidity:   ev.Liquidity,
		IsFast:      polymarket.IsFastEvent(ev, now),
		EndDate:     endDate,
		MarketsJSON: string(marketsJSON),
		FetchedAt:   time.Now(),
	}

	var existing models.MarketEvent
	result := e.db.Where("event_id = ?", ev.ID).Limit(1).Find(&existing)
	if result.Error != nil {
		return
	}
	if result.RowsAffected > 0 {
		record.ID = existing.ID
		e.db.Save(&record)
		return
	}
	e.db.Create(&record)
}

func blockedMarketList(g ExposureGuards) []string {
	return HardBlockedMarketIDs(g)
}

func (e *Engine) enrichGuardMarks(g *ExposureGuards) {
	if g == nil || e.poly == nil || len(g.OpenByMarket) == 0 {
		return
	}
	var opens []models.Trade
	if err := e.db.Where("status = ? AND token_id != ''", models.TradeStatusOpen).Find(&opens).Error; err != nil {
		return
	}
	seen := make(map[string]bool)
	for _, t := range opens {
		if t.MarketID == "" || seen[t.MarketID] {
			continue
		}
		mid, err := e.poly.GetMidpoint(t.TokenID)
		if err != nil || mid <= 0 {
			continue
		}
		g.SetOpenMark(t.MarketID, mid)
		seen[t.MarketID] = true
	}
}

func (e *Engine) applySnapshotMarks(g *ExposureGuards, snapshots []polymarket.MarketSnapshot) {
	if g == nil {
		return
	}
	for _, snap := range snapshots {
		pos, ok := g.OpenByMarket[snap.MarketID]
		if !ok {
			continue
		}
		price, _ := findOutcomePrice(snap, pos.Outcome)
		if price > 0 {
			g.SetOpenMark(snap.MarketID, price)
		}
	}
}

func formatAddableHints(positions []OpenPos, maxAddFrac float64) []string {
	if len(positions) == 0 {
		return nil
	}
	out := make([]string, 0, len(positions))
	for _, pos := range positions {
		out = append(out, formatAddableHint(pos, maxAddFrac))
	}
	return out
}

func findOutcomePrice(snap polymarket.MarketSnapshot, outcome string) (float64, string) {
	outcomeLower := strings.ToLower(strings.TrimSpace(outcome))
	for i, o := range snap.Outcomes {
		if strings.ToLower(strings.TrimSpace(o)) == outcomeLower {
			price := 0.0
			if i < len(snap.Prices) {
				price = snap.Prices[i]
			}
			tokenID := ""
			if i < len(snap.TokenIDs) {
				tokenID = snap.TokenIDs[i]
			}
			return price, tokenID
		}
	}
	for i, o := range snap.Outcomes {
		ol := strings.ToLower(o)
		if strings.Contains(ol, outcomeLower) || strings.Contains(outcomeLower, ol) {
			price := 0.0
			if i < len(snap.Prices) {
				price = snap.Prices[i]
			}
			tokenID := ""
			if i < len(snap.TokenIDs) {
				tokenID = snap.TokenIDs[i]
			}
			return price, tokenID
		}
	}
	return 0, ""
}

func roundPrice(price, tick float64) float64 {
	if tick <= 0 {
		tick = 0.001
	}
	return float64(int(price/tick+1e-9)) * tick
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

// sanitizeUTF8 strips invalid bytes so Postgres UTF8 inserts don't fail on LLM mojibake.
func sanitizeUTF8(s string) string {
	if s == "" || utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "?")
}
