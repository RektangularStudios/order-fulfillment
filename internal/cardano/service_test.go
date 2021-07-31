package cardano_test

import (
	"fmt"
	"context"
	"testing"
	"math/big"

	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/novellia_database"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/products"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/cardano"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/config"
	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
)

const (
	configPath = "/config/prod-live.yaml"
)

func setupTest(ctx context.Context) (novellia_database.Service, products.Service, cardano.Service, error) {
	err := config.LoadConfig(configPath)
	if err != nil {
		return nil, nil, nil, err
	}
	config, err := config.GetConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	novelliaDatabaseService, err := novellia_database.New(
		ctx,
		config.Postgres.Username,
		config.Postgres.Password,
		config.Postgres.Host,
		config.Postgres.Database,
		config.Postgres.QueriesPath,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	productsService := products.New(novelliaDatabaseService)
	cardanoService, err := cardano.New(novelliaDatabaseService, productsService)
	if err != nil {
		return nil, nil, nil, err
	}

	return novelliaDatabaseService, productsService, cardanoService, nil
}

func TestNativeTokensFromOrder(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, productsService, cardanoService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	order := ordf.Order{
		Items: []ordf.OrderItems{
			ordf.OrderItems{
				// starter deck
				ProductId: "PROD-01F4NAFJCAG5JDEGMR0XQARBW2",
				Quantity: 2,
			},
		},
		Customer: ordf.OrderCustomer{
			DeliveryAddress: "0xYOLO",
		},
		Description: "Test Order",
	}

	n, err := cardanoService.NativeTokensFromOrder(ctx, &order)
	if err != nil {
		t.Errorf("failed to get native tokens from order: %+v", err)
	}
	
	starterDeckCards, err := productsService.UnpackBundleProduct("PROD-01F4NAFJCAG5JDEGMR0XQARBW2")
	if err != nil {
		t.Errorf("failed to unpack starter deck: %+v", err)
	}

	products, err := productsService.GetProducts(ctx)
	if err != nil {
		t.Errorf("failed to get products list: %+v", err)
	}
	
	var sumCards *big.Int
	for _, c := range starterDeckCards {
		noCards := n[products[c].NativeTokenID]
		sumCards = big.NewInt(0).Add(noCards, sumCards)
	}
	if sumCards.Cmp(big.NewInt(24)) != 0 {
		t.Errorf("wrong sum of cards, expected 24, got %d", sumCards)
	}

	t.Errorf("native tokens: %+v", n)
}

func TestGetTTL(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, cardanoService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	ttl, err := cardanoService.GetTTL()
	if err != nil {
		t.Errorf("failed to get TTL: %+v", err)
	}
	t.Errorf("TTL: %d", ttl)
}

func TestGetUTXOs(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, cardanoService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	utxos, err := cardanoService.GetUTXOs(cardanoService.HotWalletAddress(), "ORDER-ABC")
	if err != nil {
		t.Errorf("failed to get UTXOs: %+v", err)
	}
	t.Errorf("UTXOs: %+v", utxos)
}

func TestWriteRawTX(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, cardanoService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	order := ordf.Order{
		Items: []ordf.OrderItems{
			ordf.OrderItems{
				// Voyin
				ProductId: "PROD-01F4MK4YKAJ0REHHDY63TTTEWM",
				Quantity: 1,
			},
		},
		Customer: ordf.OrderCustomer{
			DeliveryAddress: "addr1",
		},
		OrderId: "ORDER-ABC",
		Description: "Test Order",
	}

	tokenQuantities, err := cardanoService.NativeTokensFromOrder(ctx, &order)
	if err != nil {
		t.Errorf("failed to get native tokens from order: %v", err)
	}

	ttl, err := cardanoService.GetTTL()
	if err != nil {
		t.Errorf("failed to get TTL: %v", err)
	}

	utxos, err := cardanoService.GetUTXOs(cardanoService.HotWalletAddress(), order.OrderId)
	if err != nil {
		t.Errorf("failed to get UTXOs: %v", err)
	}
	txRawPath := fmt.Sprintf("tx_%s.raw", order.OrderId)
	txInCount, txOutCount, err := cardanoService.WriteRawTX(order.Customer.DeliveryAddress, tokenQuantities, utxos, txRawPath, big.NewInt(0), ttl)
	if err != nil {
		t.Errorf("failed to write raw TX without fee: %v", err)
	}

	fee, err := cardanoService.GetFee(txRawPath, txInCount, txOutCount)
	if err != nil {
		t.Errorf("failed to get fee: %v", err)
	}

	// get UTXOs again because WriteRawTX modified them, TODO: do deep copy
	utxos, err = cardanoService.GetUTXOs(cardanoService.HotWalletAddress(), order.OrderId)
	if err != nil {
		t.Errorf("failed to get UTXOs: %v", err)
	}
	_, _, err = cardanoService.WriteRawTX(order.Customer.DeliveryAddress, tokenQuantities, utxos, txRawPath, fee, ttl)
	if err != nil {
		t.Errorf("failed to write raw TX with fee: %v", err)
	}
}

func TestSignTX(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, cardanoService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	orderID := "ORDER-ABC"
	txRawPath := fmt.Sprintf("tx_%s.raw", orderID)
	txSignedPath := fmt.Sprintf("tx_%s.signed", orderID)
	err = cardanoService.SignTX(txRawPath, txSignedPath)
	if err != nil {
		t.Errorf("failed to sign tx: %v", err)
	}
}

func TestSubmitTX(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, cardanoService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	txSignedPath := fmt.Sprintf("tx_%s.signed", "ORDER-ABC")
	err = cardanoService.SubmitTX(txSignedPath)
	if err != nil {
		t.Errorf("failed to submit tx: %v", err)
	}
}

func TestValidateAddress(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, cardanoService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	err = cardanoService.ValidateAddress("ORDER-ABC")
	if err == nil {
		t.Errorf("address should have been invalid, got nil error")
	}
	err = cardanoService.ValidateAddress("addr1")
	if err != nil {
		t.Errorf("address should have been valid, got error: %+v", err)
	}
}

func TestGetStock(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, cardanoService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	stock, err := cardanoService.GetStock([]string{cardanoService.HotWalletAddress()})
	if err != nil {
		t.Errorf("failed to get stock: %+v", err)
	}
	t.Errorf("got stock: %+v", stock)
}
