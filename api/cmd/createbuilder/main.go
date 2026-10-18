package main

import (
	"context"
	"encoding/json"
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
	pk := cfg.PolyPrivateKey
	if !strings.HasPrefix(pk, "0x") {
		pk = "0x" + pk
	}
	client, err := clob.NewAuthenticatedClient(clob.Config{
		Host:          cfg.ClobHost,
		ChainID:       clob.PolygonChainID,
		PrivateKey:    pk,
		UseServerTime: true,
		SignatureType: clob.SignatureTypePoly1271,
		FunderAddress: cfg.PolyFunderAddr,
		HTTPClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
		Credentials: &clob.Credentials{
			Key:        cfg.PolyAPIKey,
			Secret:     cfg.PolyAPISecret,
			Passphrase: cfg.PolyAPIPassphrase,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	keys, err := client.GetBuilderAPIKeys(ctx)
	fmt.Println("existing keys err:", err, "count:", len(keys))
	if len(keys) > 0 {
		b, _ := json.MarshalIndent(keys, "", "  ")
		fmt.Println(string(b))
	}

	creds, err := client.CreateBuilderAPIKey(ctx)
	fmt.Println("create err:", err)
	if creds != nil {
		fmt.Printf("POLYMARKET_BUILDER_API_KEY=%s\n", creds.Key)
		fmt.Printf("POLYMARKET_BUILDER_API_SECRET=%s\n", creds.Secret)
		fmt.Printf("POLYMARKET_BUILDER_API_PASSPHRASE=%s\n", creds.Passphrase)
	}
}
