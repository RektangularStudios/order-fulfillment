UPDATE order_fulfillment.customer_order
SET order_status = 'PAID'
WHERE customer_order_id = 'ORDER-01F68P30XW6QSGR8XSV81G6G4D';

UPDATE order_fulfillment.customer_order
SET delivery_address = 'addr1'
WHERE customer_order_id = 'ORDER-01F67Z57V76S2R39VS4WVYWX0R'
