-- 000056_web_referrals.down.sql
-- Restore the legacy-only referral schema only when doing so would not discard
-- account-native web referral data or collected visit analytics.

BEGIN;

DO $$
DECLARE
    has_visit_rows BOOLEAN := FALSE;
    has_visit_aggregate_rows BOOLEAN := FALSE;
BEGIN
    IF to_regclass('web_referral_visits') IS NOT NULL THEN
        EXECUTE 'SELECT EXISTS (SELECT 1 FROM web_referral_visits)' INTO has_visit_rows;
    END IF;

    IF to_regclass('web_referral_visit_aggregates') IS NOT NULL THEN
        EXECUTE 'SELECT EXISTS (SELECT 1 FROM web_referral_visit_aggregates)' INTO has_visit_aggregate_rows;
    END IF;

    IF has_visit_rows THEN
        RAISE EXCEPTION
            'cannot rollback web referrals: web_referral_visits contains collected analytics';
    END IF;

    IF has_visit_aggregate_rows THEN
        RAISE EXCEPTION
            'cannot rollback web referrals: web_referral_visit_aggregates contains retained analytics';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM referral_codes
        WHERE user_id IS NULL
    ) THEN
        RAISE EXCEPTION
            'cannot rollback web referrals: referral_codes contains account-native rows';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM referrals
        WHERE source = 'web'
           OR referrer_user_id IS NULL
           OR referred_user_id IS NULL
    ) THEN
        RAISE EXCEPTION
            'cannot rollback web referrals: referrals contains account-native rows';
    END IF;
END $$;

DROP TABLE IF EXISTS web_referral_visit_aggregates;
DROP TABLE IF EXISTS web_referral_visits;

ALTER TABLE referral_codes
    DROP CONSTRAINT IF EXISTS referral_codes_owner_present_check,
    ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE referrals
    DROP CONSTRAINT IF EXISTS referrals_referrer_owner_present_check,
    DROP CONSTRAINT IF EXISTS referrals_referred_owner_present_check,
    DROP CONSTRAINT IF EXISTS referrals_no_self_referral,
    DROP CONSTRAINT IF EXISTS referrals_source_check,
    ALTER COLUMN referrer_user_id SET NOT NULL,
    ALTER COLUMN referred_user_id SET NOT NULL,
    ADD CONSTRAINT referrals_no_self_referral CHECK (referrer_user_id <> referred_user_id),
    ADD CONSTRAINT referrals_source_check CHECK (source IN ('vk_bot', 'vk_miniapp'));

COMMIT;
