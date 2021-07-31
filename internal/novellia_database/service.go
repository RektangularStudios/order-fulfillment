package novellia_database

import (
	"fmt"
	"time"
	"math/rand"
	"context"
	"io/ioutil"
	"path/filepath"
	"math/big"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jackc/pgtype"
	
	"github.com/oklog/ulid/v2"

	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/now_payments"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/constants"
)

const (
	insertCustomerOrder = "insertCustomerOrder"
	insertCustomerOrderItem = "insertCustomerOrderItem"
	insertNowPaymentsPayment = "insertNowPaymentsPayment"
	insertCardanoTransaction = "insertCardanoTransaction"
	insertCustomerOrderNativeTokens = "insertCustomerOrderNativeTokens"
	updateCustomerOrder = "updateCustomerOrder"
	updateNowPaymentsPayment = "updateNowPaymentsPayment"
	queryProducts = "queryProducts"
	queryCustomerOrder = "queryCustomerOrder"
	queryCustomerOrderItems = "queryCustomerOrderItems"
	queryOrdersReadyForCheck = "queryOrdersReadyForCheck"
	queryNowPaymentsPayment = "queryNowPaymentsPayment"
	queryCustomerOrderNativeTokens = "queryCustomerOrderNativeTokens"
	queryCardanoTransactions = "queryCardanoTransactions"
	queryReservedNativeTokens = "queryReservedNativeTokens"
)

type Product struct {
	ProductID string
	PriceUnitAmount float64
	PriceCurrencyID string
	MaxOrderSize int
	DateListed *time.Time
	DateAvailable *time.Time
	NativeTokenID string
}

type ServiceImpl struct {
	queriesPath string
	pool *pgxpool.Pool
	queries map[string]string
	ULIDentropy *ulid.MonotonicEntropy
}

// creates a new ServiceImpl, connecting to Postgres
func New(ctx context.Context, username, password, host, database_name string, queriesPath string) (*ServiceImpl, error) {
	// url like "postgresql://username:password@localhost:5432/database_name"
	databaseUrl := fmt.Sprintf("postgresql://%s:%s@%s/%s", username, password, host, database_name)
	pool, err := pgxpool.Connect(ctx, databaseUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to Postgres: %v", err)
	}

	t := time.Now().UTC()
	ULIDentropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)

	service := ServiceImpl {
		pool: pool,
		queriesPath: queriesPath,
		ULIDentropy: ULIDentropy,
	}

	err = service.loadQueries(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load queries")
	}

	return &service, nil
}

func (s *ServiceImpl) loadQueries(ctx context.Context) error {
	queryFiles := map[string]string {
		insertCustomerOrder: "insert_customer_order.sql",
		insertCustomerOrderItem: "insert_customer_order_item.sql",
		insertNowPaymentsPayment: "insert_now_payments_payment.sql",
		insertCardanoTransaction: "insert_cardano_transaction.sql",
		insertCustomerOrderNativeTokens: "insert_customer_order_native_tokens.sql",
		updateCustomerOrder: "update_customer_order.sql",
		updateNowPaymentsPayment: "update_now_payments_payment.sql",
		queryProducts: "query_products.sql",
		queryCustomerOrder: "query_customer_order.sql",
		queryCustomerOrderItems: "query_customer_order_items.sql",
		queryOrdersReadyForCheck: "query_orders_ready_for_check.sql",
		queryNowPaymentsPayment: "query_now_payments_payment.sql",
		queryCustomerOrderNativeTokens: "query_customer_order_native_tokens.sql",
		queryCardanoTransactions: "query_cardano_transactions.sql",
		queryReservedNativeTokens: "query_reserved_native_tokens.sql",
	}
	
	queries := make(map[string]string)
	for name, filename := range queryFiles {
		fmt.Printf("Loading SQL %s\n", filename)

		query, err := s.readQueryFile(filename)
		if err != nil {
			return err
		}

		queries[name] = query
	}
	s.queries = queries

	fmt.Printf("SQL has been loaded\n")
	return nil
}

// reads a text file using the queriesPath as the base path
func (s *ServiceImpl) readQueryFile(filename string) (string, error) {
	queryPath := filepath.Join(s.queriesPath, filename)

	bytes, err := ioutil.ReadFile(queryPath)
	if err != nil {
		return "", fmt.Errorf("failed to read query file %s: %v", filename, err)
	}

	return string(bytes), nil
}

