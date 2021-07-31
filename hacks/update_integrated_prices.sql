-- kinda rare
UPDATE novellia.product
SET price_unit_amount = 22
WHERE price_unit_amount = 17;

-- not that rare
UPDATE novellia.product
SET price_unit_amount = 13
WHERE price_unit_amount = 8;

-- starter deck
UPDATE novellia.product
SET price_unit_amount = 160
WHERE product_id = 'PROD-01F4NAFJCAG5JDEGMR0XQARBW2';

-- booster
UPDATE novellia.product
SET price_unit_amount = 35
WHERE product_id = 'PROD-01F4NAF8MANXDT26MGA5E0QXNJ';
