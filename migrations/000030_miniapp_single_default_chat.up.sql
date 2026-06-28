-- 000030_miniapp_single_default_chat.up.sql
-- Merge VK Mini App custom chat threads into one backend-owned default thread.

BEGIN;

INSERT INTO conversations (
    user_id,
    vk_peer_id,
    source,
    external_thread_id,
    status,
    title,
    created_at,
    updated_at
)
SELECT DISTINCT ON (c.user_id)
    c.user_id,
    c.vk_peer_id,
    'miniapp',
    'default',
    'active',
    '',
    c.created_at,
    GREATEST(c.created_at, c.updated_at)
FROM conversations c
WHERE c.source = 'miniapp'
ORDER BY
    c.user_id,
    CASE WHEN c.external_thread_id = 'default' AND c.status = 'active' THEN 0 ELSE 1 END,
    c.created_at,
    c.id
ON CONFLICT DO NOTHING;

WITH thread_map AS (
    SELECT
        old.id AS old_conversation_id,
        def.id AS default_conversation_id
    FROM conversations old
    JOIN conversations def
      ON def.user_id = old.user_id
     AND def.source = 'miniapp'
     AND def.external_thread_id = 'default'
     AND def.status = 'active'
    WHERE old.source = 'miniapp'
      AND old.external_thread_id <> 'default'
)
UPDATE conversation_messages m
SET conversation_id = thread_map.default_conversation_id
FROM thread_map
WHERE m.conversation_id = thread_map.old_conversation_id;

WITH custom_thread_updates AS (
    SELECT
        old.user_id,
        max(old.updated_at) AS updated_at
    FROM conversations old
    WHERE old.source = 'miniapp'
      AND old.external_thread_id <> 'default'
    GROUP BY old.user_id
)
UPDATE conversations def
SET updated_at = GREATEST(def.updated_at, custom_thread_updates.updated_at)
FROM custom_thread_updates
WHERE def.user_id = custom_thread_updates.user_id
  AND def.source = 'miniapp'
  AND def.external_thread_id = 'default'
  AND def.status = 'active';

DELETE FROM conversation_summaries s
USING conversations c
WHERE s.conversation_id = c.id
  AND c.source = 'miniapp'
  AND c.external_thread_id <> 'default';

UPDATE jobs
SET params = jsonb_set(
        jsonb_set(
            COALESCE(params, '{}'::jsonb) - 'conversation_id',
            '{conversation_source}',
            to_jsonb('miniapp'::text),
            true
        ),
        '{external_thread_id}',
        to_jsonb('default'::text),
        true
    ),
    updated_at = now()
WHERE source = 'miniapp'
  AND operation_type = 'text_generate';

DELETE FROM conversations
WHERE source = 'miniapp'
  AND external_thread_id <> 'default';

COMMIT;
