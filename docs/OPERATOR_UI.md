# Operator UI Boundaries

Operator UI is a protected operational surface for jobs, payments, refunds,
DLQ, provider health, retention and support workflows. It is not a database
client. Every read or action must go through backend API endpoints that return
safe DTOs.

## Roles

| Role | Purpose | Allowed Surface |
| --- | --- | --- |
| `owner` | Break-glass owner for protected policy and all operator/admin actions. | Full protected backend API, audited mutations only |
| `admin` | System operator for health, queues, jobs, provider status, retention and audit visibility. | System status, job triage, queue/DLQ, provider health, retention, analytics, safe users/referrals, audit |
| `operator` | Runtime operator for stuck jobs and queues. | Safe jobs, queue/DLQ, provider health, retention dry-run, analytics |
| `support` | User support without PII or payment/provider internals. | Safe job rows, safe user summaries, referral counts |
| `finance` | Payment/refund operator. | Safe payment/refund DTOs, ledger-backed refund actions, safe user summary, audit |

The versioned backend contract lives in
`internal/domain/operator_access.go`. UI code can use it for rendering and
feature-gating, but backend handlers/services remain the enforcement boundary.

## Access Contract Endpoint

Protected route:

```text
GET /admin/access/operator
Header: X-Admin-Token: <admin token>
```

The response exposes:

- current auth mode;
- effective role for the legacy token model;
- role definitions;
- permission names;
- data boundaries;
- forbidden data/actions.

It must not expose tokens, raw identities, prompts, provider payloads, payment
payloads, private artifact URLs or raw storage/provider URLs.

Current state: `ADMIN_TOKEN` is treated as `owner` / break-glass access until
per-operator identity and scoped tokens are introduced.

## Enforcement

Protected admin and billing operator routes enforce the role contract in the
backend before handler business logic runs. The legacy single-token model
defaults to `owner` for backward compatibility, while tests and future token
issuers can set a narrower effective role.

Every valid-token operator request is wrapped with sanitized audit logging,
including forbidden and rate-limited attempts. Audit entries store only stable
safe references:

- safe actor reference derived from the authenticated operator token;
- bounded action name;
- safe target type/ref;
- success or error result;
- safe request reference.

Audit entries must never contain raw tokens, idempotency keys, prompt bodies,
provider payloads, payment provider payloads, private URLs, contact data or raw
user identifiers. Refund, retry, replay, sync, cancel, product and retention
operator actions must all be auditable with actor, timestamp, action, target and
result.

Protected admin/billing endpoints also have a per-actor shared Redis-backed
rate limit. The default is `ADMIN_RATE_LIMIT_LIMIT=120` per
`ADMIN_RATE_LIMIT_WINDOW=1m`. Handler-level in-memory limiting remains only as a
fallback for unit tests and isolated local runs where no shared limiter is
injected.

## Data Boundaries

All operator surfaces must preserve these rules:

- Backend API only, no direct SQL from UI or operators.
- Safe DTOs only.
- No raw VK IDs, contact data, prompt bodies, provider payloads, payment
  provider payloads or private artifact URLs.
- Lists must be paginated and bounded.
- Mutating actions must be backend-audited and idempotent.
- Billing changes must go through ledger/payment services only.

## Support Boundary

Support can inspect safe troubleshooting context:

- job display refs;
- operation/modality/status;
- safe error class;
- safe user status/risk summary;
- referral counts.

Support cannot see:

- raw prompts;
- raw VK/user identifiers;
- payments/refunds;
- provider-native payloads or private URLs.

## Finance Boundary

Finance can inspect and operate on payments/refunds through protected backend
routes only. Refunds remain ledger-backed and provider-verified. Finance cannot
triage provider jobs or inspect prompts/media payloads.

## Payments Console

Protected route:

```text
GET /billing/operator/console
Header: X-Admin-Token: <admin token>
```

The payments console is the finance surface for payment intents, safe ledger
snapshots, refund state and webhook inbox visibility. It supports bounded
lookups by `intent_id`, `provider_payment_id`, `user_id`, `status` and
`provider`. Operators may use raw lookup ids as request filters, but responses
must return only safe display/action refs and derived states. Raw YooKassa
payloads, confirmation URLs, idempotency keys, raw user ids, raw provider
payment ids and refund provider ids must not be rendered.

Refund actions must use the existing protected backend flow:

```text
POST /billing/payment-intents/{opaque_action_ref}/refund
Header: X-Idempotency-Key: <operator idempotency key>
```

For million-user scale the lookup path is backed by indexes on payment intent
user/status/provider-payment fields, webhook event provider-payment fields and
refund intent ids. The UI must keep result pages bounded and must not expose a
raw SQL-like search surface.

## Admin Boundary

Admin sees system health and operational status:

