package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	OpenAIKey     string
	OpenAIModel   string
	OpenAIBaseURL string

	PolyPrivateKey           string
	PolyFunderAddr           string
	PolyAPIKey               string
	PolyAPISecret            string
	PolyAPIPassphrase        string
	PolyBuilderAPIKey        string
	PolyBuilderAPISecret     string
	PolyBuilderAPIPassphrase string
	PaperTrading             bool
	PolygonRPCURL            string
	SignatureType            int
	ClobHost                 string

	InitialBalance        float64
	TradingIntervalMins   int
	MaxBetPercent         float64
	MaxMarketDays         int     // only bet markets resolving within N days
	MinCashReservePercent float64 // keep this % undeployed
	MinWinRate            float64 // minimum self_win_rate to place bet
	MinEdgePercent        float64 // self_win_rate must exceed implied prob by this
	MaxTradesPerCycle     int
	LossCooldownHours     int     // skip markets lost within this window
	MaxDailyLossPercent   float64 // pause new bets after daily realized loss %
	StopLossPercent       float64 // sell open position if mark falls this % below entry
	MaxAddFraction        float64 // max add size as fraction of existing open cost
	MaxAddsPerMarket      int     // max extra buys on top of the original open trade
	ResolverIntervalMins  int
	APIPort               string

	// RSSHub news for direction judgment
	RSSHubBaseURL string
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")

	paperTrading := os.Getenv("POLYMARKET_PAPER_TRADING") != "false"
	initialBalance, _ := strconv.ParseFloat(getEnv("INITIAL_BALANCE", "1000"), 64)
	intervalMins, _ := strconv.Atoi(getEnv("TRADING_INTERVAL_MINUTES", "30"))
	maxBetPct, _ := strconv.ParseFloat(getEnv("MAX_BET_PERCENT", "8"), 64)
	maxMarketDays, _ := strconv.Atoi(getEnv("MAX_MARKET_DAYS", "2"))
	minReservePct, _ := strconv.ParseFloat(getEnv("MIN_CASH_RESERVE_PERCENT", "0"), 64)
	minWinRate, _ := strconv.ParseFloat(getEnv("MIN_WIN_RATE", "68"), 64)
	// Soft edge vs market: enough to skip pure "priced-in" / negative-EV, not so high that cycles go empty overnight.
	minEdgePct, _ := strconv.ParseFloat(getEnv("MIN_EDGE_PERCENT", "2"), 64)
	maxTradesCycle, _ := strconv.Atoi(getEnv("MAX_TRADES_PER_CYCLE", "1"))
	lossCooldownHours, _ := strconv.Atoi(getEnv("LOSS_COOLDOWN_HOURS", "24"))
	maxDailyLossPct, _ := strconv.ParseFloat(getEnv("MAX_DAILY_LOSS_PERCENT", "12"), 64)
	stopLossPct, _ := strconv.ParseFloat(getEnv("STOP_LOSS_PERCENT", "30"), 64)
	maxAddFrac, _ := strconv.ParseFloat(getEnv("MAX_ADD_FRACTION", "0.30"), 64)
	maxAddsPerMarket, _ := strconv.Atoi(getEnv("MAX_ADDS_PER_MARKET", "1"))
	resolverMins, _ := strconv.Atoi(getEnv("RESOLVER_INTERVAL_MINUTES", "3"))
	sigType, _ := strconv.Atoi(getEnv("POLYMARKET_SIGNATURE_TYPE", "0"))

	return &Config{
		DBHost:                   getEnv("DB_HOST", "localhost"),
		DBPort:                   getEnv("DB_PORT", "5432"),
		DBUser:                   getEnv("DB_USER", "postgres"),
		DBPassword:               getEnv("DB_PASSWORD", ""),
		DBName:                   getEnv("DB_NAME", "aipoly"),
		OpenAIKey:                firstEnv("OPENAI_API_KEY", "MOONSHOT_API_KEY"),
		OpenAIModel:              getEnv("OPENAI_MODEL", "kimi-k3"),
		OpenAIBaseURL:            getEnv("OPENAI_BASE_URL", "https://api.moonshot.cn/v1"),
		PolyPrivateKey:           os.Getenv("POLYMARKET_PRIVATE_KEY"),
		PolyFunderAddr:           os.Getenv("POLYMARKET_FUNDER_ADDRESS"),
		PolyAPIKey:               os.Getenv("POLYMARKET_API_KEY"),
		PolyAPISecret:            os.Getenv("POLYMARKET_API_SECRET"),
		PolyAPIPassphrase:        os.Getenv("POLYMARKET_API_PASSPHRASE"),
		PolyBuilderAPIKey:        os.Getenv("POLYMARKET_BUILDER_API_KEY"),
		PolyBuilderAPISecret:     os.Getenv("POLYMARKET_BUILDER_API_SECRET"),
		PolyBuilderAPIPassphrase: os.Getenv("POLYMARKET_BUILDER_API_PASSPHRASE"),
		PaperTrading:             paperTrading,
		PolygonRPCURL:            os.Getenv("POLYGON_RPC_URL"),
		SignatureType:            sigType,
		ClobHost:                 getEnv("POLYMARKET_CLOB_HOST", "https://clob.polymarket.com"),
		InitialBalance:           initialBalance,
		TradingIntervalMins:      intervalMins,
		MaxBetPercent:            maxBetPct,
		MaxMarketDays:            maxMarketDays,
		MinCashReservePercent:    minReservePct,
		MinWinRate:               minWinRate,
		MinEdgePercent:           minEdgePct,
		MaxTradesPerCycle:        maxTradesCycle,
		LossCooldownHours:        lossCooldownHours,
		MaxDailyLossPercent:      maxDailyLossPct,
		StopLossPercent:          stopLossPct,
		MaxAddFraction:           maxAddFrac,
		MaxAddsPerMarket:         maxAddsPerMarket,
		ResolverIntervalMins:     resolverMins,
		APIPort:                  getEnv("API_PORT", "8081"),
		RSSHubBaseURL:            getEnv("RSSHUB_BASE_URL", "https://rsshub.app"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
