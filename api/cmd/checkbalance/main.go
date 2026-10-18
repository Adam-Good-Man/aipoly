package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aipoly/api/config"

	clob "github.com/nijaru/go-clob-client/clob"
)

func bal(host, pk, funder string, sig clob.SignatureType, cfg *config.Config) {
	client, err := clob.NewAuthenticatedClient(clob.Config{
		Host: host, ChainID: clob.PolygonChainID, PrivateKey: pk, UseServerTime: true,
		SignatureType: sig, FunderAddress: funder,
		HTTPClient: &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment}},
		Credentials: &clob.Credentials{Key: cfg.PolyAPIKey, Secret: cfg.PolyAPISecret, Passphrase: cfg.PolyAPIPassphrase},
	})
	if err != nil {
		fmt.Println(funder[:10], sig, "err:", err)
		return
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := client.GetBalanceAllowance(ctx, clob.BalanceAllowanceParams{AssetType: clob.AssetTypeCollateral})
	if err != nil {
		fmt.Printf("sig=%d funder=%s err: %v\n", sig, funder, err)
		return
	}
	fmt.Printf("sig=%d funder=%s balance=%s\n", sig, funder, b.Balance)
}

func main() {
	cfg, _ := config.Load()
	pk := strings.TrimSpace(cfg.PolyPrivateKey)
	if !strings.HasPrefix(pk, "0x") {
		pk = "0x" + pk
	}
	beacon := "0x2db931adDc7F51d530F4e04552897dccc6F5e435"
	uups := "0x8198b9f43F1A6CCbf410cfaa987B3415aD8996bf"
	safe := "0xC13cd897bb9fDbC977b3bb93BE3D90710503D2AD"
	host := cfg.ClobHost

	fmt.Println("=== Balance by funder ===")
	bal(host, pk, beacon, clob.SignatureTypePoly1271, cfg)
	time.Sleep(2 * time.Second)
	bal(host, pk, uups, clob.SignatureTypePoly1271, cfg)
	time.Sleep(2 * time.Second)
	bal(host, pk, safe, clob.SignatureTypePolyGnosisSafe, cfg)
}
