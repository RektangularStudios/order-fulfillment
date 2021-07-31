SELECT
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
  now_payments_updated_at
FROM order_fulfillment.now_payments_payment
WHERE $1 = customer_order_id;
