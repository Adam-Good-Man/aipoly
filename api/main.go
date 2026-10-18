package main

import (
	"log"

	"aipoly/api/config"
	"aipoly/api/database"
	"aipoly/api/handlers"
	"aipoly/api/services/trader"
	"aipoly/api/services/wallet"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	engine := trader.NewEngine(db, cfg)
	engine.WarmClob()
	engine.StartResolver()
	engine.StartStopLossWatcher()
	engine.Start()

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	walletSvc := wallet.NewService(cfg)

	h := handlers.New(db, engine, walletSvc, cfg)
	h.Register(r)

	log.Printf("aiPoly API running on 127.0.0.1:%s (paper_trading=%v)", cfg.APIPort, cfg.PaperTrading)
	if err := r.Run("127.0.0.1:" + cfg.APIPort); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
