package trader

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"aipoly/api/database"
	"aipoly/api/models"
	"aipoly/api/services/polymarket"
)

const (
	stopLossWatchEvery = 30 * time.Second
	stopLossSellTries  = 3
)

// StartStopLossWatcher keeps trying to sell once floating loss hits StopLossPercent.
// It does NOT invent a local loss if the sell does not fill — open PnL stays factual.
func (e *Engine) StartStopLossWatcher() {
	if e.cfg.StopLossPercent <= 0 {
		log.Println("stop-loss watcher disabled (STOP_LOSS_PERCENT<=0)")
		return
	}
	go func() {
		time.Sleep(8 * time.Second)
		e.runForcedStopLoss(context.Background())
		ticker := time.NewTicker(stopLossWatchEvery)
		for range ticker.C {
			e.runForcedStopLoss(context.Background())
		}
	}()
	log.Printf("stop-loss watcher started: sell at -%.0f%% every %s (book only on fill)",
		e.cfg.StopLossPercent, stopLossWatchEvery)
}

func (e *Engine) runForcedStopLoss(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	n := e.CheckStopLosses(ctx)
	if n > 0 {
		log.Printf("stop-loss watcher: sold %d position(s)", n)
	}
}

// CheckStopLosses tries to market-sell when floating loss reaches StopLossPercent.
// Local status/PnL flip to lost only after a successful sell (or paper fill).
// Failed sells leave the trade open so dashboard floating PnL stays truthful.
func (e *Engine) CheckStopLosses(ctx context.Context) int {
	pct := e.cfg.StopLossPercent
	if pct <= 0 {
		return 0
	}

	positions, err := e.fetchOpenChainPositions(ctx)
	if err != nil {
		log.Printf("stop-loss: fetch Data API positions: %v", err)
		return e.checkStopLossesFromDB(ctx, pct, nil)
	}
	if len(positions) == 0 {
		return 0
	}

	closed := 0
	for _, pos := range positions {
		if pos.Size <= 0 {
			continue
		}
		should, reason, lossPct, mark := shouldStopOut(pos, pct)
		if !should {
			continue
		}

		trade := e.findTradeForPosition(pos)
		tokenID := pos.Asset
		shares := pos.Size
		if trade != nil && trade.TokenID != "" {
			tokenID = trade.TokenID
		}
		if tokenID == "" {
			log.Printf("stop-loss: skip %s — no token_id", truncateLog(pos.Title, 40))
			continue
		}

		pnl := pos.CashPNL
		if pnl == 0 && mark > 0 && trade != nil {
			pnl = trade.Shares*mark - trade.Amount
		}
		note := fmt.Sprintf("stop_loss: %s cur=%.4f avg=%.4f float=%.1f%% cash_pnl=$%.2f",
			reason, mark, pos.AvgPrice, lossPct, pos.CashPNL)

		// Paper: no chain fill — book at current mark (simulated exit).
		if e.cfg.PaperTrading || e.clobTrader == nil || !e.clobTrader.Ready() {
			if trade == nil {
				continue
			}
			if trade.Status == models.TradeStatusLost && strings.Contains(trade.OrderError, "stop_loss") {
				continue
			}
			now := time.Now()
			trade.Status = models.TradeStatusLost
			trade.PnL = pnl
			trade.ResolvedAt = &now
			trade.Side = "SELL"
			trade.OrderError = note + "; paper"
			if err := e.db.Save(trade).Error; err != nil {
				continue
			}
			log.Printf("stop-loss paper exit: %s | %s | %s", truncateLog(pos.Title, 48), pos.Outcome, note)
			closed++
			continue
		}

		var lastErr error
		var result *polymarket.LiveOrderResult
		sold := false
		for attempt := 1; attempt <= stopLossSellTries; attempt++ {
			sellCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			result, lastErr = e.clobTrader.PlaceSell(sellCtx, tokenID, shares)
			cancel()
			if lastErr == nil {
				sold = true
				break
			}
			log.Printf("stop-loss sell try %d/%d failed %s: %v (keeping open — factual float)",
				attempt, stopLossSellTries, truncateLog(pos.Title, 40), lastErr)
			if attempt < stopLossSellTries {
				time.Sleep(400 * time.Millisecond)
			}
		}
		if !sold {
			// Do not invent a local -30% close. Retry next watch cycle.
			continue
		}

		log.Printf("stop-loss SELL filled: %s | %s | shares=%.2f | %s",
			truncateLog(pos.Title, 48), pos.Outcome, shares, note)

		now := time.Now()
		if trade == nil {
			closed++
			continue
		}
		trade.Status = models.TradeStatusLost
		trade.PnL = pnl
		trade.ResolvedAt = &now
		trade.Side = "SELL"
		if result != nil {
			trade.ClobOrderID = result.OrderID
		}
		trade.OrderError = note
		if err := e.db.Save(trade).Error; err != nil {
			log.Printf("stop-loss save error: %v", err)
			continue
		}
		closed++
	}

	if closed > 0 {
		_ = database.UpdatePortfolioStats(e.db, !e.cfg.PaperTrading)
		if !e.cfg.PaperTrading {
			e.SyncLiveBalance()
		}
		log.Printf("stop-loss: filled %d sell(s) at -%.0f%% threshold", closed, pct)
	}
	return closed
}

