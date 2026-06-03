-- 000003_moderation_results.up.sql
-- Audit trail of moderation verdicts. Output verdicts gate delivery
-- (invariant #15: no user output before moderation passes).
CREATE TABLE IF NOT EXISTS moderation_results (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id      UUID        NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    artifact_id UUID        REFERENCES artifacts (id) ON DELETE SET NULL,
    stage       TEXT        NOT NULL,
    decision    TEXT        NOT NULL,
    categories  TEXT[]      NOT NULL DEFAULT '{}',
    provider    TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_moderation_results_job ON moderation_results (job_id);
