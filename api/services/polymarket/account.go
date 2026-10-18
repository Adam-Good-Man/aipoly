package polymarket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aipoly/api/services/wallet"

	"github.com/ethereum/go-ethereum/common"
)

const dataAPIBase = "https://data-api.polymarket.com"

// maxHistoryRows is the safety cap when paginating full account history from Polymarket.
const maxHistoryRows = 100000

const positionsPageSize = 500
const closedPositionsPageSize = 50
const activityPageSize = 500

type AccountService struct {
	httpClient    *http.Client
	proxyFallback string
	botSigner     string
}

func NewAccountService() *AccountService {
	return NewAccountServiceWithFallback("", "")
}

func NewAccountServiceWithFallback(proxyFallback, botSigner string) *AccountService {
	return &AccountService{
		proxyFallback: strings.TrimSpace(proxyFallback),
		botSigner:     strings.ToLower(strings.TrimSpace(botSigner)),
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				// Avoid stale keep-alive connections through local proxies (EOF on reuse).
				DisableKeepAlives: true,
			},
		},
	}
}

type AccountProfile struct {
	EOAAddress           string  `json:"eoa_address"`
	ProxyWallet          string  `json:"proxy_wallet"`
	DisplayName          string  `json:"display_name,omitempty"`
	Pseudonym            string  `json:"pseudonym,omitempty"`
	PortfolioValue       float64 `json:"portfolio_value"`
	OpenPositions        int     `json:"open_positions"`
	PolymarketRegistered bool    `json:"polymarket_registered"`
	HasProxyWallet       bool    `json:"has_proxy_wallet"`
}

type AccountActivityItem struct {
	Type            string  `json:"type"`
	Side            string  `json:"side"`
	Title           string  `json:"title"`
	Outcome         string  `json:"outcome"`
	Size            float64 `json:"size"`
	USDCSize        float64 `json:"usdc_size"`
	Price           float64 `json:"price"`
	Timestamp       int64   `json:"timestamp"`
	TransactionHash string  `json:"transaction_hash"`
	ConditionID     string  `json:"condition_id,omitempty"`
	Asset           string  `json:"asset,omitempty"`
	Reasoning       string  `json:"reasoning,omitempty"`
	MarketReason    string  `json:"market_reason,omitempty"`
	OutcomeReason   string  `json:"outcome_reason,omitempty"`
	IsAITrade       bool    `json:"is_ai_trade"`
	SelfWinRate     float64 `json:"self_win_rate,omitempty"`
}

type AccountPosition struct {
	Title            string  `json:"title"`
	Outcome          string  `json:"outcome"`
	Size             float64 `json:"size"`
	AvgPrice         float64 `json:"avg_price"`
	CurPrice         float64 `json:"cur_price"`
	CurrentValue     float64 `json:"current_value"`
	CashPNL          float64 `json:"cash_pnl"`
	PercentPNL       float64 `json:"percent_pnl"`
	ConditionID      string  `json:"condition_id,omitempty"`
	Asset            string  `json:"asset,omitempty"` // CLOB token id
	Redeemable       bool    `json:"redeemable"`
	Mergeable        bool    `json:"mergeable"`
	NegativeRisk     bool    `json:"negative_risk"`
	EndDate          string  `json:"end_date,omitempty"`
	SettlementStatus string  `json:"settlement_status"`
	Result           string  `json:"result"`       // profit | loss | breakeven | unknown
	ResultKnown      bool    `json:"result_known"` // false until market settled
	EntryAmount      float64 `json:"entry_amount,omitempty"`
	EntryTimestamp   int64   `json:"entry_timestamp,omitempty"`
	Reasoning        string  `json:"reasoning,omitempty"`
	MarketReason     string  `json:"market_reason,omitempty"`
	OutcomeReason    string  `json:"outcome_reason,omitempty"`
	IsAITrade        bool    `json:"is_ai_trade"`
	SelfWinRate      float64 `json:"self_win_rate,omitempty"`
}

