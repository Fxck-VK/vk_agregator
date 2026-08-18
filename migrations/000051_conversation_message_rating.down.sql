ALTER TABLE conversation_messages
    DROP CONSTRAINT IF EXISTS conversation_messages_rating_check;

ALTER TABLE conversation_messages
    DROP COLUMN IF EXISTS rating;
