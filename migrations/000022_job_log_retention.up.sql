BEGIN;

CREATE TABLE IF NOT EXISTS job_error_aggregates (
    bucket_date DATE NOT NULL,
    surface TEXT NOT NULL DEFAULT 'unknown',
    operation_type TEXT NOT NULL DEFAULT 'unknown',
    modality TEXT NOT NULL DEFAULT 'unknown',
    provider TEXT NOT NULL DEFAULT 'unknown',
    model_code TEXT NOT NULL DEFAULT 'unknown',
    job_status TEXT NOT NULL DEFAULT 'unknown',
    error_class TEXT NOT NULL DEFAULT 'unknown',
    count BIGINT NOT NULL DEFAULT 0 CHECK (count >= 0),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    retention_class TEXT NOT NULL DEFAULT 'analytics_aggregate',
    PRIMARY KEY (
        bucket_date,
        surface,
        operation_type,
        modality,
        provider,
        model_code,
        job_status,
        error_class
    ),
    CHECK (retention_class = 'analytics_aggregate')
);

CREATE INDEX IF NOT EXISTS job_error_aggregates_last_seen_idx
    ON job_error_aggregates (last_seen_at DESC);

COMMENT ON TABLE job_error_aggregates IS
    'Bounded-label aggregate error counters built before raw job/provider diagnostics are redacted.';
COMMENT ON COLUMN job_error_aggregates.error_class IS
    'Normalized error class/code only. Must not contain raw provider payloads, prompts, secrets, auth headers or private artifact URLs.';

COMMIT;
