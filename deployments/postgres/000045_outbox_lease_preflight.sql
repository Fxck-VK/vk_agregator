-- Read-only operator preflight for 000045_outbox_claim_lease.
-- Every result set marked "required: no rows" is a hard rollout stop.
-- The lease and finalization intervals below are operational thresholds. Record
-- environment approval for them before running this file; do not repair rows
-- from this script.

-- Migration 000042 checksum inventory. An absent row is allowed only when the
-- environment has not applied 000042. Any recorded checksum that the current
-- migration runner rejects is a hard stop.
SELECT version, checksum, applied_at
FROM schema_migrations
WHERE version = '000042_account_session_access_tokens';

-- Semantic result-ready duplicates (required: no rows).
SELECT aggregate_id, COUNT(*) AS event_count
FROM outbox_events
WHERE aggregate_type = 'job'
  AND event_type = 'event.job.result_ready'
GROUP BY aggregate_id
HAVING COUNT(*) > 1;

-- Quarantined semantic result-ready events (required: no rows).
-- A failed row still owns the semantic identity and must not be replaced.
SELECT id, aggregate_id, attempts, last_error_code, failed_at
FROM outbox_events
WHERE aggregate_type = 'job'
  AND event_type = 'event.job.result_ready'
  AND status = 'failed'
ORDER BY failed_at ASC NULLS FIRST, id ASC;

-- Known events that the relay cannot safely classify/execute (required: no rows).
SELECT id, aggregate_type, aggregate_id, event_type, status
FROM outbox_events
WHERE event_type IN (
        'event.job.created',
        'event.job.queued',
        'event.job.result_ready'
    )
  AND (
      aggregate_type IS DISTINCT FROM 'job'
      OR jsonb_typeof(payload) IS DISTINCT FROM 'object'
      OR COALESCE(payload ->> 'job_id', '') !~*
         '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
      OR lower(payload ->> 'job_id') IS DISTINCT FROM aggregate_id::text
      OR COALESCE(payload ->> 'operation', '') NOT IN (
          'text_generate',
          'image_generate',
          'image_edit',
          'video_generate',
          'video_image_to_video',
          'video_extend',
          'audio_tts',
          'audio_stt',
          'image_upscale'
      )
      OR COALESCE(payload ->> 'modality', '') NOT IN (
          'text',
          'image',
          'video',
          'audio'
      )
  )
ORDER BY created_at ASC, id ASC;

-- Event types outside the relay's closed allowlist (required: no rows).
SELECT event_type, status, COUNT(*) AS event_count
FROM outbox_events
WHERE event_type NOT IN (
    'event.job.created',
    'event.job.queued',
    'event.job.result_ready'
)
GROUP BY event_type, status
ORDER BY event_type, status;

-- Lease metadata must form a complete processing claim and be absent in every
-- other status (required: no rows). Claims set all three fields atomically;
-- retry, publish, and quarantine clear all three fields atomically.
SELECT id, event_type, status, claim_token, claim_owner, lease_until, attempts
FROM outbox_events
WHERE (
        status = 'processing'
        AND (
            claim_token IS NULL
            OR NULLIF(BTRIM(claim_owner), '') IS NULL
            OR lease_until IS NULL
        )
    )
   OR (
        status IS DISTINCT FROM 'processing'
        AND (
            claim_token IS NOT NULL
            OR claim_owner IS NOT NULL
            OR lease_until IS NOT NULL
        )
    )
ORDER BY created_at ASC, id ASC;

-- Processing leases expired beyond the approved five-minute grace
-- (required: no rows).
SELECT id, event_type, claim_owner, lease_until, attempts
FROM outbox_events
WHERE status = 'processing'
  AND lease_until < now() - INTERVAL '5 minutes'
ORDER BY lease_until ASC, id ASC;

-- Canonical ready jobs missing their semantic event (required: no rows after
-- bounded reconciliation reaches has_more=false).
SELECT j.id, j.result_mode, j.updated_at
FROM jobs AS j
WHERE j.status = 'result_ready'
  AND j.account_id IS NOT NULL
  AND j.result_mode IN ('account_history', 'external_push')
  AND NOT EXISTS (
      SELECT 1
      FROM outbox_events AS event
      WHERE event.aggregate_type = 'job'
        AND event.aggregate_id = j.id
        AND event.event_type = 'event.job.result_ready'
  )
ORDER BY j.created_at DESC, j.id DESC;

-- Finalization rows stale beyond the approved fifteen-minute threshold
-- (required: no rows).
SELECT id, result_mode, status, updated_at
FROM jobs
WHERE status IN ('result_ready', 'delivering')
  AND updated_at < now() - INTERVAL '15 minutes'
ORDER BY updated_at ASC, id ASC;
