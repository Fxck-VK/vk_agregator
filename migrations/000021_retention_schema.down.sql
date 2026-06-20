-- 000021_retention_schema.down.sql
-- Revert retention metadata columns. This is provided for development only;
-- production should not run destructive rollback automatically.

BEGIN;

DROP INDEX IF EXISTS artifact_variants_retention_cleanup_idx;
DROP INDEX IF EXISTS artifacts_retention_cleanup_idx;
DROP INDEX IF EXISTS referral_events_retention_cleanup_idx;
DROP INDEX IF EXISTS moderation_results_retention_cleanup_idx;
DROP INDEX IF EXISTS outbox_events_retention_cleanup_idx;
DROP INDEX IF EXISTS inbound_events_retention_cleanup_idx;
DROP INDEX IF EXISTS conversation_summaries_retention_cleanup_idx;
DROP INDEX IF EXISTS conversation_messages_retention_cleanup_idx;
DROP INDEX IF EXISTS conversations_retention_cleanup_idx;
DROP INDEX IF EXISTS provider_tasks_retention_cleanup_idx;
DROP INDEX IF EXISTS jobs_retention_cleanup_idx;

ALTER TABLE artifact_variants
    DROP CONSTRAINT IF EXISTS artifact_variants_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS artifact_variants_artifact_tier_check,
    DROP CONSTRAINT IF EXISTS artifact_variants_retention_class_check,
    DROP COLUMN IF EXISTS artifact_tier,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS artifacts_artifact_tier_check,
    DROP CONSTRAINT IF EXISTS artifacts_retention_class_check,
    DROP COLUMN IF EXISTS artifact_tier,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE referral_events
    DROP CONSTRAINT IF EXISTS referral_events_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS referral_events_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE moderation_results
    DROP CONSTRAINT IF EXISTS moderation_results_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS moderation_results_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE outbox_events
    DROP CONSTRAINT IF EXISTS outbox_events_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS outbox_events_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE inbound_events
    DROP CONSTRAINT IF EXISTS inbound_events_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS inbound_events_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE conversation_summaries
    DROP CONSTRAINT IF EXISTS conversation_summaries_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS conversation_summaries_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE conversation_messages
    DROP CONSTRAINT IF EXISTS conversation_messages_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS conversation_messages_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at,
    DROP COLUMN IF EXISTS updated_at;

ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS conversations_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE provider_tasks
    DROP CONSTRAINT IF EXISTS provider_tasks_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS provider_tasks_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE jobs
    DROP CONSTRAINT IF EXISTS jobs_retention_timestamps_check,
    DROP CONSTRAINT IF EXISTS jobs_retention_class_check,
    DROP COLUMN IF EXISTS retention_class,
    DROP COLUMN IF EXISTS redacted_at,
    DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE payment_refunds
    DROP CONSTRAINT IF EXISTS payment_refunds_retention_class_check,
    DROP COLUMN IF EXISTS retention_class;

ALTER TABLE payment_events
    DROP CONSTRAINT IF EXISTS payment_events_retention_class_check,
    DROP COLUMN IF EXISTS retention_class;

ALTER TABLE payment_intents
    DROP CONSTRAINT IF EXISTS payment_intents_retention_class_check,
    DROP COLUMN IF EXISTS retention_class;

ALTER TABLE payment_products
    DROP CONSTRAINT IF EXISTS payment_products_retention_class_check,
    DROP COLUMN IF EXISTS retention_class;

ALTER TABLE ledger_entries
    DROP CONSTRAINT IF EXISTS ledger_entries_retention_class_check,
    DROP COLUMN IF EXISTS retention_class;

ALTER TABLE credit_reservations
    DROP CONSTRAINT IF EXISTS credit_reservations_retention_class_check,
    DROP COLUMN IF EXISTS retention_class;

ALTER TABLE credit_accounts
    DROP CONSTRAINT IF EXISTS credit_accounts_retention_class_check,
    DROP COLUMN IF EXISTS retention_class;

COMMIT;
