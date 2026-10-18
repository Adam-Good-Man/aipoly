package trader

import (
	"context"
	"log"
	"strings"
	"time"

	"aipoly/api/services/polymarket"
)

// RedeemSettledPositions redeems all winning settled on-chain positions for the bot wallet.
func (e *Engine) RedeemSettledPositions(ctx context.Context) []polymarket.RedeemOutcome {
	if e.cfg.PaperTrading || e.redeemer == nil || !e.redeemer.Ready() {
		if e.redeemer != nil && e.redeemer.InitError() != nil {
			log.Printf("redeemer not ready: %v", e.redeemer.InitError())
		}
		return nil
	}

	svc := polymarket.NewAccountServiceWithFallback(e.cfg.PolyFunderAddr, e.redeemerSigner())
	positions, err := svc.WinningPositionsForRedeem(ctx, e.redeemerSigner(), 100)
	if err != nil {
		log.Printf("fetch winning positions for redeem: %v", err)
		return nil
	}
	if len(positions) == 0 {
		return nil
	}

	log.Printf("auto-redeem: found %d winning position(s) to redeem", len(positions))

	results := make([]polymarket.RedeemOutcome, 0, len(positions))
	seen := make(map[string]struct{})
	okCount := 0
	for _, pos := range positions {
		if pos.ConditionID == "" {
			continue
		}
		key := pos.ConditionID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		if ctx.Err() != nil {
			break
		}

		out := e.redeemer.Redeem(ctx, pos.ConditionID, pos.NegativeRisk, polymarket.RedeemOutcome{
			Title:   pos.Title,
			Outcome: pos.Outcome,
		})
		results = append(results, out)
		if out.Error == "" {
			okCount++
		}
	}

	if okCount > 0 {
		// Relayer submission is async; give Polygon a moment before balance refresh.
		time.Sleep(3 * time.Second)
		e.SyncLiveBalance()
		log.Printf("auto-redeem: submitted %d/%d, balance synced", okCount, len(results))
	} else if len(results) > 0 {
		errs := make([]string, 0, 3)
		for _, r := range results {
			if r.Error != "" {
				errs = append(errs, r.Error)
				if len(errs) >= 2 {
					break
				}
			}
		}
		if len(errs) > 0 {
			log.Printf("auto-redeem failed: %s", strings.Join(errs, "; "))
		}
	}
	return results
}

func (e *Engine) redeemerSigner() string {
	if e.clobTrader == nil {
		return ""
	}
	return e.clobTrader.SignerAddress()
}

func (e *Engine) RedeemerReady() bool {
	return e.redeemer != nil && e.redeemer.Ready()
}
