package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aipoly/api/services/polymarket"
	"aipoly/api/services/wallet"

	"github.com/gin-gonic/gin"
)

func (h *Handler) accountService() *polymarket.AccountService {
	botSigner := ""
	if h.cfg.PolyPrivateKey != "" {
		if addr, err := wallet.AddressFromPrivateKey(h.cfg.PolyPrivateKey); err == nil {
			botSigner = addr
		}
	}
	return polymarket.NewAccountServiceWithFallback(h.cfg.PolyFunderAddr, botSigner)
}

func parseAddress(c *gin.Context) (string, bool) {
	addr := strings.TrimSpace(c.Query("address"))
	if addr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address query parameter required"})
		return "", false
	}
	return addr, true
}

func (h *Handler) refreshLiveClobBalance() {
	if h.cfg.PaperTrading || !h.engine.ClobReady() {
		return
	}
	h.engine.SyncLiveBalance()
}

func (h *Handler) GetAccountProfile(c *gin.Context) {
	addr, ok := parseAddress(c)
	if !ok {
		return
	}

	syncNow := c.Query("sync") == "1" || c.Query("sync") == "true"

	svc := h.accountService()
	profile, err := svc.GetProfile(c.Request.Context(), addr)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{
		"profile": profile,
	}
	if syncNow {
		recent := h.engine.RecentlyResolved(10 * time.Minute)
		resp["recently_resolved"] = recent
		resp["resolved_count"] = len(recent)
		if len(recent) > 0 {
			resp["settlement_notice"] = true
		}
	}
	if botSigner, err := wallet.AddressFromPrivateKey(h.cfg.PolyPrivateKey); err == nil && botSigner != "" {
		resp["bot_signer"] = botSigner
		resp["matches_bot"] = strings.EqualFold(addr, botSigner)
		if matches, _ := resp["matches_bot"].(bool); matches && !h.cfg.PaperTrading {
			h.refreshLiveClobBalance()
			resp["clob_balance"] = h.engine.ClobBalance()
			resp["clob_ready"] = h.engine.ClobReady()
		}
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAccountActivity(c *gin.Context) {
	addr, ok := parseAddress(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	svc := h.accountService()
	items, err := svc.GetActivity(c.Request.Context(), addr, limit, offset)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	total, _ := svc.CountTradeActivity(c.Request.Context(), addr)

	// Attach AI reasoning from local DB trades
	items = h.attachAIReasoning(items)

	c.JSON(http.StatusOK, gin.H{
		"data":   items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"source": "polymarket",
	})
}

func (h *Handler) GetAccountPositions(c *gin.Context) {
	addr, ok := parseAddress(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	syncNow := c.Query("sync") == "1" || c.Query("sync") == "true"

	var resolvedCount int
	var redeemed []polymarket.RedeemOutcome

	if syncNow && !h.cfg.PaperTrading {
		resolvedCount, redeemed = h.engine.SettleAndSync(c.Request.Context())
	}

	svc := h.accountService()

	// Full history for overview stats.
	allOpen, err := svc.GetAllPositions(c.Request.Context(), addr)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	allOpen, _ = h.enrichPositions(c.Request.Context(), addr, allOpen)

	closed, closedErr := svc.GetClosedPositions(c.Request.Context(), addr, 0)
	if closedErr != nil {
		log.Printf("fetch closed positions: %v", closedErr)
		closed = nil
	}

	tradeCount, tradeErr := svc.CountTradeActivity(c.Request.Context(), addr)
	if tradeErr != nil {
		log.Printf("count trade activity: %v", tradeErr)
	}

	perf := polymarket.ComputePortfolioPerformance(allOpen, closed)
	perf.TradeCount = tradeCount
	perf.OpenFetched = len(allOpen)
	perf.ClosedFetched = len(closed)
	// total_count stays as unique positions (open+closed) so
	// total_count == open_count + won_count + lost_count.
	// trade_count is fill history and may be higher (buy+sell, multi-fills).

	// Paginate open positions for the table only.
	pageItems := paginateAccountPositions(allOpen, offset, limit)

	resp := gin.H{
		"data":           pageItems,
		"total_open":     len(allOpen),
		"source":         "polymarket",
		"performance":    perf,
		"activity_count": tradeCount,
		"limit":          limit,
		"offset":         offset,
	}
	if syncNow {
		resp["resolved_count"] = resolvedCount
		if len(redeemed) > 0 {
			resp["redeemed"] = redeemed
			resp["redeemed_count"] = len(redeemed)
		}
		if botSigner, err := wallet.AddressFromPrivateKey(h.cfg.PolyPrivateKey); err == nil &&
			strings.EqualFold(addr, botSigner) && !h.cfg.PaperTrading {
			h.refreshLiveClobBalance()
			resp["clob_balance"] = h.engine.ClobBalance()
			resp["clob_ready"] = h.engine.ClobReady()
		}
	}
	c.JSON(http.StatusOK, resp)
}
