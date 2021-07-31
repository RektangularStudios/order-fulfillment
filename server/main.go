package main

import (
	"fmt"
	"net/http"
	"os"
	"context"
	
	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
	
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/config"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/api"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/novellia_database"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/now_payments"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/orders"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/products"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/cardano"
	prometheus_monitoring "bitbucket.org/ConcurrentDragon/order-fulfillment/internal/monitoring"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx := context.Background()

	fmt.Printf("Order Fulfillment Server - Version %s\n", version)

	configPath, err := config.GetConfigPath()
	if err != nil {
		fmt.Printf("Failed to get config path: %v\n", err)
		os.Exit(configPathErr)
	}

	err = config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(configLoadErr)
	}
	config, err := config.GetConfig()
	if err != nil {
		fmt.Printf("Failed to get config from env: %v\n", err)
		os.Exit(configGetErr)
	}

	fmt.Printf("Starting server with configuration (%s):\n %+v\n", configPath, config)

	var apiService api.ApiServicer
	if config.Mocked {
		apiService = api.NewMockedApiService()
	} else {	
		novelliaDatabaseService, err := novellia_database.New(
			ctx,
			config.Postgres.Username,
			config.Postgres.Password,
			config.Postgres.Host,
			config.Postgres.Database,
			config.Postgres.QueriesPath,
		)
		if err != nil {
			fmt.Printf("Failed to make Novellia database service: %+v\n", err)
			os.Exit(novelliaDatabaseErr)
		}
		defer novelliaDatabaseService.Close()

		nowPaymentsService, err := now_payments.New(config.NowPayments.APIKey, config.NowPayments.IPNSecretKey, config.NowPayments.IsSandbox)
		if err != nil {
			fmt.Printf("Failed to create NowPayments service: %+v\n", err)
			os.Exit(nowPaymentsErr)
		}

		productsService := products.New(novelliaDatabaseService)

		cardanoService, err := cardano.New(novelliaDatabaseService, productsService)
		if err != nil {
			fmt.Printf("Failed to create Cardano service: %+v\n", err)
			os.Exit(cardanoErr)
		}

		ordersService := orders.New(
			novelliaDatabaseService,
			nowPaymentsService,	
			productsService,
			cardanoService,
		)
		ordersService.WatchOrdersForPayment(ctx)
		ordersService.WatchOrdersForFulfillment(ctx)

		apiService = api.NewApiService(
			nowPaymentsService,
			ordersService,
		)
	}

	apiController := ordf.NewDefaultApiController(apiService)
	router := ordf.NewRouter(apiController)
	
	// add IPN webhook to router
	router.Handle("/order-fulfillment/v0/ipn", http.HandlerFunc(apiService.IPNWebhook)).
		Methods("POST").
		Name("IPNWebhook").
		GetError()
	if err != nil {
		fmt.Printf("Failed to add webhook: %+v\n", err)
		os.Exit(routerErr)
	}

	// add Prometheus metrics to router
	prometheus_monitoring.RecordMetrics()
	router.Handle("/metrics", promhttp.Handler())

	hostString := fmt.Sprintf("%s:%s", config.Server.Host, config.Server.Port)
	server := http.Server {
		Addr: hostString,
		Handler: router,
	}
	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}

	os.Exit(successCode)
}
