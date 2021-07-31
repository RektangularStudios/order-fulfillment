SELECT
  product_id,
  COALESCE(price_unit_amount, 0),
  price_currency_id,
  COALESCE(max_order_size, 0),
  date_listed,
  date_available,
  COALESCE(native_token_id, '')
FROM novellia.product;
