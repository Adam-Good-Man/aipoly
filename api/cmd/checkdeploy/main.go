package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"aipoly/api/config"
	"aipoly/api/services/wallet"

	clob "github.com/nijaru/go-clob-client/clob"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	cfg, _ := config.Load()
	signer, _ := wallet.AddressFromPrivateKey(cfg.PolyPrivateKey)
	eoa := common.HexToAddress(signer)

	addrs := map[string]string{
		"EOA":              signer,
		"Gamma proxy":      "",
		"Derived Safe":     "",
		"UUPS deposit":     "",
		"Beacon deposit":   "",
		"Configured funder": cfg.PolyFunderAddr,
	}
	if p, err := wallet.FetchProxyWallet(signer); err == nil {
		addrs["Gamma proxy"] = p
	}
	if s, err := clob.DeriveSafeWallet(eoa, clob.PolygonChainID); err == nil {
		addrs["Derived Safe"] = s.Hex()
	}
	if u, err := clob.DeriveUUPSDepositWallet(eoa, clob.PolygonChainID); err == nil {
		addrs["UUPS deposit"] = u.Hex()
	}
	if b, err := clob.DeriveBeaconDepositWallet(eoa, clob.PolygonChainID); err == nil {
		addrs["Beacon deposit"] = b.Hex()
	}

	fmt.Println("=== On-chain bytecode check ===")
	for name, addr := range addrs {
		if addr == "" {
			continue
		}
		code := getCode(addr)
		status := "NOT deployed"
		if code != "0x" && code != "" {
			status = fmt.Sprintf("DEPLOYED (len=%d bytes)", (len(code)-2)/2)
		}
		fmt.Printf("%-18s %s\n  %s\n", name+":", addr, status)
	}
}

func getCode(addr string) string {
	rpc := os.Getenv("POLYGON_RPC_URL")
	if rpc == "" {
		rpc = "https://polygon-rpc.com"
	}
	body := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_getCode","params":["%s","latest"],"id":1}`, addr)
	req, _ := http.NewRequest(http.MethodPost, rpc, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	hc := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment}}
	resp, err := hc.Do(req)
	if err != nil {
		return "error:" + err.Error()
	}
	defer resp.Body.Close()
	var result struct {
		Result string `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Result
}
