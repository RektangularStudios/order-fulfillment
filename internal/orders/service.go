package orders

import (
	"fmt"
	"context"
	"time"
	"math/big"
	"sync"

	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/novellia_database"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/now_payments"
	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
	prometheus_monitoring "bitbucket.org/ConcurrentDragon/order-fulfillment/internal/monitoring"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/products"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/cardano"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/constants"
)

const (
	ORDER_STATUS_AWAITING_PAYMENT = "AWAITING_PAYMENT"
	ORDER_STATUS_PAID = "PAID"
	ORDER_STATUS_FILLED = "FILLED"
	ORDER_STATUS_PARTIALLY_FILLED = "PARTIALLY_FILLED"
	ORDER_STATUS_REFUND = "REFUND"
	ORDER_STATUS_FAILED = "FAILED"
)

const (
	PAYMENT_STATUS_WAITING = "WAITING"
	PAYMENT_STATUS_CONFIRMING = "CONFIRMING"
	PAYMENT_STATUS_CONFIRMED = "CONFIRMED"
	PAYMENT_STATUS_SENDING = "SENDING"
	PAYMENT_STATUS_PARTIALLY_PAID = "PARTIALLY_PAID"
	PAYMENT_STATUS_FINISHED = "FINISHED"
	PAYMENT_STATUS_FAILED = "FAILED"
	PAYMENT_STATUS_REFUNDED =	"REFUNDED"
	PAYMENT_STATUS_EXPIRED = "EXPIRED"
)

const (
	checkOrdersForPaymentInterval = 1 * time.Minute
	checkOrdersForPaymentRateLimit = 100 * time.Millisecond // 0.1 seconds per API call
	checkOrdersForFulfillmentInterval = 1 * time.Minute
	checkOrdersForFulfillmentRateLimit = 60 * time.Second // 3 * Cardano blocktime
)

type ServiceImpl struct {
	novelliaDatabaseService novellia_database.Service
	nowPaymentsService now_payments.Service
	productsService products.Service
	cardanoService cardano.Service
	createOrderMutex sync.Mutex
}

// creates a new ServiceImpl
func New(
	novelliaDatabaseService novellia_database.Service,
	nowPaymentsService now_payments.Service,
	productsService products.Service,
	cardanoService cardano.Service,
) *ServiceImpl {
	return &ServiceImpl {
		novelliaDatabaseService: novelliaDatabaseService,
		nowPaymentsService: nowPaymentsService,
		productsService: productsService,
		cardanoService: cardanoService,
	}
}

func (s *ServiceImpl) mapNowPaymentsStatus(nowPaymentsStatus string) (string, error) {
	switch nowPaymentsStatus {
	case "waiting":
		return PAYMENT_STATUS_WAITING, nil
	case "confirming":
		return PAYMENT_STATUS_CONFIRMING, nil
	case "confirmed":
		return PAYMENT_STATUS_CONFIRMED, nil
	case "sending":
		return PAYMENT_STATUS_SENDING, nil
	case "partially_paid":
		return PAYMENT_STATUS_PARTIALLY_PAID, nil
	case "finished":
		return PAYMENT_STATUS_FINISHED, nil
	case "failed":
		return PAYMENT_STATUS_FAILED, nil
	case "refunded":
		return PAYMENT_STATUS_REFUNDED, nil
	case "expired":
		return PAYMENT_STATUS_EXPIRED, nil
	default:
		return "", fmt.Errorf("failed to map NowPayments status, unknown status")
	}
}