func shouldStopOut(pos polymarket.AccountPosition, stopPct float64) (bool, string, float64, float64) {
	mark := pos.CurPrice
	avg := pos.AvgPrice
	lossPct := 0.0

	if pos.PercentPNL != 0 {
		lossPct = -pos.PercentPNL
		if pos.PercentPNL <= -stopPct {
			return true, "percent_pnl", -pos.PercentPNL, mark
		}
	}
	cost := pos.AvgPrice * pos.Size
	if cost > 0 && pos.CashPNL < 0 {
		cashLossPct := -pos.CashPNL / cost * 100
		if cashLossPct >= stopPct {
			return true, "cash_pnl", cashLossPct, mark
		}
		if lossPct < cashLossPct {
			lossPct = cashLossPct
		}
	}
	if avg > 0 && mark >= 0 {
		priceLossPct := (avg - mark) / avg * 100
		if mark <= avg*(1.0-stopPct/100.0) {
			return true, "cur_vs_avg", priceLossPct, mark
		}
		if lossPct < priceLossPct {
			lossPct = priceLossPct
		}
	}
	return false, "", lossPct, mark
}

func (e *Engine) fetchOpenChainPositions(ctx context.Context) ([]polymarket.AccountPosition, error) {
	addr := e.redeemerSigner()
	if addr == "" {
		return nil, fmt.Errorf("no signer")
	}
	svc := polymarket.NewAccountServiceWithFallback(e.cfg.PolyFunderAddr, addr)
	return svc.GetAllPositions(ctx, addr)
}

func (e *Engine) findTradeForPosition(pos polymarket.AccountPosition) *models.Trade {
	var trades []models.Trade
	q := e.db.Where("condition_id = ?", pos.ConditionID)
	q = q.Where("status IN ?", []string{
		string(models.TradeStatusOpen),
		string(models.TradeStatusWon),
		string(models.TradeStatusLost),
	})
	if err := q.Order("created_at desc").Find(&trades).Error; err != nil || len(trades) == 0 {
		if pos.Asset != "" {
			var byToken models.Trade
			if err := e.db.Where("token_id = ?", pos.Asset).
				Order("created_at desc").First(&byToken).Error; err == nil {
				return &byToken
			}
		}
		return nil
	}
	for i := range trades {
		if outcomesMatch(trades[i].Outcome, pos.Outcome) {
			return &trades[i]
		}
	}
	return &trades[0]
}

