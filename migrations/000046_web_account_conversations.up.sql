-- Enable account-native web conversation rows while preserving legacy
-- VK Bot and Mini App conversation creation.

ALTER TABLE conversations ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_source_check,
    ADD CONSTRAINT conversations_source_check
        CHECK (source IN ('vk_bot', 'miniapp', 'web'));

ALTER TABLE conversations
    ADD CONSTRAINT conversations_web_account_owner_check
        CHECK (source <> 'web' OR account_id IS NOT NULL);

CREATE UNIQUE INDEX IF NOT EXISTS conversations_active_account_web_thread_key
    ON conversations (account_id, source, external_thread_id)
    WHERE status = 'active' AND source = 'web' AND account_id IS NOT NULL;
