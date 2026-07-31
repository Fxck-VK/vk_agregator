-- 000045_outbox_claim_lease.up.sql
-- Add durable ownership and terminal failure metadata to outbox publication.

ALTER TABLE outbox_events
    ADD COLUMN claim_token UUID,
    ADD COLUMN claim_owner TEXT,
    ADD COLUMN lease_until TIMESTAMPTZ,
    ADD COLUMN last_error_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN failed_at TIMESTAMPTZ;

ALTER TABLE outbox_events
    ADD CONSTRAINT outbox_events_status_check
    CHECK (status IN ('pending', 'processing', 'published', 'failed'));
