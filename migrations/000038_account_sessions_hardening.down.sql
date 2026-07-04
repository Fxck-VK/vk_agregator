-- 000038_account_sessions_hardening.down.sql

BEGIN;

DROP INDEX IF EXISTS account_sessions_account_device_active_idx;
DROP INDEX IF EXISTS account_sessions_refresh_token_hash_key;

COMMIT;
