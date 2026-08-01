-- migrate: no-transaction
-- Global expiry claims order due account-owned browser image preparations by
-- deadline. This partial index matches that bounded SKIP LOCKED query exactly.
-- Drop a stranded invalid concurrent index first so migration retries can
-- construct a valid replacement before recording the migration version.
DROP INDEX CONCURRENTLY IF EXISTS jobs_web_image_prepared_expiry_reconciliation_idx;
CREATE INDEX CONCURRENTLY jobs_web_image_prepared_expiry_reconciliation_idx
    ON jobs (expires_at ASC, id ASC)
    WHERE account_id IS NOT NULL
      AND source = 'web'
      AND operation_type = 'image_generate'
      AND modality = 'image'
      AND status = 'prepared'
      AND expires_at IS NOT NULL;