- queues and DLQ;
- provider health;
- config health;
- retention/analytics status;
- safe jobs and audit.

Admin does not own finance refunds in the role contract; `owner` is the only
role that combines all surfaces.

## Jobs Console

Protected route:

```text
GET /admin/jobs/operator
Header: X-Admin-Token: <admin token>
```

The jobs console is the primary operational list for job triage. It supports
bounded filters by status, operation type, provider, safe user id, creation
time and error class. The endpoint returns safe job DTOs only: display refs,
operation/modality/status, safe lookup id, safe correlation ref, counts, costs
and sanitized error class. It must not expose raw prompts, raw VK identifiers,
provider payloads, private URLs, job params or idempotency keys.

Pagination is cursor-based. Clients pass `cursor=<next_cursor>` from the
previous response and must not use heavy `OFFSET` pagination for operator-scale
views. Backend ordering is stable by `(created_at DESC, id DESC)`, which keeps
the list usable when the project has large job volume.

At database level, jobs console queries are backed by indexes for:

- `(created_at DESC, id DESC)`;
- `(status, created_at DESC, id DESC)`;
- `(operation_type, created_at DESC, id DESC)`;
- `(modality, created_at DESC, id DESC)`;
- `(user_id, created_at DESC, id DESC)`;
- `(error_code, created_at DESC, id DESC)` for non-empty errors;
- provider lookup through `provider_tasks(provider, job_id)`.

## DLQ And Retry Tools

Protected routes:

```text
GET /admin/jobs/dlq
POST /admin/jobs/{job_id}/replay
POST /admin/jobs/dlq/replay
Header: X-Admin-Token: <admin token>
Header for mutations: X-Idempotency-Key: <unique operation key>
```

The DLQ console is derived from persisted failed jobs and bounded provider task
metadata. Rows expose safe job refs, status, sanitized error class, attempt
count, provider task count and replay guard-rail state. They must not expose raw
prompts, provider payloads, private artifact URLs, payment payloads or raw user
identifiers.

Replay guard rails:

- only `failed_retryable` jobs can be requeued;
- captured jobs are blocked and require financial triage;
- batch replay is capped at 25 jobs;
- batch replay always skips paid/provider jobs, even when the request asks for
  provider override;
- a paid/provider job can only be replayed one at a time with an explicit
  `allow_paid_provider=true` override after operator triage;
- replay writes a queued job outbox event so workers pick the job through the
  same asynchronous path as normal job intake.

Do not add bulk replay paths that bypass these rules. Paid/provider replay can
spend provider quota and can affect billing capture ordering, so it must stay a
deliberate per-job operator action.

## Provider Health Console

Protected route:

```text
GET /admin/providers/operator
Header: X-Admin-Token: <admin token>
```

The provider health console shows bounded operational health for:

- AI providers: DeepInfra, PoYo, APIMart, Runway and other configured provider
  classes;
- payment provider: YooKassa webhook inbox state;
- delivery provider: VK delivery state.

The UI must never call providers directly. It reads backend aggregates only:

- `provider_tasks` for provider error rate, p95 latency, latest normalized error
  class, quota/rate-limit observations, in-flight tasks and circuit state;
- payment webhook inbox stats for unprocessed YooKassa events;
- delivery snapshots for VK delivery failures/retries.

Provider health DTOs must not expose provider-native model ids, external task
ids, request/response payloads, raw prompts, raw YooKassa payloads, VK
attachments, VK message text, idempotency keys, private URLs, credentials or
headers. The frontend displays only safe provider/service labels, normalized
error classes, bounded counters, percentages and timestamps.

Circuit breaker, quota and cooldown fields are read-only status signals at this
stage. Operators can use them for triage, but provider disable/degrade actions
remain worker-owned and must not be triggered from the UI until a separate
audited action model exists.

## Retention And Cleanup Console

Protected routes:

```text
GET /admin/retention/operator/status
GET /admin/retention/operator/dry-run
POST /admin/retention/operator/run-cleanup
Header: X-Admin-Token: <admin token>
Header for cleanup: X-Idempotency-Key: <unique operation key>
```

The retention console reports bounded counters for old messages, job events,
artifact lifecycle state and orphan artifact groups. It is an operational
cleanup surface, not a data browser. Responses must include only table names,
retention classes, counts, sizes and timestamps/ages. They must not expose raw
prompts, conversation bodies, VK payloads, provider payloads, raw user ids,
storage bucket/key values, private artifact URLs or payment provider payloads.

Cleanup controls:

- `Dry-run cleanup` is available to operators with `retention:dry_run` and
  previews eligible cleanup actions without mutating data.
- `Run cleanup` requires `retention:cleanup` and is restricted to `owner` and
  `admin` roles. It calls the maintenance service and is audited like other
  operator mutations.