// generates a prefixed ULID like "ORDER-01D78XYFJ1PRM1WPBCBT3VHMNV"
func (s *ServiceImpl) GenerateULID(prefix string) string {
	t := time.Now().UTC()
	u := ulid.MustNew(ulid.Timestamp(t), s.ULIDentropy)
	return fmt.Sprintf("%s-%s", prefix, u.String())
}

// inserts an order
func (s *ServiceImpl) InsertOrder(ctx context.Context, order ordf.Order, payment now_payments.CreatePaymentResponse) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	batch := &pgx.Batch{}
	batch.Queue(s.queries[insertCustomerOrder], 
		order.OrderId,
		order.OrderStatus,
		order.Description,
		time.Now().Format(constants.ISO8601DateFormat),
		order.Customer.DeliveryAddress,
		order.Payment.PaymentAddress,
		order.Payment.PriceCurrencyId,
		order.Payment.PriceAmount,
	)
	batch.Queue(s.queries[insertNowPaymentsPayment],
		payment.PaymentID,
		payment.PaymentStatus,
		payment.PayAddress,
		payment.PriceAmount,
		payment.PriceCurrency,
		payment.PayAmount,
		payment.PayCurrency,
		payment.OrderID,
		payment.OrderDescription,
		payment.PurchaseID,
		payment.CreatedAt,
		payment.UpdatedAt,
		payment.IPNCallbackURL,
	)
	for _, v := range order.Items {
		batch.Queue(s.queries[insertCustomerOrderItem],
			order.OrderId,
			v.ProductId,
			v.Quantity,
		)
	}

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < 2 + len(order.Items); i += 1 {
		_, err := br.Exec()
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}

	err = br.Close()
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *ServiceImpl) UpdateOrder(ctx context.Context, order ordf.Order, payment now_payments.GetPaymentStatusResponse) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	timeNow := time.Now().Format(constants.ISO8601DateFormat)

	batch := &pgx.Batch{}
	batch.Queue(s.queries[updateCustomerOrder],
		order.OrderId,
		order.OrderStatus,
		// update "checked last"
		timeNow,
	)

	if payment.UpdatedAt == "" {
		payment.UpdatedAt = timeNow
	}
	batch.Queue(s.queries[updateNowPaymentsPayment],
		order.OrderId,
		payment.PaymentStatus,
		payment.ActuallyPaid,
		payment.UpdatedAt,
		payment.OutcomeAmount,
		payment.OutcomeCurrency,
	)

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < 2; i += 1 {
		_, err := br.Exec()
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}

	err = br.Close()
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}

