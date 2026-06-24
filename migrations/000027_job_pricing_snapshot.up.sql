-- 000027_job_pricing_snapshot.up.sql
-- Stores immutable backend pricing facts for jobs created through pricingcatalog.

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS pricing_snapshot JSONB;

COMMENT ON COLUMN jobs.pricing_snapshot IS
    'Immutable backend-owned pricingcatalog snapshot for new paid jobs; null for legacy jobs.';
