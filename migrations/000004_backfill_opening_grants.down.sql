-- 000004_backfill_opening_grants.down.sql
-- Remove the backfilled opening-balance grants. The cached balances are left
-- unchanged, so rolling back re-introduces the historical ledger/cache
-- divergence by design (the cache is a projection, not source of truth).
DELETE FROM ledger_entries
WHERE reason = 'opening balance grant (backfill)'
  AND idempotency_key LIKE 'grant:open:%';
