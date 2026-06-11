-- 000013_referral_activation_status.up.sql
-- Adds an explicit referral funnel status without removing legacy reward_status.

BEGIN;

ALTER TABLE referrals
    ADD COLUMN status TEXT,
    ADD COLUMN first_seen_at TIMESTAMPTZ,
    ADD COLUMN activated_at TIMESTAMPTZ;

UPDATE referrals
SET
    status = CASE
        WHEN reward_status = 'applied' THEN 'rewarded'
        ELSE 'registered'
    END,
    first_seen_at = created_at,
    activated_at = CASE
        WHEN reward_status = 'applied' THEN COALESCE(rewarded_at, updated_at, created_at)
        ELSE NULL
    END
WHERE status IS NULL
   OR first_seen_at IS NULL;

ALTER TABLE referrals
    ALTER COLUMN status SET NOT NULL,
    ALTER COLUMN status SET DEFAULT 'registered',
    ALTER COLUMN first_seen_at SET NOT NULL,
    ALTER COLUMN first_seen_at SET DEFAULT now(),
    ADD CONSTRAINT referrals_status_check CHECK (status IN ('registered', 'activated', 'rewarded'));

CREATE INDEX referrals_status_idx ON referrals (status);
CREATE INDEX referrals_referrer_status_idx ON referrals (referrer_user_id, status);

COMMIT;
