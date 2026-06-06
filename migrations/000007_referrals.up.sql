-- 000007_referrals.up.sql
-- Shared VK referral system for bot and future Mini App entry points.

BEGIN;

CREATE TABLE referral_codes (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT referral_codes_user_id_key UNIQUE (user_id),
    CONSTRAINT referral_codes_code_key UNIQUE (code)
);

CREATE INDEX referral_codes_user_id_idx ON referral_codes (user_id);

CREATE TABLE referrals (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_user_id UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    referred_user_id UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    referral_code    TEXT        NOT NULL,
    source           TEXT        NOT NULL,
    reward_status    TEXT        NOT NULL DEFAULT 'pending',
    rewarded_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT referrals_referred_user_id_key UNIQUE (referred_user_id),
    CONSTRAINT referrals_no_self_referral CHECK (referrer_user_id <> referred_user_id),
    CONSTRAINT referrals_source_check CHECK (source IN ('vk_bot', 'vk_miniapp')),
    CONSTRAINT referrals_reward_status_check CHECK (reward_status IN ('pending', 'applied'))
);

CREATE INDEX referrals_referrer_user_id_idx ON referrals (referrer_user_id);
CREATE INDEX referrals_referral_code_idx ON referrals (referral_code);

COMMIT;
