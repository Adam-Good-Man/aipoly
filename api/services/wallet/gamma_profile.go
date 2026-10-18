package wallet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type PublicProfile struct {
	ProxyWallet string
	Name        string
	Pseudonym   string
	Found       bool
}

type publicProfileResponse struct {
	ProxyWallet string `json:"proxyWallet"`
	Name        string `json:"name"`
	Pseudonym   string `json:"pseudonym"`
}

type profileCacheEntry struct {
	profile PublicProfile
	expires time.Time
}

var (
	profileCache   = make(map[string]profileCacheEntry)
	profileCacheMu sync.RWMutex
	profileCacheTTL = 15 * time.Minute

	gammaHTTPClient = &http.Client{
		Timeout: 45 * time.Second,
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}
)

// FetchPublicProfile loads Polymarket gamma public-profile with retry and cache.
func FetchPublicProfile(signerAddress string) (PublicProfile, error) {
	addr := common.HexToAddress(signerAddress).Hex()
	key := strings.ToLower(addr)

	profileCacheMu.RLock()
	if cached, ok := profileCache[key]; ok && time.Now().Before(cached.expires) {
		profileCacheMu.RUnlock()
		return cached.profile, nil
	}
	profileCacheMu.RUnlock()

	u := "https://gamma-api.polymarket.com/public-profile?address=" + url.QueryEscape(addr)
	body, status, err := gammaGET(u)
	if err != nil {
		return PublicProfile{}, err
	}
	if status == http.StatusNotFound {
		out := PublicProfile{Found: false}
		cacheProfile(key, out)
		return out, nil
	}
	if status >= 400 {
		return PublicProfile{}, fmt.Errorf("gamma profile %d: %s", status, strings.TrimSpace(string(body)))
	}

	var raw publicProfileResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return PublicProfile{}, err
	}

	out := PublicProfile{
		Name:      raw.Name,
		Pseudonym: raw.Pseudonym,
		Found:     true,
	}
	if proxy := strings.TrimSpace(raw.ProxyWallet); proxy != "" {
		out.ProxyWallet = common.HexToAddress(proxy).Hex()
	}
	cacheProfile(key, out)
	return out, nil
}

// FetchProxyWallet returns the Polymarket proxy/deposit wallet for an EOA.
func FetchProxyWallet(signerAddress string) (string, error) {
	p, err := FetchPublicProfile(signerAddress)
	if err != nil {
		return "", err
	}
	return p.ProxyWallet, nil
}

func cacheProfile(key string, p PublicProfile) {
	profileCacheMu.Lock()
	profileCache[key] = profileCacheEntry{profile: p, expires: time.Now().Add(profileCacheTTL)}
	profileCacheMu.Unlock()
}

func gammaGET(u string) ([]byte, int, error) {
	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "aiPoly/1.0")
		req.Header.Set("Connection", "close")

		resp, err := gammaHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts && isRetryable(err) {
				time.Sleep(time.Duration(attempt) * 600 * time.Millisecond)
				continue
			}
			return nil, 0, err
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < maxAttempts && isRetryable(readErr) {
				time.Sleep(time.Duration(attempt) * 600 * time.Millisecond)
				continue
			}
			return nil, 0, readErr
		}
		return body, resp.StatusCode, nil
	}
	return nil, 0, lastErr
}

func isRetryable(err error) bool {
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
