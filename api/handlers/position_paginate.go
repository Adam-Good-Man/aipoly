package handlers

import "aipoly/api/services/polymarket"

func paginateAccountPositions(items []polymarket.AccountPosition, offset, limit int) []polymarket.AccountPosition {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset >= len(items) {
		return []polymarket.AccountPosition{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	out := make([]polymarket.AccountPosition, end-offset)
	copy(out, items[offset:end])
	return out
}
