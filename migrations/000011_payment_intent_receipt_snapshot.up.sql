-- 000011_payment_intent_receipt_snapshot.up.sql
-- Snapshot fiscal receipt fields on payment_intents so provider retries and
-- refunds use the original 54-FZ product position, not the current catalog row.

BEGIN;

ALTER TABLE payment_intents
    ADD COLUMN receipt_description TEXT NOT NULL DEFAULT '',
    ADD COLUMN vat_code SMALLINT,
    ADD COLUMN payment_subject TEXT NOT NULL DEFAULT '',
    ADD COLUMN payment_mode TEXT NOT NULL DEFAULT '';

UPDATE payment_intents pi
SET receipt_description = pp.title,
    vat_code = pp.vat_code,
    payment_subject = pp.payment_subject,
    payment_mode = pp.payment_mode
FROM payment_products pp
WHERE pi.product_id = pp.id;

COMMIT;
