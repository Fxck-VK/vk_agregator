-- 000038_account_sessions_hardening.up.sql
-- Keep Web/Mobile refresh-token sessions unique and efficient without storing
-- raw token, device, IP or user-agent material.

BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS account_sessions_refresh_token_hash_key
    ON account_sessions (refresh_token_hash);

CREATE INDEX IF NOT EXISTS account_sessions_account_device_active_idx
    ON account_sessions (account_id, device_id, expires_at DESC)
    WHERE revoked_at IS NULL;

COMMIT;
