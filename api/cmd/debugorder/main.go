package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"aipoly/api/config"
	"aipoly/api/services/wallet"

	clob "github.com/nijaru/go-clob-client/clob"
	"github.com/quagmt/udecimal"
)

func main() {
	cfg, _ := config.Load()
	pk := strings.TrimSpace(cfg.PolyPrivateKey)
	if !strings.HasPrefix(pk, "0x") {
		pk = "0x" + pk
	}
	signer, _ := wallet.AddressFromPrivateKey(pk)
	funder := cfg.PolyFunderAddr

	fmt.Println("=== Debug Order ===")
	fmt.Println("EOA signer:", signer)
	fmt.Println("Funder:", funder)
	fmt.Println("Sig type:", cfg.SignatureType)

	// On-chain bytecode check via public RPC
	checkBytecode(funder)

	client, err := clob.NewAuthenticatedClient(clob.Config{
		Host:          cfg.ClobHost,
		ChainID:       clob.PolygonChainID,
		PrivateKey:    pk,
		UseServerTime: true,
		SignatureType: clob.SignatureTypePoly1271,
		FunderAddress: funder,
		HTTPClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
		Credentials: &clob.Credentials{
			Key: cfg.PolyAPIKey, Secret: cfg.PolyAPISecret, Passphrase: cfg.PolyAPIPassphrase,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// List API keys
	keys, err := client.GetAPIKeys(ctx)
	if err != nil {
		fmt.Println("GetAPIKeys err:", err)
	} else {
		b, _ := json.MarshalIndent(keys, "", "  ")
		fmt.Println("API Keys:", string(b))
	}

	// Derive fresh creds
	signerClient, err := clob.NewSignerClient(clob.Config{
		Host:          cfg.ClobHost,
		ChainID:       clob.PolygonChainID,
		PrivateKey:    pk,
		UseServerTime: true,
		HTTPClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
	})
	if err == nil {
		creds, err := signerClient.DeriveAPIKey(ctx, 0)
		if err != nil {
			fmt.Println("DeriveAPIKey err:", err)
		} else {
			fmt.Printf("Derived API key: %s (match env: %v)\n", creds.Key, creds.Key == cfg.PolyAPIKey)
		}
	}

	// Build order without posting - use a known cheap token
	tokenID := os.Getenv("TEST_TOKEN_ID")
	if tokenID == "" {
		tokenID = "76005700027045000587004110133818166617024719626220722682005164003117993034812"
	}
	client.SetTickSize(tokenID, clob.TickSizeThousandth)
	if nr, err := client.GetNegRisk(ctx, tokenID); err == nil {
		client.SetNegRisk(tokenID, nr.NegRisk)
	}

	order, err := client.CreateOrder(ctx, clob.OrderArgs{
		TokenID: tokenID,
		Price:   udecimal.MustFromFloat64(0.99),
		Size:    udecimal.MustFromFloat64(0.11),
		Side:    clob.SideBuy,
	}, nil)
	if err != nil {
		log.Fatalf("CreateOrder: %v", err)
	}

	fmt.Println("\n--- Signed Order ---")
	b, _ := json.MarshalIndent(order, "", "  ")
	fmt.Println(string(b))

	req, err := client.BuildPostOrderRequest(*order, clob.OrderTypeGTC, false, false)
	if err != nil {
		log.Fatalf("BuildPostOrderRequest: %v", err)
	}
	b, _ = json.MarshalIndent(req, "", "  ")
	fmt.Println("\n--- Post Request ---")
	fmt.Println(string(b))

	fmt.Println("\n--- Attempting post ---")
	resp, err := client.PostOrder(ctx, req)
	if err != nil {
		fmt.Println("PostOrder error:", err)
	} else {
		b, _ = json.MarshalIndent(resp, "", "  ")
		fmt.Println("Response:", string(b))
	}
}

func checkBytecode(addr string) {
	if addr == "" {
		return
	}
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
		fmt.Println("Bytecode check err:", err)
		return
	}
	defer resp.Body.Close()
	var result struct {
		Result string `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	code := result.Result
	if code == "0x" || code == "" {
		fmt.Println("⚠️  Deposit wallet NOT deployed (no bytecode at", addr+")")
	} else {
		fmt.Printf("✅ Deposit wallet deployed (bytecode len=%d)\n", (len(code)-2)/2)
	}
}
