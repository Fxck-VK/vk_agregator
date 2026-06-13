-- 000019_operator_audit_entries.down.sql

BEGIN;

DROP INDEX IF EXISTS operator_audit_entries_filter_idx;
DROP INDEX IF EXISTS operator_audit_entries_created_idx;
DROP TABLE IF EXISTS operator_audit_entries;

COMMIT;
