package main

import (
	"fmt"

	"aipoly/api/config"
	"aipoly/api/services/wallet"

	clob "github.com/nijaru/go-clob-client/clob"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	cfg, _ := config.Load()
	signer, _ := wallet.AddressFromPrivateKey(cfg.PolyPrivateKey)
	eoa := common.HexToAddress(signer)

	safe, _ := clob.DeriveSafeWallet(eoa, clob.PolygonChainID)
	proxy, _ := clob.DeriveProxyWallet(eoa, clob.PolygonChainID)
	uups, _ := clob.DeriveUUPSDepositWallet(eoa, clob.PolygonChainID)
	beacon, _ := clob.DeriveBeaconDepositWallet(eoa, clob.PolygonChainID)
	gamma, _ := wallet.FetchProxyWallet(signer)

	fmt.Println("EOA:", signer)
	fmt.Println("Gamma proxyWallet:", gamma)
	fmt.Println("Derived Safe:", safe.Hex())
	fmt.Println("Derived Proxy:", proxy.Hex())
	fmt.Println("Derived UUPS deposit:", uups.Hex())
	fmt.Println("Derived Beacon deposit:", beacon.Hex())
	fmt.Println("Configured funder:", cfg.PolyFunderAddr)
}
