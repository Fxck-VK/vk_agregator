-- Rollback is allowed only before web access-session material exists.
-- Refresh-token sessions remain untouched.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM account_sessions
        WHERE access_token_hash IS NOT NULL
           OR access_expires_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION
            'cannot roll back 000042: access-token session material exists';
    END IF;
END $$;

ALTER TABLE account_sessions
    DROP CONSTRAINT IF EXISTS account_sessions_access_token_pair_check;

DROP INDEX IF EXISTS account_sessions_access_token_hash_unique;

ALTER TABLE account_sessions
    DROP COLUMN IF EXISTS access_token_hash,
    DROP COLUMN IF EXISTS access_expires_at;