func (s *ServiceImpl) ValidateOrder(ctx context.Context, order ordf.Order) error {
	products, err := s.productsService.GetProducts(ctx)
	if err != nil {
		return err
	}

	var totalCost float64 = 0
	for _, v := range order.Items {
		if _, ok := products[v.ProductId]; !ok {
			return fmt.Errorf("product ID does not exist: %s", v.ProductId)
		}
		p := products[v.ProductId]


		// prevent purchasing some products
		// TODO: fix the date issue with unavailable products
		if p.ProductID == "PROD-01F5YTNB4BSBKPGRKHVHEM9F0F" {
			return fmt.Errorf("collector's kit should be delisted. this should never be hit")
		}
		if p.ProductID == "PROD-01F4MK4ZCVTKAAZF1QZAPWMPFP" {
			return fmt.Errorf("cannot order Glacial Draculi directly")
		}
		if p.ProductID == "PROD-01F4MK4ZNC8FMVR2ANHDW9E1N4" {
			return fmt.Errorf("cannot order Cryptic Cat directly")
		}
		if p.ProductID == "PROD-01F4MK4ZYC6P9EGG4W0DNFQTWS" {
			return fmt.Errorf("cannot order Ghost Rotakin directly")
		}

		// TODO: fix this
		// check that product has been listed and is available
		/*
		if p.DateListed != nil && p.DateListed.After(time.Now()) {
			return fmt.Errorf("tried to order product that is not listed yet: %s is available on %s", p.ProductID, p.DateListed.String())
		}
		if p.DateAvailable != nil && p.DateAvailable.After(time.Now()) {
			return fmt.Errorf("tried to order product that is not available yet: %s is available on %s", p.ProductID, p.DateAvailable.String())
		}
		*/

		if v.Quantity <= 0 {
			return fmt.Errorf("product quantity must be greater than 0, %d", v.Quantity)
		}
		if v.Quantity > int32(p.MaxOrderSize) {
			return fmt.Errorf("cannot order more than %d of product %s. tried to order %d", p.MaxOrderSize, p.ProductID, v.Quantity)
		}
		// this is a restriction checked on the DB, not the order
		if p.PriceUnitAmount <= 0 {
			return fmt.Errorf("price unit amount cannot be negative, %v", p.PriceUnitAmount)
		}

		// verify payment values
		totalCost += float64(v.Quantity) * p.PriceUnitAmount
		if p.PriceCurrencyID != order.Payment.PriceCurrencyId {
			return fmt.Errorf("order currency_id does not match listed currency_id: %s, %s (listing) != %s (order)", p.ProductID, p.PriceCurrencyID, order.Payment.PriceCurrencyId)
		}
	}

	if totalCost != float64(order.Payment.PriceAmount) {
		return fmt.Errorf("total order value does not match listed total value (including integrated min-ada deposit + processing fee): %f (listing) != %f (order)", totalCost, order.Payment.PriceAmount)
	}
	if totalCost <= float64(constants.MinADA + constants.OrderFee) {
		return fmt.Errorf("total order value must be greater than min-ada + processing fee")
	}

	// validate Cardano address
	err = s.cardanoService.ValidateAddress(order.Customer.DeliveryAddress)
	if err != nil {
		return fmt.Errorf("got invalid customer address: %s, %+v", order.Customer.DeliveryAddress, err)
	}

	// verify currency_id
	if order.Payment.PriceCurrencyId != "ada" {
		return fmt.Errorf("received unaccepted payment currency_id, only ADA is accepted at this time: %s", order.Payment.PriceCurrencyId)
	}

	return nil
}

func (s *ServiceImpl) ValidateStockAvailable(ctx context.Context, tokens map[string]*big.Int) error {
	reservedTokens, err := s.novelliaDatabaseService.QueryReservedNativeTokens(ctx)
	if err != nil {
		fmt.Printf("failed to query reserved native tokens: %+v\n", err)
		return err
	}

	availableTokens, err := s.cardanoService.GetStock([]string{s.cardanoService.HotWalletAddress()})
	if err != nil {
		fmt.Printf("failed to query available (wallet) native tokens: %+v\n", err)
		return err
	}

	for nativeTokenID, requiredQuantity := range tokens {
		var amountReserved *big.Int
		if _, ok := reservedTokens[nativeTokenID]; !ok {
			amountReserved = big.NewInt(0)
		} else {
			amountReserved = reservedTokens[nativeTokenID]
		}

		if _, ok := availableTokens[nativeTokenID]; !ok {
			return fmt.Errorf("%s has no tokens available in wallet, wanted %d", nativeTokenID, requiredQuantity)
		}

		// available - reserved - min-leftover in hot wallet
		adjustedStockAvailable := big.NewInt(0).Sub(availableTokens[nativeTokenID], amountReserved)
		adjustedStockAvailable = big.NewInt(0).Sub(adjustedStockAvailable, big.NewInt(constants.MinUnreservedStockPerNativeToken))
		// if stock leftover is negative, clip to 0
		if adjustedStockAvailable.Cmp(big.NewInt(0)) == -1 {
			adjustedStockAvailable = big.NewInt(0)
		}

		// throw an error if required > adjusted_available
		if requiredQuantity.Cmp(adjustedStockAvailable) == 1 {
			return fmt.Errorf("%s not enough unreserved tokens available, wanted %d > %d", nativeTokenID, requiredQuantity, adjustedStockAvailable)
		}
	}

	return nil
}

