-- 000036_backfill_vk_accounts.down.sql
-- Keep generated accounts/account_identities intact. Migration 000035 owns
-- dropping the account identity tables; this rollback only removes the
-- compatibility bridge from legacy users.

BEGIN;

DROP INDEX IF EXISTS users_account_id_idx;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_account_id_fkey,
    DROP COLUMN IF EXISTS account_id;

COMMIT;
