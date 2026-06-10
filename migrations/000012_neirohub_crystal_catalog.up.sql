UPDATE payment_products
SET is_active = false, updated_at = now()
WHERE code IN ('credits_100', 'credits_500', 'credits_1000');

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
    ('crystals_99', 'NeiroHub 99 crystals', 9900, 'rub', 99, 1, 1, 'service', 'full_prepayment', true),
    ('crystals_150', 'NeiroHub 150 crystals', 15000, 'rub', 150, 1, 1, 'service', 'full_prepayment', true),
    ('crystals_250', 'NeiroHub 250 crystals', 25000, 'rub', 250, 1, 1, 'service', 'full_prepayment', true),
    ('crystals_400', 'NeiroHub 400 crystals', 40000, 'rub', 400, 1, 1, 'service', 'full_prepayment', true),
    ('crystals_700', 'NeiroHub 700 crystals', 70000, 'rub', 700, 1, 1, 'service', 'full_prepayment', true)
ON CONFLICT (code) DO UPDATE SET
    title = EXCLUDED.title,
    amount = EXCLUDED.amount,
    currency = EXCLUDED.currency,
    credits = EXCLUDED.credits,
    vat_code = EXCLUDED.vat_code,
    payment_subject = EXCLUDED.payment_subject,
    payment_mode = EXCLUDED.payment_mode,
    is_active = true,
    updated_at = now();
