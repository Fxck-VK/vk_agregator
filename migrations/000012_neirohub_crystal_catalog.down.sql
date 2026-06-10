UPDATE payment_products
SET is_active = false, updated_at = now()
WHERE code IN (
    'crystals_99',
    'crystals_150',
    'crystals_250',
    'crystals_400',
    'crystals_700'
);

UPDATE payment_products
SET is_active = true, updated_at = now()
WHERE code IN ('credits_100', 'credits_500', 'credits_1000');
