-- migrate: no-transaction
DROP INDEX CONCURRENTLY IF EXISTS jobs_account_speech_prepared_expiry_idx;
DROP INDEX CONCURRENTLY IF EXISTS jobs_web_speech_prepared_expiry_idx;
