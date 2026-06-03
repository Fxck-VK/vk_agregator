-- 000002_inbound_events.up.sql
-- Raw inbound events received by inbound gateways (e.g. VK callbacks). Stored
-- before any business processing for audit and idempotent reprocessing.

BEGIN;

CREATE TABLE inbound_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    source          TEXT        NOT NULL,
    event_type      TEXT        NOT NULL,
    group_id        BIGINT      NOT NULL DEFAULT 0,
    vk_event_id     TEXT        NOT NULL DEFAULT '',
    peer_id         BIGINT      NOT NULL DEFAULT 0,
    vk_user_id      BIGINT      NOT NULL DEFAULT 0,
    payload         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    status          TEXT        NOT NULL DEFAULT 'received',
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT inbound_events_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX inbound_events_source_event_idx ON inbound_events (source, event_type);

COMMIT;
