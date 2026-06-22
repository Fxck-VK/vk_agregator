-- 000023_daily_analytics_aggregates.up.sql
-- Daily no-PII analytics snapshots for dashboards.
--
-- Dashboards must read these aggregate tables instead of scanning hot
-- jobs/messages/payment/referral tables on every request. Raw prompts,
-- provider payloads, VK launch params and PII must never be copied here.

BEGIN;

CREATE TABLE daily_user_activity (
    activity_date    DATE        NOT NULL,
    surface          TEXT        NOT NULL DEFAULT 'all',
    active_users     BIGINT      NOT NULL DEFAULT 0,
    new_users        BIGINT      NOT NULL DEFAULT 0,
    returning_users  BIGINT      NOT NULL DEFAULT 0,
    message_users    BIGINT      NOT NULL DEFAULT 0,
    generation_users BIGINT      NOT NULL DEFAULT 0,
    retention_class  TEXT        NOT NULL DEFAULT 'analytics_aggregate',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (activity_date, surface),
    CONSTRAINT daily_user_activity_surface_check CHECK (surface <> ''),
    CONSTRAINT daily_user_activity_counts_check CHECK (
        active_users >= 0
        AND new_users >= 0
        AND returning_users >= 0
        AND message_users >= 0
        AND generation_users >= 0
    ),
    CONSTRAINT daily_user_activity_retention_class_check
        CHECK (retention_class = 'analytics_aggregate')
);

CREATE TABLE daily_generation_stats (
    activity_date      DATE        NOT NULL,
    surface            TEXT        NOT NULL DEFAULT 'all',
    operation_type     TEXT        NOT NULL DEFAULT 'unknown',
    modality           TEXT        NOT NULL DEFAULT 'unknown',
    jobs_created       BIGINT      NOT NULL DEFAULT 0,
    jobs_succeeded     BIGINT      NOT NULL DEFAULT 0,
    jobs_failed        BIGINT      NOT NULL DEFAULT 0,
    credits_reserved   BIGINT      NOT NULL DEFAULT 0,
    credits_captured   BIGINT      NOT NULL DEFAULT 0,
    artifacts_created  BIGINT      NOT NULL DEFAULT 0,
    retention_class    TEXT        NOT NULL DEFAULT 'analytics_aggregate',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (activity_date, surface, operation_type, modality),
    CONSTRAINT daily_generation_stats_dims_check CHECK (
        surface <> '' AND operation_type <> '' AND modality <> ''
    ),
    CONSTRAINT daily_generation_stats_counts_check CHECK (
        jobs_created >= 0
        AND jobs_succeeded >= 0
        AND jobs_failed >= 0
        AND credits_reserved >= 0
        AND credits_captured >= 0
        AND artifacts_created >= 0
    ),
    CONSTRAINT daily_generation_stats_retention_class_check
        CHECK (retention_class = 'analytics_aggregate')
);

CREATE TABLE daily_provider_stats (
    activity_date     DATE        NOT NULL,
    provider          TEXT        NOT NULL DEFAULT 'unknown',
    model_code        TEXT        NOT NULL DEFAULT 'unknown',
    operation_type    TEXT        NOT NULL DEFAULT 'unknown',
    modality          TEXT        NOT NULL DEFAULT 'unknown',
    tasks_created     BIGINT      NOT NULL DEFAULT 0,
    tasks_succeeded   BIGINT      NOT NULL DEFAULT 0,
    tasks_failed      BIGINT      NOT NULL DEFAULT 0,
    avg_latency_ms    BIGINT      NOT NULL DEFAULT 0,
    total_cost_units  BIGINT      NOT NULL DEFAULT 0,
    retention_class   TEXT        NOT NULL DEFAULT 'analytics_aggregate',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (activity_date, provider, model_code, operation_type, modality),
    CONSTRAINT daily_provider_stats_dims_check CHECK (
        provider <> '' AND model_code <> '' AND operation_type <> '' AND modality <> ''
    ),
    CONSTRAINT daily_provider_stats_counts_check CHECK (
        tasks_created >= 0
        AND tasks_succeeded >= 0
        AND tasks_failed >= 0
        AND avg_latency_ms >= 0
        AND total_cost_units >= 0
    ),
    CONSTRAINT daily_provider_stats_retention_class_check
        CHECK (retention_class = 'analytics_aggregate')
);

CREATE TABLE daily_revenue_stats (
    activity_date            DATE        NOT NULL,
    provider                 TEXT        NOT NULL DEFAULT 'all',
    currency                 TEXT        NOT NULL DEFAULT 'rub',
    payment_intents_created  BIGINT      NOT NULL DEFAULT 0,
    payments_succeeded       BIGINT      NOT NULL DEFAULT 0,
    payments_canceled        BIGINT      NOT NULL DEFAULT 0,
    refunds_succeeded        BIGINT      NOT NULL DEFAULT 0,
    gross_amount_minor       BIGINT      NOT NULL DEFAULT 0,
    refunded_amount_minor    BIGINT      NOT NULL DEFAULT 0,
    net_amount_minor         BIGINT      NOT NULL DEFAULT 0,
    credits_sold             BIGINT      NOT NULL DEFAULT 0,
    retention_class          TEXT        NOT NULL DEFAULT 'analytics_aggregate',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (activity_date, provider, currency),
    CONSTRAINT daily_revenue_stats_dims_check CHECK (provider <> '' AND currency <> ''),
    CONSTRAINT daily_revenue_stats_counts_check CHECK (
        payment_intents_created >= 0
        AND payments_succeeded >= 0
        AND payments_canceled >= 0
        AND refunds_succeeded >= 0
        AND gross_amount_minor >= 0
        AND refunded_amount_minor >= 0
        AND credits_sold >= 0
    ),
    CONSTRAINT daily_revenue_stats_retention_class_check
        CHECK (retention_class = 'analytics_aggregate')
);