// checkStopLossesFromDB is the legacy path when Data API is unavailable.
// Books lost only after a successful sell (or paper).
func (e *Engine) checkStopLossesFromDB(ctx context.Context, pct float64, marks *positionMarkIndex) int {
	threshold := 1.0 - pct/100.0
	var opens []models.Trade
	if err := e.db.Where("status = ?", models.TradeStatusOpen).Find(&opens).Error; err != nil {
		return 0
	}
	closed := 0
	for i := range opens {
		trade := &opens[i]
		if trade.Price <= 0 || trade.Shares <= 0 {
			continue
		}
		mark, source, err := e.markPriceForTrade(trade, marks)
		if err != nil || mark < 0 {
			continue
		}
		trigger := trade.Price * threshold
		if mark > trigger {
			continue
		}
		lossPct := (trade.Price - mark) / trade.Price * 100
		note := fmt.Sprintf("stop_loss: cur=%.3f entry=%.3f (-%.0f%%) via=%s", mark, trade.Price, lossPct, source)

		if e.cfg.PaperTrading || e.clobTrader == nil || !e.clobTrader.Ready() || trade.TokenID == "" {
			if !e.cfg.PaperTrading && (e.clobTrader == nil || !e.clobTrader.Ready()) {
				log.Printf("stop-loss: cannot sell %s — clob not ready (leave open)", trade.MarketTitle)
				continue
			}
		} else {
			var lastErr error
			sold := false
			for attempt := 1; attempt <= stopLossSellTries; attempt++ {
				sellCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				_, lastErr = e.clobTrader.PlaceSell(sellCtx, trade.TokenID, trade.Shares)
				cancel()
				if lastErr == nil {
					sold = true
					break
				}
				log.Printf("stop-loss sell try %d/%d failed %s: %v",
					attempt, stopLossSellTries, trade.MarketTitle, lastErr)
				if attempt < stopLossSellTries {
					time.Sleep(400 * time.Millisecond)
				}
			}
			if !sold {
				continue
			}
		}

		now := time.Now()
		trade.Status = models.TradeStatusLost
		trade.PnL = trade.Shares*mark - trade.Amount
		trade.ResolvedAt = &now
		trade.Side = "SELL"
		trade.OrderError = note
		if err := e.db.Save(trade).Error; err != nil {
			continue
		}
		closed++
	}
	return closed
}

type positionMarkIndex struct {
	byAsset     map[string]polymarket.AccountPosition
	byCondition map[string]polymarket.AccountPosition
}

func (idx *positionMarkIndex) lookup(trade *models.Trade) (polymarket.AccountPosition, bool) {
	if idx == nil {
		return polymarket.AccountPosition{}, false
	}
	if trade.TokenID != "" {
		if p, ok := idx.byAsset[trade.TokenID]; ok {
			return p, true
		}
	}
	if trade.ConditionID != "" {
		key := conditionOutcomeKey(trade.ConditionID, trade.Outcome)
		if p, ok := idx.byCondition[key]; ok {
			return p, true
		}
	}
	return polymarket.AccountPosition{}, false
}

func conditionOutcomeKey(conditionID, outcome string) string {
	return strings.ToLower(strings.TrimSpace(conditionID)) + "|" + normalizeOutcome(outcome)
}

func (e *Engine) markPriceForTrade(trade *models.Trade, marks *positionMarkIndex) (mark float64, source string, err error) {
	if pos, ok := marks.lookup(trade); ok && pos.CurPrice >= 0 {
		return pos.CurPrice, "data_api_cur_price", nil
	}
	snap, _, ok := e.lookupTradeMarket(trade)
	if ok {
		price, _ := findOutcomePrice(snap, trade.Outcome)
		if price >= 0 {
			return price, "gamma_outcome", nil
		}
	}
	return 0, "", fmt.Errorf("no cur_price for market=%s outcome=%s", trade.MarketID, trade.Outcome)
}