func (s *ServiceImpl) CreateOrder(ctx context.Context, order ordf.Order) (string, error) {
	// this is not thread-safe
	s.createOrderMutex.Lock()
	defer s.createOrderMutex.Unlock()

	// set default order status
	order.OrderStatus = ORDER_STATUS_AWAITING_PAYMENT

	err := s.ValidateOrder(ctx, order)
	if err != nil {
		return "", fmt.Errorf("failed to validate order: %+v", err)
	}

	nativeTokens, err := s.cardanoService.NativeTokensFromOrder(ctx, &order)
	if err != nil {
		return "", err
	}
	err = s.ValidateStockAvailable(ctx, nativeTokens)
	if err != nil {
		prometheus_monitoring.TickValidateStockFailed()
		return "", fmt.Errorf("failed to validate stock available %+v", err)
	}

	orderULID := s.novelliaDatabaseService.GenerateULID("ORDER")
	order.OrderId = orderULID
	if order.OrderId == "" {
		return "", fmt.Errorf("failed to create order, got empty OrderId")
	}

	// check that order does not already exist
	_, _, _, err = s.novelliaDatabaseService.QueryOrder(ctx, order.OrderId)
	if err == nil {
		return "", fmt.Errorf("failed to create order, %s already exists", order.OrderId)
	}

	createPaymentRequest := now_payments.CreatePaymentRequest{
		// we record the actual amount paid X, but only require receipt of X - OrderFee on NowPayments
		PriceAmount: float64(order.Payment.PriceAmount) - float64(constants.OrderFee),
		PriceCurrency: order.Payment.PriceCurrencyId,
		PayCurrency: order.Payment.PriceCurrencyId,
		OrderID: order.OrderId,
		OrderDescription: order.Description,
	}
	createPaymentResponse, err := s.nowPaymentsService.CreatePayment(ctx, createPaymentRequest)
	if err != nil {
		return "", err
	}

	// add created payment information to order
	order.Payment.PaymentAddress = createPaymentResponse.PayAddress
	paymentStatus, err := s.mapNowPaymentsStatus(createPaymentResponse.PaymentStatus)
	if err != nil {
		return "", err
	}
	order.Payment.PaymentStatus = paymentStatus

	err = s.novelliaDatabaseService.InsertOrder(ctx, order, *createPaymentResponse)
	if err != nil {
		prometheus_monitoring.TickPaymentCreatedWithoutOrder()
		return "", err
	}

	err = s.novelliaDatabaseService.InsertOrderNativeTokens(ctx, order.OrderId, nativeTokens)
	if err != nil {
		return "", err
	}

	prometheus_monitoring.TickCreatedOrder()
	return order.OrderId, nil
}

func (s *ServiceImpl) addPaymentToOrder(order *ordf.Order, payment *now_payments.GetPaymentStatusResponse) error {
	order.Payment.PaymentAddress = payment.PayAddress
	paymentStatus, err := s.mapNowPaymentsStatus(payment.PaymentStatus)
	if err != nil {
		return err
	}
	order.Payment.PaymentStatus = paymentStatus

	return nil
}

