INSERT INTO order_fulfillment.customer_order
(
  customer_order_id,
  order_status,
  order_description,
  checked_last,
  delivery_address,
  payment_address,
  price_currency_id,
  price_amount
)
VALUES($1, $2, $3, $4, $5, $6, $7, $8);
