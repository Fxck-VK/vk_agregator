-- 000024_command_raw_text_retention.up.sql
-- Add retention metadata for commands.raw_text. This migration is additive
-- only: raw command text is redacted later by the maintenance worker.

BEGIN;

ALTER TABLE commands
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'user_content';

ALTER TABLE commands
    DROP CONSTRAINT IF EXISTS commands_retention_class_check,
    DROP CONSTRAINT IF EXISTS commands_retention_timestamps_check,
    ADD CONSTRAINT commands_retention_class_check CHECK (
        retention_class IN (
            'financial',
            'operational',
            'user_content',
            'provider_payload',
            'artifact_metadata',
            'analytics_aggregate',
            'temporary_cache'
        )
    ),
    ADD CONSTRAINT commands_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

CREATE INDEX IF NOT EXISTS commands_retention_hot_idx
    ON commands (retention_class, created_at, id)
    WHERE deleted_at IS NULL
      AND redacted_at IS NULL
      AND expires_at IS NULL;

CREATE INDEX IF NOT EXISTS commands_retention_cleanup_idx
    ON commands (retention_class, expires_at, created_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

COMMENT ON COLUMN commands.raw_text IS
    'Original user command text. Bounded-retention user content redacted by maintenance after job-safe TTL.';
COMMENT ON COLUMN commands.retention_class IS
    'User content class for raw command text. Cleanup must keep idempotency, job links and billing audit intact.';

COMMIT;
