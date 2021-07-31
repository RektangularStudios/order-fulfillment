package prometheus_monitoring

import (
	"fmt"
	"net/http"
	"time"
	"encoding/json"
	"io/ioutil"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/config"
)

// https://prometheus.io/docs/guides/go-application/

const (
	namespace = "order_fulfillment"
	status_interval = 30 * time.Second
)

var (
	microserviceStatusMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name: "microservice_status",
		Help: "Health status indicator for order-fulfillment microservice",
	})
	nowPaymentsStatusMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name: "now_payments_status",
		Help: "Health status indicator for NowPayments",
	})
	createdOrderMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name: "created_order",
		Help: "The total number of times an order was successfully created",
	})
	paymentCreateWithoutOrderMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name: "payments_created_without_order",
		Help: "The total number of times a payment was created on NowPayments, but the order wasn't registered successfully in the database",
	})
	nowPaymentsIPNFailedMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name: "now_payments_ipn_failed",
		Help: "The total number of times the IPN webhook failed",
	})
	watchOrdersForPaymentStatusMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name: "watch_orders_for_payment_status",
		Help: "Health status indicator for WatchOrdersForPayment goroutine",
	})
	watchOrdersForFulfillmentStatusMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name: "watch_orders_for_fulfillment_status",
		Help: "Health status indicator for WatchOrdersForFulfillment goroutine",
	})
	cardanoSubmitOrderFailedMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name: "cardano_submit_order_failed",
		Help: "The total number of times the fulfilling tokens through Cardano has failed",
	})
	cardanoInsufficientUTXOsMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name: "cardano_insufficient_utxos",
		Help: "The total number of times the fulfilling tokens through Cardano has failed because of insufficient UTXOs",
	})
	cardanoSubmittedMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name: "cardano_submitted",
		Help: "The total number of times an order has been submitted successfully to Cardano",
	})
	validateStockFailedMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name: "validate_stock_failed_metric",
		Help: "The total number of times there wasn't enough stock to reserve an order",
	})
	/*
	walletStockHistogramMetric = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name: "wallet_stock_histogram",
		Help: "Histogram of wallet stock",
	})
	reservedStockHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name: "reserved_stock_histogram",
		Help: "Histogram of reserved stock",
	})
	*/
)

type statusIndicators struct {
	microserviceStatus float64
	nowPaymentsStatus float64
}

func getStatus() (*statusIndicators, error) {
	config, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get config from env")
	}

	req, err := http.NewRequest("GET", config.Monitoring.StatusURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status health check failed: %+v", resp)
	}

	var respBody ordf.Status
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bodyBytes, &respBody)
	if err != nil {
		return nil, err
	}

	s := statusIndicators{
		microserviceStatus: 0,
		nowPaymentsStatus: 0,
	}
	if respBody.Status == "UP" {
		// TODO: separate these values
		s.microserviceStatus = 1
		s.nowPaymentsStatus = 1
	}
	fmt.Printf("Checked Status, result: %s\n", respBody.Status)

	return &s, nil
}

func RecordMetrics() {
	go func() {
		for {
			indicators, err := getStatus()
			if err != nil {
				indicators = &statusIndicators{
					microserviceStatus: 0,
					nowPaymentsStatus: 0,
				}
				fmt.Printf("Checked status, got error: %+v\n", err)
			}
			
			microserviceStatusMetric.Set(indicators.microserviceStatus)
			nowPaymentsStatusMetric.Set(indicators.nowPaymentsStatus)

			time.Sleep(status_interval)
		}
	}()
}

func TickCreatedOrder() {
	createdOrderMetric.Inc()
}

func TickPaymentCreatedWithoutOrder() {
	paymentCreateWithoutOrderMetric.Inc()
}

func TickNowPaymentsIPNFailed() {
	nowPaymentsIPNFailedMetric.Inc()
}

func SetWatchOrdersForPaymentStatus(status float64) {
	watchOrdersForPaymentStatusMetric.Set(status)
}

func SetWatchOrdersForFulfillmentStatus(status float64) {
	watchOrdersForFulfillmentStatusMetric.Set(status)
}

func TickCardanoSubmitOrderFailed() {
	cardanoSubmitOrderFailedMetric.Inc()
}

func TickCardanoInsufficientUTXOs() {
	cardanoInsufficientUTXOsMetric.Inc()
}

func TickSubmittedToCardano() {
	cardanoSubmittedMetric.Inc()
}

func TickValidateStockFailed() {
	validateStockFailedMetric.Inc()
}
