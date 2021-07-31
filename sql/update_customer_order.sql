UPDATE order_fulfillment.customer_order
SET 
  order_status = $2,
  checked_last = $3
WHERE customer_order_id = $1;