// AccountClosedPosition is a fully closed/redeemed holding from Polymarket history.
type AccountClosedPosition struct {
	Title        string  `json:"title"`
	Outcome      string  `json:"outcome"`
	ConditionID  string  `json:"condition_id,omitempty"`
	Asset        string  `json:"asset,omitempty"`
	AvgPrice     float64 `json:"avg_price"`
	TotalBought  float64 `json:"total_bought"`
	RealizedPNL  float64 `json:"realized_pnl"`
	CurPrice     float64 `json:"cur_price"`
	Timestamp    int64   `json:"timestamp,omitempty"`
	Result       string  `json:"result"` // profit | loss | breakeven
}

// PositionPerformance summarizes open + closed positions (Polymarket portfolio view).
type PositionPerformance struct {
	TotalCount   int     `json:"total_count"`
	OpenCount    int     `json:"open_count"`
	ClosedCount  int     `json:"closed_count"`
	WonCount     int     `json:"won_count"`
	LostCount    int     `json:"lost_count"`
	SettledCount int     `json:"settled_count"`
	TotalPnL     float64 `json:"total_pnl"`
	RealizedPnL  float64 `json:"realized_pnl"`
	OpenPnL      float64 `json:"open_pnl"`
	WinRate      float64 `json:"win_rate"`
	WinRateKnown bool    `json:"win_rate_known"`
	TradeCount    int     `json:"trade_count"`    // on-chain trade fills (full history)
	PositionCount int     `json:"position_count"` // unique market+outcome positions (open + closed)
	ClosedFetched int     `json:"closed_fetched"` // raw rows from closed-positions API
	OpenFetched   int     `json:"open_fetched"`   // raw rows from positions API
	// Legacy field names used by older dashboard builds.
	ProfitCount  int `json:"profit_count"`
	LossCount    int `json:"loss_count"`
	PendingCount int `json:"pending_count"`
}

func ComputePositionPerformance(positions []AccountPosition) PositionPerformance {
	return ComputePortfolioPerformance(positions, nil)
}

func ComputePortfolioPerformance(open []AccountPosition, closed []AccountClosedPosition) PositionPerformance {
	var perf PositionPerformance
	seen := make(map[string]struct{})

	for _, pos := range open {
		key := positionStatKey(pos.ConditionID, pos.Outcome, "")
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(pos.Title)) + "|" + strings.ToLower(strings.TrimSpace(pos.Outcome))
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		perf.OpenPnL += pos.CashPNL

		if !pos.ResultKnown {
			perf.OpenCount++
			continue
		}
		perf.SettledCount++
		if pos.Result == "loss" {
			perf.LostCount++
		} else {
			perf.WonCount++
		}
	}

	for _, c := range closed {
		key := positionStatKey(c.ConditionID, c.Outcome, c.Asset)
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(c.Title)) + "|" + strings.ToLower(strings.TrimSpace(c.Outcome))
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		perf.ClosedCount++
		perf.SettledCount++
		perf.RealizedPnL += c.RealizedPNL
		if c.Result == "loss" {
			perf.LostCount++
		} else {
			perf.WonCount++
		}
	}

	perf.PositionCount = len(seen)
	// TotalCount is set by caller to full trade history count (Polymarket History tab).
	perf.TotalCount = perf.PositionCount

	perf.PendingCount = perf.OpenCount
	perf.ProfitCount = perf.WonCount
	perf.LossCount = perf.LostCount
	perf.TotalPnL = perf.RealizedPnL + perf.OpenPnL
	perf.WinRateKnown = perf.SettledCount > 0
	if perf.SettledCount > 0 {
		perf.WinRate = float64(perf.WonCount) / float64(perf.SettledCount) * 100
	}
	return perf
}

func positionStatKey(conditionID, outcome, asset string) string {
	c := strings.ToLower(strings.TrimSpace(conditionID))
	o := strings.ToLower(strings.TrimSpace(outcome))
	if c != "" {
		return c + "|" + o
	}
	if a := strings.ToLower(strings.TrimSpace(asset)); a != "" {
		return a
	}
	return ""
}

