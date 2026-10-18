package trader

import (
	"context"
	"log"
	"sync"
	"time"

	"aipoly/api/services/polymarket"
)

var settleGate sync.Mutex

// SettleAndSync runs stop-loss, resolves DB trades, redeems winners, and refreshes CLOB balance.
// If another settle is already running, returns immediately without blocking the HTTP request.
func (e *Engine) SettleAndSync(ctx context.Context) (resolved int, redeemed []polymarket.RedeemOutcome) {
	if !e.tryBeginSettle() {
		log.Println("settle sync: skipped (already in progress)")
		return 0, nil
	}
	defer e.endSettle()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	stopped := e.CheckStopLosses(ctx)

	if e.cfg.PaperTrading {
		if stopped > 0 {
			log.Printf("settle sync: paper stop-loss closed=%d", stopped)
		}
		return stopped, nil
	}

	resolved = e.ResolveOpenTradesNow()
	redeemed = e.RedeemSettledPositions(ctx)
	okRedeems := 0
	for _, r := range redeemed {
		if r.Error == "" {
			okRedeems++
		}
	}
	if resolved > 0 || len(redeemed) > 0 || stopped > 0 {
		if okRedeems == 0 {
			e.SyncLiveBalance()
		}
		log.Printf("settle sync: stop_loss=%d resolved=%d redeemed_ok=%d redeemed_attempts=%d",
			stopped, resolved, okRedeems, len(redeemed))
	}
	return resolved + stopped, redeemed
}

func (e *Engine) tryBeginSettle() bool {
	settleGate.Lock()
	defer settleGate.Unlock()
	if e.settleBusy {
		return false
	}
	e.settleBusy = true
	return true
}

func (e *Engine) endSettle() {
	settleGate.Lock()
	e.settleBusy = false
	settleGate.Unlock()
}
