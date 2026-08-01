-- migrate: no-transaction
-- Owner-scoped keyset history for the web image generator.
-- The predicate matches the browser history filter exactly, keeping the query
-- bounded as account-native image work grows.
-- A failed CREATE INDEX CONCURRENTLY can leave an invalid index behind. Drop
-- it first so a retry can reliably build a valid replacement before recording
-- this migration version.
DROP INDEX CONCURRENTLY IF EXISTS jobs_web_image_history_cursor_idx;
CREATE INDEX CONCURRENTLY jobs_web_image_history_cursor_idx
    ON jobs (account_id, created_at DESC, id DESC)
    WHERE account_id IS NOT NULL
      AND source = 'web'
      AND operation_type = 'image_generate'
      AND modality = 'image';
