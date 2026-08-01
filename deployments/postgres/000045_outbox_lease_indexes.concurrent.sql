-- NON-TRANSACTIONAL OPERATOR SCRIPT: do not run inside cmd/migrate or any
-- transaction block. PostgreSQL CREATE INDEX CONCURRENTLY requires autocommit.
-- Run only after 000045_outbox_claim_lease is applied and the read-only
-- 000045_outbox_lease_preflight.sql output has been reviewed. The final unique
-- index has the stricter zero-duplicate gate stated immediately above it.

CREATE INDEX CONCURRENTLY IF NOT EXISTS outbox_events_pending_schedule_idx
    ON outbox_events (next_attempt_at, id)
    WHERE status = 'pending';

CREATE INDEX CONCURRENTLY IF NOT EXISTS outbox_events_processing_lease_idx
    ON outbox_events (lease_until, id)
    WHERE status = 'processing';

CREATE INDEX CONCURRENTLY IF NOT EXISTS jobs_result_ready_reconciliation_idx
    ON jobs (created_at DESC, id DESC)
    WHERE status = 'result_ready'
      AND account_id IS NOT NULL
      AND result_mode IN ('account_history', 'external_push');

CREATE INDEX CONCURRENTLY IF NOT EXISTS jobs_stale_finalization_idx
    ON jobs (updated_at, id)
    WHERE status IN ('result_ready', 'delivering');

-- HARD STOP: execute this final statement only after the semantic-duplicate
-- query in 000045_outbox_lease_preflight.sql returns no rows. Do not delete,
-- merge, or rewrite duplicate rows to make the index build pass.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS outbox_events_job_result_ready_unique_idx
    ON outbox_events (aggregate_id)
    WHERE aggregate_type = 'job'
      AND event_type = 'event.job.result_ready';
