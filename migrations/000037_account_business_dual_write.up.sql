-- 000037_account_business_dual_write.up.sql
-- Add canonical account ownership to business tables while preserving legacy
-- user_id reads/writes during the rollout.

BEGIN;

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts (id) ON DELETE SET NULL;

ALTER TABLE payment_intents
    ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts (id) ON DELETE SET NULL;

ALTER TABLE artifacts
    ADD COLUMN IF NOT EXISTS owner_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL;

ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts (id) ON DELETE SET NULL;

ALTER TABLE referral_codes
    ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts (id) ON DELETE SET NULL;

ALTER TABLE referrals
    ADD COLUMN IF NOT EXISTS referrer_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS referred_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL;

ALTER TABLE credit_accounts
    ADD COLUMN IF NOT EXISTS owner_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL;

ALTER TABLE credit_reservations
    ADD COLUMN IF NOT EXISTS owner_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL;

ALTER TABLE ledger_entries
    ADD COLUMN IF NOT EXISTS owner_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL;

UPDATE jobs j
SET account_id = u.account_id
FROM users u
WHERE j.user_id = u.id
  AND j.account_id IS NULL
  AND u.account_id IS NOT NULL;

UPDATE payment_intents pi
SET account_id = u.account_id
FROM users u
WHERE pi.user_id = u.id
  AND pi.account_id IS NULL
  AND u.account_id IS NOT NULL;

UPDATE artifacts a
SET owner_account_id = u.account_id
FROM users u
WHERE a.owner_user_id = u.id
  AND a.owner_account_id IS NULL
  AND u.account_id IS NOT NULL;

UPDATE conversations c
SET account_id = u.account_id
FROM users u
WHERE c.user_id = u.id
  AND c.account_id IS NULL
  AND u.account_id IS NOT NULL;

UPDATE referral_codes rc
SET account_id = u.account_id
FROM users u
WHERE rc.user_id = u.id
  AND rc.account_id IS NULL
  AND u.account_id IS NOT NULL;

UPDATE referrals r
SET referrer_account_id = u.account_id
FROM users u
WHERE r.referrer_user_id = u.id
  AND r.referrer_account_id IS NULL
  AND u.account_id IS NOT NULL;

UPDATE referrals r
SET referred_account_id = u.account_id
FROM users u
WHERE r.referred_user_id = u.id
  AND r.referred_account_id IS NULL
  AND u.account_id IS NOT NULL;

UPDATE credit_accounts ca
SET owner_account_id = u.account_id
FROM users u
WHERE ca.user_id = u.id
  AND ca.owner_account_id IS NULL
  AND u.account_id IS NOT NULL;

UPDATE credit_reservations cr
SET owner_account_id = ca.owner_account_id
FROM credit_accounts ca
WHERE cr.account_id = ca.id
  AND cr.owner_account_id IS NULL
  AND ca.owner_account_id IS NOT NULL;

UPDATE ledger_entries le
SET owner_account_id = ca.owner_account_id
FROM credit_accounts ca
WHERE le.account_id = ca.id
  AND le.owner_account_id IS NULL
  AND ca.owner_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS jobs_account_id_created_idx
    ON jobs (account_id, created_at DESC, id DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS jobs_account_id_status_idx
    ON jobs (account_id, status, created_at DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS payment_intents_account_id_created_idx
    ON payment_intents (account_id, created_at DESC, id DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS payment_intents_account_id_status_updated_idx
    ON payment_intents (account_id, status, updated_at DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS artifacts_owner_account_id_created_idx
    ON artifacts (owner_account_id, created_at DESC, id DESC)
    WHERE owner_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS artifacts_owner_account_id_sha_idx
    ON artifacts (owner_account_id, sha256)
    WHERE owner_account_id IS NOT NULL AND sha256 <> '';

CREATE INDEX IF NOT EXISTS conversations_account_id_source_updated_idx
    ON conversations (account_id, source, updated_at DESC, id DESC)
    WHERE account_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS referral_codes_account_id_key
    ON referral_codes (account_id)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS referrals_referrer_account_id_idx
    ON referrals (referrer_account_id, created_at DESC, id DESC)
    WHERE referrer_account_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS referrals_referred_account_id_key
    ON referrals (referred_account_id)
    WHERE referred_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS credit_accounts_owner_account_id_currency_idx
    ON credit_accounts (owner_account_id, currency)
    WHERE owner_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS credit_reservations_owner_account_id_status_idx
    ON credit_reservations (owner_account_id, status, created_at DESC)
    WHERE owner_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ledger_entries_owner_account_id_created_idx
    ON ledger_entries (owner_account_id, created_at DESC, id DESC)
    WHERE owner_account_id IS NOT NULL;

COMMIT;
