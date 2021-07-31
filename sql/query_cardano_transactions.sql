SELECT
  txid
FROM order_fulfillment.cardano_transaction
WHERE $1 = customer_order_id;
