package products

import (
	"context"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/novellia_database"
)

type Service interface {
	GetProducts(ctx context.Context) (map[string]novellia_database.Product, error)
	// converts a product ID representing a bundle into a list of atomic product IDs
	UnpackBundleProduct(productID string) ([]string, error)
}
