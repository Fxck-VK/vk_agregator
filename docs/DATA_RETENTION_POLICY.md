# Data Retention Policy

This document defines the default retention policy for hot product data,
generated content, provider payloads and analytics. It is a policy contract, not
the cleanup implementation itself.

Retention windows are intentionally configurable. Future cleanup jobs may read
these values from environment variables or a `retention_policies` table, but the
classes below define the safe defaults and the data that must not be deleted
automatically.

## Principles

- Financial and audit records are protected. Ledger, payment and balance history
  must not be automatically deleted by generic cleanup jobs.
- User-generated content is minimized. Prompts, conversation messages and raw
  provider payloads are short-lived unless there is an explicit product/legal
  reason to keep them.
- Raw provider payloads should normally not be stored. If they are needed for
  debugging, store redacted payloads only and keep them for a short window.
- Analytics must be built from bounded aggregate tables, not by scanning raw hot
  prompt/message/job-log tables forever.
- Binary artifact cleanup must keep Postgres metadata and S3/MinIO objects
  consistent. Delete or expire storage objects only through owner-aware backend
  maintenance paths.
- Redis data is runtime state. Rate limits, cooldowns, locks, menu state and
  short-lived dialog state must use TTLs.
- Cleanup must be idempotent, batched and observable. It must not break billing
  idempotency, job recovery, artifact ownership checks or payment reconciliation.

## Retention Classes

Every persisted row/object that enters a cleanup or analytics path should be
classified with one of the data classes below. The code contract lives in
`internal/domain/data_classification.go`; storage and maintenance code should
branch on these bounded values instead of table-name string matching.

| Data class | Examples | Cleanup stance |
|------------|----------|----------------|
| `financial` | ledger entries, balance history, payment intents/events/refunds, billing audit records | Protected audit data. No generic automatic deletion. |
| `operational` | jobs metadata, delivery state, provider task ids, idempotency state, support diagnostics | Cleanup allowed only for non-audit diagnostics and old event/log details. |
| `user_content` | prompts, conversation messages, assistant replies, user generation inputs | Bounded retention; redact/delete raw content after the configured window. |
| `provider_payload` | raw provider request/response payloads and provider-native debug data | Avoid storage; if stored, redact and keep short-lived only. |
| `artifact_metadata` | artifact ownership, lifecycle, moderation and private storage coordinates | Cleanup must stay consistent with S3/MinIO object lifecycle and owner checks. |
| `analytics_aggregate` | DAU/WAU/MAU, jobs by model, payment funnel, provider error counters | Long-lived aggregate data, no prompts/raw PII/raw provider payloads. |
| `temporary_cache` | Redis rate limits, cooldowns, locks, menu/dialog state, runtime counters | TTL-based; never the durable source of truth. |

| Class | Examples | Default retention | Rule |
|-------|----------|-------------------|------|
| Financial audit | `ledger_entries`, account/balance history, payment intents, payment events, refunds, payment product snapshots | Long-term / effectively indefinite | Never delete through generic cleanup. Needed for disputes, refunds, audit and reconciliation. |
| Job metadata | jobs, reservation/capture links, delivery state, artifact ownership links, provider task ids needed for support | Long-term, 365+ days or indefinite lightweight metadata | Keep metadata needed for support and accounting. Do not keep full raw request/response bodies here. |
| Job event logs | job lifecycle events, retry logs, provider lifecycle diagnostics, outbox terminal events | 14-30 days | Aggregate useful counters before deletion. Keep enough history for support and incident review. |
| Conversation content | `conversation_messages`, prompts, assistant replies, short chat context | 30-90 days | Delete or redact raw content after the window. Summaries may live longer if they do not contain sensitive raw text. |
| Conversation summaries | `conversation_summaries` | 90-180 days | Keep only compact context needed for product UX; allow user reset/delete in later privacy work. |
| Provider raw payloads | raw provider request/response/debug payloads | Avoid storage; if enabled, 1-7 days redacted | Never expose to users. Never log secrets, private URLs, prompt bodies or raw PII. |
| Artifact binaries | generated photos/videos/files, provider originals, VK-ready variants, reference uploads | By product tier and artifact type | Free/temp/failed artifacts can expire earlier; paid/user-visible artifacts live longer. Metadata and objects must expire consistently. |
| Analytics aggregates | daily/monthly DAU, jobs by model, payments, funnel, provider errors | Long-term | Store aggregate, bounded-label, non-PII records. Do not use raw prompts/provider payloads as analytics source of truth. |
| Temporary runtime state | Redis rate limits, cooldowns, locks, queues, menu/dialog state | Minutes to days | Use TTLs. Redis is not the durable source of truth. |

## Suggested Config Defaults

These names are the runtime policy knobs used by worker maintenance. They stay
configurable so operators can lengthen/shorten windows without schema changes.