// queries for an order
func (s *ServiceImpl) QueryOrder(ctx context.Context, orderID string) (*ordf.Order, *now_payments.GetPaymentStatusResponse, *time.Time, error) {
	var order ordf.Order
	var checkedLast pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, s.queries[queryCustomerOrder], orderID).Scan(
		&order.OrderId,
		&order.OrderStatus,
		&order.Description,
		&order.Customer.DeliveryAddress,
		&order.Payment.PaymentAddress,
		&order.Payment.PriceCurrencyId,
		&order.Payment.PriceAmount,
		&checkedLast,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	rows, err := s.pool.Query(ctx, s.queries[queryCustomerOrderItems], orderID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var i ordf.OrderItems
		err = rows.Scan(
			&i.ProductId,
			&i.Quantity,
		)
		if err != nil {
			return nil, nil, nil, err
		}

		order.Items = append(order.Items, i)
	}

	// TODO: handle case of multiple payments, e.g. if the first one expired
	var payment now_payments.GetPaymentStatusResponse

	var createdAt pgtype.Timestamptz
	var updatedAt pgtype.Timestamptz
	err = s.pool.QueryRow(ctx, s.queries[queryNowPaymentsPayment], orderID).Scan(
		&payment.PaymentID,
		&payment.PaymentStatus,
		&payment.PayAddress,
		&payment.PriceAmount,
		&payment.PriceCurrency,
		&payment.PayAmount,
		&payment.PayCurrency,
		&payment.OrderID,
		&payment.OrderDescription,
		&payment.PurchaseID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	payment.CreatedAt = createdAt.Time.UTC().Format(constants.ISO8601DateFormat)
	payment.UpdatedAt = updatedAt.Time.UTC().Format(constants.ISO8601DateFormat)

	return &order, &payment, &checkedLast.Time, nil
}

func (s *ServiceImpl ) QueryOrdersReadyForCheck(ctx context.Context, interval time.Duration, requiredStatus string) ([]string, error) {
	minCheckedLast := time.Now().Add(-1 * interval).Format(constants.ISO8601DateFormat)
	rows, err := s.pool.Query(ctx, s.queries[queryOrdersReadyForCheck], minCheckedLast, requiredStatus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orderIDs := []string{}
	for rows.Next() {
		var orderID string
		err = rows.Scan(
			&orderID,
		)
		if err != nil {
			return nil, err
		}

		orderIDs = append(orderIDs, orderID)
	}

	return orderIDs, nil
}

func (s *ServiceImpl) QueryProducts(ctx context.Context) ([]Product, error) {
	rows, err := s.pool.Query(ctx, s.queries[queryProducts])
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		var p Product
		var dateListed pgtype.Timestamptz
		var dateAvailable pgtype.Timestamptz

		err = rows.Scan(
			&p.ProductID,
			&p.PriceUnitAmount,
			&p.PriceCurrencyID,
			&p.MaxOrderSize,
			&dateListed,
			&dateAvailable,
			&p.NativeTokenID,
		)
		if err != nil {
			return nil, fmt.Errorf("query products failed: %v", err)
		}

		// convert dates to time.Time
		p.DateListed = nil
		if dateListed.Status == pgtype.Present {
			p.DateListed = &dateListed.Time
		}
		p.DateAvailable = nil
		if dateAvailable.Status == pgtype.Present {
			p.DateAvailable = &dateAvailable.Time
		}

		products = append(products, p)
	}

	return products, nil
}

func (s *ServiceImpl) InsertCardanoTransaction(ctx context.Context, orderID string, txid string) error {
	_, err := s.pool.Exec(ctx, s.queries[insertCardanoTransaction], orderID, txid)
	if err != nil {
		return fmt.Errorf("insert cardano transaction failed: %v", err)
	}
	return nil
}

func (s *ServiceImpl) Close() {
	s.pool.Close()
}

func (s *ServiceImpl) QueryOrderNativeTokens(ctx context.Context, orderID string) (map[string]*big.Int, error) {
	rows, err := s.pool.Query(ctx, s.queries[queryCustomerOrderNativeTokens], orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	t := map[string]*big.Int{}
	for rows.Next() {
		var nativeTokenID string
		var quantity int64

		err = rows.Scan(
			&nativeTokenID,
			&quantity,
		)
		if err != nil {
			return nil, fmt.Errorf("query order native tokens failed: %v", err)
		}

		t[nativeTokenID] = big.NewInt(quantity)
	}

	return t, err
}

func (s *ServiceImpl) InsertOrderNativeTokens(ctx context.Context, orderID string, tokens map[string]*big.Int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	batch := &pgx.Batch{}
	for native_token_id, quantity := range tokens {
		batch.Queue(s.queries[insertCustomerOrderNativeTokens], 
			orderID,
			native_token_id,
			quantity.Int64(),
		)
	}

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < len(tokens); i++ {
		_, err := br.Exec()
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}

	err = br.Close()
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *ServiceImpl) QueryCardanoTransactions(ctx context.Context, orderID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, s.queries[queryCardanoTransactions], orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	txids := []string{}
	for rows.Next() {
		var txid string

		err = rows.Scan(
			&txid,
		)
		if err != nil {
			return nil, fmt.Errorf("query Cardano transactions failed: %v", err)
		}

		txids = append(txids, txid)
	}

	return txids, err
}

func (s *ServiceImpl) QueryReservedNativeTokens(ctx context.Context) (map[string]*big.Int, error) {
	rows, err := s.pool.Query(ctx, s.queries[queryReservedNativeTokens])
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	t := map[string]*big.Int{}
	for rows.Next() {
		var nativeTokenID string
		var quantity int64

		err = rows.Scan(
			&nativeTokenID,
			&quantity,
		)
		if err != nil {
			return nil, fmt.Errorf("query reserved native tokens failed: %v", err)
		}

		t[nativeTokenID] = big.NewInt(quantity)
	}

	return t, err
}
