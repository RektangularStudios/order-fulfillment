package api

import (
	"context"
	"net/http"

	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
)

type MockedApiService struct{}

// MockedNewApiService creates an api service
func NewMockedApiService() ApiServicer {
	return &MockedApiService {}
}

// Health check for microservice
func (s *MockedApiService) GetStatus(ctx context.Context) (ordf.ImplResponse, error) {
	resp := ordf.Status{
		Maintenance: false,
		Status: "UP",
	}

	return ordf.Response(200, resp), nil
}

// Gets an order by id
func (s *MockedApiService) GetOrders(ctx context.Context, productId string) (ordf.ImplResponse, error) {
	order := ordf.Order{
		Items: []ordf.OrderItems{
			ordf.OrderItems{
				ProductId: "PROD-01D78XYFJ1PRM1WPBAOU8JQMNV",
				Quantity: 4,
			},
			ordf.OrderItems{
				ProductId: "PROD-01D78XYFJ1PRM1WPBCBT3VHMNV",
				Quantity: 2,
			},
		},
		Customer: ordf.OrderCustomer{
			DeliveryAddress: "addr1",
		},
		Payment: ordf.OrderPayment{
			PaymentAddress: "addr1",
			PriceCurrencyId: "ada",
			PriceAmount: 20,
			PaymentStatus: "AWAITING_PAYMENT",
		},
		OrderStatus: "AWAITING_PAYMENT",
		Description: "Occulta Novellia Presale Order",
		OrderId: "ORDER-01D78XYFJ1PRM1WPBCBT3VHMNV",
	}

	return ordf.Response(200, order), nil
}

// Creates an order and returns the order_id
func (s *MockedApiService) PostOrders(context.Context, ordf.Order) (ordf.ImplResponse, error) {
	orderCreated := ordf.OrderCreated{
		OrderId: "ORDER-01D78XYFJ1PRM1WPBCBT3VHMNV",
	}

	return ordf.Response(200, orderCreated), nil
}

// receives NowPayments IPN callbacks
func (s *MockedApiService) IPNWebhook(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
