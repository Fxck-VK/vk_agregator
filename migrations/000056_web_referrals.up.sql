-- 000056_web_referrals.up.sql
-- Account-native web referral visits and nullable legacy referral owner ids.

BEGIN;

ALTER TABLE referral_codes
    ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE referrals
    ALTER COLUMN referrer_user_id DROP NOT NULL,
    ALTER COLUMN referred_user_id DROP NOT NULL;

ALTER TABLE referral_codes
    DROP CONSTRAINT IF EXISTS referral_codes_owner_present_check,
    ADD CONSTRAINT referral_codes_owner_present_check
    CHECK (user_id IS NOT NULL OR account_id IS NOT NULL);

ALTER TABLE referrals
    DROP CONSTRAINT IF EXISTS referrals_referrer_owner_present_check,
    DROP CONSTRAINT IF EXISTS referrals_referred_owner_present_check,
    DROP CONSTRAINT IF EXISTS referrals_no_self_referral,
    ADD CONSTRAINT referrals_referrer_owner_present_check
    CHECK (referrer_user_id IS NOT NULL OR referrer_account_id IS NOT NULL),
    ADD CONSTRAINT referrals_referred_owner_present_check
    CHECK (referred_user_id IS NOT NULL OR referred_account_id IS NOT NULL),
    ADD CONSTRAINT referrals_no_self_referral
    CHECK (
        (referrer_user_id IS NULL OR referred_user_id IS NULL OR referrer_user_id <> referred_user_id)
        AND (referrer_account_id IS NULL OR referred_account_id IS NULL OR referrer_account_id <> referred_account_id)
    );

ALTER TABLE referrals
    DROP CONSTRAINT IF EXISTS referrals_source_check,
    ADD CONSTRAINT referrals_source_check
    CHECK (source IN ('vk_bot', 'vk_miniapp', 'web'));

CREATE TABLE IF NOT EXISTS web_referral_visits (
    token_hash          TEXT        PRIMARY KEY,
    code                TEXT        NOT NULL REFERENCES referral_codes (code) ON DELETE CASCADE,
    referrer_account_id UUID        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ NOT NULL,
    expires_at          TIMESTAMPTZ NOT NULL,
    accepted_account_id UUID        REFERENCES accounts (id) ON DELETE SET NULL,
    CONSTRAINT web_referral_visits_token_hash_check CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT web_referral_visits_code_check CHECK (btrim(code) <> ''),
    CONSTRAINT web_referral_visits_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT web_referral_visits_no_self_accept_check CHECK (
        accepted_account_id IS NULL OR accepted_account_id <> referrer_account_id
    )
);

CREATE TABLE IF NOT EXISTS web_referral_visit_aggregates (
    referrer_account_id UUID        PRIMARY KEY REFERENCES accounts (id) ON DELETE CASCADE,
    visits_total        BIGINT      NOT NULL DEFAULT 0,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT web_referral_visit_aggregates_total_check CHECK (visits_total >= 0)
);

CREATE INDEX IF NOT EXISTS web_referral_visits_referrer_created_idx
    ON web_referral_visits (referrer_account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS web_referral_visits_accepted_account_idx
    ON web_referral_visits (accepted_account_id)
    WHERE accepted_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS web_referral_visits_expires_idx
    ON web_referral_visits (expires_at);

COMMIT;
