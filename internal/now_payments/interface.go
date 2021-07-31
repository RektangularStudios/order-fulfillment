package now_payments

import (
	"context"
	"net/http"
)

type Service interface {
	Status(ctx context.Context) (string, error)
	CreatePayment(ctx context.Context, createPaymentRequest CreatePaymentRequest) (*CreatePaymentResponse, error)
	GetPaymentStatus(ctx context.Context, paymentID string) (*GetPaymentStatusResponse, error)
	IPNWebhookValidate(r *http.Request) (*GetPaymentStatusResponse, error)
}
