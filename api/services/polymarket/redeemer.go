package polymarket

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aipoly/api/config"

	polyclob "github.com/bububa/polymarket-client/clob"
	polyrelayer "github.com/bububa/polymarket-client/relayer"
	"github.com/ethereum/go-ethereum/common"
)

type PositionRedeemer struct {
	cfg           *config.Config
	client        *polyclob.Client
	depositWallet string
	mu            sync.Mutex
	initialized   bool
	ready         bool
	initErr       error
}

type RedeemOutcome struct {
	ConditionID   string `json:"condition_id"`
	Title         string `json:"title,omitempty"`
	Outcome       string `json:"outcome,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	State         string `json:"state,omitempty"`
	Error         string `json:"error,omitempty"`
}

func NewPositionRedeemer(cfg *config.Config) *PositionRedeemer {
	return &PositionRedeemer{cfg: cfg}
}

func (r *PositionRedeemer) Ready() bool {
	r.ensureInit()
	return r.ready
}

func (r *PositionRedeemer) InitError() error {
	r.ensureInit()
	return r.initErr
}

func (r *PositionRedeemer) DepositWallet() string {
	r.ensureInit()
	return r.depositWallet
}

func (r *PositionRedeemer) ensureInit() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.initialized {
		return
	}
	r.initialized = true
	r.initErr = r.init()
	r.ready = r.initErr == nil
}

func (r *PositionRedeemer) init() error {
	if r.cfg.PaperTrading {
		return fmt.Errorf("paper trading mode")
	}
	if r.cfg.PolyPrivateKey == "" {
		return fmt.Errorf("POLYMARKET_PRIVATE_KEY not configured")
	}
	if r.cfg.PolyAPIKey == "" || r.cfg.PolyAPISecret == "" || r.cfg.PolyAPIPassphrase == "" {
		return fmt.Errorf("POLYMARKET API credentials not configured")
	}

	pk := strings.TrimSpace(r.cfg.PolyPrivateKey)
	if !strings.HasPrefix(pk, "0x") {
		pk = "0x" + pk
	}

	signer, err := polyclob.ParsePrivateKey(pk)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	depositWallet := strings.TrimSpace(r.cfg.PolyFunderAddr)
	if depositWallet != "" && !strings.HasPrefix(depositWallet, "0x") {
		depositWallet = "0x" + depositWallet
	}
	if depositWallet == "" {
		return fmt.Errorf("POLYMARKET_FUNDER_ADDRESS not configured")
	}
	r.depositWallet = common.HexToAddress(depositWallet).Hex()

	clobCreds := polyclob.Credentials{
		Key:        r.cfg.PolyAPIKey,
		Secret:     r.cfg.PolyAPISecret,
		Passphrase: r.cfg.PolyAPIPassphrase,
	}

	opts := []polyclob.Option{
		polyclob.WithSigner(signer),
		polyclob.WithCredentials(clobCreds),
		polyclob.WithChainID(polyclob.PolygonChainID),
		polyclob.WithDefaultSignatureType(polyclob.SignatureTypePoly1271),
	}
	if rpc := strings.TrimSpace(r.cfg.PolygonRPCURL); rpc != "" {
		opts = append(opts, polyclob.WithRPCURL(rpc))
	}

	builderCreds, err := ResolveBuilderCredentials(r.cfg)
	if err != nil {
		return fmt.Errorf("builder API credentials required for auto-redeem: %w (run: go run ./cmd/createbuilder/main.go)", err)
	}

	relayerClient := polyrelayer.New(polyrelayer.Config{
		HTTPClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
		BuilderCredentials: &builderCreds,
	})

	opts = append(opts, polyclob.WithRelayerClient(relayerClient))
	r.client = polyclob.NewClient(r.cfg.ClobHost, opts...)
	log.Printf("position redeemer ready for deposit wallet %s", r.depositWallet)
	return nil
}

func (r *PositionRedeemer) Redeem(ctx context.Context, conditionID string, negativeRisk bool, meta RedeemOutcome) RedeemOutcome {
	out := meta
	out.ConditionID = conditionID
	if !r.Ready() {
		out.Error = r.InitError().Error()
		return out
	}
	conditionID = strings.TrimSpace(conditionID)
	if conditionID == "" {
		out.Error = "condition id required"
		return out
	}
	if !strings.HasPrefix(conditionID, "0x") {
		conditionID = "0x" + conditionID
	}

	cc, err := polyclob.Contracts(polyclob.PolygonChainID)
	if err != nil {
		out.Error = err.Error()
		return out
	}

	args := &polyclob.DepositWalletCTFArgs{
		DepositWallet: r.depositWallet,
		Deadline:      strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10),
		Metadata:      "auto-redeem",
	}

	var resp polyrelayer.SubmitTransactionResponse
	if negativeRisk {
		err = r.client.RedeemNegRiskWithDepositWallet(ctx, &polyclob.RedeemNegRiskRequest{
			ConditionID: common.HexToHash(conditionID),
		}, args, &resp)
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "revert") {
			log.Printf("neg-risk redeem reverted for %s, trying binary redeem", meta.Title)
			req := polyclob.RedeemBinary(cc.Collateral, common.HexToHash(conditionID))
			err = r.client.RedeemPositionsWithDepositWallet(ctx, &req, args, &resp)
		}
	} else {
		req := polyclob.RedeemBinary(cc.Collateral, common.HexToHash(conditionID))
		err = r.client.RedeemPositionsWithDepositWallet(ctx, &req, args, &resp)
	}
	if err != nil {
		out.Error = err.Error()
		log.Printf("redeem failed %s (%s): %v", conditionID, meta.Title, err)
		return out
	}

	out.TransactionID = resp.TransactionID
	out.State = resp.State
	log.Printf("redeem submitted: %s | %s | tx=%s state=%s", meta.Title, meta.Outcome, resp.TransactionID, resp.State)
	return out
}
