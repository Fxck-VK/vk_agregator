-- Roll back only when doing so does not discard account-native web rows or
-- make existing rows violate the restored legacy user_id requirement.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM conversations
        WHERE user_id IS NULL OR source = 'web'
    ) THEN
        RAISE EXCEPTION
            'cannot roll back 000046: conversations contain account-native web or userless rows';
    END IF;
END $$;

ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_web_account_owner_check;

DROP INDEX IF EXISTS conversations_active_account_web_thread_key;

ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_source_check,
    ADD CONSTRAINT conversations_source_check
        CHECK (source IN ('vk_bot', 'miniapp'));

ALTER TABLE conversations ALTER COLUMN user_id SET NOT NULL;
