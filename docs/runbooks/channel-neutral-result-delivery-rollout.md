# Channel-neutral result-delivery rollout

**Scope:** migrations `000043_account_native_legacy_nullable`,
`000044_channel_context_and_result_mode`, and
`000045_outbox_claim_lease`. This runbook is for operators rolling out
channel-neutral result delivery and the leased outbox relay. It does not
authorize data repair.

## Deployment gates

Proceed in this order, stopping at any failed gate:

1. Keep web prepared-job activation disabled.
2. Record the target environment's approved lease-expiry, stale-finalization,
   backlog-age, retry, quarantine, and latency thresholds. Take and verify a
   backup of PostgreSQL and required artifact storage.
3. Run the `000042` checksum inventory below before applying this branch. A
   checksum rejected by the current migration runner is a hard stop; never
   overwrite the recorded checksum or enable a generic bypass.
4. Apply the additive migrations `000043`, `000044`, and `000045` through the
   normal migration runner. Deploy API compatibility code, but do not start the
   new job worker/relay yet.
5. Run the read-only preflight at
   `deployments/postgres/000045_outbox_lease_preflight.sql`. The missing-event
   and stale-finalization queries may identify the bounded reconciliation work
   that remains, but duplicates, failed semantic events, invalid executable
   events, unknown event types, and over-threshold expired leases stop the
   rollout immediately.
6. After the semantic-duplicate query returns no rows, run
   `deployments/postgres/000045_outbox_lease_indexes.concurrent.sql` with
   autocommit enabled, outside `cmd/migrate` and outside every transaction.
   Observe each build and require every resulting index to be valid before
   continuing.
7. Stop every old `cmd/worker` job-mode process. Use a deployment strategy that
   cannot start the new job worker while any old pod or process is alive.
8. Wait for each old process to log `workers stopped`, or complete the
   incident-approved termination procedure. Verify that the old
   deployment/pod/process count is exactly zero. The producer and relay are
   co-located, so zero means both are absent.
9. While the job-worker and relay count remains zero, retain the result of one
   bounded reconciliation pass at a time in the private rollout-evidence
   directory. In a shell that propagates pipeline failures:

   ```bash
   : "${ROLLOUT_EVIDENCE_DIR:?set this to a private rollout-evidence path}"
   mkdir -p "$ROLLOUT_EVIDENCE_DIR"
   set -o pipefail
   page_at=$(date -u +%Y%m%dT%H%M%SZ)
   page_file=$(mktemp "$ROLLOUT_EVIDENCE_DIR/reconciliation-${page_at}.XXXXXXXX")
   reconcile-result-ready --limit 1000 \
     | tee "$page_file"
   ```

   Each retained JSON document is the durable, privacy-safe observation for one
   page: `duration_seconds`, `candidates`, `eligible`, `existing`, `created`,
   `blocked`, and `has_more`. `duration_seconds` covers the one bounded
   reconciliation call, not database-pool startup. The output contains no job,
   account, correlation, or payload fields. Do not rely on the short-lived
   command's Prometheus metrics after it exits; retain this JSON evidence with
   the rollout ticket. A nonzero exit status or nonzero `blocked` count aborts
   the rollout. Repeat bounded passes until `has_more` is `false`; never infer
   completion from page size or a lifetime total.
10. Rerun the complete preflight. Every hard acceptance result below must be
    zero/no rows. Do not run ad-hoc `UPDATE` or `DELETE` repair.
11. Start exactly one new relay canary with `WORKER_MODE=relay`. This mode runs
    only the leased outbox relay; it does not start generation, provider-poll,
    delivery, or maintenance engines. Prove that pending work drains, expired
    leases recover, claims are not double-owned, and failed/quarantine counts
    remain within the approved gate. Then scale relay-only processes only to
    the environment-approved relay count and repeat the same checks.
12. Start only the new job-worker artifact with `WORKER_MODE=jobs`. Existing
    `all` and `jobs` modes retain their embedded relay for compatibility, so
    include those processes in the total relay count before adding any more
    relay-only processes. The new workers may now relay reconstructed pending
    events and produce new result-ready events atomically.
