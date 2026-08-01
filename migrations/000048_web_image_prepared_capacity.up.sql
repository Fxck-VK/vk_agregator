-- migrate: no-transaction
-- Capacity lookup for unconfirmed browser image jobs. The partial predicate
-- is intentionally identical to CountUnexpiredPreparedByAccountOperation.
-- Remove an invalid stranded index from an interrupted concurrent build so a
-- retry creates a usable index before the migration version is recorded.
DROP INDEX CONCURRENTLY IF EXISTS jobs_web_image_prepared_capacity_idx;
CREATE INDEX CONCURRENTLY jobs_web_image_prepared_capacity_idx
    ON jobs (account_id, expires_at)
    WHERE account_id IS NOT NULL
      AND source = 'web'
      AND operation_type = 'image_generate'
      AND modality = 'image'
      AND status = 'prepared';
