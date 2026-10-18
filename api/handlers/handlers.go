package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"aipoly/api/config"
	"aipoly/api/database"
	"aipoly/api/models"
	"aipoly/api/services/trader"
	"aipoly/api/services/wallet"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db     *gorm.DB
	engine *trader.Engine
	wallet *wallet.Service
	cfg    *config.Config
}

func New(db *gorm.DB, engine *trader.Engine, walletSvc *wallet.Service, cfg *config.Config) *Handler {
	return &Handler{db: db, engine: engine, wallet: walletSvc, cfg: cfg}
}

func (h *Handler) Register(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/dashboard", h.GetDashboard)
		api.GET("/portfolio", h.GetPortfolio)
		api.GET("/trades", h.ListTrades)
		api.GET("/trades/:id", h.GetTrade)
		api.GET("/events", h.ListEvents)
		api.GET("/events/:id", h.GetEvent)
		api.GET("/analysis", h.ListAnalysis)
		api.GET("/wallet", h.GetWallet)
		api.POST("/wallet/generate", h.GenerateWallet)
		api.GET("/account/profile", h.GetAccountProfile)
		api.GET("/account/activity", h.GetAccountActivity)
		api.GET("/account/positions", h.GetAccountPositions)
		api.POST("/trades/:id/resolve", h.ResolveTrade)
		api.POST("/trading/run", h.TriggerTrading)
	}
}

func (h *Handler) GetDashboard(c *gin.Context) {
	if !h.cfg.PaperTrading {
		h.refreshLiveClobBalance()
	}
	_ = database.UpdatePortfolioStats(h.db, !h.cfg.PaperTrading)

	portfolio, err := database.GetPortfolio(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tradeScope := h.db.Model(&models.Trade{})
	if !h.cfg.PaperTrading {
		tradeScope = tradeScope.Where("is_paper = ?", false)
	}

	var openTrades int64
	tradeScope.Where("status = ?", models.TradeStatusOpen).Count(&openTrades)

	recentQuery := h.db.Where("status <> ?", models.TradeStatusCanceled).
		Order("created_at DESC").Limit(5)
	if !h.cfg.PaperTrading {
		recentQuery = recentQuery.Where("is_paper = ?", false)
	}
	var recentTrades []models.Trade
	recentQuery.Find(&recentTrades)

	var latestAnalysis *models.AnalysisLog
	var la models.AnalysisLog
	if err := h.db.Order("created_at DESC").First(&la).Error; err == nil && la.Summary != "" {
		latestAnalysis = &la
	}

	settled := portfolio.SettledCount()
	resp := gin.H{
		"portfolio": gin.H{
			"balance":         portfolio.Balance,
			"initial_balance": portfolio.InitialBalance,
			"total_profit":    portfolio.TotalProfit,
			"total_trades":    portfolio.TotalTrades,
			"won_trades":      portfolio.WonTrades,
			"lost_trades":     portfolio.LostTrades,
			"settled_count":   settled,
			"pending_count":   int(openTrades),
			"profit_count":    portfolio.WonTrades,
			"loss_count":      portfolio.LostTrades,
			"win_rate":        portfolio.WinRate(),
			"win_rate_known":  settled > 0,
		},
		"open_trades":   openTrades,
		"recent_trades": recentTrades,
		"clob_ready":    h.engine.ClobReady(),
		"clob_error":    h.engine.ClobError(),
		"clob_balance":  h.engine.ClobBalance(),
		"paper_trading": h.cfg.PaperTrading,
		"trading": gin.H{
			"interval_minutes":      h.engine.TradingIntervalMins(),
			"max_market_days":       h.cfg.MaxMarketDays,
			"min_win_rate":          h.cfg.MinWinRate,
			"min_cash_reserve_pct":  h.cfg.MinCashReservePercent,
			"max_trades_per_cycle":  h.cfg.MaxTradesPerCycle,
			"loss_cooldown_hours":   h.cfg.LossCooldownHours,
			"stop_loss_percent":     h.cfg.StopLossPercent,
			"max_add_fraction":      h.cfg.MaxAddFraction,
			"max_adds_per_market":   h.cfg.MaxAddsPerMarket,
			"last_cycle":             h.engine.LastCycleStatus(),
			"ai_paused":              h.engine.LastCycleStatus().AIPaused,
			"min_deployable_to_scan": trader.MinDeployableToScan(!h.cfg.PaperTrading),
			"min_edge_percent":       h.cfg.MinEdgePercent,
			"geoblock_blocked":       h.engine.LastCycleStatus().GeoblockBlocked,
			"geoblock_detail":        h.engine.LastCycleStatus().GeoblockDetail,
		},
	}
	if latestAnalysis != nil {
		resp["latest_analysis"] = latestAnalysis
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetPortfolio(c *gin.Context) {
	portfolio, err := database.GetPortfolio(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"portfolio": portfolio,
		"win_rate":  portfolio.WinRate(),
	})
}

func (h *Handler) ListTrades(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	query := h.db.Model(&models.Trade{}).Where("status <> ?", models.TradeStatusCanceled)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if !h.cfg.PaperTrading || c.Query("live_only") == "true" {
		query = query.Where("is_paper = ?", false)
	}

	var total int64
	query.Count(&total)

	var trades []models.Trade
	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&trades)

	c.JSON(http.StatusOK, gin.H{
		"data":  trades,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *Handler) GetTrade(c *gin.Context) {
	var trade models.Trade
	if err := h.db.Where("id = ?", c.Param("id")).First(&trade).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trade not found"})
		return
	}
	c.JSON(http.StatusOK, trade)
}

type resolveRequest struct {
	Status string  `json:"status" binding:"required"`
	PnL    float64 `json:"pnl"`
}

func (h *Handler) ResolveTrade(c *gin.Context) {
	var req resolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var trade models.Trade
	if err := h.db.Where("id = ?", c.Param("id")).First(&trade).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trade not found"})
		return
	}

	switch req.Status {
	case "won":
		trade.Status = models.TradeStatusWon
		trade.PnL = req.PnL
	case "lost":
		trade.Status = models.TradeStatusLost
		trade.PnL = req.PnL
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be won or lost"})
		return
	}

	now := time.Now()
	trade.ResolvedAt = &now
	if err := h.db.Save(&trade).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_ = database.UpdatePortfolioStats(h.db, !h.cfg.PaperTrading)
	c.JSON(http.StatusOK, trade)
}

func (h *Handler) ListAnalysis(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	var logs []models.AnalysisLog
	h.db.Order("created_at DESC").Limit(limit).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func (h *Handler) TriggerTrading(c *gin.Context) {
	maxTrades, _ := strconv.Atoi(c.DefaultQuery("max_trades", "0"))
	go func() {
		if maxTrades > 0 {
			h.engine.RunCycleMax(maxTrades)
		} else {
			h.engine.RunCycle()
		}
	}()
	msg := "trading cycle triggered"
	if maxTrades > 0 {
		msg = fmt.Sprintf("trading cycle triggered (max %d trade(s))", maxTrades)
	}
	c.JSON(http.StatusOK, gin.H{"message": msg, "max_trades": maxTrades})
}
