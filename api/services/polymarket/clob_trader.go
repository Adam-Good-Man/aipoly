package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"aipoly/api/config"
	"aipoly/api/services/wallet"

	clob "github.com/nijaru/go-clob-client/clob"
	"github.com/quagmt/udecimal"
)

type ClobTrader struct {
	client        *clob.AuthenticatedClient
	ready         bool
	initErr       error
	mu            sync.Mutex
	initialized   bool
	cfg           *config.Config
	balanceUSDC   float64
	signerAddress string
	funderAddress string
	signatureType clob.SignatureType
}

type LiveOrderResult struct {
	OrderID      string
	Status       string
	PlannedUSDC  float64
	FilledUSDC   float64 // BUY: USDC spent (makingAmount)
	FilledShares float64 // BUY: shares received (takingAmount)
}

func NewClobTrader(cfg *config.Config) *ClobTrader {
	return &ClobTrader{cfg: cfg}
}

func (t *ClobTrader) Ready() bool {
	t.ensureInit()
	return t.ready
}

func (t *ClobTrader) InitError() error {
	t.ensureInit()
	return t.initErr
}

func (t *ClobTrader) BalanceUSDC() float64 {
	t.ensureInit()
	return t.balanceUSDC
}

func (t *ClobTrader) FunderAddress() string {
	t.ensureInit()
	return t.funderAddress
}

func (t *ClobTrader) SignerAddress() string {
	t.ensureInit()
	return t.signerAddress
}

