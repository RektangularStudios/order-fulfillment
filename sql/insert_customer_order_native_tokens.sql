INSERT INTO order_fulfillment.customer_order_native_tokens
(
  customer_order_id,
  native_token_id,
  quantity
)
VALUES($1, $2, $3);
