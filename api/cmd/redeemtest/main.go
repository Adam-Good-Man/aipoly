package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"aipoly/api/config"
	"aipoly/api/services/polymarket"
	"aipoly/api/services/trader"
	"aipoly/api/database"
)

func main() {
	cfg, _ := config.Load()
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	engine := trader.NewEngine(db, cfg)
	time.Sleep(2 * time.Second)

	fmt.Println("redeemer ready:", engine.RedeemerReady())
	redeemer := polymarket.NewPositionRedeemer(cfg)
	fmt.Println("init err:", redeemer.InitError())

	svc := polymarket.NewAccountServiceWithFallback(cfg.PolyFunderAddr, "0x99574D6A68D22AA2FaA1250AB6a07Bd43c056bA8")
	win, err := svc.WinningPositionsForRedeem(context.Background(), "0x99574D6A68D22AA2FaA1250AB6a07Bd43c056bA8", 100)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("winning to redeem: %d\n", len(win))
	for _, p := range win {
		fmt.Printf("  %s | %s | %s | negRisk=%v\n", p.Title, p.Outcome, p.SettlementStatus, p.NegativeRisk)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	results := engine.RedeemSettledPositions(ctx)
	fmt.Printf("redeem results: %d\n", len(results))
	for _, r := range results {
		fmt.Printf("  %s | tx=%s err=%s\n", r.Title, r.TransactionID, r.Error)
	}
	fmt.Println("clob balance:", engine.ClobBalance())
	os.Exit(0)
}
