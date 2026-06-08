-- 000009_payments.down.sql

BEGIN;

DROP TABLE IF EXISTS payment_refunds;
DROP TABLE IF EXISTS payment_events;
DROP TABLE IF EXISTS payment_intents;
DROP TABLE IF EXISTS payment_products;

COMMIT;
