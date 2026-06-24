-- 000028_runtime_pricing_catalog.down.sql
-- Reverts DB-backed runtime pricing tables. This does not mutate jobs or
-- historical jobs.pricing_snapshot values.

DROP TABLE IF EXISTS runtime_pricing_audit_events;
DROP TABLE IF EXISTS runtime_generation_prices;
DROP TABLE IF EXISTS runtime_pricing_catalog_versions;