CREATE TABLE daily_referral_stats (
    activity_date           DATE        NOT NULL,
    source                  TEXT        NOT NULL DEFAULT 'all',
    link_opened_count       BIGINT      NOT NULL DEFAULT 0,
    registered_count        BIGINT      NOT NULL DEFAULT 0,
    activated_count         BIGINT      NOT NULL DEFAULT 0,
    rewarded_count          BIGINT      NOT NULL DEFAULT 0,
    first_generation_count  BIGINT      NOT NULL DEFAULT 0,
    first_payment_count     BIGINT      NOT NULL DEFAULT 0,
    retention_class         TEXT        NOT NULL DEFAULT 'analytics_aggregate',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (activity_date, source),
    CONSTRAINT daily_referral_stats_source_check CHECK (source <> ''),
    CONSTRAINT daily_referral_stats_counts_check CHECK (
        link_opened_count >= 0
        AND registered_count >= 0
        AND activated_count >= 0
        AND rewarded_count >= 0
        AND first_generation_count >= 0
        AND first_payment_count >= 0
    ),
    CONSTRAINT daily_referral_stats_retention_class_check
        CHECK (retention_class = 'analytics_aggregate')
);

CREATE TABLE daily_retention_stats (
    activity_date    DATE        NOT NULL,
    cohort_date      DATE        NOT NULL,
    surface          TEXT        NOT NULL DEFAULT 'all',
    day_number       INTEGER     NOT NULL,
    cohort_users     BIGINT      NOT NULL DEFAULT 0,
    retained_users   BIGINT      NOT NULL DEFAULT 0,
    retention_class  TEXT        NOT NULL DEFAULT 'analytics_aggregate',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (activity_date, cohort_date, surface),
    CONSTRAINT daily_retention_stats_dims_check CHECK (
        surface <> '' AND activity_date >= cohort_date AND day_number >= 0
    ),
    CONSTRAINT daily_retention_stats_counts_check CHECK (
        cohort_users >= 0
        AND retained_users >= 0
        AND retained_users <= cohort_users
    ),
    CONSTRAINT daily_retention_stats_retention_class_check
        CHECK (retention_class = 'analytics_aggregate')
);

CREATE TABLE daily_funnel_stats (
    activity_date    DATE        NOT NULL,
    surface          TEXT        NOT NULL DEFAULT 'all',
    funnel_step      TEXT        NOT NULL,
    users_count      BIGINT      NOT NULL DEFAULT 0,
    events_count     BIGINT      NOT NULL DEFAULT 0,
    retention_class  TEXT        NOT NULL DEFAULT 'analytics_aggregate',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (activity_date, surface, funnel_step),
    CONSTRAINT daily_funnel_stats_dims_check CHECK (surface <> '' AND funnel_step <> ''),
    CONSTRAINT daily_funnel_stats_counts_check CHECK (
        users_count >= 0
        AND events_count >= 0
    ),
    CONSTRAINT daily_funnel_stats_retention_class_check
        CHECK (retention_class = 'analytics_aggregate')
);

CREATE INDEX daily_generation_stats_date_idx ON daily_generation_stats (activity_date DESC);
CREATE INDEX daily_provider_stats_date_idx ON daily_provider_stats (activity_date DESC);
CREATE INDEX daily_revenue_stats_date_idx ON daily_revenue_stats (activity_date DESC);
CREATE INDEX daily_referral_stats_date_idx ON daily_referral_stats (activity_date DESC);
CREATE INDEX daily_retention_stats_cohort_idx ON daily_retention_stats (cohort_date, activity_date);
CREATE INDEX daily_funnel_stats_date_idx ON daily_funnel_stats (activity_date DESC);

COMMENT ON TABLE daily_user_activity IS
    'Daily no-PII active user aggregate. Dashboards should use this instead of raw messages/jobs.';
COMMENT ON TABLE daily_generation_stats IS
    'Daily no-PII generation aggregate by surface, operation type and modality.';
COMMENT ON TABLE daily_provider_stats IS
    'Daily no-PII provider aggregate with bounded provider/model dimensions.';
COMMENT ON TABLE daily_revenue_stats IS
    'Daily financial aggregate derived from payment intents/refunds; source ledger remains authoritative.';
COMMENT ON TABLE daily_referral_stats IS
    'Daily referral funnel aggregate without invited-user PII.';
COMMENT ON TABLE daily_retention_stats IS
    'Daily cohort retention aggregate; no raw user identifiers are stored.';
COMMENT ON TABLE daily_funnel_stats IS
    'Daily product funnel aggregate for dashboards without scanning hot event tables.';

COMMIT;