```env
RETENTION_JOB_EVENTS_DAYS=30
RETENTION_CONVERSATION_MESSAGES_DAYS=90
RETENTION_CONVERSATION_SUMMARIES_DAYS=180
CONVERSATION_RETENTION_BATCH_SIZE=500
RETENTION_PROVIDER_PAYLOAD_DAYS=7
JOB_LOG_RETENTION_BATCH_SIZE=500
JOB_ERROR_AGGREGATE_LOOKBACK_DAYS=30
ANALYTICS_AGGREGATE_LOOKBACK_DAYS=7
ARTIFACT_FREE_RETENTION_DAYS=7
ARTIFACT_PAID_RETENTION_DAYS=365
ARTIFACT_TEMP_RETENTION_DAYS=1
ARTIFACT_ORPHAN_RETENTION_DAYS=7
RETENTION_ANALYTICS_AGGREGATES_DAYS=0
```

`0` means keep indefinitely for aggregate data only. Do not use `0` as a
shortcut for raw prompts, raw provider payloads or job diagnostic logs.

## Schema Contract

Migration `000021_retention_schema` prepares the database for future retention
jobs. It adds metadata only; it does not delete or redact data by itself.

Cleanup-eligible non-financial tables use these fields where applicable:

| Field | Purpose |
|-------|---------|
| `created_at` / `updated_at` | Existing audit timestamps used for ordering and safe batches. |
| `expires_at` | Retention deadline for cleanup candidates. For payment tables this may already mean payment lifecycle and must not be treated as retention. |
| `deleted_at` | Soft-delete marker for rows whose binary objects/content were removed by backend maintenance. |
| `retention_class` | Bounded data class from `internal/domain/data_classification.go`. Generic cleanup must branch on this, not table names alone. |
| `artifact_tier` | Artifact product tier: `standard`, `free`, `paid` or `temporary`. Paid artifacts can use longer retention than free/temp artifacts. |
| `redacted_at` | Marker that raw user/provider content was removed or replaced with a redacted representation. |

Financial tables are explicitly tagged with `retention_class = 'financial'`,
but they are not given generic `deleted_at` retention cleanup semantics. Ledger,
payment intent, payment event, refund and balance records remain protected audit
data.

## Conversation Retention Runtime

The worker maintenance loop owns conversation cleanup. VK itself still stores
message history on VK's side; this policy applies only to server-side
`conversation_messages` and `conversation_summaries` used to build compact
NeuroHub context.

Runtime variables:

```env
RETENTION_CONVERSATION_MESSAGES_DAYS=90
RETENTION_CONVERSATION_SUMMARIES_DAYS=180
CONVERSATION_RETENTION_BATCH_SIZE=500
```

Cleanup behavior:

- `conversation_messages` older than `RETENTION_CONVERSATION_MESSAGES_DAYS`
  are marked with `expires_at`.
- Expired message rows are redacted in place: `text=''`, `token_count=0`,
  `redacted_at=<cleanup time>`. Rows are kept so sequence/job metadata and
  idempotency remain stable.
- Dialog context queries ignore `deleted_at` and `redacted_at` rows, so old raw
  prompts/assistant replies are not sent back into provider prompts.
- `conversation_summaries` use the longer
  `RETENTION_CONVERSATION_SUMMARIES_DAYS` window. This keeps compact context
  available after raw messages have been redacted.
- Expired summaries are also redacted in place instead of hard-deleted.

The retention job is batched and idempotent. It must never touch ledger,
payments, balance history or VK-side message history.

## Job Logs And Provider Payload Runtime

Jobs remain the durable operational record. Maintenance must preserve:

- `jobs.status`;
- `jobs.output_artifact_ids`;
- normalized provider task status, provider name, model code, external id,
  timing and error class.

Raw provider request/response data is short-lived diagnostics. It must never be
used as the source of truth for user-visible output, billing, delivery or
support history.

Runtime variables:

```env
RETENTION_JOB_EVENTS_DAYS=30
RETENTION_PROVIDER_PAYLOAD_DAYS=7
JOB_LOG_RETENTION_BATCH_SIZE=500
JOB_ERROR_AGGREGATE_LOOKBACK_DAYS=30
ANALYTICS_AGGREGATE_LOOKBACK_DAYS=7
```

Cleanup behavior:

- Provider errors are aggregated into `job_error_aggregates` before raw
  diagnostics are redacted. Aggregates use bounded labels only and must not
  contain prompts, auth headers, raw provider JSON, PII or private URLs.
- `provider_tasks.request` and `provider_tasks.result` are marked with
  `expires_at` after `RETENTION_PROVIDER_PAYLOAD_DAYS`, then redacted in place:
  `request='{}'`, `result=NULL`, `redacted_at=<cleanup time>`.
- Provider task rows are kept so job recovery/support can still see provider,
  model, external id, terminal status, timings and normalized error class.
- `job_events` is treated as an optional lifecycle diagnostics table. When it
  exists, rows older than `RETENTION_JOB_EVENTS_DAYS` are deleted in batches.
  Absence of the table is a no-op for current deployments.
- Job status/result/artifact references are not deleted by this cleanup.

Operator-facing diagnostics should read aggregates and normalized job/provider
metadata, not raw provider payloads.

## Analytics Aggregates Runtime