func deriveClosedResult(pnl float64) string {
	if pnl > positionPnLEpsilon {
		return "profit"
	}
	if pnl >= -positionPnLEpsilon {
		return "breakeven"
	}
	return "loss"
}

func (s *AccountService) ResolveTradingAddress(eoa string) (string, error) {
	eoa = normalizeAddr(eoa)
	if eoa == "" {
		return "", fmt.Errorf("address required")
	}
	checksummed := common.HexToAddress(eoa).Hex()
	p, err := wallet.FetchPublicProfile(checksummed)
	if err != nil {
		if s.proxyFallback != "" && s.botSigner != "" && strings.EqualFold(checksummed, s.botSigner) {
			return common.HexToAddress(s.proxyFallback).Hex(), nil
		}
		return "", err
	}
	if p.ProxyWallet != "" {
		return p.ProxyWallet, nil
	}
	return checksummed, nil
}

func (s *AccountService) GetProfile(ctx context.Context, eoa string) (*AccountProfile, error) {
	eoa = normalizeAddr(eoa)
	tradingAddr, err := s.ResolveTradingAddress(eoa)
	if err != nil {
		return nil, err
	}

	checksummedEOA := common.HexToAddress(eoa).Hex()
	profile := &AccountProfile{
		EOAAddress:     eoa,
		ProxyWallet:    tradingAddr,
		HasProxyWallet: !strings.EqualFold(tradingAddr, checksummedEOA),
	}

	if p, err := wallet.FetchPublicProfile(checksummedEOA); err == nil && p.Found {
		if p.Name != "" {
			profile.DisplayName = p.Name
		}
		if p.Pseudonym != "" {
			profile.Pseudonym = p.Pseudonym
		}
		profile.PolymarketRegistered = true
	}

	if v, err := s.fetchValue(tradingAddr); err == nil {
		profile.PortfolioValue = v
		if v > 0 {
			profile.PolymarketRegistered = true
		}
	}

	if profile.HasProxyWallet {
		profile.PolymarketRegistered = true
	}

	if positions, err := s.GetAllPositions(ctx, eoa); err == nil {
		profile.OpenPositions = len(positions)
		if len(positions) > 0 {
			profile.PolymarketRegistered = true
		}
	}

	return profile, nil
}

func (s *AccountService) GetActivity(ctx context.Context, eoa string, limit, offset int) ([]AccountActivityItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	tradingAddr, err := s.ResolveTradingAddress(normalizeAddr(eoa))
	if err != nil {
		return nil, err
	}

	u := dataAPIBase + "/activity?user=" + url.QueryEscape(tradingAddr) +
		"&limit=" + strconv.Itoa(limit) +
		"&offset=" + strconv.Itoa(offset) +
		"&sortBy=TIMESTAMP&sortDirection=DESC"

	var raw []struct {
		Type            string  `json:"type"`
		Side            string  `json:"side"`
		Title           string  `json:"title"`
		Outcome         string  `json:"outcome"`
		Size            float64 `json:"size"`
		USDCSize        float64 `json:"usdcSize"`
		Price           float64 `json:"price"`
		Timestamp       int64   `json:"timestamp"`
		TransactionHash string  `json:"transactionHash"`
		ConditionID     string  `json:"conditionId"`
		Asset           string  `json:"asset"`
	}
	if err := s.getJSON(ctx, u, &raw); err != nil {
		return nil, err
	}

	out := make([]AccountActivityItem, 0, len(raw))
	for _, r := range raw {
		if r.Type != "" && r.Type != "TRADE" {
			continue
		}
		out = append(out, AccountActivityItem{
			Type:            r.Type,
			Side:            r.Side,
			Title:           r.Title,
			Outcome:         r.Outcome,
			Size:            r.Size,
			USDCSize:        r.USDCSize,
			Price:           r.Price,
			Timestamp:       r.Timestamp,
			TransactionHash: r.TransactionHash,
			ConditionID:     r.ConditionID,
			Asset:           r.Asset,
		})
	}
	return out, nil
}