// GeoblockStatus is the Polymarket IP trading restriction check.
type GeoblockStatus struct {
	Blocked bool   `json:"blocked"`
	IP      string `json:"ip,omitempty"`
	Country string `json:"country,omitempty"`
	Region  string `json:"region,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CheckGeoblock probes https://polymarket.com/api/geoblock via the same HTTP proxy as CLOB orders.
func (t *ClobTrader) CheckGeoblock(ctx context.Context) GeoblockStatus {
	hc := &http.Client{
		Timeout:   12 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://polymarket.com/api/geoblock", nil)
	if err != nil {
		return GeoblockStatus{Blocked: true, Error: err.Error()}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return GeoblockStatus{Blocked: true, Error: err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return GeoblockStatus{Blocked: true, Error: err.Error()}
	}
	if resp.StatusCode >= 400 {
		return GeoblockStatus{Blocked: true, Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}
	var out GeoblockStatus
	if err := json.Unmarshal(body, &out); err != nil {
		return GeoblockStatus{Blocked: true, Error: err.Error()}
	}
	return out
}

// IsGeoblockOrderError reports whether a live order error is Polymarket region restriction.
func IsGeoblockOrderError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "geoblock") ||
		strings.Contains(msg, "trading restricted in your region")
}

func (t *ClobTrader) ensureInit() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.initialized {
		return
	}
	t.initialized = true
	t.initErr = t.init()
	t.ready = t.initErr == nil
}

func (t *ClobTrader) init() error {
	if t.cfg.PolyPrivateKey == "" {
		return fmt.Errorf("POLYMARKET_PRIVATE_KEY not configured")
	}
	if t.cfg.PolyAPIKey == "" || t.cfg.PolyAPISecret == "" || t.cfg.PolyAPIPassphrase == "" {
		return fmt.Errorf("POLYMARKET API credentials not configured")
	}

	pk := strings.TrimSpace(t.cfg.PolyPrivateKey)
	if !strings.HasPrefix(pk, "0x") {
		pk = "0x" + pk
	}

	signer, err := wallet.AddressFromPrivateKey(pk)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}
	t.signerAddress = signer

	sigType, funder := t.resolveWalletType(signer)
	t.signatureType = sigType
	t.funderAddress = funder

	httpClient := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}

	log.Printf("CLOB client host: %s", t.cfg.ClobHost)

	client, err := clob.NewAuthenticatedClient(clob.Config{
		Host:                 t.cfg.ClobHost,
		ChainID:              clob.PolygonChainID,
		PrivateKey:           pk,
		HTTPClient:           httpClient,
		UseServerTime:        true,
		SignatureType:        sigType,
		FunderAddress:        funder,
		DisableAutoHeartbeat: true,
		Credentials: &clob.Credentials{
			Key:        t.cfg.PolyAPIKey,
			Secret:     t.cfg.PolyAPISecret,
			Passphrase: t.cfg.PolyAPIPassphrase,
		},
	})
	if err != nil {
		return fmt.Errorf("create CLOB V2 client: %w", err)
	}
	t.client = client

	if err := t.refreshBalance(); err != nil {
		log.Printf("CLOB balance check warning: %v", err)
	}

	return nil
}

func (t *ClobTrader) refreshBalance() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = t.client.UpdateBalanceAllowance(ctx, clob.BalanceAllowanceParams{
		AssetType: clob.AssetTypeCollateral,
	})

	bal, err := t.client.GetBalanceAllowance(ctx, clob.BalanceAllowanceParams{
		AssetType: clob.AssetTypeCollateral,
	})
	if err != nil {
		return err
	}
	t.balanceUSDC = parseUSDCBalance(bal.Balance)
	log.Printf("CLOB USDC balance: %.4f (raw: %s, funder: %s)", t.balanceUSDC, bal.Balance, t.funderAddress)
	return nil
}

func parseUSDCBalance(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	// Polymarket returns balance in micro-USDC (6 decimals)
	if v > 1000 {
		return v / 1_000_000
	}
	return v
}

func (t *ClobTrader) resolveWalletType(signer string) (clob.SignatureType, string) {
	funder := strings.TrimSpace(t.cfg.PolyFunderAddr)
	if funder != "" && !strings.HasPrefix(funder, "0x") {
		funder = "0x" + funder
	}

	switch t.cfg.SignatureType {
	case 1:
		return clob.SignatureTypePolyProxy, funder
	case 2:
		return clob.SignatureTypePolyGnosisSafe, funder
	case 3:
		if funder == "" {
			if proxy, err := wallet.FetchProxyWallet(signer); err == nil && proxy != "" {
				log.Printf("auto-detected Polymarket deposit wallet %s for signer %s (POLY_1271)", proxy, signer)
				return clob.SignatureTypePoly1271, proxy
			}
		}
		return clob.SignatureTypePoly1271, funder
	default:
		if funder != "" && !strings.EqualFold(funder, signer) {
			log.Printf("using configured funder %s with Gnosis Safe signature", funder)
			return clob.SignatureTypePolyGnosisSafe, funder
		}
		// EOA mode: if funder equals signer or is unset, try Polymarket proxy wallet
		if funder == "" || strings.EqualFold(funder, signer) {
			if proxy, err := wallet.FetchProxyWallet(signer); err == nil && proxy != "" && !strings.EqualFold(proxy, signer) {
				log.Printf("auto-detected Polymarket proxy wallet %s for signer %s (Gnosis Safe mode)", proxy, signer)
				return clob.SignatureTypePolyGnosisSafe, proxy
			}
		}
		return clob.SignatureTypeEOA, ""
	}
}

func roundToTick(price, tickSize float64) float64 {
	if tickSize <= 0 {
		tickSize = 0.001
	}
	steps := math.Floor(price/tickSize + 1e-9)
	return steps * tickSize
}

func (t *ClobTrader) prefetchMarketMeta(ctx context.Context, tokenID string) {
	if err := t.prefetchTickSize(ctx, tokenID); err != nil {
		log.Printf("tick size prefetch: %v, defaulting to 0.001", err)
		t.client.SetTickSize(tokenID, clob.TickSizeThousandth)
	}
	if nr, err := t.client.GetNegRisk(ctx, tokenID); err == nil {
		t.client.SetNegRisk(tokenID, nr.NegRisk)
	}
}

func (t *ClobTrader) prefetchTickSize(ctx context.Context, tokenID string) error {
	u := strings.TrimRight(t.cfg.ClobHost, "/") + "/tick-size?token_id=" + url.QueryEscape(tokenID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	hc := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("tick-size %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		MinimumTickSize json.RawMessage `json:"minimum_tick_size"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}

	tickStr, err := parseTickSizeValue(raw.MinimumTickSize)
	if err != nil {
		return err
	}
	t.client.SetTickSize(tokenID, clob.TickSize(tickStr))
	return nil
}

func parseTickSizeValue(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s, nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64), nil
	}
	return "", fmt.Errorf("unsupported tick size: %s", string(raw))
}

