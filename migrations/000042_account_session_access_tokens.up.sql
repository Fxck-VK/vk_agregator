-- 000042_account_session_access_tokens.up.sql
-- Access credentials are short-lived, hash-only session material. Existing
-- refresh-only rows intentionally remain valid for refresh but not access auth.

ALTER TABLE account_sessions
    ADD COLUMN IF NOT EXISTS access_token_hash TEXT,
    ADD COLUMN IF NOT EXISTS access_expires_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS account_sessions_access_token_hash_unique
    ON account_sessions (access_token_hash)
    WHERE access_token_hash IS NOT NULL;

ALTER TABLE account_sessions
    ADD CONSTRAINT account_sessions_access_token_pair_check
    CHECK (
        (access_token_hash IS NULL AND access_expires_at IS NULL)
        OR
        (access_token_hash IS NOT NULL AND access_expires_at IS NOT NULL
         AND access_expires_at <= expires_at)
    ) NOT VALID;
