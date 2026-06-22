\set ON_ERROR_STOP on
\pset pager off
\pset null '(null)'

\if :{?limit}
\else
  \set limit 20
\endif

\if :{?long_query_seconds}
\else
  \set long_query_seconds 30
\endif

\if :{?min_table_mb}
\else
  \set min_table_mb 1
\endif

\echo '== PostgreSQL load diagnostics =='
SELECT
  current_database() AS database,
  current_user AS user_name,
  inet_server_addr() AS server_addr,
  inet_server_port() AS server_port,
  now() AS captured_at,
  version() AS postgres_version;

\echo ''
\echo '== Migration readiness =='
SELECT CASE
  WHEN to_regclass('public.schema_migrations') IS NULL THEN 'false'
  ELSE 'true'
END AS has_schema_migrations
\gset

\if :has_schema_migrations
  SELECT
    count(*) AS applied_migrations,
    max(applied_at) AS last_applied_at,
    max(version) AS latest_version
  FROM schema_migrations;

  SELECT
    version,
    applied_at,
    left(checksum, 12) AS checksum_prefix
  FROM schema_migrations
  ORDER BY version DESC
  LIMIT :limit;
\else
  SELECT 'schema_migrations table is missing; migrations/readiness are not complete' AS note;
\endif

\echo ''
\echo '== Active and waiting queries =='
SELECT
  pid,
  usename,
  application_name,
  client_addr,
  state,
  wait_event_type,
  wait_event,
  now() - xact_start AS xact_age,
  now() - query_start AS query_age,
  left(regexp_replace(query, '\s+', ' ', 'g'), 240) AS query_sample
FROM pg_stat_activity
WHERE datname = current_database()
  AND pid <> pg_backend_pid()
  AND (state <> 'idle' OR wait_event IS NOT NULL)
ORDER BY query_start NULLS LAST
LIMIT :limit;

\echo ''
\echo '== Long-running queries =='
SELECT
  pid,
  usename,
  application_name,
  client_addr,
  state,
  wait_event_type,
  wait_event,
  now() - query_start AS query_age,
  left(regexp_replace(query, '\s+', ' ', 'g'), 240) AS query_sample
FROM pg_stat_activity
WHERE datname = current_database()
  AND pid <> pg_backend_pid()
  AND query_start IS NOT NULL
  AND now() - query_start >= make_interval(secs => :long_query_seconds::int)
ORDER BY query_start
LIMIT :limit;

\echo ''
\echo '== Blocking locks =='
SELECT
  waiting.pid AS waiting_pid,
  waiting.usename AS waiting_user,
  waiting.application_name AS waiting_app,
  waiting.wait_event_type,
  waiting.wait_event,
  now() - waiting.query_start AS waiting_age,
  blocker.blocking_pid,
  blocking.usename AS blocking_user,
  blocking.application_name AS blocking_app,
  blocking.state AS blocking_state,
  now() - blocking.query_start AS blocking_age,
  left(regexp_replace(waiting.query, '\s+', ' ', 'g'), 180) AS waiting_query,
  left(regexp_replace(blocking.query, '\s+', ' ', 'g'), 180) AS blocking_query
FROM pg_stat_activity AS waiting
CROSS JOIN LATERAL unnest(pg_blocking_pids(waiting.pid)) AS blocker(blocking_pid)
LEFT JOIN pg_stat_activity AS blocking ON blocking.pid = blocker.blocking_pid
WHERE waiting.datname = current_database()
ORDER BY waiting.query_start
LIMIT :limit;

\echo ''
\echo '== Largest and hottest tables =='
SELECT
  relname AS table_name,
  pg_size_pretty(pg_total_relation_size(relid)) AS total_size,
  pg_size_pretty(pg_relation_size(relid)) AS table_size,
  pg_size_pretty(pg_indexes_size(relid)) AS indexes_size,
  n_live_tup,
  n_dead_tup,
  seq_scan,
  idx_scan,
  last_vacuum,
  last_autovacuum,
  last_analyze,
  last_autoanalyze
FROM pg_stat_user_tables
WHERE pg_total_relation_size(relid) >= (:min_table_mb::bigint * 1024 * 1024)
ORDER BY pg_total_relation_size(relid) DESC, n_dead_tup DESC
LIMIT :limit;

\echo ''
\echo '== Sequential scan candidates =='
SELECT
  relname AS table_name,
  seq_scan,
  seq_tup_read,
  idx_scan,
  idx_tup_fetch,
  n_live_tup,
  n_dead_tup,
  pg_size_pretty(pg_total_relation_size(relid)) AS total_size
FROM pg_stat_user_tables
WHERE seq_scan > 0
ORDER BY seq_tup_read DESC, seq_scan DESC
LIMIT :limit;

\echo ''
\echo '== Index usage and size =='
SELECT
  schemaname,
  relname AS table_name,
  indexrelname AS index_name,
  idx_scan,
  idx_tup_read,
  idx_tup_fetch,
  pg_size_pretty(pg_relation_size(indexrelid)) AS index_size