func (s *ServiceImpl) updateOrderStatus(order *ordf.Order, payment *now_payments.GetPaymentStatusResponse) error {
	paymentStatus, err := s.mapNowPaymentsStatus(payment.PaymentStatus)
	if err != nil {
		return err
	}
	if paymentStatus == PAYMENT_STATUS_FINISHED && order.OrderStatus == ORDER_STATUS_AWAITING_PAYMENT {
		order.OrderStatus = ORDER_STATUS_PAID
	}
	// TODO: handle reservations
	if paymentStatus == PAYMENT_STATUS_EXPIRED || paymentStatus == PAYMENT_STATUS_FAILED {
		order.OrderStatus = ORDER_STATUS_FAILED
	}

	return nil
}

func (s *ServiceImpl) GetOrder(ctx context.Context, orderID string) (*ordf.Order, error) {
	order, payment, _, err := s.novelliaDatabaseService.QueryOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	err = s.addPaymentToOrder(order, payment)
	if err != nil {
		return nil, err
	}

	return order, nil
}

func (s *ServiceImpl) CheckAndUpdateOrderPayment(ctx context.Context, orderID string) (*ordf.Order, error) {
	// this function doesn't verify a check interval, the caller will have to do that

	order, payment, _, err := s.novelliaDatabaseService.QueryOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	err = s.addPaymentToOrder(order, payment)
	if err != nil {
		return nil, err
	}

	// update checked last
	err = s.novelliaDatabaseService.UpdateOrder(ctx, *order, *payment)
	if err != nil {
		return nil, err
	}

	refreshedPayment, err := s.nowPaymentsService.GetPaymentStatus(ctx, payment.PaymentID.String())
	if err != nil {
		return nil, err
	}

	if payment.PaymentStatus != refreshedPayment.PaymentStatus || order.OrderStatus == ORDER_STATUS_AWAITING_PAYMENT {
		err = s.addPaymentToOrder(order, refreshedPayment)
		if err != nil {
			return nil, err
		}

		err = s.updateOrderStatus(order, refreshedPayment)
		if err != nil {
			return nil, err
		}

		err = s.novelliaDatabaseService.UpdateOrder(ctx, *order, *refreshedPayment)
		if err != nil {
			return nil, err
		}
	}

	return order, nil
}

func (s *ServiceImpl) CheckAndUpdateOrderFulfillment(ctx context.Context, orderID string) (*ordf.Order, error) {
	// this function doesn't verify a check interval, the caller will have to do that

	order, payment, _, err := s.novelliaDatabaseService.QueryOrder(ctx, orderID)
	if err != nil {
		fmt.Printf("Failed to query order: %+v (%s)\n", order.OrderId, err)
		return nil, err
	}

	err = s.addPaymentToOrder(order, payment)
	if err != nil {
		fmt.Printf("Failed to add payment to order: %+v (%s)\n", order.OrderId, err)
		return nil, err
	}

	// update checked last
	err = s.novelliaDatabaseService.UpdateOrder(ctx, *order, *payment)
	if err != nil {
		fmt.Printf("Failed to update order (updating checked last): %+v (%s), (order) %+v, (payment) %+v\n", order.OrderId, err, *order, *payment)
		return nil, err
	}

	if order.OrderStatus == ORDER_STATUS_PAID {
		txid, err := s.cardanoService.SubmitOrder(ctx, order)
		if err != nil {
			fmt.Printf("SubmitOrder failure: %+v (%s)\n", order.OrderId, err)
			prometheus_monitoring.TickCardanoSubmitOrderFailed()
			return nil, err
		}

		fmt.Printf("Filling order %s\n", order.OrderId)

		order.OrderStatus = ORDER_STATUS_FILLED
		err = s.updateOrderStatus(order, payment)
		if err != nil {
			fmt.Printf("Failed to update order: %+v (%s), (order) %+v, (payment) %+v\n", order.OrderId, err, *order, *payment)
			return nil, err
		}

		err = s.novelliaDatabaseService.UpdateOrder(ctx, *order, *payment)
		if err != nil {
			fmt.Printf("Failed to update order: %+v (%s)\n", order.OrderId, err)
			return nil, err
		}

		err = s.novelliaDatabaseService.InsertCardanoTransaction(ctx, order.OrderId, txid)
		if err != nil {
			fmt.Printf("Failed to insert Cardano transaction: %+v (%s)\n", order.OrderId, err)
			return nil, err
		}

		fmt.Printf("Successfully fulfilled order %s\n", order.OrderId)
	}

	return order, nil
}