func (t *ClobTrader) PlaceBuy(ctx context.Context, tokenID string, amountUSDC float64) (*LiveOrderResult, error) {
	t.ensureInit()
	if !t.ready {
		return nil, fmt.Errorf("CLOB not ready: %w", t.initErr)
	}
	if amountUSDC <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if t.balanceUSDC > 0 && amountUSDC > t.balanceUSDC {
		amountUSDC = t.balanceUSDC
	}
	// CLOB market buy: maker amount max 2 decimal places (USDC).
	// Use string parse to avoid float64 binary noise past 2dp.
	amountUSDC = math.Floor(amountUSDC*100+1e-9) / 100
	if amountUSDC < 1.0 {
		amountUSDC = 1.0
	}
	amountDec, err := udecimal.Parse(fmt.Sprintf("%.2f", amountUSDC))
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	t.prefetchMarketMeta(ctx, tokenID)

	resp, err := t.client.CreateAndPostMarketOrder(ctx, clob.MarketOrderArgs{
		TokenID: tokenID,
		Amount:  amountDec,
		Side:    clob.SideBuy,
	}, nil, clob.OrderTypeFOK)
	if err != nil {
		return nil, err
	}
	if !resp.Success && resp.ErrorMsg != "" {
		return nil, fmt.Errorf("%s", resp.ErrorMsg)
	}
	out := &LiveOrderResult{
		OrderID:     resp.OrderID,
		Status:      resp.Status,
		PlannedUSDC: amountUSDC,
	}
	// BUY: makingAmount = USDC spent, takingAmount = shares received.
	if v, err := strconv.ParseFloat(strings.TrimSpace(resp.MakingAmount), 64); err == nil && v > 0 {
		out.FilledUSDC = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(resp.TakingAmount), 64); err == nil && v > 0 {
		out.FilledShares = v
	}
	// FOK should fill all-or-nothing; treat empty amounts as full planned fill.
	if out.FilledUSDC <= 0 {
		out.FilledUSDC = amountUSDC
	}
	return out, nil
}

// PlaceSell market-sells shares of a conditional token (FAK). Amount is share count, not USDC.
func (t *ClobTrader) PlaceSell(ctx context.Context, tokenID string, shares float64) (*LiveOrderResult, error) {
	t.ensureInit()
	if !t.ready {
		return nil, fmt.Errorf("CLOB not ready: %w", t.initErr)
	}
	if shares <= 0 {
		return nil, fmt.Errorf("shares must be positive")
	}
	shares = roundToTick(shares, 0.01)
	if shares < 0.01 {
		return nil, fmt.Errorf("shares too small after rounding")
	}

	t.prefetchMarketMeta(ctx, tokenID)

	resp, err := t.client.CreateAndPostMarketOrder(ctx, clob.MarketOrderArgs{
		TokenID: tokenID,
		Amount:  udecimal.MustFromFloat64(shares),
		Side:    clob.SideSell,
	}, nil, clob.OrderTypeFAK)
	if err != nil {
		return nil, err
	}
	if !resp.Success && resp.ErrorMsg != "" {
		return nil, fmt.Errorf("%s", resp.ErrorMsg)
	}
	return &LiveOrderResult{OrderID: resp.OrderID, Status: resp.Status}, nil
}

func (t *ClobTrader) PlaceLimitBuy(ctx context.Context, tokenID string, price, amountUSDC float64) (*LiveOrderResult, error) {
	t.ensureInit()
	if !t.ready {
		return nil, fmt.Errorf("CLOB not ready: %w", t.initErr)
	}
	if price <= 0 || price >= 1 {
		return nil, fmt.Errorf("invalid price %.4f", price)
	}
	price = roundToTick(price, 0.001)
	if t.balanceUSDC > 0 && amountUSDC > t.balanceUSDC {
		amountUSDC = t.balanceUSDC
	}

	shares := roundToTick(amountUSDC/price, 0.01)
	t.prefetchMarketMeta(ctx, tokenID)

	resp, err := t.client.CreateAndPostOrder(ctx, clob.OrderArgs{
		TokenID: tokenID,
		Price:   udecimal.MustFromFloat64(price),
		Size:    udecimal.MustFromFloat64(shares),
		Side:    clob.SideBuy,
	}, nil, clob.OrderTypeGTC, false)
	if err != nil {
		return nil, err
	}
	if !resp.Success && resp.ErrorMsg != "" {
		return nil, fmt.Errorf("%s", resp.ErrorMsg)
	}
	return &LiveOrderResult{OrderID: resp.OrderID, Status: resp.Status}, nil
}

func (t *ClobTrader) GetBalance(ctx context.Context) (string, error) {
	t.ensureInit()
	if !t.ready {
		return "", t.initErr
	}
	bal, err := t.client.GetBalanceAllowance(ctx, clob.BalanceAllowanceParams{
		AssetType: clob.AssetTypeCollateral,
	})
	if err != nil {
		return "", err
	}
	t.balanceUSDC = parseUSDCBalance(bal.Balance)
	return fmt.Sprintf("%.4f", t.balanceUSDC), nil
}

func (t *ClobTrader) Close() {
	if t.client != nil {
		t.client.Close()
	}
}
