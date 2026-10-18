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
	"aipoly/api/services/wallet"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	fmt.Println("=== Polymarket 最小额真实下单测试 ===")
	fmt.Printf("host=%s sig_type=%d funder=%s\n", cfg.ClobHost, cfg.SignatureType, cfg.PolyFunderAddr)
	fmt.Printf("proxy=%s\n\n", os.Getenv("HTTP_PROXY"))

	signer, _ := wallet.AddressFromPrivateKey(cfg.PolyPrivateKey)
	fmt.Printf("signer=%s\n", signer)

	trader := polymarket.NewClobTrader(cfg)
	if !trader.Ready() {
		log.Fatalf("CLOB not ready: %v", trader.InitError())
	}
	fmt.Printf("funder=%s\n", trader.FunderAddress())

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	balStr, err := trader.GetBalance(ctx)
	if err != nil {
		log.Fatalf("balance: %v", err)
	}
	balance := trader.BalanceUSDC()
	fmt.Printf("CLOB 余额: %s ($%.4f)\n\n", balStr, balance)
	if balance < 0.05 {
		fmt.Println("⚠️  USDC 现金余额不足，仍尝试 $0.10 测试下单（验证 API 权限）...\n")
		balance = 0.10
	}

	poly := polymarket.NewClient()
	events, err := poly.FetchTrendingEvents(20)
	if err != nil {
		log.Fatalf("fetch events: %v", err)
	}
	snaps := poly.BuildSnapshots(events)

	// Pick a high-probability cheap outcome (price >= 0.90) to minimize cost
	var tokenID, marketName, outcome string
	var price float64
	for _, s := range snaps {
		for i, o := range s.Outcomes {
			p := 0.0
			if i < len(s.Prices) {
				p = s.Prices[i]
			}
			tid := ""
			if i < len(s.TokenIDs) {
				tid = s.TokenIDs[i]
			}
			if tid == "" || p < 0.85 || p > 0.99 {
				continue
			}
			if tokenID == "" || p > price {
				tokenID = tid
				marketName = s.Question
				outcome = o
				price = p
			}
		}
	}
	if tokenID == "" {
		for _, s := range snaps {
			if len(s.TokenIDs) > 0 && s.TokenIDs[0] != "" {
				tokenID = s.TokenIDs[0]
				marketName = s.Question
				if len(s.Outcomes) > 0 {
					outcome = s.Outcomes[0]
				}
				if len(s.Prices) > 0 {
					price = s.Prices[0]
				}
				break
			}
		}
	}
	if tokenID == "" {
		log.Fatal("no market found")
	}
	fmt.Printf("测试市场: %s\n", marketName)
	fmt.Printf("方向: %s @ %.3f\n\n", outcome, price)

	amounts := []float64{1.00, 1.50, 2.00, 5.00}
	for _, amount := range amounts {
		if amount > balance {
			continue
		}
		fmt.Printf("尝试下单 $%.2f ... ", amount)
		result, err := trader.PlaceBuy(ctx, tokenID, amount)
		if err != nil {
			fmt.Printf("失败: %v\n", err)
			if strings.Contains(err.Error(), "deposit wallet") {
				fmt.Println("\n仍需要 deposit wallet 流程，请联系 Polymarket 或尝试 signature type 3")
				os.Exit(1)
			}
			continue
		}
		fmt.Printf("成功!\n")
		fmt.Printf("   order_id: %s\n", result.OrderID)
		fmt.Printf("   status:   %s\n", result.Status)
		balStr, _ = trader.GetBalance(ctx)
		fmt.Printf("   剩余余额: %s\n", balStr)
		return
	}

	fmt.Println("\n❌ 所有金额均失败，请查看上方错误信息")
	os.Exit(1)
}