- The UI must refresh the safe retention snapshot after cleanup instead of
  showing deleted row bodies.

Financial data is explicitly outside automatic cleanup. Ledger entries, payment
intents, payment events, refunds, balance history and other money/audit tables
must not be deleted by retention cleanup. Any future financial archival policy
requires a separate design and manual approval path.

## Scalable Read Models

Operator dashboards must not scan hot raw tables as their primary data source
when the product reaches high volume. The safe read-model layer is built by the
maintenance worker and exposed through protected backend endpoints.

Current aggregate tables:

- `daily_user_activity` - active/new/returning user counters by surface;
- `daily_generation_stats` - jobs, artifacts and captured credits by surface,
  operation type and modality;
- `daily_provider_stats` - provider task counts, failures and latency by
  bounded provider/model labels;
- `daily_revenue_stats` - payment/refund/credit counters derived from payment
  tables while ledger remains authoritative;
- `daily_dlq_stats` - retryable/terminal failed job counters by surface,
  operation type, modality and normalized error class;
- `daily_referral_stats` - referral funnel counters without invited-user PII;
- `daily_retention_stats` - cohort retention counters;
- `daily_funnel_stats` - product funnel counters;
- `job_error_aggregates` - bounded error-class snapshots before raw
  diagnostics are redacted.

Protected route:

```text
GET /admin/analytics/operator/status
Header: X-Admin-Token: <admin token>
```

The route reports only freshness, row counts and latest aggregate dates. It is
safe for the UI to poll because it reads aggregate metadata instead of raw
prompts, messages, provider payloads, payment payloads or private storage
paths.

New dashboard cards for jobs, providers, payments, DLQ and retention should
prefer these read models. If a new UI panel needs raw-table access for triage,
it must be a bounded, paginated drill-down endpoint with explicit indexes and
safe DTOs.

## Frontend Admin App

`web/admin` is the first operator console. Its first job is fast and safe
problem resolution, not decorative analytics. The app should keep these product
rules:

- list screens are tables with bounded filters and cursor/bounded pagination;
- detail panels show safe DTO fields only and never raw prompts, provider
  payloads, private URLs, tokens, payment payloads or raw PII;
- operator mutations require an explicit confirmation dialog and backend
  idempotency key;
- refunds, sync, cancel, DLQ replay and retention cleanup stay backend-owned
  actions;
- destructive or provider-spending actions must explain their guard rails before
  the operator confirms;
- dashboards should prefer aggregate/read-model endpoints and link to bounded
  drill-down tables for triage.

Current frontend surfaces:

- Jobs: filters, cursor pagination, queue summary and safe job detail panel.
- DLQ: failed-job table, single replay, guarded provider override and guarded
  safe batch replay.
- Payments: intent table, reconciliation, safe ledger snapshot, webhook inbox,
  refunds and reasoned sync/cancel/refund actions.
- Providers/Media: read-only provider health, route risk and media lifecycle
  state.
- Retention: counters, dry-run preview, guarded cleanup and read-model
  freshness.
- Users/Referrals/Audit: safe support and audit views without raw identifiers or
  PII dumps.

## Scale Requirements

Operator UI must stay usable at 1M monthly users:

- no unbounded list endpoints;
- no dashboard queries over raw messages/prompts as the primary source;
- prefer aggregate/read-model endpoints for dashboards;
- keep high-cardinality labels and raw PII out of metrics/logs;
- use dry-run endpoints for retention/cleanup previews.

## Production Hardening

`/admin/*` is not a public product surface. In production it must be protected
by at least one of these controls:

- reverse proxy or Cloudflare Tunnel rule that blocks public `/admin/*`;
- Cloudflare Access or VPN-only access for operator users;
- strict backend auth that returns only `401`, `403` or `404` to unauthenticated
  public requests.

A public `2xx` response from `/admin/*` is a production incident. `3xx` is
acceptable only when it is the expected Cloudflare Access/VPN identity redirect,
not an application redirect to an admin page.
Production smoke checks probe representative admin routes on `neiirohub.ru`,
`vk.neiirohub.ru` and `app.neiirohub.ru`. Prometheus also runs the
`blackbox-public-admin` job and raises `PublicAdminSurfaceOpen` when a public
admin route becomes reachable.

Before enabling a new admin action in production:

- the backend route must enforce role permissions and write an audit entry;
- the action must require an idempotency key when it can mutate state;
- responses must use safe DTOs and must not include raw prompts, provider
  payloads, payment payloads, private artifact URLs, tokens or raw PII;
- `tests/k6/admin-actions.js` must pass against a loadtest or DEV contour with
  production-like table sizes;
- `K6_ADMIN_MUTATION_SMOKE=true` may be used only in `APP_ENV=loadtest` after a
  backup, never against production data.
