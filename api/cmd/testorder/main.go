package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"aipoly/api/config"
	"aipoly/api/services/polymarket"
)

func main() {
	cfg, _ := config.Load()
	trader := polymarket.NewClobTrader(cfg)
	if !trader.Ready() {
		log.Fatalf("CLOB not ready: %v", trader.InitError())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	balStr, _ := trader.GetBalance(ctx)
	fmt.Printf("余额: %s\n", balStr)

	poly := polymarket.NewClient()
	events, _ := poly.FetchTrendingEvents(10)
	snaps := poly.BuildSnapshots(events)
	tokenID := ""
	for _, s := range snaps {
		if len(s.TokenIDs) > 0 && s.TokenIDs[0] != "" {
			tokenID = s.TokenIDs[0]
			fmt.Printf("市场: %s\n", s.Question)
			break
		}
	}

	amounts := []float64{0.10, 0.50, 1.00}
	for _, amount := range amounts {
		fmt.Printf("\n试 $%.2f: ", amount)
		result, err := trader.PlaceBuy(ctx, tokenID, amount)
		if err != nil {
			fmt.Println(err)
			if strings.Contains(err.Error(), "deposit wallet") {
				os.Exit(1)
			}
			continue
		}
		fmt.Printf("成功 order_id=%s status=%s\n", result.OrderID, result.Status)
		return
	}
}
