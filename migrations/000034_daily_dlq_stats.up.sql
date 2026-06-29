BEGIN;

CREATE TABLE IF NOT EXISTS daily_dlq_stats (
    activity_date     DATE        NOT NULL,
    surface           TEXT        NOT NULL DEFAULT 'unknown',
    operation_type    TEXT        NOT NULL DEFAULT 'unknown',
    modality          TEXT        NOT NULL DEFAULT 'unknown',
    job_status        TEXT        NOT NULL DEFAULT 'unknown',
    error_class       TEXT        NOT NULL DEFAULT 'unknown',
    retryable_count   BIGINT      NOT NULL DEFAULT 0,
    terminal_count    BIGINT      NOT NULL DEFAULT 0,
    latest_failure_at TIMESTAMPTZ,
    retention_class   TEXT        NOT NULL DEFAULT 'analytics_aggregate',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (activity_date, surface, operation_type, modality, job_status, error_class),
    CONSTRAINT daily_dlq_stats_dims_check CHECK (
        surface <> ''
        AND operation_type <> ''
        AND modality <> ''
        AND job_status <> ''
        AND error_class <> ''
    ),
    CONSTRAINT daily_dlq_stats_counts_check CHECK (
        retryable_count >= 0
        AND terminal_count >= 0
    ),
    CONSTRAINT daily_dlq_stats_retention_class_check
        CHECK (retention_class = 'analytics_aggregate')
);

CREATE INDEX IF NOT EXISTS daily_dlq_stats_date_idx
    ON daily_dlq_stats (activity_date DESC);

CREATE INDEX IF NOT EXISTS daily_dlq_stats_latest_failure_idx
    ON daily_dlq_stats (latest_failure_at DESC)
    WHERE latest_failure_at IS NOT NULL;

COMMENT ON TABLE daily_dlq_stats IS
    'Daily no-PII DLQ aggregate for operator dashboards. Dashboards should use this instead of scanning raw failed jobs.';
COMMENT ON COLUMN daily_dlq_stats.error_class IS
    'Normalized error class only. Must not contain raw prompts, provider payloads, secrets, auth headers or private URLs.';

COMMIT;