func (s *AccountService) GetPositions(ctx context.Context, eoa string, limit, offset int) ([]AccountPosition, error) {
	if limit <= 0 || limit > positionsPageSize {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	tradingAddr, err := s.ResolveTradingAddress(normalizeAddr(eoa))
	if err != nil {
		return nil, err
	}
	raw, err := s.fetchPositionsPage(ctx, tradingAddr, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]AccountPosition, 0, len(raw))
	for _, r := range raw {
		out = append(out, positionFromRaw(r))
	}
	return out, nil
}

// GetAllPositions returns every current open position (paginated from Polymarket until exhausted).
func (s *AccountService) GetAllPositions(ctx context.Context, eoa string) ([]AccountPosition, error) {
	tradingAddr, err := s.ResolveTradingAddress(normalizeAddr(eoa))
	if err != nil {
		return nil, err
	}
	raw, err := s.fetchAllPositions(ctx, tradingAddr)
	if err != nil {
		return nil, err
	}
	out := make([]AccountPosition, 0, len(raw))
	for _, r := range raw {
		out = append(out, positionFromRaw(r))
	}
	return out, nil
}

func (s *AccountService) GetClosedPositions(ctx context.Context, eoa string, limit int) ([]AccountClosedPosition, error) {
	tradingAddr, err := s.ResolveTradingAddress(normalizeAddr(eoa))
	if err != nil {
		return nil, err
	}
	max := limit
	if max <= 0 {
		max = maxHistoryRows
	}
	return s.fetchAllClosedPositions(ctx, tradingAddr, max)
}

func (s *AccountService) fetchAllClosedPositions(ctx context.Context, user string, max int) ([]AccountClosedPosition, error) {
	out := make([]AccountClosedPosition, 0, closedPositionsPageSize)
	for offset := 0; len(out) < max; {
		batchLimit := closedPositionsPageSize
		if remaining := max - len(out); remaining < batchLimit {
			batchLimit = remaining
		}
		raw, err := s.fetchClosedPositions(ctx, user, batchLimit, offset)
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			break
		}
		for _, r := range raw {
			out = append(out, closedPositionFromRaw(r))
		}
		offset += len(raw)
		if len(raw) < batchLimit {
			break
		}
	}
	return out, nil
}

// CountTradeActivity returns all on-chain TRADE rows (matches Polymarket History tab).
func (s *AccountService) CountTradeActivity(ctx context.Context, eoa string) (int, error) {
	tradingAddr, err := s.ResolveTradingAddress(normalizeAddr(eoa))
	if err != nil {
		return 0, err
	}

	// Prefer /trades — same data Polymarket history uses for fills.
	if n, err := s.countAllTrades(ctx, tradingAddr); err == nil {
		return n, nil
	}

	total := 0
	for offset := 0; total < maxHistoryRows; {
		n, pageLen, err := s.fetchActivityTradePage(ctx, tradingAddr, activityPageSize, offset)
		if err != nil {
			return total, err
		}
		if pageLen == 0 {
			break
		}
		total += n
		offset += pageLen
		if pageLen < activityPageSize {
			break
		}
	}
	return total, nil
}

func (s *AccountService) fetchActivityTradePage(ctx context.Context, user string, limit, offset int) (tradeCount, pageLen int, err error) {
	u := dataAPIBase + "/activity?user=" + url.QueryEscape(user) +
		"&limit=" + strconv.Itoa(limit) +
		"&offset=" + strconv.Itoa(offset) +
		"&sortBy=TIMESTAMP&sortDirection=DESC"
	var raw []struct {
		Type string `json:"type"`
	}
	if err := s.getJSON(ctx, u, &raw); err != nil {
		return 0, 0, err
	}
	for _, r := range raw {
		if r.Type == "" || r.Type == "TRADE" {
			tradeCount++
		}
	}
	return tradeCount, len(raw), nil
}

