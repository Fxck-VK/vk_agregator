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
    ('crystals_10_dev', 'NeiroHub DEV 10 crystals', 1000, 'rub', 10, 1, 1, 'service', 'full_prepayment', true)
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
