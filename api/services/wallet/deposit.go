package wallet

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func (s *Service) fetchOnChainValue(address string) (float64, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	}
	u := "https://data-api.polymarket.com/value?user=" + url.QueryEscape(address)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("data api %d", resp.StatusCode)
	}

	var rows []struct {
		Value float64 `json:"value"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Value, nil
}

// DeriveDepositWalletAddress placeholder — deposit wallet address should be
// set via POLYMARKET_FUNDER_ADDRESS after deploying through Polymarket relayer.
// See: https://docs.polymarket.com/trading/deposit-wallets
func DeriveDepositWalletAddress(_, _ string) (string, error) {
	return "", fmt.Errorf("not implemented")
}
