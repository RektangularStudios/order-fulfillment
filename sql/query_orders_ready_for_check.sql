SELECT
  customer_order_id
FROM order_fulfillment.customer_order
WHERE
  checked_last < $1 AND
  order_status = $2;
