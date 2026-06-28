-- 000029_reduce_opening_balance_to_30.down.sql
-- Development rollback for the opening-balance normalization.

BEGIN;

WITH removed AS (
    DELETE FROM ledger_entries
    WHERE idempotency_key LIKE 'grant:open:reduce-to-30:%'
      AND reason = 'opening balance reduced to 30'
    RETURNING account_id, amount
)
UPDATE credit_accounts c
SET balance_cached = c.balance_cached - removed.amount,
    updated_at = now()
FROM removed
WHERE c.id = removed.account_id;

COMMIT;
