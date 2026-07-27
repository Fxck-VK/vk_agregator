-- 000041_star_denomination.up.sql
-- Introduces the public star denomination: 1 star = 50 kopecks.
-- Historical financial rows keep their original denomination. Current
-- balances are converted with one append-only ledger adjustment per account.

BEGIN;

ALTER TABLE credit_accounts
    ADD COLUMN IF NOT EXISTS credit_denomination_version SMALLINT NOT NULL DEFAULT 1
        CHECK (credit_denomination_version IN (1, 2));
ALTER TABLE credit_reservations
    ADD COLUMN IF NOT EXISTS credit_denomination_version SMALLINT NOT NULL DEFAULT 1
        CHECK (credit_denomination_version IN (1, 2));
ALTER TABLE ledger_entries
    ADD COLUMN IF NOT EXISTS credit_denomination_version SMALLINT NOT NULL DEFAULT 1
        CHECK (credit_denomination_version IN (1, 2));
ALTER TABLE payment_products
    ADD COLUMN IF NOT EXISTS credit_denomination_version SMALLINT NOT NULL DEFAULT 1
        CHECK (credit_denomination_version IN (1, 2));
ALTER TABLE payment_intents
    ADD COLUMN IF NOT EXISTS credit_denomination_version SMALLINT NOT NULL DEFAULT 1
        CHECK (credit_denomination_version IN (1, 2));

ALTER TABLE credit_accounts
    ALTER COLUMN credit_denomination_version SET DEFAULT 2;
ALTER TABLE credit_reservations
    ALTER COLUMN credit_denomination_version SET DEFAULT 2;
ALTER TABLE ledger_entries
    ALTER COLUMN credit_denomination_version SET DEFAULT 2;
ALTER TABLE payment_products
    ALTER COLUMN credit_denomination_version SET DEFAULT 2;
ALTER TABLE payment_intents
    ALTER COLUMN credit_denomination_version SET DEFAULT 2;

-- One legacy credit becomes two current stars. The adjustment amount equals
-- the old cached balance, so the account total doubles without rewriting any
-- historical ledger row.
INSERT INTO ledger_entries (
    account_id,
    owner_account_id,
    type,
    amount,
    credit_denomination_version,
    status,
    idempotency_key,
    reason
)
SELECT
    account.id,
    account.owner_account_id,
    'adjustment',
    account.balance_cached,
    2,
    'committed',
    'denomination:v2:' || account.id::text,
    'convert legacy credits to stars at 1:2'
FROM credit_accounts AS account
WHERE account.credit_denomination_version = 1
ON CONFLICT (idempotency_key) DO NOTHING;

UPDATE credit_accounts
SET balance_cached = balance_cached * 2,
    credit_denomination_version = 2,
    updated_at = now()
WHERE credit_denomination_version = 1;

-- Only live holds move to the current denomination. Historical captured and
-- released reservations remain in the version used when they were settled.
UPDATE credit_reservations
SET amount = amount * 2,
    credit_denomination_version = 2,
    updated_at = now()
WHERE status = 'reserved'
  AND credit_denomination_version = 1;

