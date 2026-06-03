-- 000002_inbound_events.down.sql
-- Reverts 000002_inbound_events.up.sql.

BEGIN;

DROP TABLE IF EXISTS inbound_events;

COMMIT;
