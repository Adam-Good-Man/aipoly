package trader

import "aipoly/api/services/polymarket"

const (
	screenBatchSize      = 30 // light-AI batch size per screen round
	screenShortlistSize  = 3  // deep-analyze up to 3
	finalBetsPerCycle    = 2  // after deep analysis of 3, place top 0–2 that clear min win rate
	maxCryptoDirPerBatch = 4  // keep UP/Down from dominating each screen round
	maxScreenRounds      = 5  // hard cap on light-AI rounds per cycle
)

// BuildScreenBatch picks the next batch of markets for light AI screening.
// Previously skipped/shown IDs are never re-included. Primary (non UP/Down crypto)
// markets fill first; crypto direction markets get at most maxCryptoDir slots.
func BuildScreenBatch(pool []polymarket.MarketSnapshot, skip map[string]bool, size, maxCryptoDir int) []polymarket.MarketSnapshot {
	if size <= 0 {
		size = screenBatchSize
	}
	if maxCryptoDir < 0 {
		maxCryptoDir = 0
	}
	if maxCryptoDir > size {
		maxCryptoDir = size
	}

	primary := make([]polymarket.MarketSnapshot, 0, size)
	crypto := make([]polymarket.MarketSnapshot, 0, maxCryptoDir)
	for _, s := range pool {
		if s.MarketID == "" || (skip != nil && skip[s.MarketID]) {
			continue
		}
		if IsCryptoDirectionMarket(s) {
			crypto = append(crypto, s)
			continue
		}
		primary = append(primary, s)
	}

	out := make([]polymarket.MarketSnapshot, 0, size)
	// Reserve up to maxCryptoDir slots, fill the rest with primary first.
	primaryTarget := size - maxCryptoDir
	for _, s := range primary {
		if len(out) >= primaryTarget {
			break
		}
		out = append(out, s)
	}
	cryptoAdded := 0
	for _, s := range crypto {
		if len(out) >= size || cryptoAdded >= maxCryptoDir {
			break
		}
		out = append(out, s)
		cryptoAdded++
	}
	// Fill remaining with more primary.
	if len(out) < size {
		seen := make(map[string]bool, len(out))
		for _, s := range out {
			seen[s.MarketID] = true
		}
		for _, s := range primary {
			if len(out) >= size {
				break
			}
			if seen[s.MarketID] {
				continue
			}
			out = append(out, s)
			seen[s.MarketID] = true
		}
	}
	return out
}
