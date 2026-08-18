-- Store optional account feedback on assistant messages. No index is needed:
-- every write is already scoped by the message primary key and conversation.

ALTER TABLE conversation_messages
    ADD COLUMN rating TEXT;

ALTER TABLE conversation_messages
    ADD CONSTRAINT conversation_messages_rating_check
        CHECK (rating IS NULL OR rating IN ('like', 'dislike'));
