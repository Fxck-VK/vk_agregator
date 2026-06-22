-- 000021_retention_schema.up.sql
-- Add retention metadata to cleanup-eligible tables and explicitly mark
-- billing/ledger tables as protected financial data. This migration is
-- additive only: it does not enable any automatic deletion by itself.

BEGIN;

-- --------------------------------------------------------------------------
-- Protected financial/billing tables.
-- These rows are classified, but intentionally do not get deleted_at or
-- retention expires_at fields for generic cleanup. Some tables already have
-- business expires_at values; retention code must skip retention_class=financial.
-- --------------------------------------------------------------------------

ALTER TABLE credit_accounts
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'financial';

ALTER TABLE credit_accounts
    DROP CONSTRAINT IF EXISTS credit_accounts_retention_class_check,
    ADD CONSTRAINT credit_accounts_retention_class_check
        CHECK (retention_class = 'financial');

ALTER TABLE credit_reservations
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'financial';

ALTER TABLE credit_reservations
    DROP CONSTRAINT IF EXISTS credit_reservations_retention_class_check,
    ADD CONSTRAINT credit_reservations_retention_class_check
        CHECK (retention_class = 'financial');

ALTER TABLE ledger_entries
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'financial';

ALTER TABLE ledger_entries
    DROP CONSTRAINT IF EXISTS ledger_entries_retention_class_check,
    ADD CONSTRAINT ledger_entries_retention_class_check
        CHECK (retention_class = 'financial');

ALTER TABLE payment_products
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'financial';

ALTER TABLE payment_products
    DROP CONSTRAINT IF EXISTS payment_products_retention_class_check,
    ADD CONSTRAINT payment_products_retention_class_check
        CHECK (retention_class = 'financial');

ALTER TABLE payment_intents
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'financial';

ALTER TABLE payment_intents
    DROP CONSTRAINT IF EXISTS payment_intents_retention_class_check,
    ADD CONSTRAINT payment_intents_retention_class_check
        CHECK (retention_class = 'financial');

ALTER TABLE payment_events
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'financial';

ALTER TABLE payment_events
    DROP CONSTRAINT IF EXISTS payment_events_retention_class_check,
    ADD CONSTRAINT payment_events_retention_class_check
        CHECK (retention_class = 'financial');

ALTER TABLE payment_refunds
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'financial';

ALTER TABLE payment_refunds
    DROP CONSTRAINT IF EXISTS payment_refunds_retention_class_check,
    ADD CONSTRAINT payment_refunds_retention_class_check
        CHECK (retention_class = 'financial');

COMMENT ON COLUMN ledger_entries.retention_class IS
    'Protected financial audit data. Generic retention cleanup must not delete ledger rows.';
COMMENT ON COLUMN payment_intents.retention_class IS
    'Protected financial/payment data. payment_intents.expires_at is payment lifecycle, not retention cleanup.';
COMMENT ON COLUMN payment_events.retention_class IS
    'Protected payment webhook inbox data. Generic retention cleanup must not delete payment events.';

-- --------------------------------------------------------------------------
-- Job/provider operational and provider payload data.
-- --------------------------------------------------------------------------

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'operational';

