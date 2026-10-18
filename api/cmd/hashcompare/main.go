package main

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

func main() {
	funder := "0x2db931adDc7F51d530F4e04552897dccc6F5e435"
	contract := "0xE111180000d2663C0091e4f400237545B87B996B"
	msg := apitypes.TypedDataMessage{
		"salt": "12345", "maker": funder, "signer": funder,
		"tokenId": "76005700027045000587004110133818166617024719626220722682005164003117993034812",
		"makerAmount": "1000000", "takerAmount": "1010100",
		"side": "0", "signatureType": "3", "timestamp": "1783532382023",
		"metadata": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"builder":  "0x0000000000000000000000000000000000000000000000000000000000000000",
	}
	// replicate pack functions via exported test - inline here
	chainID := int64(137)
	depositWallet := common.HexToAddress(funder)
	contractAddr := common.HexToAddress(contract)

	appDomainSep := crypto.Keccak256(packDomain(chainID, contractAddr))
	contentsHash := crypto.Keccak256(packContents(msg))
	typedSignHash := crypto.Keccak256(packTypedSign(contentsHash, chainID, depositWallet))

	fmt.Println("app_domain_sep", hex.EncodeToString(appDomainSep))
	fmt.Println("contents_hash", hex.EncodeToString(contentsHash))
	fmt.Println("typed_sign_hash", hex.EncodeToString(typedSignHash))
}

func packDomain(chainID int64, contract common.Address) []byte {
	domainTypeHash := crypto.Keccak256Hash([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))
	nameHash := crypto.Keccak256Hash([]byte("Polymarket CTF Exchange"))
	versionHash := crypto.Keccak256Hash([]byte("2"))
	buf := make([]byte, 160)
	copy(buf[0:32], domainTypeHash.Bytes())
	copy(buf[32:64], nameHash.Bytes())
	copy(buf[64:96], versionHash.Bytes())
	putU256(buf[96:128], chainID)
	copy(buf[128+12:160], contract.Bytes())
	return buf
}

func packContents(msg apitypes.TypedDataMessage) []byte {
	orderTypeString := "Order(uint256 salt,address maker,address signer,uint256 tokenId," +
		"uint256 makerAmount,uint256 takerAmount,uint8 side,uint8 signatureType," +
		"uint256 timestamp,bytes32 metadata,bytes32 builder)"
	orderTypeHash := crypto.Keccak256Hash([]byte(orderTypeString))
	buf := make([]byte, 384)
	copy(buf[0:32], orderTypeHash.Bytes())
	putU256(buf[32:64], 12345)
	copy(buf[64+12:96], common.HexToAddress(msg["maker"].(string)).Bytes())
	copy(buf[96+12:128], common.HexToAddress(msg["signer"].(string)).Bytes())
	putU256(buf[128:160], parseInt(msg["tokenId"].(string)))
	putU256(buf[160:192], parseInt(msg["makerAmount"].(string)))
	putU256(buf[192:224], parseInt(msg["takerAmount"].(string)))
	putU256(buf[224:256], 0)
	putU256(buf[256:288], 3)
	putU256(buf[288:320], parseInt(msg["timestamp"].(string)))
	var zero [32]byte
	copy(buf[320:352], zero[:])
	copy(buf[352:384], zero[:])
	return buf
}

func packTypedSign(contents []byte, chainID int64, wallet common.Address) []byte {
	soladyType := "TypedDataSign(Order contents,string name,string version,uint256 chainId," +
		"address verifyingContract,bytes32 salt)Order(uint256 salt,address maker,address signer,uint256 tokenId," +
		"uint256 makerAmount,uint256 takerAmount,uint8 side,uint8 signatureType," +
		"uint256 timestamp,bytes32 metadata,bytes32 builder)"
	buf := make([]byte, 224)
	copy(buf[0:32], crypto.Keccak256Hash([]byte(soladyType)).Bytes())
	copy(buf[32:64], contents)
	copy(buf[64:96], crypto.Keccak256Hash([]byte("DepositWallet")).Bytes())
	copy(buf[96:128], crypto.Keccak256Hash([]byte("1")).Bytes())
	putU256(buf[128:160], chainID)
	copy(buf[160+12:192], wallet.Bytes())
	return buf
}

func putU256(dst []byte, v int64) {
	b := new(big.Int).SetInt64(v).Bytes()
	copy(dst[32-len(b):], b)
}
func parseInt(s string) int64 {
	var n int64
	fmt.Sscan(s, &n)
	return n
}
