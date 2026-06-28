-- 000029_reduce_opening_balance_to_30.up.sql
-- Normalize the legacy free opening balance from 1000 credits down to 30.
--
-- Safety rules:
--   * Only accounts with an opening grant above 30 are considered.
--   * Other positive committed ledger entries are preserved so paid top-ups,
--     refunds and referral rewards are not reduced.
--   * Accounts already at or below 30 are left unchanged.
--   * The correction is an idempotent committed ledger adjustment, preserving
--     append-only accounting and keeping balance_cached aligned with ledger.

BEGIN;

WITH account_ledger AS (
    SELECT
        c.id,
        c.balance_cached,
        COALESCE(SUM(l.amount) FILTER (
            WHERE l.status = 'committed'
              AND l.idempotency_key = 'grant:open:' || c.id::text
        ), 0) AS opening_grant,
        COALESCE(SUM(l.amount) FILTER (
            WHERE l.status = 'committed'
              AND l.amount > 0
              AND l.idempotency_key <> 'grant:open:' || c.id::text
        ), 0) AS other_positive_amount
    FROM credit_accounts c
    LEFT JOIN ledger_entries l ON l.account_id = c.id
    WHERE c.currency = 'credits'
    GROUP BY c.id, c.balance_cached
),
targets AS (
    SELECT
        id,
        GREATEST(30, 30 + other_positive_amount) - balance_cached AS adjustment_amount
    FROM account_ledger
    WHERE opening_grant > 30
      AND balance_cached > GREATEST(30, 30 + other_positive_amount)
),
inserted AS (
    INSERT INTO ledger_entries (account_id, type, amount, status, idempotency_key, reason)
    SELECT
        id,
        'adjustment',
        adjustment_amount,
        'committed',
        'grant:open:reduce-to-30:' || id::text,
        'opening balance reduced to 30'
    FROM targets
    ON CONFLICT (idempotency_key) DO NOTHING
    RETURNING account_id, amount
)
UPDATE credit_accounts c
SET balance_cached = c.balance_cached + inserted.amount,
    updated_at = now()
FROM inserted
WHERE c.id = inserted.account_id;

COMMIT;
