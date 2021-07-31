SELECT
  native_token_id,
  SUM(quantity)
FROM order_fulfillment.customer_order_native_tokens
INNER JOIN order_fulfillment.customer_order ON order_fulfillment.customer_order.customer_order_id = order_fulfillment.customer_order_native_tokens.customer_order_id
WHERE
  order_fulfillment.customer_order.order_status = 'AWAITING_PAYMENT' OR
  order_fulfillment.customer_order.order_status = 'PAID'
GROUP BY native_token_id;
