INSERT INTO order_fulfillment.customer_order_item
(
  customer_order_id,
  product_id,
  quantity
)
VALUES($1, $2, $3);
