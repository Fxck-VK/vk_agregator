-- 000008_conversation_sources.up.sql
-- Add source-specific durable conversation identity for VK bot and Mini App.

BEGIN;

ALTER TABLE conversations
    ADD COLUMN source TEXT NOT NULL DEFAULT 'vk_bot',
    ADD COLUMN external_thread_id TEXT NOT NULL DEFAULT '',
    ADD CONSTRAINT conversations_source_check CHECK (source IN ('vk_bot', 'miniapp'));

DROP INDEX IF EXISTS conversations_active_user_peer_key;

CREATE UNIQUE INDEX conversations_active_vk_bot_user_peer_key
    ON conversations (user_id, vk_peer_id)
    WHERE status = 'active' AND source = 'vk_bot';

CREATE UNIQUE INDEX conversations_active_source_thread_key
    ON conversations (user_id, source, external_thread_id)
    WHERE status = 'active' AND source <> 'vk_bot';

CREATE INDEX conversations_user_source_updated_idx
    ON conversations (user_id, source, updated_at DESC);

COMMIT;
