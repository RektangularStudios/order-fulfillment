SELECT
  customer_order_id,
  order_status,
  order_description,
  delivery_address,
  payment_address,
  price_currency_id,
  price_amount,
  checked_last
FROM order_fulfillment.customer_order
WHERE $1 = customer_order_id;
