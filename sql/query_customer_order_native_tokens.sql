SELECT
  native_token_id,
  quantity
FROM order_fulfillment.customer_order_native_tokens
WHERE $1 = customer_order_id;
