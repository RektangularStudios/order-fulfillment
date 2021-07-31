package cardano

import (
	"context"
	"math/big"

	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
)

type Service interface {
	NativeTokensFromOrder(ctx context.Context, order *ordf.Order) (map[string]*big.Int, error)
	GetUTXOs(address string, filenameSalt string) (*UTXOs, error)
	GetTTL() (*big.Int, error)
	WriteRawTX(deliveryAddress string, nativeTokens map[string]*big.Int, utxos *UTXOs, txRawPathOut string, feeLovelace *big.Int, ttl *big.Int) (int, int, error)
	GetFee(txRawPath string, txInCount, txOutCount int) (*big.Int, error)
	SignTX(txRawPath string, txSignedOutPath string) error
	SubmitTX(txSignedPath string) error
	GetTXID(txSignedPath string) (string, error)
	// processed an order to Cardano, returning the TXID
	SubmitOrder(ctx context.Context, order *ordf.Order) (string, error)
	ValidateAddress(address string) (error)
	GetStock(addresses []string) (map[string]*big.Int, error)
	HotWalletAddress() string
}