13. Wait for the finalization backlog and pending result-ready events to drain,
    then rerun the complete preflight.
14. Run the VK Bot external-push and Mini App account-history canaries. Run a
    web account-history read canary without activating web prepared jobs.
15. Run the reviewed load envelope in `APP_ENV=loadtest` or staging and retain
    its report. Missing capacity inputs or a failed backlog, lease, latency,
    retry, quarantine, Redis, or PostgreSQL gate blocks activation.
16. Enable web prepared-job activation only after every prior gate is green and
    a separate operator change explicitly authorizes it.

A rolling overlap is unsafe: an old open-classifier relay can acknowledge a
new executable event without scheduling it. The worker outage in steps 7-12 is
intentional and required. Queued work and outbox rows are durable, so preserve
them and resume with the reviewed new artifact; do not claim a zero-downtime
worker replacement is safe.

## Read-only operator checks

Run these queries only with environment-specific database connection
credentials supplied by the operator. They are read-only; do not put connection
strings or credentials in this runbook. Record the output in the rollout
ticket, without customer content or other sensitive values.

Inventory the recorded `000042` checksum before applying this branch:

```sql
SELECT version, checksum, applied_at
FROM schema_migrations
WHERE version = '000042_account_session_access_tokens';
```

Interpretation is strict:

- No row means the environment has not applied `000042`; the corrected
  migration may be applied through the current runner after the normal backup
  and review gates.
- A checksum accepted by the current runner as the normalized or legacy
  line-ending form may continue through the normal runner checks.
- Any checksum rejected by the current runner is a hard stop. Retrieve the
  exact released migration bytes and obtain a reviewed, version-scoped
  compatibility or forward-migration decision.
- Never edit `schema_migrations`, change the `000042` file again, or weaken
  checksum validation globally to force a rollout.

This is inventory only; it does not authorize a migration or mutation.

Confirm all three schema versions are applied:

```sql
SELECT version, applied_at
FROM schema_migrations
WHERE version IN (
    '000043_account_native_legacy_nullable',
    '000044_channel_context_and_result_mode',
    '000045_outbox_claim_lease'
)
ORDER BY version;
```

### Hard acceptance checklist

Record each result in the rollout ticket. Every item is required before the
relay canary, again before scale load, and again before web activation:

- [ ] Zero duplicate semantic `event.job.result_ready` job keys.
- [ ] Zero failed/quarantined semantic `event.job.result_ready` rows.
- [ ] Zero known executable events with an invalid payload or job aggregate.
- [ ] Zero outbox rows with an event type outside the deployed closed allowlist.
- [ ] Zero malformed claim/lease metadata (a complete claim only while
      `processing`, no stranded metadata in another status).
- [ ] Zero processing leases expired beyond the approved lease-expiry grace.
- [ ] Zero canonical ready jobs without a semantic result-ready event after
      bounded reconciliation reaches `has_more=false`.
- [ ] Zero `result_ready` or `delivering` jobs older than the approved
      stale-finalization threshold.

The reviewed query bundle is
`deployments/postgres/000045_outbox_lease_preflight.sql`. Its shipped examples
use a five-minute expired-lease grace and a fifteen-minute stale-finalization
threshold. If the environment-approved values differ, record the exact values
and review the query-only change before execution. A nonzero result is evidence
for diagnosis, not permission to mutate customer or queue data.

Count compatibility rows that need review:

```sql
SELECT source, channel, COUNT(*) AS jobs
FROM jobs
WHERE result_mode = 'legacy_unknown'
GROUP BY source, channel
ORDER BY jobs DESC, source, channel;
```

Count external-push rows with a missing or malformed target according to the
`000044` target shape:

```sql
SELECT COUNT(*) AS invalid_external_push_targets
FROM jobs
WHERE result_mode = 'external_push'
  AND (
      target_channel IS DISTINCT FROM 'vk_bot'
      OR target_recipient_ref IS NULL
      OR char_length(target_recipient_ref) NOT BETWEEN 1 AND 512
      OR (
          target_thread_ref IS NOT NULL
          AND char_length(target_thread_ref) NOT BETWEEN 1 AND 512
      )
  );
```

