-- 000027_job_pricing_snapshot.down.sql
-- Reverts 000027_job_pricing_snapshot.up.sql.

ALTER TABLE jobs
    DROP COLUMN IF EXISTS pricing_snapshot;
