-- 000040_provider_task_payload_redaction.up.sql
-- Redact legacy provider_tasks payload material without changing schema shape.
-- This is additive/idempotent: it preserves task ids, provider ids, status,
-- external ids, timing and normalized error class for polling/support.

BEGIN;

ALTER TABLE provider_tasks
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS redacted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retention_class TEXT NOT NULL DEFAULT 'provider_payload';

WITH sanitized AS (
    SELECT id,
           CASE
               WHEN result IS NULL THEN NULL
               ELSE jsonb_strip_nulls(jsonb_build_object(
                   'status', NULLIF(COALESCE(NULLIF(result->>'status', ''), NULLIF(status, '')), ''),
                   'error_class', NULLIF(COALESCE(NULLIF(result->>'error_class', ''), NULLIF(error_class, '')), '')
               ))
           END AS safe_result
    FROM provider_tasks
    WHERE retention_class = 'provider_payload'
      AND deleted_at IS NULL
)
UPDATE provider_tasks AS task
SET request = '{}'::jsonb,
    result = sanitized.safe_result,
    redacted_at = COALESCE(redacted_at, now()),
    updated_at = now()
FROM sanitized
WHERE task.id = sanitized.id
  AND (
      task.request <> '{}'::jsonb
      OR task.result IS DISTINCT FROM sanitized.safe_result
      OR task.redacted_at IS NULL
  );

COMMENT ON COLUMN provider_tasks.result IS
    'Bounded provider task status metadata only. Output URLs, inline text, raw provider payloads and provider error bodies are not durable.';

COMMENT ON COLUMN provider_tasks.request IS
    'Empty compatibility snapshot only. Prompts, signed input URLs and provider parameters are not durable.';

COMMIT;
