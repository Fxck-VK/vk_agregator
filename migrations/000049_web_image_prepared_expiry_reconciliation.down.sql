-- migrate: no-transaction
DROP INDEX CONCURRENTLY IF EXISTS jobs_web_image_prepared_expiry_reconciliation_idx;