FROM pg_stat_user_indexes
ORDER BY idx_scan ASC, pg_relation_size(indexrelid) DESC
LIMIT :limit;

\echo ''
\echo '== Large unused index candidates =='
SELECT
  schemaname,
  relname AS table_name,
  indexrelname AS index_name,
  idx_scan,
  pg_size_pretty(pg_relation_size(indexrelid)) AS index_size
FROM pg_stat_user_indexes
WHERE idx_scan = 0
  AND pg_relation_size(indexrelid) >= (:min_table_mb::bigint * 1024 * 1024)
ORDER BY pg_relation_size(indexrelid) DESC
LIMIT :limit;

\echo ''
\echo '== pg_stat_statements slow query summary =='
SELECT CASE
  WHEN EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements') THEN 'true'
  ELSE 'false'
END AS has_pg_stat_statements
\gset

\if :has_pg_stat_statements
  SELECT
    calls,
    round(total_exec_time::numeric, 2) AS total_exec_ms,
    round(mean_exec_time::numeric, 2) AS mean_exec_ms,
    round(max_exec_time::numeric, 2) AS max_exec_ms,
    rows,
    left(regexp_replace(query, '\s+', ' ', 'g'), 240) AS query_sample
  FROM pg_stat_statements
  WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
  ORDER BY total_exec_time DESC
  LIMIT :limit;
\else
  SELECT 'pg_stat_statements is not installed. Enable it on load/staging to rank slow SQL by total_exec_time/calls.' AS note;
\endif

\echo ''
\echo '== Retention cleanup candidates =='
SELECT CASE
  WHEN EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'conversation_messages'
      AND column_name = 'expires_at'
  ) THEN 'true'
  ELSE 'false'
END AS has_retention_schema
\gset

\if :has_retention_schema
  SELECT table_name, expired_rows, oldest_expired_at, newest_expired_at
  FROM (
    SELECT 'jobs' AS table_name, count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL) AS expired_rows, min(expires_at) AS oldest_expired_at, max(expires_at) AS newest_expired_at FROM jobs WHERE retention_class <> 'financial'
    UNION ALL
    SELECT 'provider_tasks', count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL), min(expires_at), max(expires_at) FROM provider_tasks
    UNION ALL
    SELECT 'conversation_messages', count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL), min(expires_at), max(expires_at) FROM conversation_messages
    UNION ALL
    SELECT 'conversation_summaries', count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL), min(expires_at), max(expires_at) FROM conversation_summaries
    UNION ALL
    SELECT 'artifacts', count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL), min(expires_at), max(expires_at) FROM artifacts
    UNION ALL
    SELECT 'artifact_variants', count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL), min(expires_at), max(expires_at) FROM artifact_variants
    UNION ALL
    SELECT 'inbound_events', count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL), min(expires_at), max(expires_at) FROM inbound_events
    UNION ALL
    SELECT 'outbox_events', count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL), min(expires_at), max(expires_at) FROM outbox_events
    UNION ALL
    SELECT 'moderation_results', count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL), min(expires_at), max(expires_at) FROM moderation_results
    UNION ALL
    SELECT 'referral_events', count(*) FILTER (WHERE expires_at <= now() AND deleted_at IS NULL), min(expires_at), max(expires_at) FROM referral_events
  ) AS retention
  ORDER BY expired_rows DESC, table_name;
\else
  SELECT 'retention columns are missing; run retention migrations before retention load diagnostics' AS note;
\endif

\echo ''
\echo '== Analytics aggregate freshness =='
SELECT CASE
  WHEN to_regclass('public.daily_user_activity') IS NULL THEN 'false'
  ELSE 'true'
END AS has_analytics_schema
\gset

\if :has_analytics_schema
  SELECT aggregate_name, rows_count, latest_day
  FROM (
    SELECT 'daily_user_activity' AS aggregate_name, count(*) AS rows_count, max(activity_date) AS latest_day FROM daily_user_activity
    UNION ALL
    SELECT 'daily_generation_stats', count(*), max(activity_date) FROM daily_generation_stats
    UNION ALL
    SELECT 'daily_provider_stats', count(*), max(activity_date) FROM daily_provider_stats
    UNION ALL
    SELECT 'daily_revenue_stats', count(*), max(activity_date) FROM daily_revenue_stats
    UNION ALL
    SELECT 'daily_referral_stats', count(*), max(activity_date) FROM daily_referral_stats
    UNION ALL
    SELECT 'daily_retention_stats', count(*), max(activity_date) FROM daily_retention_stats
    UNION ALL
    SELECT 'daily_funnel_stats', count(*), max(activity_date) FROM daily_funnel_stats
  ) AS aggregates
  ORDER BY aggregate_name;
\else
  SELECT 'analytics aggregate tables are missing; run analytics migrations before analytics load diagnostics' AS note;
\endif

\echo ''
\echo '== Notes =='
SELECT 'This script is read-only. Treat index recommendations as candidates; create production indexes manually, preferably CONCURRENTLY and after reviewing query plans.' AS note;
