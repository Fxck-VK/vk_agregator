-- 000001_init_schema.down.sql
-- Reverts 000001_init_schema.up.sql. Tables are dropped in reverse dependency
-- order so that foreign keys never block a drop.

BEGIN;

DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS credit_reservations;
DROP TABLE IF EXISTS credit_accounts;
DROP TABLE IF EXISTS deliveries;
DROP TABLE IF EXISTS artifact_variants;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS provider_tasks;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS commands;
DROP TABLE IF EXISTS users;

COMMIT;
