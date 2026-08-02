ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_title_origin_check;

ALTER TABLE conversations
    DROP COLUMN IF EXISTS title_origin;
