-- 000037_account_business_dual_write.down.sql
-- Remove additive account ownership columns. Existing legacy user_id columns
-- remain untouched.

BEGIN;

DROP INDEX IF EXISTS ledger_entries_owner_account_id_created_idx;
DROP INDEX IF EXISTS credit_reservations_owner_account_id_status_idx;
DROP INDEX IF EXISTS credit_accounts_owner_account_id_currency_idx;
DROP INDEX IF EXISTS referrals_referred_account_id_key;
DROP INDEX IF EXISTS referrals_referrer_account_id_idx;
DROP INDEX IF EXISTS referral_codes_account_id_key;
DROP INDEX IF EXISTS conversations_account_id_source_updated_idx;
DROP INDEX IF EXISTS artifacts_owner_account_id_sha_idx;
DROP INDEX IF EXISTS artifacts_owner_account_id_created_idx;
DROP INDEX IF EXISTS payment_intents_account_id_status_updated_idx;
DROP INDEX IF EXISTS payment_intents_account_id_created_idx;
DROP INDEX IF EXISTS jobs_account_id_status_idx;
DROP INDEX IF EXISTS jobs_account_id_created_idx;

ALTER TABLE ledger_entries
    DROP COLUMN IF EXISTS owner_account_id;

ALTER TABLE credit_reservations
    DROP COLUMN IF EXISTS owner_account_id;

ALTER TABLE credit_accounts
    DROP COLUMN IF EXISTS owner_account_id;

ALTER TABLE referrals
    DROP COLUMN IF EXISTS referred_account_id,
    DROP COLUMN IF EXISTS referrer_account_id;

ALTER TABLE referral_codes
    DROP COLUMN IF EXISTS account_id;

ALTER TABLE conversations
    DROP COLUMN IF EXISTS account_id;

ALTER TABLE artifacts
    DROP COLUMN IF EXISTS owner_account_id;

ALTER TABLE payment_intents
    DROP COLUMN IF EXISTS account_id;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS account_id;

COMMIT;
