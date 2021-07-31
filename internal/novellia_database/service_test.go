package novellia_database_test

import (
	"fmt"
	"time"
	"context"
	"testing"
	"math/big"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/orders"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/now_payments"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/novellia_database"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/config"
	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
)

const (
	configPath = "/config/prod-live.yaml"
)

func setupTest(ctx context.Context) (novellia_database.Service, error) {
	err := config.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	config, err := config.GetConfig()
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return novelliaDatabaseService, nil
}

func TestInsertOrder(t *testing.T) {
	ctx := context.Background()

	service, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer service.Close()

	order := ordf.Order{
		Items: []ordf.OrderItems{
			ordf.OrderItems{
				ProductId: "PROD-01F4MK45QJS4WZ1VBZW1A1THD7",
				Quantity: 3,
			},
		},
		Customer: ordf.OrderCustomer{
			DeliveryAddress: "addr1q8hax2z9wav0prwhmls59g2dz5aja7jnsz9kyqr7sa8rp0ew08lffp5n2kzt72ez93m5zev2v4fm9sawnrqnvllmyhmst2jnww",
		},
		Payment: ordf.OrderPayment{
			PriceCurrencyId: "ada",
			PriceAmount: 60,
			PaymentAddress: "addr1q8hax2z9wav0prwhmls59g2dz5aja7jnsz9kyqr7sa8rp0ew08lffp5n2kzt72ez93m5zev2v4fm9sawnrqnvllmyhmst2jnww",
			PaymentStatus: "WAITING",
		},
		Description: "Test Order",
		OrderId: "ORDER-ABC",
		OrderStatus: "AWAITING_PAYMENT",
	}

	payment := now_payments.CreatePaymentResponse{
		PaymentID: "4945313421",
		PaymentStatus: "waiting",
		PayAddress: "addr1q8hax2z9wav0prwhmls59g2dz5aja7jnsz9kyqr7sa8rp0ew08lffp5n2kzt72ez93m5zev2v4fm9sawnrqnvllmyhmst2jnww",
		PriceAmount: 60,
		PriceCurrency: "ada",
		PayAmount: 60,
		PayCurrency: "ada",
		OrderID: "ORDER-ABC",
		OrderDescription: "Test Order",
		PayInExtraID: "213546",
		IPNCallbackURL: "https://server.com/ipn",
		CreatedAt: "2021-05-11T02:00:03.859Z",
		UpdatedAt: "2021-05-11T02:00:03.859Z",
		PurchaseID: "5831731753",
	}

	err = service.InsertOrder(ctx, order, payment)
	if err != nil {
		t.Errorf("insert order failed: %+v", err)
	}
}

func TestQueryOrder(t *testing.T) {
	ctx := context.Background()

	service, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer service.Close()

	order, payment, checked_last, err := service.QueryOrder(ctx, "ORDER-ABC")
	if err != nil {
		t.Errorf("query order failed: %+v", err)
	}
	fmt.Printf("%+v, %+v, %+v", order, payment, checked_last)
}

func TestUpdateOrder(t *testing.T) {
	ctx := context.Background()

	order := ordf.Order{
		Items: []ordf.OrderItems{
			ordf.OrderItems{
				ProductId: "PROD-01F4MK45QJS4WZ1VBZW1A1THD7",
				Quantity: 3,
			},
		},
		Customer: ordf.OrderCustomer{
			DeliveryAddress: "addr1q8hax2z9wav0prwhmls59g2dz5aja7jnsz9kyqr7sa8rp0ew08lffp5n2kzt72ez93m5zev2v4fm9sawnrqnvllmyhmst2jnww",
		},
		Payment: ordf.OrderPayment{
			PriceCurrencyId: "ada",
			PriceAmount: 60,
			PaymentAddress: "addr1q8hax2z9wav0prwhmls59g2dz5aja7jnsz9kyqr7sa8rp0ew08lffp5n2kzt72ez93m5zev2v4fm9sawnrqnvllmyhmst2jnww",
			PaymentStatus: "FINISHED",
		},
		Description: "Test Order",
		OrderId: "ORDER-ABC",
		OrderStatus: "PAID",
	}

	payment := now_payments.GetPaymentStatusResponse{
		PaymentID: "4945313421",
		PaymentStatus: "finished",
		PayAddress: "addr1q8hax2z9wav0prwhmls59g2dz5aja7jnsz9kyqr7sa8rp0ew08lffp5n2kzt72ez93m5zev2v4fm9sawnrqnvllmyhmst2jnww",
		PriceAmount: 60,
		PriceCurrency: "ada",
		PayAmount: 60,
		PayCurrency: "ada",
		OrderID: "ORDER-ABC",
		OrderDescription: "Test Order",
		CreatedAt: "2021-05-11T02:00:03.859Z",
		UpdatedAt: "2021-05-12T02:00:03.859Z",
		PurchaseID: "5831731753",
		ActuallyPaid: 60,
		OutcomeAmount: 60,
		OutcomeCurrency: "ada",
	}

	service, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer service.Close()

	err = service.UpdateOrder(ctx, order, payment)
	if err != nil {
		t.Errorf("update order failed: %+v", err)
	}
}

func TestQueryOrdersReadyForCheck(t *testing.T) {
	ctx := context.Background()

	service, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer service.Close()

	interval := 1 * time.Minute
	orderIDs, err := service.QueryOrdersReadyForCheck(ctx, interval, orders.ORDER_STATUS_AWAITING_PAYMENT)
	if err != nil {
		t.Errorf("query products failed: %+v", err)
	}
	t.Errorf("%+v", orderIDs)
}

func TestQueryProducts(t *testing.T) {
	ctx := context.Background()

	service, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer service.Close()

	products, err := service.QueryProducts(ctx)
	if err != nil {
		t.Errorf("query products failed: %+v", err)
	}
	fmt.Printf("%+v", products)
}

func TestQueryOrderNativeTokens(t *testing.T) {
	ctx := context.Background()

	service, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer service.Close()

	orderID := "ORDER-ABC"
	tokens, err := service.QueryOrderNativeTokens(ctx, orderID)
	if err != nil {
		t.Errorf("query order native tokens failed: %+v", err)
	}
	t.Errorf("%+v", tokens)
}

func TestInsertOrderNativeTokens(t *testing.T) {
	ctx := context.Background()

	service, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer service.Close()

	orderID := "ORDER-ABC"
	tokens := map[string]*big.Int{
		"0xRektangularStudios.Draculi": big.NewInt(2),
		"0xRektangularStudios.IscaraTheTenThousandGuns": big.NewInt(4),
	}
	err = service.InsertOrderNativeTokens(ctx, orderID, tokens)
	if err != nil {
		t.Errorf("insert order native tokens failed: %+v", err)
	}
}

func TestQueryCardanoTransactions(t *testing.T) {
	ctx := context.Background()

	service, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer service.Close()

	orderID := "ORDER-01F68KR9PRJBGPPRGRJEPNP85J"
	txids, err := service.QueryCardanoTransactions(ctx, orderID)
	if err != nil {
		t.Errorf("query Cardano transactions failed: %+v", err)
	}
	t.Errorf("txids: %+v", txids)
}

func TestQueryReservedNativeTokens(t *testing.T) {
	ctx := context.Background()

	service, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer service.Close()

	reservedTokens, err := service.QueryReservedNativeTokens(ctx)
	if err != nil {
		t.Errorf("query reserved native tokens failed: %+v", err)
	}
	t.Errorf("reserved tokens: %+v", reservedTokens)
}