Account-history rows must have no delivery target, and account-native sources
must retain their canonical account owner:

```sql
SELECT COUNT(*) AS account_history_rows_with_targets
FROM jobs
WHERE result_mode = 'account_history'
  AND (
      target_channel IS NOT NULL
      OR target_recipient_ref IS NOT NULL
      OR target_thread_ref IS NOT NULL
  );

SELECT source, COUNT(*) AS account_native_jobs_missing_account_id
FROM jobs
WHERE source IN ('miniapp', 'web')
  AND account_id IS NULL
GROUP BY source
ORDER BY source;
```

Investigate jobs still in finalization states after the operational threshold
(the example uses 15 minutes; use the incident-approved threshold for the
environment):

```sql
SELECT id, result_mode, status, updated_at
FROM jobs
WHERE status IN ('result_ready', 'delivering')
  AND updated_at < now() - INTERVAL '15 minutes'
ORDER BY updated_at ASC;
```

Required result: no rows at each hard acceptance checkpoint.

Check result-ready outbox rows that are pending publication:

```sql
SELECT id, aggregate_id AS job_id, attempts, next_attempt_at, created_at
FROM outbox_events
WHERE event_type = 'event.job.result_ready'
  AND status = 'pending'
ORDER BY next_attempt_at ASC, created_at ASC;
```

Failed semantic rows are quarantined incidents, not candidates for automatic
replacement:

```sql
SELECT id, aggregate_id, attempts, last_error_code, failed_at
FROM outbox_events
WHERE aggregate_type = 'job'
  AND event_type = 'event.job.result_ready'
  AND status = 'failed'
ORDER BY failed_at ASC NULLS FIRST, id ASC;
```

Required result: no rows.

Known event types must carry the same valid job envelope the relay enforces:

```sql
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
```

Required result: no rows. Do not expose payload contents in the rollout ticket.

Lease metadata must form a complete claim only while an event is `processing`.
This detects nullable metadata left absent or stranded in a terminal or pending
row:

```sql
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
```

Required result: no rows. This is evidence of a malformed lease state, not
permission to clear or rewrite it from this runbook.

Processing leases must not remain expired beyond the approved grace. The
shipped preflight uses five minutes:

```sql
SELECT id, event_type, claim_owner, lease_until, attempts
FROM outbox_events
WHERE status = 'processing'
  AND lease_until < now() - INTERVAL '5 minutes'
ORDER BY lease_until ASC, id ASC;
```

Required result: no rows.

The following checks are hard go/no-go gates after reconciliation.

Every canonical ready job must have a corresponding result-ready event in any
outbox status:

```sql
SELECT COUNT(*) AS unreconciled_canonical_result_ready_jobs
FROM jobs AS j
WHERE j.status = 'result_ready'
  AND j.account_id IS NOT NULL
  AND j.result_mode IN ('account_history', 'external_push')
  AND NOT EXISTS (
      SELECT 1
      FROM outbox_events AS e
      WHERE e.aggregate_type = 'job'
        AND e.aggregate_id = j.id
        AND e.event_type = 'event.job.result_ready'
  );
```

Required result: `0`. The query deliberately ignores outbox status. A
corresponding failed row is a separate incident, not permission to create a
duplicate.

Duplicate semantic result-ready events require separately reviewed diagnosis:

```sql
SELECT COUNT(*) AS duplicate_result_ready_job_keys
FROM (
    SELECT aggregate_id
    FROM outbox_events
    WHERE aggregate_type = 'job'
      AND event_type = 'event.job.result_ready'
    GROUP BY aggregate_id
    HAVING COUNT(*) > 1
) AS duplicates;
```

Required result: `0`. Do not delete or merge duplicate rows from this runbook.

The closed relay must have no unallowlisted event in any status:

