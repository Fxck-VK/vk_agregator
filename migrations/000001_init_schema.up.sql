-- 000001_init_schema.up.sql
-- Initial schema for the VK AI Aggregator domain model.
-- Conventions:
--   * UUID primary keys generated via gen_random_uuid().
--   * JSONB for free-form context, metadata and payloads.
--   * created_at / updated_at default to now().

BEGIN;

-- pgcrypto provides gen_random_uuid().
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    vk_user_id   BIGINT      NOT NULL,
    role         TEXT        NOT NULL DEFAULT 'user',
    status       TEXT        NOT NULL DEFAULT 'active',
    locale       TEXT        NOT NULL DEFAULT 'ru',
    timezone     TEXT        NOT NULL DEFAULT 'Europe/Moscow',
    risk_level   INTEGER     NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_vk_user_id_key UNIQUE (vk_user_id)
);

-- ---------------------------------------------------------------------------
-- commands
-- ---------------------------------------------------------------------------
CREATE TABLE commands (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vk_peer_id               BIGINT      NOT NULL,
    inbound_event_id         UUID        NOT NULL,
    type                     TEXT        NOT NULL,
    raw_text                 TEXT        NOT NULL DEFAULT '',
    args                     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    attachment_artifact_ids  UUID[]      NOT NULL DEFAULT '{}',
    idempotency_key          TEXT        NOT NULL,
    correlation_id           TEXT        NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT commands_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX commands_user_id_idx ON commands (user_id);

-- ---------------------------------------------------------------------------
-- jobs
-- ---------------------------------------------------------------------------
CREATE TABLE jobs (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vk_peer_id          BIGINT      NOT NULL,
    command_id          UUID        REFERENCES commands (id) ON DELETE SET NULL,
    operation_type      TEXT        NOT NULL,
    modality            TEXT        NOT NULL,
    provider_id         UUID,
    model_id            UUID,
    status              TEXT        NOT NULL DEFAULT 'received',
    priority            INTEGER     NOT NULL DEFAULT 0,
    idempotency_key     TEXT        NOT NULL,
    correlation_id      TEXT        NOT NULL DEFAULT '',
    input_artifact_ids  UUID[]      NOT NULL DEFAULT '{}',
    output_artifact_ids UUID[]      NOT NULL DEFAULT '{}',
    params              JSONB       NOT NULL DEFAULT '{}'::jsonb,
    cost_estimate       BIGINT      NOT NULL DEFAULT 0,
    cost_reserved       BIGINT      NOT NULL DEFAULT 0,
    cost_captured       BIGINT      NOT NULL DEFAULT 0,
    error_code          TEXT        NOT NULL DEFAULT '',
    error_message       TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at          TIMESTAMPTZ,
    CONSTRAINT jobs_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX jobs_user_id_idx ON jobs (user_id);
CREATE INDEX jobs_status_idx ON jobs (status);

-- ---------------------------------------------------------------------------
-- provider_tasks
-- ---------------------------------------------------------------------------
CREATE TABLE provider_tasks (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID        NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    provider        TEXT        NOT NULL,
    model_code      TEXT        NOT NULL DEFAULT '',
    external_id     TEXT        NOT NULL DEFAULT '',
    attempt_no      INTEGER     NOT NULL DEFAULT 1,
    status          TEXT        NOT NULL DEFAULT 'pending',
    request         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    result          JSONB,
    error_class     TEXT        NOT NULL DEFAULT '',
    idempotency_key TEXT        NOT NULL,
    submitted_at    TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT provider_tasks_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX provider_tasks_job_id_idx ON provider_tasks (job_id);
CREATE INDEX provider_tasks_external_id_idx ON provider_tasks (provider, external_id);

-- ---------------------------------------------------------------------------
-- artifacts
-- ---------------------------------------------------------------------------
CREATE TABLE artifacts (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id  UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_id         UUID        REFERENCES jobs (id) ON DELETE SET NULL,
    kind           TEXT        NOT NULL,
    media_type     TEXT        NOT NULL,
    mime_type      TEXT        NOT NULL DEFAULT '',
    storage_bucket TEXT        NOT NULL DEFAULT '',
    storage_key    TEXT        NOT NULL DEFAULT '',
    public_url     TEXT        NOT NULL DEFAULT '',
    sha256         TEXT        NOT NULL DEFAULT '',
    size_bytes     BIGINT      NOT NULL DEFAULT 0,
    width          INTEGER     NOT NULL DEFAULT 0,
    height         INTEGER     NOT NULL DEFAULT 0,
    duration_ms    BIGINT      NOT NULL DEFAULT 0,
    status         TEXT        NOT NULL DEFAULT 'pending',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX artifacts_owner_user_id_idx ON artifacts (owner_user_id);
CREATE INDEX artifacts_job_id_idx ON artifacts (job_id);
CREATE INDEX artifacts_owner_sha256_idx ON artifacts (owner_user_id, sha256);

-- ---------------------------------------------------------------------------
-- artifact_variants
-- ---------------------------------------------------------------------------
CREATE TABLE artifact_variants (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    artifact_id    UUID        NOT NULL REFERENCES artifacts (id) ON DELETE CASCADE,
    variant_type   TEXT        NOT NULL,
    storage_bucket TEXT        NOT NULL DEFAULT '',
    storage_key    TEXT        NOT NULL DEFAULT '',
    mime_type      TEXT        NOT NULL DEFAULT '',
    size_bytes     BIGINT      NOT NULL DEFAULT 0,
    width          INTEGER     NOT NULL DEFAULT 0,
    height         INTEGER     NOT NULL DEFAULT 0,
    duration_ms    BIGINT      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT artifact_variants_artifact_type_key UNIQUE (artifact_id, variant_type)
);

CREATE INDEX artifact_variants_artifact_id_idx ON artifact_variants (artifact_id);

-- ---------------------------------------------------------------------------
-- deliveries
-- ---------------------------------------------------------------------------
CREATE TABLE deliveries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID        NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    user_id         UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vk_peer_id      BIGINT      NOT NULL,
    artifact_id     UUID        REFERENCES artifacts (id) ON DELETE SET NULL,
    type            TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending',
    vk_random_id    BIGINT      NOT NULL,
    vk_message_id   BIGINT,
    attachment      TEXT        NOT NULL DEFAULT '',
    text            TEXT        NOT NULL DEFAULT '',
    attempt_no      INTEGER     NOT NULL DEFAULT 1,
    idempotency_key TEXT        NOT NULL,
    error_code      TEXT        NOT NULL DEFAULT '',
    error_message   TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT deliveries_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX deliveries_job_id_idx ON deliveries (job_id);

-- ---------------------------------------------------------------------------
-- credit_accounts
-- ---------------------------------------------------------------------------
CREATE TABLE credit_accounts (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    currency       TEXT        NOT NULL DEFAULT 'credits',
    balance_cached BIGINT      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT credit_accounts_user_currency_key UNIQUE (user_id, currency)
);

-- ---------------------------------------------------------------------------
-- credit_reservations
-- ---------------------------------------------------------------------------
CREATE TABLE credit_reservations (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID        NOT NULL REFERENCES credit_accounts (id) ON DELETE CASCADE,
    job_id          UUID        NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    amount          BIGINT      NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'reserved',
    idempotency_key TEXT        NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT credit_reservations_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX credit_reservations_account_id_idx ON credit_reservations (account_id);
CREATE INDEX credit_reservations_job_id_idx ON credit_reservations (job_id);

-- ---------------------------------------------------------------------------
-- ledger_entries (append-only)
-- ---------------------------------------------------------------------------
CREATE TABLE ledger_entries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID        NOT NULL REFERENCES credit_accounts (id) ON DELETE CASCADE,
    job_id          UUID        REFERENCES jobs (id) ON DELETE SET NULL,
    reservation_id  UUID        REFERENCES credit_reservations (id) ON DELETE SET NULL,
    type            TEXT        NOT NULL,
    amount          BIGINT      NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'committed',
    idempotency_key TEXT        NOT NULL,
    reason          TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ledger_entries_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX ledger_entries_account_id_idx ON ledger_entries (account_id);
CREATE INDEX ledger_entries_job_id_idx ON ledger_entries (job_id);

-- ---------------------------------------------------------------------------
-- outbox_events
-- ---------------------------------------------------------------------------
CREATE TABLE outbox_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type  TEXT        NOT NULL,
    aggregate_id    UUID        NOT NULL,
    event_type      TEXT        NOT NULL,
    payload         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    status          TEXT        NOT NULL DEFAULT 'pending',
    attempts        INTEGER     NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ
);

CREATE INDEX outbox_events_pending_idx ON outbox_events (status, next_attempt_at);

-- ---------------------------------------------------------------------------
-- idempotency_keys
-- ---------------------------------------------------------------------------
CREATE TABLE idempotency_keys (
    key           TEXT        PRIMARY KEY,
    scope         TEXT        NOT NULL,
    resource_type TEXT        NOT NULL DEFAULT '',
    resource_id   UUID,
    status        TEXT        NOT NULL DEFAULT 'started',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);

COMMIT;
