package polymarket

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"aipoly/api/config"

	polyrelayer "github.com/bububa/polymarket-client/relayer"
	clob "github.com/nijaru/go-clob-client/clob"
)

func ResolveBuilderCredentials(cfg *config.Config) (polyrelayer.BuilderCredentials, error) {
	if k := strings.TrimSpace(cfg.PolyBuilderAPIKey); k != "" {
		return polyrelayer.BuilderCredentials{
			APIKey:     k,
			Secret:     strings.TrimSpace(cfg.PolyBuilderAPISecret),
			Passphrase: strings.TrimSpace(cfg.PolyBuilderAPIPassphrase),
		}, nil
	}

	creds, err := createBuilderAPIKey(cfg)
	if err != nil {
		return polyrelayer.BuilderCredentials{}, err
	}
	log.Printf("auto-created builder API key — add to .env: POLYMARKET_BUILDER_API_KEY=%s", creds.Key)
	return polyrelayer.BuilderCredentials{
		APIKey:     creds.Key,
		Secret:     creds.Secret,
		Passphrase: creds.Passphrase,
	}, nil
}

func createBuilderAPIKey(cfg *config.Config) (*clob.Credentials, error) {
	pk := strings.TrimSpace(cfg.PolyPrivateKey)
	if !strings.HasPrefix(pk, "0x") {
		pk = "0x" + pk
	}
	funder := strings.TrimSpace(cfg.PolyFunderAddr)
	if funder != "" && !strings.HasPrefix(funder, "0x") {
		funder = "0x" + funder
	}

	client, err := clob.NewAuthenticatedClient(clob.Config{
		Host:          cfg.ClobHost,
		ChainID:       clob.PolygonChainID,
		PrivateKey:    pk,
		UseServerTime: true,
		SignatureType: clob.SignatureTypePoly1271,
		FunderAddress: funder,
		HTTPClient: &http.Client{
			Timeout:   45 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
		Credentials: &clob.Credentials{
			Key:        cfg.PolyAPIKey,
			Secret:     cfg.PolyAPISecret,
			Passphrase: cfg.PolyAPIPassphrase,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("builder clob client: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	return client.CreateBuilderAPIKey(ctx)
}
