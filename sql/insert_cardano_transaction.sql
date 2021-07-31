INSERT INTO order_fulfillment.cardano_transaction
(
  customer_order_id,
  txid
)
VALUES($1, $2);