func (s *ServiceImpl) IPNUpdateOrder(ctx context.Context, payment now_payments.GetPaymentStatusResponse) error {
	order, _, _, err := s.novelliaDatabaseService.QueryOrder(ctx, payment.OrderID)
	if err != nil {
		return err
	}

	newPaymentStatus, err := s.mapNowPaymentsStatus(payment.PaymentStatus)
	if err != nil {
		return err
	}
	order.Payment.PaymentStatus = newPaymentStatus

	err = s.novelliaDatabaseService.UpdateOrder(ctx, *order, payment)
	if err != nil {
		return err
	}

	return nil
}

func (s *ServiceImpl) WatchOrdersForPayment(ctx context.Context) {
	go func() {
		for {
			time.Sleep(checkOrdersForPaymentInterval)
			fmt.Printf("WatchOrdersForPayment, running iteration\n")

			// get list of orders to check
			orderIDs, err := s.novelliaDatabaseService.QueryOrdersReadyForCheck(ctx, checkOrdersForPaymentInterval, ORDER_STATUS_AWAITING_PAYMENT)
			if err != nil {
				fmt.Printf("WatchOrdersForPayment error (query OrderIDs): %+v\n", err)
				prometheus_monitoring.SetWatchOrdersForPaymentStatus(0)
				continue
			}

			failedUpdate := false
			for _, orderID := range orderIDs {
				_, err := s.CheckAndUpdateOrderPayment(ctx, orderID)
				if err != nil {
					fmt.Printf("WatchOrdersForPayment error (query update order %s): %+v\n", orderID, err)
					failedUpdate = true
					break
				}
				time.Sleep(checkOrdersForPaymentRateLimit)
			}
			if failedUpdate {
				prometheus_monitoring.SetWatchOrdersForPaymentStatus(0)
				continue
			}
			
			prometheus_monitoring.SetWatchOrdersForPaymentStatus(1)
			fmt.Printf("WatchOrdersForPayment, completed iteration\n")
		}
	}()
}

func (s *ServiceImpl) WatchOrdersForFulfillment(ctx context.Context) {
	go func() {
		for {
			time.Sleep(checkOrdersForFulfillmentInterval)
			fmt.Printf("WatchOrdersForFulfillment, running iteration\n")

			// get list of orders to check
			orderIDs, err := s.novelliaDatabaseService.QueryOrdersReadyForCheck(ctx, checkOrdersForFulfillmentInterval, ORDER_STATUS_PAID)
			if err != nil {
				fmt.Printf("WatchOrdersForFulfillment error (query OrderIDs): %+v\n", err)
				prometheus_monitoring.SetWatchOrdersForFulfillmentStatus(0)
				continue
			}

			failedUpdate := false
			for _, orderID := range orderIDs {
				_, err := s.CheckAndUpdateOrderFulfillment(ctx, orderID)
				if err != nil {
					fmt.Printf("WatchOrdersForFulfillment error (query update order %s): %+v\n", orderID, err)
					failedUpdate = true
					break
				}
				time.Sleep(checkOrdersForFulfillmentRateLimit)
			}
			if failedUpdate {
				fmt.Printf("WatchOrdersForFulfillment error: failed an order update")
				prometheus_monitoring.SetWatchOrdersForFulfillmentStatus(0)
				continue
			}
			
			prometheus_monitoring.SetWatchOrdersForFulfillmentStatus(1)
			fmt.Printf("WatchOrdersForFulfillment, completed iteration\n")
		}
	}()
}

