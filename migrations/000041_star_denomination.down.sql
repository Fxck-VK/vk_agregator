-- 000041_star_denomination.down.sql
-- Financial denomination conversion is intentionally irreversible. Rolling
-- back application code must not rewrite balances, ledger history, settled
-- payments or immutable pricing snapshots.

BEGIN;
COMMIT;
