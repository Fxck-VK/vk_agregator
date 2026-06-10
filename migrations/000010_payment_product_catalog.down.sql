-- 000010_payment_product_catalog.down.sql

BEGIN;

DELETE FROM payment_products
WHERE code IN ('credits_100', 'credits_500', 'credits_1000');

COMMIT;
