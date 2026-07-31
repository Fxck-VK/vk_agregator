-- 000045_outbox_claim_lease.down.sql
-- Roll back only before claim/lease/failure state has been persisted.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM outbox_events
        WHERE claim_token IS NOT NULL
           OR claim_owner IS NOT NULL
           OR lease_until IS NOT NULL
           OR last_error_code <> ''
           OR failed_at IS NOT NULL
           OR status = 'failed'
    ) THEN
        RAISE EXCEPTION
            'cannot roll back 000045: outbox claim, lease, or terminal failure state exists';
    END IF;
END $$;

ALTER TABLE outbox_events
    DROP CONSTRAINT IF EXISTS outbox_events_status_check;

ALTER TABLE outbox_events
    DROP COLUMN IF EXISTS failed_at,
    DROP COLUMN IF EXISTS last_error_code,
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS claim_owner,
    DROP COLUMN IF EXISTS claim_token;
