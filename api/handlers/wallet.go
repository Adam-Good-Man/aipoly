package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetWallet(c *gin.Context) {
	info, err := h.wallet.GetInfo()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

func (h *Handler) GenerateWallet(c *gin.Context) {
	if h.cfg.PolyPrivateKey != "" {
		c.JSON(http.StatusConflict, gin.H{
			"error": "钱包已配置。如需重新生成，请先清空 .env 中的 POLYMARKET_PRIVATE_KEY 并重启 API。",
		})
		return
	}

	w, err := h.wallet.Generate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, w)
}
