-- Persist bounded channel provenance separately from result publication.
-- This migration is intentionally additive and is executed inside the
-- migration runner's outer transaction: do not add BEGIN/COMMIT here.

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS channel TEXT,
    ADD COLUMN IF NOT EXISTS recipient_ref TEXT,
    ADD COLUMN IF NOT EXISTS thread_ref TEXT,
    ADD COLUMN IF NOT EXISTS result_mode TEXT NOT NULL DEFAULT 'legacy_unknown',
    ADD COLUMN IF NOT EXISTS target_channel TEXT,
    ADD COLUMN IF NOT EXISTS target_recipient_ref TEXT,
    ADD COLUMN IF NOT EXISTS target_thread_ref TEXT;

ALTER TABLE deliveries
    ADD COLUMN IF NOT EXISTS account_id UUID,
    ADD COLUMN IF NOT EXISTS channel TEXT,
    ADD COLUMN IF NOT EXISTS recipient_ref TEXT,
    ADD COLUMN IF NOT EXISTS thread_ref TEXT;

ALTER TABLE deliveries
    ALTER COLUMN user_id DROP NOT NULL,
    ALTER COLUMN vk_peer_id DROP NOT NULL,
    ALTER COLUMN vk_random_id DROP NOT NULL;

ALTER TABLE deliveries
    ADD CONSTRAINT deliveries_account_id_fkey
    FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE SET NULL NOT VALID;

-- Backfill only legacy VK rows with a real, publishable peer into an explicit
-- push contract. A legacy VK row without a peer must remain legacy_unknown
-- without a target so an old CreateJob deployment stays writable during the
-- rolling migration.
UPDATE jobs
SET result_mode = 'external_push',
    target_channel = 'vk_bot',
    target_recipient_ref = vk_peer_id::text
WHERE result_mode = 'legacy_unknown'
  AND source LIKE 'vk%'
  AND vk_peer_id IS NOT NULL
  AND vk_peer_id <> 0
  AND target_channel IS NULL
  AND target_recipient_ref IS NULL
  AND target_thread_ref IS NULL;

-- Mini App and web are pull/account-history surfaces. Existing ownerless
-- rows stay legacy_unknown rather than being assigned an invalid contract.
UPDATE jobs
SET result_mode = 'account_history'
WHERE result_mode = 'legacy_unknown'
  AND source IN ('miniapp', 'web')
  AND account_id IS NOT NULL;

UPDATE jobs
SET channel = CASE
        WHEN source LIKE 'vk%' THEN 'vk_bot'
        WHEN source = 'miniapp' THEN 'vk_miniapp'
        WHEN source = 'web' THEN 'web'
        ELSE channel
    END
WHERE channel IS NULL
  AND (source LIKE 'vk%' OR source = 'miniapp' OR source = 'web');

UPDATE deliveries d
SET account_id = j.account_id
FROM jobs j
WHERE d.job_id = j.id
  AND d.account_id IS NULL
  AND j.account_id IS NOT NULL;

UPDATE deliveries
SET channel = 'vk_bot',
    recipient_ref = vk_peer_id::text
WHERE channel IS NULL
  AND recipient_ref IS NULL
  AND thread_ref IS NULL
  AND vk_peer_id IS NOT NULL
  AND vk_peer_id <> 0;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_result_mode_known_check
    CHECK (result_mode IN ('external_push', 'account_history', 'legacy_unknown')) NOT VALID;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_channel_context_shape_check
    CHECK (
        (channel IS NULL AND recipient_ref IS NULL AND thread_ref IS NULL)
        OR
        (
            channel IN ('vk_bot', 'vk_miniapp', 'web')
            AND (recipient_ref IS NULL OR char_length(recipient_ref) BETWEEN 1 AND 512)
            AND (thread_ref IS NULL OR char_length(thread_ref) BETWEEN 1 AND 512)
        )
    ) NOT VALID;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_delivery_target_shape_check
    CHECK (
        (target_channel IS NULL AND target_recipient_ref IS NULL AND target_thread_ref IS NULL)
        OR
        (
            target_channel = 'vk_bot'
            AND target_recipient_ref IS NOT NULL
            AND char_length(target_recipient_ref) BETWEEN 1 AND 512
            AND (target_thread_ref IS NULL OR char_length(target_thread_ref) BETWEEN 1 AND 512)
        )
    ) NOT VALID;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_result_mode_shape_check
    CHECK (
        (result_mode <> 'external_push' OR target_channel IS NOT NULL)
        AND
        (
            result_mode <> 'account_history'
            OR (
                account_id IS NOT NULL
                AND target_channel IS NULL
                AND target_recipient_ref IS NULL
                AND target_thread_ref IS NULL
            )
        )
        AND
        (
            result_mode <> 'legacy_unknown'
            OR (
                target_channel IS NULL
                AND target_recipient_ref IS NULL
                AND target_thread_ref IS NULL
            )
        )
    ) NOT VALID;

ALTER TABLE deliveries
    ADD CONSTRAINT deliveries_target_shape_check
    CHECK (
        (channel IS NULL AND recipient_ref IS NULL AND thread_ref IS NULL)
        OR
        (
            channel = 'vk_bot'
            AND recipient_ref IS NOT NULL
            AND char_length(recipient_ref) BETWEEN 1 AND 512
            AND (thread_ref IS NULL OR char_length(thread_ref) BETWEEN 1 AND 512)
        )
    ) NOT VALID;

CREATE INDEX IF NOT EXISTS jobs_account_history_owner_created_idx
    ON jobs (account_id, created_at DESC, id DESC)
    WHERE result_mode = 'account_history' AND account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS jobs_external_push_target_created_idx
    ON jobs (target_channel, created_at DESC, id DESC)
    WHERE result_mode = 'external_push' AND target_channel IS NOT NULL;

CREATE INDEX IF NOT EXISTS deliveries_account_id_created_idx
    ON deliveries (account_id, created_at DESC, id DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS deliveries_channel_created_idx
    ON deliveries (channel, created_at DESC, id DESC)
    WHERE channel IS NOT NULL;
