-- 000008_conversation_sources.down.sql

BEGIN;

DROP INDEX IF EXISTS conversations_user_source_updated_idx;
DROP INDEX IF EXISTS conversations_active_source_thread_key;
DROP INDEX IF EXISTS conversations_active_vk_bot_user_peer_key;

ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_source_check,
    DROP COLUMN IF EXISTS external_thread_id,
    DROP COLUMN IF EXISTS source;

CREATE UNIQUE INDEX conversations_active_user_peer_key
    ON conversations (user_id, vk_peer_id)
    WHERE status = 'active';

COMMIT;
