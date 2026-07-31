-- Roll back only when doing so does not discard channel/result contracts or
-- restore NOT NULL requirements that existing deliveries violate.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM jobs
        WHERE channel IS NOT NULL
           OR recipient_ref IS NOT NULL
           OR thread_ref IS NOT NULL
           OR result_mode <> 'legacy_unknown'
           OR target_channel IS NOT NULL
           OR target_recipient_ref IS NOT NULL
           OR target_thread_ref IS NOT NULL
    ) OR EXISTS (
        SELECT 1
        FROM deliveries
        WHERE account_id IS NOT NULL
           OR channel IS NOT NULL
           OR recipient_ref IS NOT NULL
           OR thread_ref IS NOT NULL
           OR user_id IS NULL
           OR vk_peer_id IS NULL
           OR vk_random_id IS NULL
    ) THEN
        RAISE EXCEPTION
            'cannot roll back 000044: rows use channel/result fields or require nullable delivery legacy IDs';
    END IF;
END $$;

ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_result_mode_known_check;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_channel_context_shape_check;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_delivery_target_shape_check;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_result_mode_shape_check;
ALTER TABLE deliveries DROP CONSTRAINT IF EXISTS deliveries_target_shape_check;
ALTER TABLE deliveries DROP CONSTRAINT IF EXISTS deliveries_account_id_fkey;

DROP INDEX IF EXISTS jobs_account_history_owner_created_idx;
DROP INDEX IF EXISTS jobs_external_push_target_created_idx;
DROP INDEX IF EXISTS deliveries_account_id_created_idx;
DROP INDEX IF EXISTS deliveries_channel_created_idx;

ALTER TABLE deliveries
    ALTER COLUMN user_id SET NOT NULL,
    ALTER COLUMN vk_peer_id SET NOT NULL,
    ALTER COLUMN vk_random_id SET NOT NULL;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS target_thread_ref,
    DROP COLUMN IF EXISTS target_recipient_ref,
    DROP COLUMN IF EXISTS target_channel,
    DROP COLUMN IF EXISTS result_mode,
    DROP COLUMN IF EXISTS thread_ref,
    DROP COLUMN IF EXISTS recipient_ref,
    DROP COLUMN IF EXISTS channel;

ALTER TABLE deliveries
    DROP COLUMN IF EXISTS thread_ref,
    DROP COLUMN IF EXISTS recipient_ref,
    DROP COLUMN IF EXISTS channel,
    DROP COLUMN IF EXISTS account_id;