```sql
SELECT event_type, status, COUNT(*) AS event_count
FROM outbox_events
WHERE event_type NOT IN (
    'event.job.created',
    'event.job.queued',
    'event.job.result_ready'
)
GROUP BY event_type, status
ORDER BY event_type, status;
```

Required result: no rows. An unknown row requires a code/rollout decision;
never mark it published manually.

Reconciliation is an operator-invoked, database-only operation after the hard
drain. These checks must not be turned into `UPDATE`, `DELETE`,
down-migration, or ad-hoc data repair commands. The command does not convert
`legacy_unknown` rows or invent a delivery target.

## Abort and rollback rules

If a gate fails, turn off web prepared-job activation and stop newly deployed
workers before diagnosing. Preserve queued messages and durable outbox rows.
Do not down-migrate and do not drop legacy columns. Diagnose from the
read-only evidence, then forward-fix or reconcile under a separately reviewed
operation; do not use destructive rollback.

Do not proceed to worker start or web activation while any old worker/relay may
still be running, while the deployment mechanism can overlap versions, while
reconciliation has not completed a successful page with `has_more=false`,
while reconciliation reports any blocked candidate, or while any hard
go/no-go query fails.

## Concurrent index build and observation

The index script is an operator-reviewed, non-transactional operation:

```text
deployments/postgres/000045_outbox_lease_indexes.concurrent.sql
```

Never pass it to `cmd/migrate`, wrap it in `BEGIN`/`COMMIT`, or concatenate it
with transactional migrations. Execute the semantic unique-index statement
only after the exact duplicate preflight returns no rows. The script is
idempotent by name, but `IF NOT EXISTS` does not prove that an existing index
has the right definition or is valid.

Observe active builds without changing database state:

```sql
SELECT pid, datname, relid::regclass AS table_name,
       index_relid::regclass AS index_name, phase,
       lockers_total, lockers_done, blocks_total, blocks_done,
       tuples_total, tuples_done
FROM pg_stat_progress_create_index
ORDER BY pid;
```

After each build, require the five named indexes to exist, be ready, valid, and
semantically match the intended definition. `pg_get_indexdef` exposes a
same-named index that `IF NOT EXISTS` would otherwise leave untouched:

```sql
SELECT expected.schema_name,
       expected.index_name,
       expected.definition_contract,
       index_state.indisready,
       index_state.indisvalid,
       pg_get_indexdef(index_state.indexrelid) AS actual_definition
FROM (
    VALUES
        (
            'public',
            'outbox_events_pending_schedule_idx',
            'NON-UNIQUE (next_attempt_at, id) WHERE status = ''pending'''
        ),
        (
            'public',
            'outbox_events_processing_lease_idx',
            'NON-UNIQUE (lease_until, id) WHERE status = ''processing'''
        ),
        (
            'public',
            'jobs_result_ready_reconciliation_idx',
            'NON-UNIQUE (created_at DESC, id DESC) WHERE status = ''result_ready'' '
            || 'AND account_id IS NOT NULL AND result_mode IN ('
            || '''account_history'', ''external_push'')'
        ),
        (
            'public',
            'jobs_stale_finalization_idx',
            'NON-UNIQUE (updated_at, id) WHERE status IN ('
            || '''result_ready'', ''delivering'')'
        ),
        (
            'public',
            'outbox_events_job_result_ready_unique_idx',
            'UNIQUE (aggregate_id) WHERE aggregate_type = ''job'' '
            || 'AND event_type = ''event.job.result_ready'''
        )
) AS expected(schema_name, index_name, definition_contract)
LEFT JOIN pg_catalog.pg_namespace AS index_schema
  ON index_schema.nspname = expected.schema_name
LEFT JOIN pg_catalog.pg_class AS index_relation
  ON index_relation.relnamespace = index_schema.oid
 AND index_relation.relname = expected.index_name
LEFT JOIN pg_catalog.pg_index AS index_state
  ON index_state.indexrelid = index_relation.oid
ORDER BY expected.schema_name, expected.index_name;
```

