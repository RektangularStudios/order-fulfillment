INSERT INTO order_fulfillment.now_payments_payment
(
  payment_id,
  payment_status,
  pay_address,
  price_amount,
  price_currency,
  pay_amount,
  pay_currency,
  customer_order_id,
  order_description,
  purchase_id,
  now_payments_created_at,
  now_payments_updated_at,
  ipn_callback_url
)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);
