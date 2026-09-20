-- migrate: no-transaction
DROP INDEX CONCURRENTLY IF EXISTS jobs_account_media_prepared_expiry_idx;
DROP INDEX CONCURRENTLY IF EXISTS jobs_web_media_prepared_expiry_idx;