Required result: five rows with both booleans `true`; for each row,
`actual_definition` must match its contract in table, uniqueness, column order,
and partial predicate (schema qualification, casts, and harmless parentheses
may differ). The check targets the `public` schema explicitly and does not use
the operator's `search_path`; a non-`public` deployment requires a reviewed
schema-specific query before use. A missing, invalid, or semantically different
same-named index is a hard stop: obtain a separately reviewed index recovery
operation. Do not drop or recreate it from this runbook. Keep existing generic
indexes until target-environment query plans have been observed.

## Canary acceptance

Run one canary at a time and retain each resulting job ID in the rollout
ticket. Start with exactly one `WORKER_MODE=relay` process. Do not scale it
until one full lease interval plus the approved expiry grace has elapsed with
zero duplicate claims, zero over-threshold expired leases, zero
failed/quarantined rows, and no unbounded pending-age growth.

1. Submit one VK Bot external-push job. Confirm one `capture` ledger entry and
   exactly one delivery row for that job.
2. Submit one Mini App account-history result. Confirm one `capture` ledger
   entry and no delivery row for that job.
3. Read one existing web account-history result as its owning account. Web
   prepared-job activation remains disabled, so do not activate or submit a web
   prepared job from this runbook.

For each canary, inspect the owner-safe result/history surface as the owning
account. Any extra capture, unexpected delivery row, missing result, or VK
publication for an account-history job fails the canary and returns rollout to
the abort rules above.

After the single-relay canary is green, scale only to the relay count declared
in the approved load envelope. At the start and end of every scale step, and
every 60 seconds while that step is held, capture the count-only health snapshot
to the same private evidence directory:

```bash
: "${ROLLOUT_EVIDENCE_DIR:?set this to a private rollout-evidence path}"
mkdir -p "$ROLLOUT_EVIDENCE_DIR"
set -o pipefail
sample_at=$(date -u +%Y%m%dT%H%M%SZ)
sample_file=$(mktemp "$ROLLOUT_EVIDENCE_DIR/outbox-health-${sample_at}.XXXXXXXX")
observe-outbox-health \
  | tee "$sample_file"
```

The snapshot is the sole source for `pending`, `processing`, `failed`,
`oldest_pending_age_seconds`, and `expired_leases`. It intentionally does not
contain a `published` field. Record published throughput at the same 60-second
cadence from the private Prometheus query below, summed over every active relay:

```promql
sum(increase(vkagg_outbox_relay_resolutions_total{outcome="published"}[1m]))
```

This is the count durably resolved to `published` during that one-minute
interval. Do not substitute `COUNT(*) FROM outbox_events WHERE status =
'published'`: that is a lifetime database total, not a scale-step throughput
measurement. Attach the health JSON files and the Prometheus interval values to
the rollout ticket. At every scale step record:

- pending, processing, and failed outbox snapshot counts;
- published outbox resolution throughput from the query above;
- age of the oldest pending event;
- expired leases beyond the approved grace;
- claim, publish, finalization, and capture p95/p99 latency;
- retry and quarantine counts/rates;
- Redis publish throughput and delivery-stream backlog;
- PostgreSQL connection usage and lock/index-build observations.

Any threshold breach stops further scaling and keeps web activation disabled.
Passing these canaries is not evidence of a one-million-MAU production launch;
the exact environment-specific load envelope in `docs/LOAD_TESTING.md` must
also pass.

## Web-activation hold

This runbook prepares the backend boundary but does not itself authorize web
prepared-job activation. Keep that feature disabled until all of the following
evidence is attached to the rollout decision:

- migration checksum inventory and verified backup;
- all hard acceptance queries green after reconciliation and relay drain;
- five valid concurrent indexes and reviewed target-environment query plans;
- single-relay and scaled-relay canary evidence;
- VK external-push, Mini App account-history, and owner-safe web read canaries;
- an environment-specific load report that meets every declared SLO and
  capacity gate;
- independent reviews for the completed implementation tasks and one
  cross-layer review of migration, relay, reconciliation, billing capture,
  channel delivery, and owner-safe reads.

Until that evidence exists, the only valid verdict is “backend prepared,
activation held,” not “production 1M-MAU ready.”
