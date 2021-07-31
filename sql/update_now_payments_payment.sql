UPDATE order_fulfillment.now_payments_payment
SET
  payment_status = $2,
  actually_paid = $3,
  now_payments_updated_at = $4,
  outcome_amount = $5,
  outcome_currency = $6
WHERE customer_order_id = $1;