-- Jobs with a live reservation must keep the same ruble value after account
-- conversion. Completed jobs and their immutable historical snapshots remain
-- untouched.
UPDATE jobs AS job
SET cost_estimate = job.cost_estimate * 2,
    cost_reserved = job.cost_reserved * 2,
    cost_captured = job.cost_captured * 2,
    pricing_snapshot = CASE
        WHEN job.pricing_snapshot IS NULL THEN NULL
        ELSE jsonb_set(
            job.pricing_snapshot
                || jsonb_build_object(
                    'credit_denomination_version', 2,
                    'internal_credits',
                        COALESCE((job.pricing_snapshot ->> 'internal_credits')::bigint, job.cost_reserved) * 2,
                    'internal_credit_cap',
                        COALESCE((job.pricing_snapshot ->> 'internal_credit_cap')::bigint, 0) * 2,
                    'default_display_credits',
                        COALESCE((job.pricing_snapshot ->> 'default_display_credits')::bigint, 0) * 2
                ),
            '{multiplier,numerator}',
            to_jsonb(
                COALESCE(
                    (job.pricing_snapshot #>> '{multiplier,numerator}')::bigint,
                    0
                ) * 2
            ),
            true
        )
    END,
    updated_at = now()
FROM credit_reservations AS reservation
WHERE reservation.job_id = job.id
  AND reservation.status = 'reserved'
  AND reservation.credit_denomination_version = 2
  AND COALESCE((job.pricing_snapshot ->> 'credit_denomination_version')::integer, 1) = 1;

-- Payment product rows are future offers, so they move to v2. Existing
-- payment intents retain v1 and are converted only when applied/refunded.
UPDATE payment_products
SET title = CASE code
        WHEN 'crystals_10_dev' THEN 'NeiroHub DEV 20 stars'
        WHEN 'crystals_99' THEN 'NeiroHub 198 stars'
        WHEN 'crystals_150' THEN 'NeiroHub 300 stars'
        WHEN 'crystals_250' THEN 'NeiroHub 500 stars'
        WHEN 'crystals_400' THEN 'NeiroHub 800 stars'
        WHEN 'crystals_700' THEN 'NeiroHub 1400 stars'
        ELSE title
    END,
    credits = CASE code
        WHEN 'crystals_10_dev' THEN 20
        WHEN 'crystals_99' THEN 198
        WHEN 'crystals_150' THEN 300
        WHEN 'crystals_250' THEN 500
        WHEN 'crystals_400' THEN 800
        WHEN 'crystals_700' THEN 1400
        ELSE credits * 2
    END,
    credit_denomination_version = 2,
    price_version = price_version + 1,
    updated_at = now()
WHERE credit_denomination_version = 1;

-- Runtime prices are immutable by version. If a DB-backed catalog is active,
-- clone it into a new version with doubled star amounts, then activate it.
DO $$
DECLARE
    old_version_id UUID;
    new_version_id UUID;
    next_price_version INTEGER;
BEGIN
    SELECT id
    INTO old_version_id
    FROM runtime_pricing_catalog_versions
    WHERE status = 'active'
      AND COALESCE((metadata ->> 'credit_denomination_version')::integer, 1) = 1
    ORDER BY price_version DESC
    LIMIT 1;

    IF old_version_id IS NOT NULL THEN
        SELECT COALESCE(MAX(price_version), 0) + 1
        INTO next_price_version
        FROM runtime_pricing_catalog_versions;

        INSERT INTO runtime_pricing_catalog_versions (
            price_version,
            status,
            effective_from,
            created_by,
            updated_by,
            note,
            metadata
        )
        VALUES (
            next_price_version,
            'draft',
            now(),
            'migration-000041',
            'migration-000041',
            'star denomination v2',
            jsonb_build_object('credit_denomination_version', 2)
        )
        RETURNING id INTO new_version_id;

        INSERT INTO runtime_generation_prices (
            catalog_version_id,
            operation,
            modality,
            image_model_id,
            video_route_alias,
            quality,
            resolution,
            duration_sec,
            floor_amount,
            floor_unit,
            multiplier_numerator,
            multiplier_denominator,
            internal_credit_cap,
            floor_amount_cap,
            enabled,
            created_by,
            updated_by,
            metadata
        )
        SELECT
            new_version_id,
            operation,
            modality,
            image_model_id,
            video_route_alias,
            quality,
            resolution,
            duration_sec,
            floor_amount,
            floor_unit,
            multiplier_numerator * 2,
            multiplier_denominator,
            internal_credit_cap * 2,
            floor_amount_cap,
            enabled,
            'migration-000041',
            'migration-000041',
            metadata || jsonb_build_object('credit_denomination_version', 2)
        FROM runtime_generation_prices
        WHERE catalog_version_id = old_version_id;

        UPDATE runtime_pricing_catalog_versions
        SET status = 'retired',
            effective_until = now(),
            updated_by = 'migration-000041',
            updated_at = now()
        WHERE id = old_version_id;

        UPDATE runtime_pricing_catalog_versions
        SET status = 'active',
            updated_at = now()
        WHERE id = new_version_id;

        INSERT INTO runtime_pricing_audit_events (
            catalog_version_id,
            action,
            actor_ref,
            reason,
            metadata
        )
        VALUES (
            new_version_id,
            'version_activated',
            'migration-000041',
            'convert active generation prices to star denomination v2',
            jsonb_build_object('credit_denomination_version', 2)
        );
    END IF;
END
$$;

COMMIT;
