-- 000014_referral_events.up.sql
-- No-PII referral analytics event stream for future funnel steps.

BEGIN;

CREATE TABLE referral_events (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    referral_id      UUID        REFERENCES referrals (id) ON DELETE SET NULL,
    referrer_user_id UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    referred_user_id UUID        REFERENCES users (id) ON DELETE SET NULL,
    referral_code    TEXT,
    event_type       TEXT        NOT NULL,
    source           TEXT,
    idempotency_key  TEXT        NOT NULL,
    metadata         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT referral_events_event_type_check CHECK (
        event_type IN (
            'link_opened',
            'registered',
            'activated',
            'rewarded',
            'first_generation',
            'first_payment'
        )
    ),
    CONSTRAINT referral_events_source_check CHECK (
        source IS NULL OR source IN ('vk_bot', 'vk_miniapp')
    ),
    CONSTRAINT referral_events_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX referral_events_referrer_type_created_idx
    ON referral_events (referrer_user_id, event_type, created_at DESC);

CREATE INDEX referral_events_referral_id_idx
    ON referral_events (referral_id);

COMMIT;
