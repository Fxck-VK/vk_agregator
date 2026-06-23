-- 000024_command_raw_text_retention.down.sql
-- Revert command retention metadata. Development only; do not run automatic
-- production schema rollback after raw_text redaction has occurred.

BEGIN;

DROP INDEX IF EXISTS commands_retention_cleanup_idx;
DROP INDEX IF EXISTS commands_retention_hot_idx;

ALTER TABLE commands
    DROP CONSTRAINT IF EXISTS commands_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS commands_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

COMMENT ON COLUMN commands.raw_text IS NULL;

COMMIT;
