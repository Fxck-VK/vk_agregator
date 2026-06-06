-- 000006_conversation_context.down.sql

BEGIN;

DROP TABLE IF EXISTS conversation_summaries;
DROP TABLE IF EXISTS conversation_messages;
DROP TABLE IF EXISTS conversations;

COMMIT;