ALTER TABLE jobs
    DROP CONSTRAINT IF EXISTS jobs_retention_class_check,
    DROP CONSTRAINT IF EXISTS jobs_retention_timestamps_check,
    ADD CONSTRAINT jobs_retention_class_check CHECK (
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
    ADD CONSTRAINT jobs_retention_timestamps_check CHECK (
        (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

ALTER TABLE provider_tasks
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'provider_payload';

ALTER TABLE provider_tasks
    DROP CONSTRAINT IF EXISTS provider_tasks_retention_class_check,
    DROP CONSTRAINT IF EXISTS provider_tasks_retention_timestamps_check,
    ADD CONSTRAINT provider_tasks_retention_class_check CHECK (
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
    ADD CONSTRAINT provider_tasks_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

CREATE INDEX IF NOT EXISTS jobs_retention_cleanup_idx
    ON jobs (retention_class, expires_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS provider_tasks_retention_cleanup_idx
    ON provider_tasks (retention_class, expires_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

COMMENT ON COLUMN jobs.retention_class IS
    'Retention data class for job metadata. Raw params may be redacted without deleting financial ledger data.';
COMMENT ON COLUMN provider_tasks.retention_class IS
    'Provider task payload class. Raw request/result JSON should be short-lived or redacted.';

-- --------------------------------------------------------------------------
-- Conversation/user content data.
-- --------------------------------------------------------------------------

ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'operational';

ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_retention_class_check,
    DROP CONSTRAINT IF EXISTS conversations_retention_timestamps_check,
    ADD CONSTRAINT conversations_retention_class_check CHECK (
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
    ADD CONSTRAINT conversations_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

ALTER TABLE conversation_messages
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'user_content';

ALTER TABLE conversation_messages
    DROP CONSTRAINT IF EXISTS conversation_messages_retention_class_check,
    DROP CONSTRAINT IF EXISTS conversation_messages_retention_timestamps_check,
    ADD CONSTRAINT conversation_messages_retention_class_check CHECK (
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
    ADD CONSTRAINT conversation_messages_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

ALTER TABLE conversation_summaries
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'user_content';

ALTER TABLE conversation_summaries
    DROP CONSTRAINT IF EXISTS conversation_summaries_retention_class_check,
    DROP CONSTRAINT IF EXISTS conversation_summaries_retention_timestamps_check,
    ADD CONSTRAINT conversation_summaries_retention_class_check CHECK (
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
    ADD CONSTRAINT conversation_summaries_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

CREATE INDEX IF NOT EXISTS conversations_retention_cleanup_idx
    ON conversations (retention_class, expires_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS conversation_messages_retention_cleanup_idx
    ON conversation_messages (retention_class, expires_at, conversation_id, seq)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS conversation_summaries_retention_cleanup_idx
    ON conversation_summaries (retention_class, expires_at, conversation_id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

COMMENT ON COLUMN conversation_messages.retention_class IS
    'User content class. Conversation messages are bounded-retention and redaction-eligible.';
COMMENT ON COLUMN conversation_summaries.retention_class IS
    'User content class. Summaries may live longer than messages but remain redaction-eligible.';

-- --------------------------------------------------------------------------
-- Inbound/outbox/moderation/referral event data.
-- --------------------------------------------------------------------------

ALTER TABLE inbound_events
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'user_content';

ALTER TABLE inbound_events
    DROP CONSTRAINT IF EXISTS inbound_events_retention_class_check,
    DROP CONSTRAINT IF EXISTS inbound_events_retention_timestamps_check,
    ADD CONSTRAINT inbound_events_retention_class_check CHECK (
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
    ADD CONSTRAINT inbound_events_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'operational';

ALTER TABLE outbox_events
    DROP CONSTRAINT IF EXISTS outbox_events_retention_class_check,
    DROP CONSTRAINT IF EXISTS outbox_events_retention_timestamps_check,
    ADD CONSTRAINT outbox_events_retention_class_check CHECK (
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
    ADD CONSTRAINT outbox_events_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

ALTER TABLE moderation_results
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'operational';

ALTER TABLE moderation_results
    DROP CONSTRAINT IF EXISTS moderation_results_retention_class_check,
    DROP CONSTRAINT IF EXISTS moderation_results_retention_timestamps_check,
    ADD CONSTRAINT moderation_results_retention_class_check CHECK (
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
    ADD CONSTRAINT moderation_results_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

ALTER TABLE referral_events
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'analytics_aggregate';

ALTER TABLE referral_events
    DROP CONSTRAINT IF EXISTS referral_events_retention_class_check,
    DROP CONSTRAINT IF EXISTS referral_events_retention_timestamps_check,
    ADD CONSTRAINT referral_events_retention_class_check CHECK (
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
    ADD CONSTRAINT referral_events_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
    );

CREATE INDEX IF NOT EXISTS inbound_events_retention_cleanup_idx
    ON inbound_events (retention_class, expires_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS outbox_events_retention_cleanup_idx
    ON outbox_events (retention_class, expires_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS moderation_results_retention_cleanup_idx
    ON moderation_results (retention_class, expires_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS referral_events_retention_cleanup_idx
    ON referral_events (retention_class, expires_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL;

-- --------------------------------------------------------------------------
-- Artifact metadata and storage lifecycle.
-- --------------------------------------------------------------------------

ALTER TABLE artifacts
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'artifact_metadata',
    ADD COLUMN IF NOT EXISTS artifact_tier TEXT NOT NULL DEFAULT 'standard';

ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_retention_class_check,
    DROP CONSTRAINT IF EXISTS artifacts_artifact_tier_check,
    DROP CONSTRAINT IF EXISTS artifacts_retention_timestamps_check,
    ADD CONSTRAINT artifacts_retention_class_check CHECK (
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
    ADD CONSTRAINT artifacts_artifact_tier_check CHECK (
        artifact_tier IN ('standard', 'free', 'paid', 'temporary')
    ),
    ADD CONSTRAINT artifacts_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

ALTER TABLE artifact_variants
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'artifact_metadata',
    ADD COLUMN IF NOT EXISTS artifact_tier TEXT NOT NULL DEFAULT 'standard';

ALTER TABLE artifact_variants
    DROP CONSTRAINT IF EXISTS artifact_variants_retention_class_check,
    DROP CONSTRAINT IF EXISTS artifact_variants_artifact_tier_check,
    DROP CONSTRAINT IF EXISTS artifact_variants_retention_timestamps_check,
    ADD CONSTRAINT artifact_variants_retention_class_check CHECK (
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
    ADD CONSTRAINT artifact_variants_artifact_tier_check CHECK (
        artifact_tier IN ('standard', 'free', 'paid', 'temporary')
    ),
    ADD CONSTRAINT artifact_variants_retention_timestamps_check CHECK (
        (expires_at IS NULL OR expires_at >= created_at)
        AND (deleted_at IS NULL OR deleted_at >= created_at)
        AND (redacted_at IS NULL OR redacted_at >= created_at)
    );

CREATE INDEX IF NOT EXISTS artifacts_retention_cleanup_idx
    ON artifacts (retention_class, artifact_tier, expires_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL
      AND storage_bucket <> ''
      AND storage_key <> '';

CREATE INDEX IF NOT EXISTS artifact_variants_retention_cleanup_idx
    ON artifact_variants (retention_class, artifact_tier, expires_at, artifact_id, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL
      AND storage_bucket <> ''
      AND storage_key <> '';

COMMENT ON COLUMN artifacts.artifact_tier IS
    'Product retention tier for artifact lifecycle: standard, free, paid or temporary.';
COMMENT ON COLUMN artifacts.retention_class IS
    'Artifact metadata class. Cleanup must keep Postgres metadata and S3/MinIO objects consistent.';
COMMENT ON COLUMN artifact_variants.artifact_tier IS
    'Product retention tier inherited from the parent artifact when variants are created.';

COMMIT;