func (s *AccountService) countAllTrades(ctx context.Context, user string) (int, error) {
	const pageSize = 500
	total := 0
	for offset := 0; total < maxHistoryRows; {
		u := dataAPIBase + "/trades?user=" + url.QueryEscape(user) +
			"&limit=" + strconv.Itoa(pageSize) +
			"&offset=" + strconv.Itoa(offset)
		var raw []struct {
			Side string `json:"side"`
		}
		if err := s.getJSON(ctx, u, &raw); err != nil {
			return total, err
		}
		if len(raw) == 0 {
			break
		}
		total += len(raw)
		offset += len(raw)
		if len(raw) < pageSize {
			break
		}
	}
	return total, nil
}

// WinningPositionsForRedeem returns settled winning holdings that should be redeemed to USDC.
func (s *AccountService) WinningPositionsForRedeem(ctx context.Context, eoa string, limit int) ([]AccountPosition, error) {
	positions, err := s.GetAllPositions(ctx, eoa)
	if err != nil {
		return nil, err
	}
	out := make([]AccountPosition, 0)
	for _, p := range positions {
		if p.ConditionID == "" || p.Size <= 0 {
			continue
		}
		// Only positions Polymarket marks on-chain redeemable; resolved_won without
		// redeemable=true often reverts (already claimed, dust, or pending settlement).
		if !p.Redeemable {
			continue
		}
		if p.Result == "loss" {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *AccountService) RedeemablePositions(ctx context.Context, eoa string, limit int) ([]AccountPosition, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	tradingAddr, err := s.ResolveTradingAddress(normalizeAddr(eoa))
	if err != nil {
		return nil, err
	}
	raw, err := s.fetchPositions(tradingAddr, limit)
	if err != nil {
		return nil, err
	}
	out := make([]AccountPosition, 0)
	for _, r := range raw {
		if !r.Redeemable || r.Size <= 0 {
			continue
		}
		out = append(out, positionFromRaw(r))
	}
	return out, nil
}

type rawPosition struct {
	Title        string  `json:"title"`
	Outcome      string  `json:"outcome"`
	Size         float64 `json:"size"`
	AvgPrice     float64 `json:"avgPrice"`
	CurPrice     float64 `json:"curPrice"`
	CurrentValue float64 `json:"currentValue"`
	CashPNL      float64 `json:"cashPnl"`
	PercentPNL   float64 `json:"percentPnl"`
	ConditionID  string  `json:"conditionId"`
	Asset        string  `json:"asset"`
	Redeemable   bool    `json:"redeemable"`
	Mergeable    bool    `json:"mergeable"`
	NegativeRisk bool    `json:"negativeRisk"`
	EndDate      string  `json:"endDate"`
}

type rawClosedPosition struct {
	Title       string  `json:"title"`
	Outcome     string  `json:"outcome"`
	ConditionID string  `json:"conditionId"`
	Asset       string  `json:"asset"`
	AvgPrice    float64 `json:"avgPrice"`
	TotalBought float64 `json:"totalBought"`
	RealizedPNL float64 `json:"realizedPnl"`
	CurPrice    float64 `json:"curPrice"`
	Timestamp   int64   `json:"timestamp"`
}

func closedPositionFromRaw(r rawClosedPosition) AccountClosedPosition {
	result := deriveClosedResult(r.RealizedPNL)
	return AccountClosedPosition{
		Title:       r.Title,
		Outcome:     r.Outcome,
		ConditionID: r.ConditionID,
		Asset:       r.Asset,
		AvgPrice:    r.AvgPrice,
		TotalBought: r.TotalBought,
		RealizedPNL: r.RealizedPNL,
		CurPrice:    r.CurPrice,
		Timestamp:   r.Timestamp,
		Result:      result,
	}
}

func positionFromRaw(r rawPosition) AccountPosition {
	status := deriveSettlementStatus(r)
	result, known := derivePositionResult(r, status)
	return AccountPosition{
		Title:            r.Title,
		Outcome:          r.Outcome,
		Size:             r.Size,
		AvgPrice:         r.AvgPrice,
		CurPrice:         r.CurPrice,
		CurrentValue:     r.CurrentValue,
		CashPNL:          r.CashPNL,
		PercentPNL:       r.PercentPNL,
		ConditionID:      r.ConditionID,
		Asset:            r.Asset,
		Redeemable:       r.Redeemable,
		Mergeable:        r.Mergeable,
		NegativeRisk:     r.NegativeRisk,
		EndDate:          r.EndDate,
		SettlementStatus: status,
		Result:           result,
		ResultKnown:      known,
	}
}

func deriveSettlementStatus(r rawPosition) string {
	// Win/loss on open holdings only when Polymarket marks claimable (true settle).
	// Near-zero floating marks stay "open" — count as loss only after stop-loss sell
	// or after the position appears in closed-positions.
	if r.Redeemable && r.CurPrice >= 0.95 {
		return "redeemable"
	}
	if r.Redeemable && r.CashPNL >= 0 && r.CurPrice >= 0.90 {
		return "redeemable"
	}
	return "open"
}

const positionPnLEpsilon = 0.01

func derivePositionResult(r rawPosition, status string) (result string, known bool) {
	if status == "open" {
		return "unknown", false
	}
	pnl := r.CashPNL
	switch status {
	case "resolved_won", "redeemable":
		// Market resolved in your favor; tiny negative PnL is fees/rounding, not a real loss.
		if pnl < -positionPnLEpsilon {
			return "loss", true
		}
		if pnl > positionPnLEpsilon {
			return "profit", true
		}
		return "breakeven", true
	default: // resolved_lost
		if pnl > positionPnLEpsilon {
			return "profit", true
		}
		if pnl >= -positionPnLEpsilon {
			return "breakeven", true
		}
		return "loss", true
	}
}

func (s *AccountService) fetchPositionsPage(ctx context.Context, user string, limit, offset int) ([]rawPosition, error) {
	u := dataAPIBase + "/positions?user=" + url.QueryEscape(user) +
		"&limit=" + strconv.Itoa(limit) +
		"&offset=" + strconv.Itoa(offset)
	var out []rawPosition
	if err := s.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AccountService) fetchAllPositions(ctx context.Context, user string) ([]rawPosition, error) {
	out := make([]rawPosition, 0, positionsPageSize)
	for offset := 0; len(out) < maxHistoryRows; {
		batch, err := s.fetchPositionsPage(ctx, user, positionsPageSize, offset)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		out = append(out, batch...)
		offset += len(batch)
		if len(batch) < positionsPageSize {
			break
		}
	}
	return out, nil
}

func (s *AccountService) fetchPositions(user string, limit int) ([]rawPosition, error) {
	return s.fetchPositionsPage(context.Background(), user, limit, 0)
}

func (s *AccountService) fetchClosedPositions(ctx context.Context, user string, limit, offset int) ([]rawClosedPosition, error) {
	u := dataAPIBase + "/closed-positions?user=" + url.QueryEscape(user) +
		"&limit=" + strconv.Itoa(limit) +
		"&offset=" + strconv.Itoa(offset) +
		"&sortBy=TIMESTAMP&sortDirection=DESC"
	var out []rawClosedPosition
	if err := s.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AccountService) fetchValue(user string) (float64, error) {
	u := dataAPIBase + "/value?user=" + url.QueryEscape(user)
	var rows []struct {
		Value float64 `json:"value"`
	}
	if err := s.getJSON(context.Background(), u, &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Value, nil
}

func (s *AccountService) getJSON(ctx context.Context, u string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "aiPoly/1.0")
	req.Header.Set("Connection", "close")

	body, status, err := s.doRequest(req)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("polymarket data api %d: %s", status, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (s *AccountService) doRequest(req *http.Request) ([]byte, int, error) {
	const maxAttempts = 4
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts && isRetryableHTTPError(err) {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, 0, err
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < maxAttempts && isRetryableHTTPError(readErr) {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, 0, readErr
		}
		return body, resp.StatusCode, nil
	}
	return nil, 0, lastErr
}

func isRetryableHTTPError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "eof") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "tls:")
}

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if !strings.HasPrefix(addr, "0x") {
		addr = "0x" + addr
	}
	return strings.ToLower(addr)
}
