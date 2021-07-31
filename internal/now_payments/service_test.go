package now_payments_test

import (
	"bytes"
	"net/http"
	"fmt"
	"context"
	"testing"
	now_payments "bitbucket.org/ConcurrentDragon/order-fulfillment/internal/now_payments"
)

const (
	sandboxAPIKey = "X"
	sandboxIPNSecretKey = "X"
	demoIPNWebhookURL = "https://api-demo.rektangularstudios.com/order-fulfillment/ipn"
)

func TestStatus(t *testing.T) {
	ctx := context.Background()
	service, err := now_payments.New(sandboxAPIKey, sandboxIPNSecretKey, true)
	if err != nil {
		t.Errorf("failed to create NowPayments service")
	}

	status, err := service.Status(ctx)
	if err != nil {
		t.Errorf("failed to get status: %+v", err)
	}
	if status != "OK" {
		t.Errorf("API not OK: %+v", err)
	}
}

func TestCreatePayment(t *testing.T) {
	ctx := context.Background()
	service, err := now_payments.New(sandboxAPIKey, sandboxIPNSecretKey, true)
	if err != nil {
		t.Errorf("failed to create NowPayments service")
	}

	req := now_payments.CreatePaymentRequest{
		PriceAmount : 10,
		PriceCurrency: "ada",
		PayCurrency: "ada",
		IPNCallbackURL: demoIPNWebhookURL,
		OrderID: "ORDER-123",
		OrderDescription: "Test Order",
		Case: "success",
	}
	resp, err := service.CreatePayment(ctx, req)
	if err != nil {
		t.Errorf("failed to create payment: %+v", err)
	}
	
	fmt.Printf("\nCreatePayment resp: %+v\n", resp)
}

func TestGetPaymentStatus(t *testing.T) {
	ctx := context.Background()
	service, err := now_payments.New(sandboxAPIKey, sandboxIPNSecretKey, true)
	if err != nil {
		t.Errorf("failed to create NowPayments service")
	}

	resp, err := service.GetPaymentStatus(ctx, "6199378400")
	if err != nil {
		t.Errorf("failed to get payment status: %+v", err)
	}

	fmt.Printf("\nGetPaymentStatus resp: %+v\n", resp)
}

func TestIPNWebhookValidateWithoutCase(t *testing.T) {
	service, err := now_payments.New(sandboxAPIKey, sandboxIPNSecretKey, true)
	if err != nil {
		t.Errorf("failed to create NowPayments service")
	}

	jsonBody := `
	{
		"payment_id":4945313421,
		"payment_status":"confirming",
		"pay_address":"sandBox_ada_address",
		"price_amount":10,
		"price_currency":"ada",
		"pay_amount":10,
		"actually_paid":10,
		"pay_currency":"ada",
		"order_id":"ORDER-66",
		"order_description":"Test Order",
		"purchase_id":"5831731753",
		"created_at":"2021-05-11T02:00:03.859Z",
		"updated_at":"2021-05-11T02:04:00.061Z",
		"outcome_amount":9.9,
		"outcome_currency":"ada"
 }
	`
	sigHeader := "bdae844b7cae55306d5bae9ed52f86ca96e667ee4e22180a7aa7b3ac4fadb2ed437da53ff62556ef6cfb7c4749271b2acb407f92659643ae69c0395d94cc7529"

	r, err := http.NewRequest("POST", demoIPNWebhookURL, bytes.NewBuffer([]byte(jsonBody)))
	if err != nil {
		t.Errorf("failed to create IPN webhook validation request: %+v", err)
	}
	r.Header.Add("X-Nowpayments-Sig", sigHeader)

	resp, err := service.IPNWebhookValidate(r)
	if err != nil {
		t.Errorf("failed IPN Webhook validate: %+v", err)
	}

	fmt.Printf("\nIPNWebhookValidate resp: %+v\n", resp)
}
