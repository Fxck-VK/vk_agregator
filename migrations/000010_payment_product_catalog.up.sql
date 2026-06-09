-- 000010_payment_product_catalog.up.sql
-- Initial active YooKassa top-up catalog. Amounts are stored in kopecks.

BEGIN;

INSERT INTO payment_products (
    code,
    title,
    amount,
    currency,
    credits,
    price_version,
    vat_code,
    payment_subject,
    payment_mode,
    is_active
)
VALUES
    ('credits_100', 'NeiroHub 100 credits', 10000, 'rub', 100, 1, 1, 'service', 'full_prepayment', true),
    ('credits_500', 'NeiroHub 500 credits', 45000, 'rub', 500, 1, 1, 'service', 'full_prepayment', true),
    ('credits_1000', 'NeiroHub 1000 credits', 80000, 'rub', 1000, 1, 1, 'service', 'full_prepayment', true)
ON CONFLICT (code) DO UPDATE
SET title = EXCLUDED.title,
    amount = EXCLUDED.amount,
    currency = EXCLUDED.currency,
    credits = EXCLUDED.credits,
    price_version = EXCLUDED.price_version,
    vat_code = EXCLUDED.vat_code,
    payment_subject = EXCLUDED.payment_subject,
    payment_mode = EXCLUDED.payment_mode,
    is_active = EXCLUDED.is_active,
    updated_at = now();

COMMIT;
