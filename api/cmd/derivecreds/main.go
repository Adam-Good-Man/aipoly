package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"aipoly/api/config"

	clob "github.com/nijaru/go-clob-client/clob"
)

func main() {
	cfg, _ := config.Load()
	pk := strings.TrimSpace(cfg.PolyPrivateKey)
	if !strings.HasPrefix(pk, "0x") {
		pk = "0x" + pk
	}

	client, err := clob.NewSignerClient(clob.Config{
		Host:          cfg.ClobHost,
		ChainID:       clob.PolygonChainID,
		PrivateKey:    pk,
		UseServerTime: true,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	creds, err := client.CreateOrDeriveAPIKey(ctx, 0)
	if err != nil {
		log.Fatalf("derive api key: %v", err)
	}

	fmt.Println("=== 新 API 凭证（请更新 .env）===")
	fmt.Printf("POLYMARKET_API_KEY=%s\n", creds.Key)
	fmt.Printf("POLYMARKET_API_SECRET=%s\n", creds.Secret)
	fmt.Printf("POLYMARKET_API_PASSPHRASE=%s\n", creds.Passphrase)
}
