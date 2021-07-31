package orders_test

import (
	"fmt"
	"context"
	"testing"

	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/novellia_database"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/now_payments"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/products"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/orders"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/config"
	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
)

const (
	configPath = "/config/prod-live.yaml"
)

func setupTest(ctx context.Context) (novellia_database.Service, now_payments.Service, products.Service, orders.Service, error) {
	err := config.LoadConfig(configPath)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	config, err := config.GetConfig()
	if err != nil {
		return nil, nil, nil, nil, err
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
		return nil, nil, nil, nil, err
	}

	nowPaymentsService, err := now_payments.New(
		config.NowPayments.APIKey,
		config.NowPayments.IPNSecretKey,
		config.NowPayments.IsSandbox,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	productsService := products.New(novelliaDatabaseService)
	ordersService := orders.New(novelliaDatabaseService, nowPaymentsService, productsService)

	return novelliaDatabaseService, nowPaymentsService, productsService, ordersService, nil
}

// TODO: add more test cases (including failures)
// this will fail rn because there are no presently listed products to test
func TestValidateOrder(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, _, ordersService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	order := ordf.Order{
		Items: []ordf.OrderItems{
			ordf.OrderItems{
				ProductId: "PROD-01F4MK45QJS4WZ1VBZW1A1THD7",
				Quantity: 3,
			},
			ordf.OrderItems{
				ProductId: "PROD-01F4MK4YVW4JSV717E0XK920AZ",
				Quantity: 2,
			},
		},
		Customer: ordf.OrderCustomer{
			DeliveryAddress: "addr1q8hax2z9wav0prwhmls59g2dz5aja7jnsz9kyqr7sa8rp0ew08lffp5n2kzt72ez93m5zev2v4fm9sawnrqnvllmyhmst2jnww",
		},
		Payment: ordf.OrderPayment{
			PriceCurrencyId: "ada",
			PriceAmount: 80,
			PaymentAddress: "addr1q8hax2z9wav0prwhmls59g2dz5aja7jnsz9kyqr7sa8rp0ew08lffp5n2kzt72ez93m5zev2v4fm9sawnrqnvllmyhmst2jnww",
			PaymentStatus: "FINISHED",
		},
		Description: "Test Order",
		OrderId: "ORDER-ABC",
		OrderStatus: "PAID",
	}

	err = ordersService.ValidateOrder(ctx, order)
	if err != nil {
		t.Errorf("validate order failed: %+v", err)
	}
}

func TestCreateOrder(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, _, ordersService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	order := ordf.Order{
		Items: []ordf.OrderItems{
			ordf.OrderItems{
				ProductId: "PROD-01F4MK45QJS4WZ1VBZW1A1THD7",
				Quantity: 3,
			},
			ordf.OrderItems{
				ProductId: "PROD-01F4MK4YVW4JSV717E0XK920AZ",
				Quantity: 2,
			},
		},
		Customer: ordf.OrderCustomer{
			DeliveryAddress: "addr1q8hax2z9wav0prwhmls59g2dz5aja7jnsz9kyqr7sa8rp0ew08lffp5n2kzt72ez93m5zev2v4fm9sawnrqnvllmyhmst2jnww",
		},
		Payment: ordf.OrderPayment{
			PriceCurrencyId: "ada",
			PriceAmount: 80,
		},
		Description: "Test Order",
	}

	orderID, err := ordersService.CreateOrder(ctx, order)
	if err != nil {
		t.Errorf("validate order failed: %+v", err)
	}
	fmt.Printf("order ID: %s", orderID)
}

func TestGetOrder(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, _, ordersService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	// set this from your own run of TestCreateOrder
	orderID := "ORDER-01F5NY7WJ93YFC7Q00B2EWDPJ3"
	order, err := ordersService.GetOrder(ctx, orderID)
	if err != nil {
		t.Errorf("validate order failed: %+v", err)
	}
	fmt.Printf("order: %+v", order)
}

func TestCheckAndUpdateOrderPayment(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, _, _, ordersService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	// set this from your own run of TestCreateOrder
	orderID := "ORDER-01F5NY7WJ93YFC7Q00B2EWDPJ3"
	order, err := ordersService.CheckAndUpdateOrderPayment(ctx, orderID)
	if err != nil {
		t.Errorf("check and update order failed: %+v", err)
	}
	fmt.Printf("order: %+v", order)
}

func TestIPNUpdateOrder(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, nowPaymentsService, _, ordersService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()

	// set this from your own run of TestCreateOrder
	paymentID := "5533008157"
	payment, err := nowPaymentsService.GetPaymentStatus(ctx, paymentID)
	if err != nil {
		t.Errorf("IPN update order failed at getting payment status: %+v", err)
	}

	err = ordersService.IPNUpdateOrder(ctx, *payment)
	if err != nil {
		t.Errorf("IPN update order failed: %+v", err)
	}
}
