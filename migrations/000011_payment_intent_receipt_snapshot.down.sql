-- 000011_payment_intent_receipt_snapshot.down.sql

BEGIN;

ALTER TABLE payment_intents
    DROP COLUMN IF EXISTS payment_mode,
    DROP COLUMN IF EXISTS payment_subject,
    DROP COLUMN IF EXISTS vat_code,
    DROP COLUMN IF EXISTS receipt_description;

COMMIT;
