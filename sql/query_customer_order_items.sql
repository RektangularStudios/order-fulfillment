SELECT
  product_id,
  quantity
FROM order_fulfillment.customer_order_item
WHERE $1 = customer_order_id
