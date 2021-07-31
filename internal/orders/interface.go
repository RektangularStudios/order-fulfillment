package orders

import (
	"context"
	"math/big"

	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/now_payments"
)

type Service interface {
	ValidateOrder(ctx context.Context, order ordf.Order) error
	ValidateStockAvailable(ctx context.Context, tokens map[string]*big.Int) error
	CreateOrder(ctx context.Context, order ordf.Order) (string, error)
	GetOrder(ctx context.Context, orderID string) (*ordf.Order, error)
	CheckAndUpdateOrderPayment(ctx context.Context, orderID string) (*ordf.Order, error)
	IPNUpdateOrder(ctx context.Context, payment now_payments.GetPaymentStatusResponse) error
	WatchOrdersForPayment(ctx context.Context)
	WatchOrdersForFulfillment(ctx context.Context)
}
