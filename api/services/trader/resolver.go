package trader

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"aipoly/api/database"
	"aipoly/api/models"
	"aipoly/api/services/polymarket"
)

var resolveMu sync.Mutex

// StartResolver periodically settles open trades and syncs live balance.
func (e *Engine) StartResolver() {
	interval := e.cfg.ResolverIntervalMins
	if interval <= 0 {
		interval = 3
	}
	go func() {
		// Run once shortly after startup
		time.Sleep(15 * time.Second)
		e.SettleAndSync(context.Background())

		ticker := time.NewTicker(time.Duration(interval) * time.Minute)
		for range ticker.C {
			e.SettleAndSync(context.Background())
		}
	}()
}

// ResolveOpenTradesNow checks all open trades and returns how many were settled.
func (e *Engine) ResolveOpenTradesNow() int {
	resolveMu.Lock()
	defer resolveMu.Unlock()
	return e.resolveOpenTrades()
}

// RecentlyResolved returns trades settled within the given duration.
func (e *Engine) RecentlyResolved(within time.Duration) []models.Trade {
	since := time.Now().Add(-within)
	var trades []models.Trade
	e.db.Where("status IN ? AND resolved_at >= ?", []models.TradeStatus{
		models.TradeStatusWon, models.TradeStatusLost,
	}, since).Order("resolved_at DESC").Limit(20).Find(&trades)
	return trades
}

func (e *Engine) resolveOpenTrades() int {
	var trades []models.Trade
	if err := e.db.Where("status = ?", models.TradeStatusOpen).Find(&trades).Error; err != nil {
		log.Printf("resolve trades query error: %v", err)
		return 0
	}

	resolved := 0
	if len(trades) > 0 {
		for i := range trades {
			trade := trades[i]
			snap, marketClosed, ok := e.lookupTradeMarket(&trade)
			if !ok {
				if trade.IsPaper && time.Since(trade.CreatedAt) > 24*time.Hour {
					if midSnap, midOk := e.midpointSnapshot(&trade); midOk {
						e.resolveTrade(&trade, midSnap, false)
						resolved++
					}
				}
				continue
			}

			if marketClosed {
				e.resolveTrade(&trade, snap, true)
				resolved++
				continue
			}

			if trade.IsPaper && time.Since(trade.CreatedAt) > 24*time.Hour {
				e.resolveTrade(&trade, snap, false)
				resolved++
			}
		}
	}

	resolved += e.resolveFromChainSettlements()
	resolved += e.resolveFromClosedPositions()

	if resolved > 0 {
		_ = database.UpdatePortfolioStats(e.db, !e.cfg.PaperTrading)
		if !e.cfg.PaperTrading {
			e.SyncLiveBalance()
		}
		log.Printf("resolved %d open trades, synced balance", resolved)
	}

	return resolved
}

// resolveFromChainSettlements marks DB open trades as won only when Polymarket
// shows a claimable win (redeemable). Losses are marked via market-closed
// settlement in resolveOpenTrades, or via stop-loss sell.
func (e *Engine) resolveFromChainSettlements() int {
	if e.cfg.PaperTrading {
		return 0
	}
	signer := e.redeemerSigner()
	if signer == "" {
		return 0
	}
	svc := polymarket.NewAccountServiceWithFallback(e.cfg.PolyFunderAddr, signer)
	positions, err := svc.GetAllPositions(context.Background(), signer)
	if err != nil || len(positions) == 0 {
		return 0
	}

	resolved := 0
	for _, p := range positions {
		if p.ConditionID == "" || p.Size <= 0 {
			continue
		}
		// Wins: only when Polymarket says redeemable (claimable), not mere high mid.
		// Near-zero prices stay open so stop-loss can sell on floating PnL.
		if !(p.Redeemable && p.CurPrice >= 0.90) {
			continue
		}
		var trades []models.Trade
		if err := e.db.Where("status = ? AND condition_id = ?", models.TradeStatusOpen, p.ConditionID).
			Find(&trades).Error; err != nil || len(trades) == 0 {
			continue
		}
		for i := range trades {
			trade := &trades[i]
			if !outcomesMatch(trade.Outcome, p.Outcome) {
				continue
			}
			snap := polymarket.MarketSnapshot{
				MarketID: trade.MarketID,
				Outcomes: []string{trade.Outcome},
				Prices:   []float64{p.CurPrice},
			}
			e.resolveTrade(trade, snap, true)
			resolved++
		}
	}
	return resolved
}

// resolveFromClosedPositions syncs Polymarket closed-positions into local AI trades.
// Overview 「已赢/已输」 comes from this API — matching open DB rows must reflect the same.
func (e *Engine) resolveFromClosedPositions() int {
	if e.cfg.PaperTrading {
		return 0
	}
	signer := e.redeemerSigner()
	if signer == "" {
		return 0
	}
	svc := polymarket.NewAccountServiceWithFallback(e.cfg.PolyFunderAddr, signer)
	closed, err := svc.GetClosedPositions(context.Background(), signer, 0)
	if err != nil {
		log.Printf("resolve closed-positions: %v", err)
		return 0
	}
	if len(closed) == 0 {
		return 0
	}

	var opens []models.Trade
	if err := e.db.Where("status = ?", models.TradeStatusOpen).Find(&opens).Error; err != nil || len(opens) == 0 {
		return 0
	}

	resolved := 0
	for i := range opens {
		trade := &opens[i]
		cp, ok := matchClosedPosition(trade, closed)
		if !ok {
			continue
		}
		now := time.Now()
		trade.ResolvedAt = &now
		trade.PnL = cp.RealizedPNL
		switch {
		case cp.Result == "loss" || cp.RealizedPNL < -0.01:
			trade.Status = models.TradeStatusLost
		case cp.Result == "profit" || cp.RealizedPNL > 0.01:
			trade.Status = models.TradeStatusWon
		default:
			trade.Status = models.TradeStatusWon
		}
		note := fmt.Sprintf("chain_settle: closed-positions realized=$%.2f", cp.RealizedPNL)
		if trade.OrderError != "" {
			trade.OrderError = trade.OrderError + "; " + note
		} else {
			trade.OrderError = note
		}
		if err := e.db.Save(trade).Error; err != nil {
			log.Printf("save closed-sync trade error: %v", err)
			continue
		}
		log.Printf("trade settled from closed-positions: %s | %s | %s | pnl=$%.2f",
			trade.MarketTitle, trade.Outcome, trade.Status, trade.PnL)
		resolved++
	}
	return resolved
}

