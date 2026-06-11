-- 000013_referral_activation_status.down.sql

BEGIN;

DROP INDEX IF EXISTS referrals_referrer_status_idx;
DROP INDEX IF EXISTS referrals_status_idx;

ALTER TABLE referrals
    DROP CONSTRAINT IF EXISTS referrals_status_check,
    DROP COLUMN IF EXISTS activated_at,
    DROP COLUMN IF EXISTS first_seen_at,
    DROP COLUMN IF EXISTS status;

COMMIT;
