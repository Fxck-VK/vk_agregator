-- 000006_conversation_context.up.sql
-- Persist compact dialog memory for VK text mode.

BEGIN;

CREATE TABLE conversations (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vk_peer_id  BIGINT      NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'active',
    title       TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX conversations_active_user_peer_key
    ON conversations (user_id, vk_peer_id)
    WHERE status = 'active';

CREATE INDEX conversations_user_updated_idx
    ON conversations (user_id, updated_at DESC);

CREATE TABLE conversation_messages (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID        NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    job_id          UUID        NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    seq             BIGSERIAL   NOT NULL,
    role            TEXT        NOT NULL,
    text            TEXT        NOT NULL DEFAULT '',
    token_count     INTEGER     NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT conversation_messages_job_role_key UNIQUE (job_id, role)
);

CREATE UNIQUE INDEX conversation_messages_conversation_seq_key
    ON conversation_messages (conversation_id, seq);

CREATE INDEX conversation_messages_recent_idx
    ON conversation_messages (conversation_id, seq DESC);

CREATE TABLE conversation_summaries (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id       UUID        NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    text                 TEXT        NOT NULL DEFAULT '',
    token_count          INTEGER     NOT NULL DEFAULT 0,
    summarized_until_seq BIGINT      NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT conversation_summaries_conversation_key UNIQUE (conversation_id)
);

CREATE INDEX conversation_summaries_conversation_seq_idx
    ON conversation_summaries (conversation_id, summarized_until_seq DESC);

COMMIT;
