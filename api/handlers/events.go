package handlers

import (
	"net/http"
	"strconv"
	"time"

	"aipoly/api/services/polymarket"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	category := c.Query("category")
	fastOnly := c.Query("fast") == "true"
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	now := time.Now()
	var (
		events []polymarket.GammaEvent
		total  int
		err    error
	)

	categories := []map[string]any{}
	useDirectPage := category == "" && !fastOnly

	if useDirectPage {
		offset := (page - 1) * limit
		fetchN := limit + 1
		events, err = h.engine.FetchLiveEventsPaged(fetchN, offset, false)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		hasMore := len(events) > limit
		if hasMore {
			events = events[:limit]
		}
		total = offset + len(events)
		if hasMore {
			total++
		}

		if page == 1 {
			batch, batchErr := h.engine.FetchLiveEventsPaged(polymarket.EventsFetchCap(), 0, false)
			if batchErr == nil {
				categories = polymarket.BuildCategoryCounts(batch)
			}
		}
	} else {
		batch, err := h.engine.FetchLiveEventsPaged(polymarket.EventsFetchCap(), 0, fastOnly)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		categories = polymarket.BuildCategoryCounts(batch)
		filtered := polymarket.FilterEventsByCategory(batch, category)
		events, total = polymarket.PaginateEvents(filtered, page, limit)
	}

	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, polymarket.EventListItem(e, now))
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       out,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"categories": categories,
		"live":       true,
	})
}

func (h *Handler) GetEvent(c *gin.Context) {
	id := c.Param("id")
	event, err := h.engine.FetchEventByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": polymarket.EventDetailItem(*event, time.Now()),
		"live": true,
	})
}
