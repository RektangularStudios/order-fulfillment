package api

import (
	"fmt"
	"context"
	"net/http"

	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/now_payments"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/orders"
	//prometheus_monitoring "bitbucket.org/ConcurrentDragon/order-fulfillment/internal/monitoring"
)

type ApiServicer interface {
	ordf.DefaultApiServicer
	IPNWebhook(w http.ResponseWriter, r *http.Request)
}

type ApiService struct{
	nowPaymentsService now_payments.Service
	ordersService orders.Service
}

// NewApiService creates an api service
func NewApiService(
	nowPaymentsService now_payments.Service,
	ordersService orders.Service,
	) ApiServicer {
	return &ApiService {
		nowPaymentsService: nowPaymentsService,
		ordersService: ordersService,
	}
}

// Health check for microservice
func (s *ApiService) GetStatus(ctx context.Context) (ordf.ImplResponse, error) {
	status := ordf.Status{
		Status: "UP",
	}

	// check NowPayments API
	nowPaymentsStatus, err := s.nowPaymentsService.Status(ctx)
	if err != nil {
		status.Status = fmt.Sprintf("Failed to check NowPayments status: %+v", err)
	} else if nowPaymentsStatus != "OK" {
		status.Status = fmt.Sprintf("NowPayments is down: %s", nowPaymentsStatus)
	}

	return ordf.Response(200, status), nil
}

// Gets an order by id
func (s *ApiService) GetOrders(ctx context.Context, orderId string) (ordf.ImplResponse, error) {
	// TODO: implement long polling on separate polling API
	// will need to update SDK to pass in last known state and check for change
	order, err := s.ordersService.GetOrder(ctx, orderId)
	if err != nil {
		return ordf.Response(500, nil), err
	}

	return ordf.Response(200, order), nil
}

// Creates an order and returns the order_id
func (s *ApiService) PostOrders(ctx context.Context, order ordf.Order) (ordf.ImplResponse, error) {
	orderID, err := s.ordersService.CreateOrder(ctx, order)
	if err != nil {
		return ordf.Response(500, nil), err
	}

	return ordf.Response(200, ordf.OrderCreated{
		OrderId: orderID,
	}), nil
}

type IPNResponse struct {
	Code string
	Body interface{}
}

// receives NowPayments IPN callbacks
func (s *ApiService) IPNWebhook(w http.ResponseWriter, r *http.Request) {
	// return 200 on success, otherwise nothing (NowPayments will retry)

	/*
	ctx := context.Background()

	// cryptographically validate webhook payload
	payment, err := s.nowPaymentsService.IPNWebhookValidate(r)
	if err != nil {
		fmt.Printf("Failed to validate IPNWebhook: %+v", err)
		prometheus_monitoring.TickNowPaymentsIPNFailed()
		return
	}

	// update order in database
	err = s.ordersService.IPNUpdateOrder(ctx, *payment)
	if err != nil {
		fmt.Printf("Failed to update order in IPNWebhook: %+v", err)
		prometheus_monitoring.TickNowPaymentsIPNFailed()
		return
	}
	*/

	fmt.Printf("Hit IPN\n")
	w.WriteHeader(http.StatusOK)
}
