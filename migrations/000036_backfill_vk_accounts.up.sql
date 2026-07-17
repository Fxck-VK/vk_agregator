-- 000036_backfill_vk_accounts.up.sql
-- Backfill the account identity layer from current VK-first users.
-- Runtime ownership remains on users until the IdentityResolver rollout.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS account_id UUID;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'users'::regclass
          AND conname = 'users_account_id_fkey'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_account_id_fkey
            FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS users_account_id_idx
    ON users (account_id)
    WHERE account_id IS NOT NULL;

WITH legacy_users AS (
    SELECT
        u.id AS user_id,
        u.vk_user_id,
        gen_random_uuid() AS account_id,
        CASE
            WHEN u.status IN ('active', 'blocked', 'deleted') THEN u.status
            ELSE 'active'
        END AS account_status,
        CASE
            WHEN u.role IN ('user', 'moderator', 'admin', 'operator') THEN u.role
            ELSE 'user'
        END AS account_role,
        u.locale,
        u.timezone,
        GREATEST(u.risk_level, 0) AS risk_level,
        u.first_seen_at,
        u.last_seen_at,
        u.created_at,
        u.updated_at
    FROM users u
    WHERE NOT EXISTS (
        SELECT 1
        FROM account_identities ai
        WHERE ai.provider = 'vk'
          AND ai.normalized_id = u.vk_user_id::TEXT
    )
),
inserted_accounts AS (
    INSERT INTO accounts (
        id,
        status,
        role,
        account_type,
        locale,
        timezone,
        risk_level,
        created_at,
        updated_at
    )
    SELECT
        lu.account_id,
        lu.account_status,
        lu.account_role,
        'personal',
        COALESCE(NULLIF(lu.locale, ''), 'ru'),
        COALESCE(NULLIF(lu.timezone, ''), 'Europe/Moscow'),
        lu.risk_level,
        lu.created_at,
        lu.updated_at
    FROM legacy_users lu
    RETURNING id
),
inserted_identities AS (
    INSERT INTO account_identities (
        account_id,
        provider,
        external_id,
        normalized_id,
        verified_at,
        last_used_at,
        created_at,
        updated_at
    )
    SELECT
        lu.account_id,
        'vk',
        lu.vk_user_id::TEXT,
        lu.vk_user_id::TEXT,
        COALESCE(lu.first_seen_at, lu.created_at),
        COALESCE(lu.last_seen_at, lu.updated_at),
        lu.created_at,
        lu.updated_at
    FROM legacy_users lu
    INNER JOIN inserted_accounts ia ON ia.id = lu.account_id
    ON CONFLICT (provider, normalized_id) DO NOTHING
    RETURNING id, account_id
)
INSERT INTO account_links_audit (
    account_id,
    actor_account_id,
    action,
    provider,
    identity_id,
    created_at
)
SELECT
    ii.account_id,
    NULL,
    'linked',
    'vk',
    ii.id,
    now()
FROM inserted_identities ii
WHERE NOT EXISTS (
    SELECT 1
    FROM account_links_audit audit
    WHERE audit.identity_id = ii.id
      AND audit.action = 'linked'
      AND audit.provider = 'vk'
);

UPDATE users u
SET account_id = ai.account_id
FROM account_identities ai
WHERE ai.provider = 'vk'
  AND ai.normalized_id = u.vk_user_id::TEXT
  AND (u.account_id IS NULL OR u.account_id <> ai.account_id);

COMMIT;
