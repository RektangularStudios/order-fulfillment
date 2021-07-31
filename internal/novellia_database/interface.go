package novellia_database

import (
	"context"
	"time"

	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/now_payments"
	"math/big"
)

type Service interface {
	InsertOrder(ctx context.Context, order ordf.Order, payment now_payments.CreatePaymentResponse) error
	QueryOrder(ctx context.Context, orderID string) (*ordf.Order, *now_payments.GetPaymentStatusResponse, *time.Time, error)
	UpdateOrder(ctx context.Context, order ordf.Order, payment now_payments.GetPaymentStatusResponse) error
	QueryOrdersReadyForCheck(ctx context.Context, interval time.Duration, requiredStatus string) ([]string, error)
	QueryProducts(ctx context.Context) ([]Product, error)
	GenerateULID(prefix string) string
	InsertCardanoTransaction(ctx context.Context, orderID string, txid string) error
	QueryOrderNativeTokens(ctx context.Context, orderID string) (map[string]*big.Int, error)
	InsertOrderNativeTokens(ctx context.Context, orderID string, tokens map[string]*big.Int) error
	QueryCardanoTransactions(ctx context.Context, orderID string) ([]string, error)
	QueryReservedNativeTokens(ctx context.Context) (map[string]*big.Int, error)
	Close()
}
