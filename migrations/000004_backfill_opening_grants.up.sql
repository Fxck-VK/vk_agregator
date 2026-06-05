-- 000004_backfill_opening_grants.up.sql
-- Backfill opening-balance ledger entries for accounts created before the
-- starting grant was recorded as a committed ledger entry. Historically a new
-- account's balance_cached was seeded directly with the starting grant, which
-- left balance_cached above the committed ledger sum and triggered the worker's
-- "billing balance mismatch" reconciliation warning (invariant #14, audit B1).
--
-- This inserts exactly the missing opening grant for each diverging account: the
-- positive difference between balance_cached and the committed ledger sum. It
-- never reduces a balance and never touches already-spent (negative) movements,
-- so captured/refunded history is preserved untouched.

BEGIN;

INSERT INTO ledger_entries (account_id, type, amount, status, idempotency_key, reason)
SELECT c.id,
       'topup',
       c.balance_cached - COALESCE(SUM(l.amount) FILTER (WHERE l.status = 'committed'), 0),
       'committed',
       'grant:open:' || c.id::text,
       'opening balance grant (backfill)'
FROM credit_accounts c
LEFT JOIN ledger_entries l ON l.account_id = c.id
GROUP BY c.id, c.balance_cached
HAVING c.balance_cached - COALESCE(SUM(l.amount) FILTER (WHERE l.status = 'committed'), 0) <> 0
ON CONFLICT (idempotency_key) DO NOTHING;

COMMIT;
