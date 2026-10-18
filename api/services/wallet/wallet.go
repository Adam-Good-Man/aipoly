package wallet

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"

	"aipoly/api/config"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	polygonChainID = 137
)

type Service struct {
	cfg *config.Config
}

type Info struct {
	Configured         bool    `json:"configured"`
	SignerAddress      string  `json:"signer_address"`
	FunderAddress      string  `json:"funder_address"`
	ProxyWallet        string  `json:"proxy_wallet,omitempty"`
	PaperTrading       bool    `json:"paper_trading"`
	ChainID            int     `json:"chain_id"`
	SignatureType      int     `json:"signature_type"`
	EffectiveSigType   int     `json:"effective_signature_type,omitempty"`
	NeedsDepositWallet bool    `json:"needs_deposit_wallet"`
	OnChainValue       float64 `json:"on_chain_value"`
	ProxyValue         float64 `json:"proxy_value,omitempty"`
	FundingNote        string  `json:"funding_note"`
}

type GeneratedWallet struct {
	PrivateKey    string `json:"private_key"`
	SignerAddress string `json:"signer_address"`
	FunderAddress string `json:"funder_address"`
	Warning       string `json:"warning"`
}

func NewService(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) GetInfo() (*Info, error) {
	info := &Info{
		PaperTrading:  s.cfg.PaperTrading,
		ChainID:       polygonChainID,
		SignatureType: s.cfg.SignatureType,
		FundingNote:   "Polymarket 交易使用 Polygon 链上的 USDC（pUSD）。请向充值地址转入 USDC，而非 USDT。",
	}

	if s.cfg.PolyPrivateKey == "" {
		info.FundingNote = "请先在 .env 中配置 POLYMARKET_PRIVATE_KEY，或调用 POST /api/wallet/generate 生成新钱包。"
		return info, nil
	}

	signer, err := AddressFromPrivateKey(s.cfg.PolyPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	info.Configured = true
	info.SignerAddress = signer
	info.ProxyWallet, _ = FetchProxyWallet(signer)
	info.FunderAddress = s.resolveFunderAddress(signer, info.ProxyWallet)
	info.NeedsDepositWallet = info.ProxyWallet != "" && strings.EqualFold(info.FunderAddress, signer)

	if info.ProxyWallet != "" && !strings.EqualFold(info.ProxyWallet, signer) {
		if v, err := s.fetchOnChainValue(info.ProxyWallet); err == nil {
			info.ProxyValue = v
		}
	}

	if info.FunderAddress != "" {
		if v, err := s.fetchOnChainValue(info.FunderAddress); err == nil {
			info.OnChainValue = v
		}
	}

	if info.NeedsDepositWallet {
		info.FundingNote = "检测到 Polymarket 代理钱包。请在 .env 设置 POLYMARKET_FUNDER_ADDRESS=" + info.ProxyWallet + " 和 POLYMARKET_SIGNATURE_TYPE=2，或直接让系统自动检测（当前已支持）。"
	}

	return info, nil
}

func (s *Service) Generate() (*GeneratedWallet, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	privBytes := crypto.FromECDSA(key)
	privateKey := "0x" + hex.EncodeToString(privBytes)
	signer, err := AddressFromPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	return &GeneratedWallet{
		PrivateKey:    privateKey,
		SignerAddress: signer,
		FunderAddress: s.resolveFunderAddress(signer, ""),
		Warning:       "请立即将 private_key 写入 .env 的 POLYMARKET_PRIVATE_KEY，切勿泄露或提交到 Git。重启 API 后生效。",
	}, nil
}

func (s *Service) resolveFunderAddress(signer, proxyWallet string) string {
	if s.cfg.PolyFunderAddr != "" {
		addr := strings.TrimSpace(s.cfg.PolyFunderAddr)
		if !strings.HasPrefix(addr, "0x") {
			addr = "0x" + addr
		}
		return common.HexToAddress(addr).Hex()
	}
	if proxyWallet != "" {
		return common.HexToAddress(proxyWallet).Hex()
	}
	if derived, err := FetchProxyWallet(signer); err == nil && derived != "" {
		return derived
	}
	return common.HexToAddress(signer).Hex()
}

func AddressFromPrivateKey(privateKey string) (string, error) {
	key, err := parsePrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	return crypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}

func parsePrivateKey(privateKey string) (*ecdsa.PrivateKey, error) {
	raw := strings.TrimSpace(privateKey)
	raw = strings.TrimPrefix(raw, "0x")
	b, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}
	return crypto.ToECDSA(b)
}