func matchClosedPosition(trade *models.Trade, closed []polymarket.AccountClosedPosition) (polymarket.AccountClosedPosition, bool) {
	for _, c := range closed {
		condOK := trade.ConditionID != "" && c.ConditionID != "" &&
			strings.EqualFold(trade.ConditionID, c.ConditionID)
		titleOK := titlesLooselyMatch(trade.MarketTitle, c.Title)
		if !condOK && !titleOK {
			continue
		}
		if !outcomesMatch(trade.Outcome, c.Outcome) {
			continue
		}
		return c, true
	}
	return polymarket.AccountClosedPosition{}, false
}

func titlesLooselyMatch(a, b string) bool {
	na := normalizeOutcome(a)
	nb := normalizeOutcome(b)
	if na == "" || nb == "" {
		return false
	}
	return na == nb || strings.Contains(na, nb) || strings.Contains(nb, na)
}

func outcomesMatch(a, b string) bool {
	na := normalizeOutcome(a)
	nb := normalizeOutcome(b)
	return na != "" && na == nb
}

func normalizeOutcome(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "’", "")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func (e *Engine) lookupTradeMarket(trade *models.Trade) (snap polymarket.MarketSnapshot, closed bool, ok bool) {
	var m *polymarket.GammaMarket
	var err error

	if trade.ConditionID != "" {
		m, err = e.poly.FetchMarketByConditionID(trade.ConditionID)
	}
	if (err != nil || m == nil) && trade.MarketID != "" {
		m, err = e.poly.FetchMarketByID(trade.MarketID)
	}
	if err != nil || m == nil {
		return polymarket.MarketSnapshot{}, false, false
	}

	ev := polymarket.GammaEvent{
		ID:    trade.MarketID,
		Title: trade.EventTitle,
	}
	snap = polymarket.MarketSnapshotFromGamma(*m, ev)
	return snap, m.Closed, true
}

func (e *Engine) midpointSnapshot(trade *models.Trade) (polymarket.MarketSnapshot, bool) {
	if trade.TokenID == "" {
		return polymarket.MarketSnapshot{}, false
	}
	mid, err := e.poly.GetMidpoint(trade.TokenID)
	if err != nil || mid <= 0 {
		return polymarket.MarketSnapshot{}, false
	}
	snap := polymarket.MarketSnapshot{
		MarketID: trade.MarketID,
		Outcomes: []string{trade.Outcome},
		Prices:   []float64{mid},
	}
	return snap, true
}

func isMarketClosed(snap polymarket.MarketSnapshot, events []polymarket.GammaEvent) bool {
	for _, ev := range events {
		if ev.ID != snap.EventID {
			continue
		}
		for _, m := range ev.Markets {
			if m.ID == snap.MarketID && m.Closed {
				return true
			}
		}
	}
	return false
}

func (e *Engine) resolveTrade(trade *models.Trade, snap polymarket.MarketSnapshot, marketClosed bool) {
	currentPrice, _ := findOutcomePrice(snap, trade.Outcome)

	// Live: win/loss only after true market settlement (or stop-loss elsewhere).
	// Paper: allow mid-mark heuristics when market never closes cleanly.
	if !trade.IsPaper {
		if !marketClosed {
			return
		}
		if currentPrice > 0.05 && currentPrice < 0.95 {
			// Market flagged closed but price not terminal yet — wait.
			return
		}
	} else if !marketClosed {
		impliedProb := currentPrice * 100
		now := time.Now()
		trade.ResolvedAt = &now
		if impliedProb >= trade.SelfWinRate*0.8 {
			trade.Status = models.TradeStatusWon
			trade.PnL = trade.Shares*currentPrice - trade.Amount
		} else {
			trade.Status = models.TradeStatusLost
			trade.PnL = -trade.Amount * (1 - currentPrice)
		}
		if err := e.db.Save(trade).Error; err != nil {
			log.Printf("save resolved trade error: %v", err)
			return
		}
		log.Printf("trade settled (paper): %s | %s | %s | pnl=$%.2f",
			trade.MarketTitle, trade.Outcome, trade.Status, trade.PnL)
		return
	}

	now := time.Now()
	trade.ResolvedAt = &now
	if currentPrice >= 0.95 {
		trade.Status = models.TradeStatusWon
		trade.PnL = trade.Shares*1.0 - trade.Amount
	} else if currentPrice <= 0.05 {
		trade.Status = models.TradeStatusLost
		trade.PnL = -trade.Amount
	} else {
		// Paper only reaches here with marketClosed + mid price.
		trade.Status = models.TradeStatusWon
		trade.PnL = trade.Shares*currentPrice - trade.Amount
	}

	if err := e.db.Save(trade).Error; err != nil {
		log.Printf("save resolved trade error: %v", err)
		return
	}
	log.Printf("trade settled: %s | %s | %s | pnl=$%.2f",
		trade.MarketTitle, trade.Outcome, trade.Status, trade.PnL)
}