Dashboards must read daily aggregate tables instead of repeatedly scanning hot
`jobs`, `conversation_messages`, provider task and payment rows. Raw prompts,
assistant replies, provider payloads, auth headers, private URLs and PII must
not be copied into analytics aggregates.

Runtime variables:

```env
ANALYTICS_AGGREGATE_LOOKBACK_DAYS=7
RETENTION_ANALYTICS_AGGREGATES_DAYS=0
```

The worker maintenance loop refreshes the most recent
`ANALYTICS_AGGREGATE_LOOKBACK_DAYS` days. The lookback is intentionally
configurable so late webhooks, delayed jobs and provider retries can be
absorbed without changing schema or dashboard queries.

Aggregate tables:

- `daily_user_activity`: active/new/returning/message/generation users by
  surface.
- `daily_generation_stats`: created/succeeded/failed jobs, reserved/captured
  credits and artifact counts by surface, operation type and modality.
- `daily_provider_stats`: provider/model task volume, terminal status counts,
  latency and bounded cost counters.
- `daily_revenue_stats`: payment/refund totals, net amount and sold credits by
  provider/currency.
- `daily_referral_stats`: referral funnel events by source.
- `daily_retention_stats`: cohort retention by activity date and surface.
- `daily_funnel_stats`: bounded product funnel steps for registration,
  messages, generations, payments and referral activation.

Aggregates are idempotent upserts keyed by day and bounded dimensions. They may
be retained long-term, but they are not a replacement for financial audit
tables. Ledger, payment intents/events and refunds remain the source of truth
for money.

## Artifact Lifecycle Runtime

Postgres artifact metadata is the source of truth for cleanup decisions.
S3/MinIO contains only the binary object bytes. The maintenance worker must
expire metadata first and delete object storage only in a later, retry-safe
step.

Runtime variables:

```env
ARTIFACT_FREE_RETENTION_DAYS=7
ARTIFACT_PAID_RETENTION_DAYS=365
ARTIFACT_TEMP_RETENTION_DAYS=1
ARTIFACT_ORPHAN_RETENTION_DAYS=7
MEDIA_INPUT_RETENTION_DAYS=0
MEDIA_FAILED_RETENTION_DAYS=0
MEDIA_ORIGINAL_RETENTION_DAYS=0
MEDIA_VARIANT_RETENTION_DAYS=0
```

Lifecycle rules:

- Free artifacts use `ARTIFACT_FREE_RETENTION_DAYS`.
- Paid artifacts use the longer `ARTIFACT_PAID_RETENTION_DAYS` window.
- Temporary artifacts and temp uploads use `ARTIFACT_TEMP_RETENTION_DAYS`.
- Failed/deleted media uses `MEDIA_FAILED_RETENTION_DAYS` when configured.
- Orphan artifacts with no job/delivery reference use
  `ARTIFACT_ORPHAN_RETENTION_DAYS`.
- Unused uploaded input references still use `MEDIA_INPUT_RETENTION_DAYS`.
- Unreferenced provider originals and delivery variants remain controlled by
  `MEDIA_ORIGINAL_RETENTION_DAYS` and `MEDIA_VARIANT_RETENTION_DAYS`.

Cleanup order:

1. Worker maintenance selects eligible artifacts/variants and sets
   `expires_at=<cleanup time>` in Postgres.
2. A later cleanup pass selects only expired rows with private storage
   coordinates.
3. The storage adapter deletes the S3/MinIO object.
4. Postgres is updated with `deleted_at`, cleared storage coordinates and a
   deleted lifecycle/status marker.

This DB-first flow keeps deletion idempotent: if object deletion or the worker
crashes midway, the next maintenance pass can retry from `expires_at` without
guessing from object storage. Generic artifact cleanup must not touch ledger,
payment, balance, job status or owner metadata.

## Operator Control

Retention and analytics must be observable without exposing user content or
provider data. The admin API exposes protected read-only endpoints for that
purpose:

- `GET /admin/retention/operator/status`: retention posture by table and data
  class.
- `GET /admin/retention/operator/dry-run`: cleanup candidate counts before any
  mutation.
- `GET /admin/analytics/operator/status`: aggregate-table freshness.
- `GET /admin/data/operator/hot-rows`: oldest still-hot rows by table and data
  class.
- `GET /admin/artifacts/operator/orphans`: orphan artifact object counts by
  safe metadata.

All endpoints require `X-Admin-Token`. Responses are safe DTOs only: counts,
bounded labels, byte totals and timestamps. They must not include raw prompts,
assistant replies, provider request/response payloads, VK user ids, owner ids,
storage buckets, storage keys, signed URLs or private provider URLs.

## Rollout Rules

- Shortening a retention window in production requires an operator-reviewed
  rollout and a dry-run report.
- Cleanup jobs must report candidate counts before deleting data.
- Payment, ledger and billing idempotency keys are outside generic retention.
- Artifact cleanup must delete S3/MinIO objects and update Postgres metadata in
  a retry-safe way.
- Provider raw payload cleanup must prefer redaction over long-term retention.
- Retention changes are product/security decisions and must be documented in
  `RUNBOOK.md` when they affect operations.
