-- 000009_payments.up.sql
-- Payment intent foundation for shared VK Bot / VK Mini App top-ups.

BEGIN;

CREATE TABLE payment_products (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    code            TEXT        NOT NULL,
    title           TEXT        NOT NULL,
    amount          BIGINT      NOT NULL CHECK (amount > 0),
    currency        TEXT        NOT NULL DEFAULT 'rub',
    credits         BIGINT      NOT NULL CHECK (credits > 0),
    price_version   INTEGER     NOT NULL DEFAULT 1 CHECK (price_version > 0),
    vat_code        SMALLINT,
    payment_subject TEXT        NOT NULL DEFAULT '',
    payment_mode    TEXT        NOT NULL DEFAULT '',
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payment_products_code_key UNIQUE (code),
    CONSTRAINT payment_products_currency_check CHECK (currency IN ('rub'))
);

CREATE INDEX payment_products_active_idx ON payment_products (is_active, code);

CREATE TABLE payment_intents (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_id          UUID        REFERENCES payment_products (id) ON DELETE SET NULL,
    status              TEXT        NOT NULL,
    amount              BIGINT      NOT NULL CHECK (amount > 0),
    currency            TEXT        NOT NULL DEFAULT 'rub',
    credits             BIGINT      NOT NULL CHECK (credits > 0),
    price_version       INTEGER     NOT NULL CHECK (price_version > 0),
    provider            TEXT        NOT NULL DEFAULT 'mock',
    provider_payment_id TEXT UNIQUE,
    confirmation_url    TEXT        NOT NULL DEFAULT '',
    idempotency_key     TEXT        NOT NULL,
    receipt_email       TEXT,
    receipt_phone       TEXT,
    metadata            JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at          TIMESTAMPTZ,
    CONSTRAINT payment_intents_idempotency_key_key UNIQUE (idempotency_key),
    CONSTRAINT payment_intents_status_check CHECK (
        status IN (
            'created',
            'provider_pending',
            'waiting_for_user',
            'succeeded',
            'canceled',
            'expired',
            'failed',
            'refunded',
            'partially_refunded'
        )
    ),
    CONSTRAINT payment_intents_currency_check CHECK (currency IN ('rub')),
    CONSTRAINT payment_intents_provider_check CHECK (provider IN ('mock', 'yookassa')),
    CONSTRAINT payment_intents_receipt_contact_check CHECK (
        (receipt_email IS NOT NULL AND btrim(receipt_email) <> '')
        OR
        (receipt_phone IS NOT NULL AND btrim(receipt_phone) <> '')
    )
);

CREATE INDEX payment_intents_user_id_idx ON payment_intents (user_id);
CREATE INDEX payment_intents_status_idx ON payment_intents (status);
CREATE INDEX payment_intents_recon_idx ON payment_intents (status, updated_at);
CREATE INDEX payment_intents_user_created_idx ON payment_intents (user_id, created_at DESC);

CREATE TABLE payment_events (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    provider            TEXT        NOT NULL,
    event_type          TEXT        NOT NULL,
    provider_payment_id TEXT,
    provider_refund_id  TEXT,
    dedup_key           TEXT        NOT NULL,
    payload             JSONB       NOT NULL,
    processed_at        TIMESTAMPTZ,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payment_events_dedup_key_key UNIQUE (dedup_key),
    CONSTRAINT payment_events_provider_check CHECK (provider IN ('mock', 'yookassa'))
);

CREATE INDEX payment_events_provider_payment_idx ON payment_events (provider, provider_payment_id);
CREATE INDEX payment_events_provider_refund_idx ON payment_events (provider, provider_refund_id);
CREATE INDEX payment_events_unprocessed_idx ON payment_events (received_at)
    WHERE processed_at IS NULL;

CREATE TABLE payment_refunds (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    intent_id          UUID        NOT NULL REFERENCES payment_intents (id) ON DELETE CASCADE,
    provider_refund_id TEXT UNIQUE,
    amount             BIGINT      NOT NULL CHECK (amount > 0),
    status             TEXT        NOT NULL,
    idempotency_key    TEXT        NOT NULL,
    reason             TEXT        NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payment_refunds_idempotency_key_key UNIQUE (idempotency_key),
    CONSTRAINT payment_refunds_status_check CHECK (
        status IN ('created', 'provider_pending', 'succeeded', 'failed', 'canceled')
    )
);

CREATE INDEX payment_refunds_intent_id_idx ON payment_refunds (intent_id);
CREATE INDEX payment_refunds_status_idx ON payment_refunds (status);

COMMIT;
