-- 000035_account_identity.up.sql
-- Account identity foundation for future multi-surface login.
-- This is additive only: current VK-first user ownership remains unchanged.

BEGIN;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS accounts (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    status       TEXT        NOT NULL DEFAULT 'active',
    role         TEXT        NOT NULL DEFAULT 'user',
    account_type TEXT        NOT NULL DEFAULT 'personal',
    locale       TEXT        NOT NULL DEFAULT 'ru',
    timezone     TEXT        NOT NULL DEFAULT 'Europe/Moscow',
    risk_level   INTEGER     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT accounts_status_check CHECK (
        status IN ('active', 'blocked', 'deleted')
    ),
    CONSTRAINT accounts_role_check CHECK (
        role IN ('user', 'moderator', 'admin', 'operator')
    ),
    CONSTRAINT accounts_account_type_check CHECK (
        account_type IN ('personal', 'business')
    ),
    CONSTRAINT accounts_risk_level_check CHECK (risk_level >= 0)
);

CREATE INDEX IF NOT EXISTS accounts_status_created_idx
    ON accounts (status, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS account_identities (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id    UUID        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    provider      TEXT        NOT NULL,
    external_id   TEXT        NOT NULL,
    normalized_id TEXT        NOT NULL,
    verified_at   TIMESTAMPTZ,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT account_identities_provider_normalized_key UNIQUE (provider, normalized_id),
    CONSTRAINT account_identities_provider_check CHECK (
        provider IN ('vk', 'telegram', 'google', 'apple', 'email', 'phone', 'password')
    ),
    CONSTRAINT account_identities_external_id_check CHECK (btrim(external_id) <> ''),
    CONSTRAINT account_identities_normalized_id_check CHECK (btrim(normalized_id) <> ''),
    CONSTRAINT account_identities_timestamps_check CHECK (
        (verified_at IS NULL OR verified_at >= created_at)
        AND (last_used_at IS NULL OR last_used_at >= created_at)
    )
);

CREATE INDEX IF NOT EXISTS account_identities_account_id_idx
    ON account_identities (account_id);

CREATE INDEX IF NOT EXISTS account_identities_account_created_idx
    ON account_identities (account_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS account_identities_last_used_idx
    ON account_identities (provider, last_used_at DESC)
    WHERE last_used_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS account_sessions (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id         UUID        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    identity_id        UUID        REFERENCES account_identities (id) ON DELETE SET NULL,
    refresh_token_hash TEXT        NOT NULL,
    device_id          TEXT        NOT NULL DEFAULT '',
    ip_hash            TEXT        NOT NULL DEFAULT '',
    user_agent_hash    TEXT        NOT NULL DEFAULT '',
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT account_sessions_refresh_token_hash_check CHECK (
        btrim(refresh_token_hash) <> ''
    ),
    CONSTRAINT account_sessions_timestamps_check CHECK (
        expires_at > created_at
        AND (revoked_at IS NULL OR revoked_at >= created_at)
    )
);

CREATE INDEX IF NOT EXISTS account_sessions_account_id_idx
    ON account_sessions (account_id);

CREATE INDEX IF NOT EXISTS account_sessions_account_created_idx
    ON account_sessions (account_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS account_sessions_account_expires_idx
    ON account_sessions (account_id, expires_at DESC);

CREATE INDEX IF NOT EXISTS account_sessions_active_idx
    ON account_sessions (expires_at DESC, account_id)
    WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS account_credentials (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    credential_type TEXT        NOT NULL,
    secret_hash     TEXT        NOT NULL,
    changed_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT account_credentials_type_check CHECK (
        credential_type IN ('password', 'otp', 'passkey')
    ),
    CONSTRAINT account_credentials_secret_hash_check CHECK (btrim(secret_hash) <> ''),
    CONSTRAINT account_credentials_changed_at_check CHECK (
        changed_at IS NULL OR changed_at >= created_at
    )
);

CREATE INDEX IF NOT EXISTS account_credentials_account_id_idx
    ON account_credentials (account_id);

CREATE INDEX IF NOT EXISTS account_credentials_account_created_idx
    ON account_credentials (account_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS account_links_audit (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id       UUID        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    actor_account_id UUID        REFERENCES accounts (id) ON DELETE SET NULL,
    action           TEXT        NOT NULL,
    provider         TEXT        NOT NULL DEFAULT '',
    identity_id      UUID        REFERENCES account_identities (id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT account_links_audit_action_check CHECK (
        action IN ('linked', 'unlinked', 'login', 'merge_requested', 'merge_completed')
    ),
    CONSTRAINT account_links_audit_provider_check CHECK (
        provider = ''
        OR provider IN ('vk', 'telegram', 'google', 'apple', 'email', 'phone', 'password')
    )
);

CREATE INDEX IF NOT EXISTS account_links_audit_account_id_idx
    ON account_links_audit (account_id);

CREATE INDEX IF NOT EXISTS account_links_audit_account_created_idx
    ON account_links_audit (account_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS account_links_audit_actor_created_idx
    ON account_links_audit (actor_account_id, created_at DESC)
    WHERE actor_account_id IS NOT NULL;

COMMENT ON TABLE accounts IS
    'Future canonical account owner for billing, jobs, artifacts, referrals and history.';
COMMENT ON TABLE account_identities IS
    'External login/channel identities bound to accounts. Business ownership must use account_id, not provider IDs.';
COMMENT ON TABLE account_sessions IS
    'Future account sessions. Refresh tokens and device/network identifiers must be stored only as hashes.';
COMMENT ON TABLE account_credentials IS
    'Future credential verifier storage. No raw passwords, OTP secrets or passkeys.';
COMMENT ON TABLE account_links_audit IS
    'Safe audit trail for identity link/unlink/login/merge events. No raw provider tokens or PII payloads.';

COMMIT;
