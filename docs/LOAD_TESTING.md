# Load Testing Plan

This document defines the load-testing rollout for VK AI Aggregator. It is a
test contract, not an instruction to hit production traffic or paid providers.

## Chapter 1. Goal And Boundaries

### Goal

The first goal is to find the current bottleneck in the existing architecture
before moving data stores, adding servers or changing queue topology.

Load tests must answer these questions:

- how much traffic `cmd/api` can accept before latency or errors grow;
- whether VK webhook handling stays idempotent under repeated events;
- whether Mini App BFF routes keep acceptable latency under concurrent users;
- whether Redis queue depth grows faster than workers can drain it;
- whether Postgres queries, locks or indexes become the first bottleneck;
- whether workers process text/image/video jobs without starving lighter work;
- whether artifact storage/readiness becomes a bottleneck;
- whether billing mock/payment-intent paths stay ledger-safe under load.

### Surfaces In Scope

The load-test scope includes:

| Surface | What To Exercise |
|---|---|
| `cmd/api` | health/readiness, VK webhook intake, Mini App BFF, admin-safe observability where applicable |
| VK webhook | `confirmation`, `/start`, menu buttons, plain text, duplicate event ids, callback payloads |
| Mini App BFF | launch-authenticated user routes, balance, jobs, artifacts, payments in mock/test mode |
| Redis | job streams, rate-limit keys, dialog state, active-job limits, DLQ counters |
| Postgres | users, jobs, ledger reads/writes, conversations, artifacts, payment mock/intents |
| `cmd/worker` | generation jobs, provider mock submit/poll, artifact storage, delivery mock, billing capture/release |
| Job lifecycle | queued -> processing -> artifact -> delivery -> capture/succeeded/failure paths |
| Artifacts | metadata rows, S3/MinIO object write/read, owner-checked access paths |
| Billing mock | reservation, capture, release, top-up/payment intent mock flows, idempotency |

### Explicitly Out Of Scope

The first load tests must not:

- call real AI providers such as OpenAI, DeepInfra, FAL, Runway, APIMart or PoYo;
- spend provider balance or YooKassa money;
- send high-volume messages to the production VK community;
- use production VK callback settings or production VK delivery for synthetic load;
- use production YooKassa webhook settings for synthetic load;
- expose `/admin/*`, `/metrics`, raw prompts, provider payloads, tokens or PII;
- mutate production Postgres, Redis or S3/MinIO data.

### Required Test Contour

Load tests must run against one of these safe contours:

1. **Local mock contour**: Docker Postgres/Redis/MinIO, mock providers, mock
   payment provider, mock or disabled VK delivery.
2. **DEV contour**: `dev-vk.neiirohub.ru`, `dev-app.neiirohub.ru` and
   `dev.neiirohub.ru` with DEV VK community and mock providers by default.
3. **Dedicated staging/loadtest contour**: production-shaped infrastructure,
   separate data stores, separate secrets and no paid provider calls unless a
   separate live-smoke approval is given.

Production traffic is not a load-test target. Production can be used only for
safe smoke/readiness checks already described in the runbook.

### Required Runtime Defaults

The load-test environment should fail closed unless these properties are true:

```text
APP_ENV=loadtest
PROVIDER=mock
PROVIDER_CHAIN=mock
IMAGE_PROVIDER=mock
VIDEO_PROVIDER=mock
PAYMENT_PROVIDER=mock
VK_DELIVERY_MODE=mock
MODERATION_PROVIDER=keyword
ARTIFACT_SCANNER=none
```

Any test that enables real providers, real VK delivery, YooKassa or production
data stores is not a generic load test. It is a separate credential-bound live
smoke and requires explicit human approval.

### Success Output For Chapter 1

Chapter 1 is complete when the repository has:

- a written scope for what the load tests measure;
- explicit exclusions for paid providers, production VK and production data;
- a safe contour definition for future k6 scripts;
- a clear list of components whose saturation must be measured.

Later chapters will add actual k6 scenarios, metrics collection and reports.

## Chapter 2. Test Environment

### Runtime Mode

Synthetic load tests use a dedicated runtime mode:

```text
APP_ENV=loadtest
```

This mode is not production, staging or normal local development. It exists to
make load tests production-shaped enough to exercise the backend, while still
failing closed before any paid provider, YooKassa payment or real VK delivery
can be used accidentally.

### Load-Test Env Template

Use the checked-in template:

```powershell
Copy-Item .env.loadtest.example .env.loadtest
notepad .env.loadtest
```

The template intentionally sets:

```text
PROVIDER=mock
PROVIDER_CHAIN=mock
IMAGE_PROVIDER=mock
VIDEO_PROVIDER=mock
PAYMENT_PROVIDER=mock
VK_DELIVERY_MODE=mock
MODERATION_PROVIDER=keyword
ARTIFACT_SCANNER=none
```

Postgres, Redis and MinIO use the current isolated infrastructure by default:

```text
DATA_SERVICES_MODE=local
POSTGRES_MODE=local
REDIS_MODE=local
S3_MODE=local
```

For a local run, start the current data stack first:

```powershell
docker compose up -d postgres redis minio
```

Then start the API, worker and provider-webhook with `.env.loadtest` loaded by
the shell or copied to `.env`. Future chapters will add one-command runners and
k6 scenarios.

### Fail-Closed Validation

`internal/platform/config` rejects `APP_ENV=loadtest` unless these safe modes
are active:

- generation provider: mock only;
- image provider: mock or empty;
- video provider: mock or empty;
- payment provider: mock;
- VK delivery: mock;
- moderation: keyword or empty;
- artifact scanner: none.

This prevents a bad load-test env from spending provider balance, sending real
VK messages or touching YooKassa.

### What Can Be Loaded

Chapter 2 enables safe load tests for:

- HTTP and webhook intake;
- menu/callback routing;
- Mini App BFF request paths;
- mock job creation and lifecycle;
- Redis queue pressure;
- Postgres reads/writes;
- MinIO artifact writes;
- mock billing reservation/capture/release/top-up flows.

It does not yet add k6 scripts or performance budgets. Those belong to later
chapters.

## Chapter 3. Basic k6 Scenarios

### Goal

Chapter 3 adds the first safe k6 script for the API surface without heavy
generation calls or paid providers. The script covers:

```text
GET  /health
GET  /readyz
POST /webhooks/vk
GET  /miniapp/balance
POST /miniapp/jobs
GET  /miniapp/jobs/{id}
```

The Mini App job request uses `text_generate` and relies on mock providers in
`APP_ENV=loadtest`. It creates real backend Jobs in the isolated load-test data
stores, but it must not call real AI providers, YooKassa or production VK.

### Script

The script lives at:

```text
tests/k6/basic-api.js
```

The nightly quality workflow already discovers `tests/k6/*.js`. To avoid
failing CI when no API server is running, the script is disabled by default and
performs a no-op unless `K6_BASE_URL` or `K6_RUN=1` is set.

All checked-in k6 scripts refuse known production hostnames such as
`vk.neiirohub.ru`, `app.neiirohub.ru` and `neiirohub.ru` by default. The
override `K6_ALLOW_PRODUCTION_LIVE_SMOKE=true` is reserved only for a separate,
human-approved live smoke; it must not be used for generic load tests or
capacity runs.

### Local Run

Start a load-test API contour first:

```powershell
Copy-Item .env.loadtest.example .env.loadtest
docker compose up -d postgres redis minio

# Load .env.loadtest into the shell or copy it to .env before starting API,
# worker and provider-webhook in APP_ENV=loadtest.
```

Then run k6:

```powershell
$env:K6_BASE_URL = "http://127.0.0.1:8080"
$env:K6_BASIC_DURATION = "30s"
$env:K6_VK_SECRET = "loadtest-secret"
$env:K6_VK_GROUP_ID = "0"
k6 run tests/k6/basic-api.js
```

Equivalent Docker run:

```powershell
docker run --rm -i `
  -e K6_BASE_URL=http://host.docker.internal:8080 `
  -e K6_BASIC_DURATION=30s `
  -e K6_VK_SECRET=loadtest-secret `
  -e K6_VK_GROUP_ID=0 `
  -v "${PWD}:/src:ro" `
  grafana/k6:latest run /src/tests/k6/basic-api.js
```

### Authentication Behavior

For `/miniapp/*`, the script sends `X-VK-User-ID` by default. This works only in
dev/loadtest mode when `VK_APP_SECRET` is empty. If you test against a contour
that enforces VK Mini App signatures, pass signed launch params instead:

```powershell
$env:K6_MINIAPP_LAUNCH_PARAMS = "<signed VK launch params>"
```

Do not commit real launch params or use production user data for load tests.

### Tunables

The first script is intentionally modest:

```text
K6_BASIC_DURATION=30s
K6_HEALTH_VUS=1
K6_VK_VUS=1
K6_MINIAPP_VUS=1
K6_SLEEP_SECONDS=1
```

Increase these only on a safe contour. Production is not a generic load-test
target.

### Success Criteria

The basic scenario is green when:

- `/health` and `/readyz` return 200;
- `POST /webhooks/vk` returns `ok`;
- `GET /miniapp/balance` returns `balance_credits`;
- `POST /miniapp/jobs` returns a job id with status 201;
- `GET /miniapp/jobs/{id}` returns the same job id;
- p95 request latency stays under the configured threshold;
- HTTP failures remain below the configured threshold.

## Chapter 4. VK Bot Load Scenario

### Goal

Chapter 4 adds a VK Bot-specific k6 scenario that exercises the same inbound
surface VK uses in production, but with synthetic users and load-test secrets:

```text
POST /webhooks/vk
```

The script covers:

- `/start`;
- `Показать меню`;
- callback buttons for menu, "Спросить у НейроХаб", account, video menu and
  back;
- ordinary text while the bot is in the text-dialog mode;
- duplicate event replay for idempotency;
- a burst of ordinary messages to exercise rate limit/cooldown paths.

The scenario does not call VK, paid providers or YooKassa. It only sends
synthetic callback bodies to the local/dev/load-test API.

### Script

The script lives at:

```text
tests/k6/vk-bot.js
```

Like the basic scenario, it is disabled by default and performs a no-op unless
`K6_BASE_URL` or `K6_RUN=1` is set.

### Local Run

Start the load-test contour first, then run:

```powershell
$env:K6_BASE_URL = "http://127.0.0.1:8080"
$env:K6_VK_SECRET = "loadtest-secret"
$env:K6_VK_GROUP_ID = "0"
$env:K6_VK_BOT_ITERATIONS = "1"
$env:K6_VK_RATE_LIMIT_EVENTS = "45"
k6 run tests/k6/vk-bot.js
```

Equivalent Docker run:

```powershell
docker run --rm -i `
  -e K6_BASE_URL=http://host.docker.internal:8080 `
  -e K6_VK_SECRET=loadtest-secret `
  -e K6_VK_GROUP_ID=0 `
  -e K6_VK_BOT_ITERATIONS=1 `
  -e K6_VK_RATE_LIMIT_EVENTS=45 `
  -v "${PWD}:/src:ro" `
  grafana/k6:latest run /src/tests/k6/vk-bot.js
```

### Callback Payloads

By default, the script sends the same command payload shape as VK inline
buttons:

```text
{"command":"show_menu"}
{"command":"menu.text"}
{"command":"account"}
{"command":"menu.video"}
```

If the command payload contract changes, override these without editing the
script:

```powershell
$env:K6_VK_PAYLOAD_SHOW_MENU = '{"command":"show_menu"}'
$env:K6_VK_PAYLOAD_ASK_NEUROHUB = '{"command":"menu.text"}'
$env:K6_VK_PAYLOAD_ACCOUNT = '{"command":"account"}'
$env:K6_VK_PAYLOAD_VIDEO_MENU = '{"command":"menu.video"}'
$env:K6_VK_PAYLOAD_BACK = '{"command":"show_menu"}'
```

### Rate Limit And Cooldown

The burst scenario uses one synthetic VK user and sends
`K6_VK_RATE_LIMIT_EVENTS` message events quickly. The webhook is expected to
return `200 ok` even when the anti-spam path denies normal processing and sends
a cooldown response through delivery. The cooldown text itself is observable in
mock delivery logs/metrics, not in the webhook HTTP response.

### Success Criteria

The VK bot scenario is green when:

- every synthetic VK webhook returns `200 ok`;
- duplicate `/start` and duplicate prompt events return `200 ok` without
  duplicate processing errors;
- callback payloads do not break menu/dialog state;
- ordinary text after "Спросить у НейроХаб" stays in the text-dialog path;
- burst traffic does not crash the API and remains idempotent/retry-safe;
- rate-limit/cooldown behavior is visible in service logs or metrics during
  manual load-test review.

## Chapter 5. Job Queue / Worker Load

### Goal

Chapter 5 adds a job/worker scenario that creates real backend Jobs through the
Mini App BFF and lets workers process them through mock providers:

```text
POST /miniapp/jobs
GET  /miniapp/jobs/{id}
```

It is designed to answer:

- how many jobs per second the API accepts;
- whether Redis job queues grow faster than workers drain them;
- how quickly workers complete text, image and video mock jobs;
- whether retries or DLQ entries appear;
- whether video jobs increase text job completion latency.

The script must run only against loadtest/dev/staging contours with mock
providers. It must not call paid provider APIs or production VK/YooKassa.

### Script

The script lives at:

```text
tests/k6/job-worker.js
```

It supports these workload modes:

```text
K6_JOB_WORKLOAD=text   # only text_generate jobs
K6_JOB_WORKLOAD=image  # only image_generate jobs
K6_JOB_WORKLOAD=video  # only video_generate jobs
K6_JOB_WORKLOAD=mixed  # weighted mix, default
K6_JOB_WORKLOAD=all    # text/image/video scenarios in parallel
```

### Local Run

Start the load-test contour first, including API and workers. Then run:

```powershell
$env:K6_BASE_URL = "http://127.0.0.1:8080"
$env:K6_JOB_WORKLOAD = "mixed"
$env:K6_JOB_DURATION = "30s"
$env:K6_JOB_MIXED_RATE = "2"
k6 run tests/k6/job-worker.js
```

Run text-only and video-only comparisons:

```powershell
$env:K6_JOB_WORKLOAD = "text"
$env:K6_JOB_TEXT_RATE = "5"
k6 run tests/k6/job-worker.js

$env:K6_JOB_WORKLOAD = "video"
$env:K6_JOB_VIDEO_RATE = "1"
$env:K6_JOB_VIDEO_ROUTE_ALIAS = "video_kling_o3_standard"
k6 run tests/k6/job-worker.js
```

Equivalent Docker run:

```powershell
docker run --rm -i `
  -e K6_BASE_URL=http://host.docker.internal:8080 `
  -e K6_JOB_WORKLOAD=mixed `
  -e K6_JOB_DURATION=30s `
  -e K6_JOB_MIXED_RATE=2 `
  -v "${PWD}:/src:ro" `
  grafana/k6:latest run /src/tests/k6/job-worker.js
```

### Tunables

The default run is intentionally small:

```text
K6_JOB_DURATION=30s
K6_JOB_MIXED_RATE=1
K6_JOB_TEXT_RATE=1
K6_JOB_IMAGE_RATE=1
K6_JOB_VIDEO_RATE=1
K6_JOB_POLL=true
K6_JOB_POLL_ATTEMPTS=10
K6_JOB_POLL_INTERVAL_SECONDS=0.5
K6_JOB_VIDEO_ROUTE_ALIAS=video_kling_o3_standard
K6_JOB_VIDEO_DURATION_SEC=5
```

For mixed traffic, adjust weights:

```text
K6_JOB_TEXT_WEIGHT=60
K6_JOB_IMAGE_WEIGHT=25
K6_JOB_VIDEO_WEIGHT=15
```

The script uses many synthetic users by default to avoid measuring only
per-user anti-spam limits. To deliberately test one-user backpressure:

```powershell
$env:K6_JOB_SAME_USER = "true"
```

### k6 Metrics

The script emits custom metrics:

| Metric | Meaning |
|---|---|
| `job_created_total` | Jobs accepted by the API |
| `job_terminal_total` | Jobs that reached terminal status during polling |
| `job_create_ok` | Create success rate |
| `job_poll_ok` | Polling success rate |
| `job_create_duration` | POST `/miniapp/jobs` latency |
| `job_completion_duration` | Time from create to terminal status |
| `job_pending_after_poll` | Jobs still non-terminal after polling attempts |

### What To Watch Outside k6

k6 measures API acceptance and observed completion. Queue growth and worker
health must be checked from the runtime:

```powershell
docker compose exec redis redis-cli XLEN stream:jobs:text
docker compose exec redis redis-cli XLEN stream:jobs:image
docker compose exec redis redis-cli XLEN stream:jobs:video
docker compose exec redis redis-cli XLEN stream:jobs:provider_poll
docker compose exec redis redis-cli XLEN stream:jobs:delivery
docker compose exec redis redis-cli XLEN stream:jobs:dlq
docker compose exec redis redis-cli XINFO GROUPS stream:jobs:text
docker compose exec redis redis-cli XPENDING stream:jobs:text workers
docker compose logs worker --tail=200
```

The default worker consumer group is `workers`. If the stream names differ in
the current Redis adapter, inspect keys first:

```powershell
docker compose exec redis redis-cli --scan --pattern "*job*"
docker compose exec redis redis-cli --scan --pattern "*dlq*"
```

Video load uses `video_route_alias`, not provider model IDs. If the API rejects
the route, enable a mock-safe video route in the isolated loadtest/dev contour.
Do not point this scenario at real paid video providers unless that is an
explicit paid capacity test.

Also check service metrics if the contour exposes private `/metrics` locally:

- job create/started/succeeded/failed counters;
- worker processing duration;
- queue depth/pending gauges;
- retry and DLQ counters;
- billing reserve/capture/release counters.

### Starvation Check

To check whether video jobs block text jobs:

1. Run text-only at a stable rate and record `job_completion_duration`.
2. Run `K6_JOB_WORKLOAD=all` with video enabled.
3. Compare text completion latency and pending jobs.

Text jobs should remain within the agreed budget even when video jobs are slow.
If text latency grows sharply, split worker pools, queue groups or concurrency
limits by modality before scaling traffic.

### Success Criteria

The job/worker scenario is green when:

- `POST /miniapp/jobs` returns 201 for accepted mock jobs;
- polling sees terminal statuses for most jobs within configured attempts;
- HTTP failures stay below the threshold;
- Redis queue depth does not grow without bound at the target rate;
- retries/DLQ do not grow during a healthy mock-provider run;
- video load does not starve text jobs;
- billing reservation/capture/release remains idempotent under repeated polling.

## Chapter 6. PostgreSQL And Indexes

### Goal

Chapter 6 adds a read-only database diagnostic pass around load tests. It is
used to identify Postgres bottlenecks before adding indexes or changing query
patterns.

The diagnostic pass watches:

- slow or frequently executed SQL;
- active and long-running queries;
- lock waits and blockers;
- table size growth and dead tuples;
- sequential scans on hot tables;
- index usage and large unused index candidates;
- migration/readiness state;
- retention cleanup candidates;
- daily analytics aggregate freshness.

### Script

The runner lives at:

```text
scripts/loadtest/postgres-diagnostics.ps1
```

The SQL is in:

```text
scripts/loadtest/postgres-diagnostics.sql
```

The script reads `DATABASE_URL` from `.env.loadtest` by default and does not
print it.

### Local Run

Capture a baseline before k6:

```powershell
.\scripts\loadtest\postgres-diagnostics.ps1 -EnvFile .env.loadtest -OutputFile .runtime/loadtest/postgres-before.txt
```

Run a k6 scenario:

```powershell
$env:K6_BASE_URL = "http://127.0.0.1:8080"
$env:K6_JOB_WORKLOAD = "mixed"
k6 run tests/k6/job-worker.js
```

Capture the after snapshot:

```powershell
.\scripts\loadtest\postgres-diagnostics.ps1 -EnvFile .env.loadtest -OutputFile .runtime/loadtest/postgres-after.txt
```

If local `psql` is not installed, run through the local Docker Postgres
container:

```powershell
.\scripts\loadtest\postgres-diagnostics.ps1 -EnvFile .env.loadtest -UseDockerCompose
```

Useful tunables:

```powershell
.\scripts\loadtest\postgres-diagnostics.ps1 `
  -EnvFile .env.loadtest `
  -Limit 50 `
  -LongQuerySeconds 10 `
  -MinTableMB 1
```

### pg_stat_statements

The script uses `pg_stat_statements` when the extension is available. If it is
not installed, the script prints a note and continues.

Enable it first on load/staging, not blindly on production. PostgreSQL requires
configuration such as:

```text
shared_preload_libraries = 'pg_stat_statements'
compute_query_id = on
pg_stat_statements.track = all
```

That change requires a Postgres restart.

### Interpreting Results

Do not automatically create indexes from this report. The report produces
candidates only.

For each candidate:

1. Verify the real query path in code.
2. Review the query plan.
3. Confirm the index is additive and does not conflict with existing indexes.
4. Add a migration intentionally.
5. On production-sized tables, prefer `CREATE INDEX CONCURRENTLY` in a
   dedicated rollout.

Sequential scans are not always bad. A small table or a highly selective
planner decision can make a sequential scan correct. Treat high `seq_tup_read`
on large/hot tables as the stronger signal.

Large unused indexes are also only candidates. An index may be unused in a
short test but still needed for rare operator, reconciliation or retention
queries.

### Privacy

The diagnostics truncate SQL samples, but SQL text can still contain sensitive
context if a bad query embeds literals. Do not paste raw diagnostic output into
public chats. Redact anything that could contain prompts, launch params,
provider payloads, tokens or PII.

### Success Criteria

Chapter 6 is useful when it produces a short review list:

- slow SQL to inspect;
- blocking queries to fix;
- growing tables that need retention or partition planning;
- missing-index candidates;
- unused-index candidates;
- retention/analytics queries that become heavy under load.

No schema change is part of this chapter by default. Schema/index changes are a
follow-up optimization step after the bottleneck is proven.

## Chapter 7. Redis And Backpressure

### Goal

Chapter 7 checks whether Redis queues and short-lived state protect the system
from one user, one job type or one slow worker pool filling the runtime faster
than workers can drain it.

It watches:

- Redis Streams used by workers;
- consumer group lag and pending entries;
- DLQ growth;
- anti-spam/rate-limit keys;
- VK dialog-mode keys;
- Redis memory, clients, stats and slowlog;
- whether video/image backlog can starve text jobs.

The chapter is diagnostic only. It does not change queue behavior or call real
providers.

### Script

The runner lives at:

```text
scripts/loadtest/redis-diagnostics.ps1
```

It reads Redis settings from `.env.loadtest` by default and uses read-only
Redis commands. It does not fetch key values or stream task payloads.

### Local Run

Capture a baseline before k6:

```powershell
.\scripts\loadtest\redis-diagnostics.ps1 `
  -EnvFile .env.loadtest `
  -OutputFile .runtime/loadtest/redis-before.txt
```

If local `redis-cli` is not installed, use the Redis container:

```powershell
.\scripts\loadtest\redis-diagnostics.ps1 `
  -EnvFile .env.loadtest `
  -UseDockerCompose `
  -OutputFile .runtime/loadtest/redis-before.txt
```

Run a job workload:

```powershell
$env:K6_BASE_URL = "http://127.0.0.1:8080"
$env:K6_JOB_WORKLOAD = "mixed"
k6 run tests/k6/job-worker.js
```

Capture the after snapshot:

```powershell
.\scripts\loadtest\redis-diagnostics.ps1 `
  -EnvFile .env.loadtest `
  -UseDockerCompose `
  -OutputFile .runtime/loadtest/redis-after.txt
```

Useful tunables:

```powershell
.\scripts\loadtest\redis-diagnostics.ps1 `
  -EnvFile .env.loadtest `
  -UseDockerCompose `
  -PendingIdleMs 60000 `
  -PendingLimit 50 `
  -ScanCount 200
```

### What It Checks

Default streams:

```text
stream:jobs:text
stream:jobs:image
stream:jobs:video
stream:jobs:delivery
stream:jobs:provider_poll
stream:jobs:dlq
```

Default consumer group:

```text
workers
```

For each worker stream the script records:

- `XLEN`;
- `XINFO STREAM`;
- `XINFO GROUPS`;
- `XPENDING` summary;
- idle pending entries above `LOADTEST_REDIS_PENDING_IDLE_MS`.

For sampled keys it records only metadata:

- key name;
- type;
- TTL;
- memory usage;
- collection length/cardinality.

It deliberately uses `SCAN`, not `KEYS`, and never reads values with `GET`.

### Backpressure Signals

Treat these as actionable signals:

- `stream:jobs:*` length grows continuously while the worker is healthy;
- `XINFO GROUPS` shows lag growing faster than it is drained;
- `XPENDING` grows or entries stay idle above the threshold;
- `stream:jobs:dlq` grows during a mock-provider run;
- one modality stream grows while others drain normally;
- `rate:vk:user:*` keys are created without TTL;
- `vk:peer:*:dialog_mode` keys are created without TTL;
- Redis memory grows without returning to baseline after the run.

One modality should not starve another. If mixed video load makes text jobs
slow, split worker pools/concurrency or enforce lower per-modality queue
thresholds before increasing traffic.

### Active Jobs

Active jobs per user are not stored as Redis counters in the current
architecture. They are checked through the job repository in Postgres:

- VK GPT active-job protection is configured by
  `VK_ANTISPAM_ACTIVE_GPT_JOB_LIMIT`;
- video active-job protection is configured by
  `MEDIA_MAX_ACTIVE_VIDEO_JOBS_PER_USER`;
- queue-wide pressure is guarded by `MEDIA_QUEUE_DEGRADE_THRESHOLD`.

Use Chapter 6 Postgres diagnostics together with this Redis snapshot to review
active-job SQL and queue pressure as one system.

### Privacy

Redis key names can still contain user identifiers. Keep diagnostics private
and redact samples before sharing externally. Do not paste stream entries,
provider payloads, prompts, full launch params or tokens into chats/docs.

### Success Criteria

Chapter 7 is green when:

- worker streams do not grow without bound at the target rate;
- pending entries are acknowledged or reclaimed;
- DLQ stays flat during healthy mock-provider load;
- rate-limit and dialog-state keys have TTLs;
- one user or one modality cannot keep all worker capacity occupied forever;
- Redis memory returns near baseline after the run;
- any backpressure issue produces a concrete follow-up: limit, worker split,
  queue threshold, retry/DLQ fix or SQL/index review.

## Chapter 8. Billing / Payments Mock Load

### Goal

Chapter 8 checks payment and ledger behavior under synthetic load without
YooKassa:

```text
POST /miniapp/payments/intents
GET  /miniapp/payments
GET  /billing/payment-intents/{id}
POST /billing/payment-intents/{id}/mock-status
POST /billing/payment-intents/{id}/refund
```

It verifies:

- payment intent creation through the user-facing Mini App BFF;
- payment history reads;
- mock provider completion through the normal provider-verified sync path;
- ledger top-up idempotency;
- duplicate status replay safety;
- mock refund idempotency;
- optional mock webhook replay when `cmd/provider-webhook` is running.

This is not a YooKassa smoke. It must run only with:

```text
APP_ENV=loadtest
PAYMENT_PROVIDER=mock
```

### Protected Load-Test Endpoint

The scenario uses a protected endpoint:

```text
POST /billing/payment-intents/{id}/mock-status
```

The endpoint:

- requires `X-Admin-Token`;
- is enabled only when API wiring detects `APP_ENV=loadtest` and
  `PAYMENT_PROVIDER=mock`;
- returns `404` outside that gate;
- updates only the mock provider state;
- then calls the same `SyncIntent` path as webhook/reconciliation;
- never mutates balance directly.

This lets load tests prove the real ledger path without creating a fake
production-only shortcut.

Request body:

```json
{
  "status": "succeeded",
  "reason": "loadtest mock payment completion"
}
```

`X-Idempotency-Key` is still required to keep the operator-action shape
consistent, even though the endpoint itself relies on payment/ledger
idempotency for safe replay.

### Script

The k6 script lives at:

```text
tests/k6/billing-payments.js
```

It is disabled by default and performs a no-op unless `K6_BASE_URL` or
`K6_RUN=1` is set.

### Local Run

Start an isolated load-test contour with API, worker, data services and mock
payments. Then run:

```powershell
$env:K6_BASE_URL = "http://127.0.0.1:8080"
$env:K6_ADMIN_TOKEN = "loadtest-admin-token"
$env:K6_PAYMENT_PRODUCT_CODE = "crystals_99"
$env:K6_PAYMENT_RATE = "1"
k6 run tests/k6/billing-payments.js
```

Equivalent Docker run:

```powershell
docker run --rm -i `
  -e K6_BASE_URL=http://host.docker.internal:8080 `
  -e K6_ADMIN_TOKEN=loadtest-admin-token `
  -e K6_PAYMENT_PRODUCT_CODE=crystals_99 `
  -e K6_PAYMENT_RATE=1 `
  -v "${PWD}:/src:ro" `
  grafana/k6:latest run /src/tests/k6/billing-payments.js
```

If `cmd/provider-webhook` is also running with `PAYMENT_PROVIDER=mock`, enable
optional webhook replay checks:

```powershell
$env:K6_PAYMENT_WEBHOOK_BASE_URL = "http://127.0.0.1:8082"
k6 run tests/k6/billing-payments.js
```

### Tunables

```text
K6_PAYMENT_DURATION=30s
K6_PAYMENT_RATE=1
K6_PAYMENT_PREALLOCATED_VUS=5
K6_PAYMENT_MAX_VUS=20
K6_PAYMENT_USER_BASE=840000000
K6_PAYMENT_SAME_USER=false
K6_PAYMENT_PRODUCT_CODE=crystals_99
K6_PAYMENT_FORCE_NEW=true
K6_PAYMENT_WEBHOOK_BASE_URL=
```

Use unique users by default. Set `K6_PAYMENT_SAME_USER=true` only when you
want to measure same-user payment contention/idempotency behavior.

### k6 Metrics

The script emits:

| Metric | Meaning |
|---|---|
| `payment_intent_created_total` | User-facing payment intents accepted by Mini App BFF |
| `payment_history_ok` | Payment history success rate |
| `payment_mock_topup_ok` | Mock status -> SyncIntent -> ledger top-up success rate |
| `payment_refund_ok` | Manual mock refund success rate |
| `payment_idempotency_ok` | Replay safety for create/top-up/refund steps |
| `payment_mock_journey_duration` | Full create -> top-up -> refund journey duration |

### What To Watch Outside k6

Use operator endpoints and SQL diagnostics to confirm:

- `payment_intents.status` reaches `succeeded`, then `refunded`;
- `ledger_entries` gets one top-up entry per paid intent;
- replayed mock-status does not create a second top-up;
- refund replay returns the same refund row;
- `payment_events` remains clean unless optional webhook replay is enabled;
- payment history DTOs do not expose raw provider payloads.

Useful protected endpoints:

```text
GET /billing/payment-history?provider=mock
GET /billing/payment-events/unprocessed?provider=mock
GET /billing/payment-intents/pending?provider=mock
```

### Success Criteria

Chapter 8 is green when:

- `POST /miniapp/payments/intents` returns 200/201 and is idempotent;
- `GET /miniapp/payments` remains healthy under the target load;
- protected mock-status returns 200 only in `APP_ENV=loadtest`;
- repeated mock-status does not double-credit the ledger;
- repeated refund does not double-debit the ledger;
- optional webhook replay is accepted without duplicate processing;
- all payment/admin DTOs stay sanitized and do not expose raw provider payloads.

## Chapter 9. Metrics And Report

Goal: collect one operator-readable report after a load run and make the first
bottleneck visible without manually stitching together k6, Redis, Postgres and
Docker outputs.

### Report Runner

The report runner lives at:

```text
scripts/loadtest/loadtest-report.ps1
```

Default output goes to:

```text
reports/loadtest/<timestamp>/
```

The generated files are ignored by git and are not meant to be committed.

### Run With Fresh k6 Scenarios

Start the isolated load-test contour first. Then run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File scripts/loadtest/loadtest-report.ps1 `
  -EnvFile .env.loadtest `
  -RunK6 `
  -UseDockerCompose
```

If Redis/Postgres are external and local CLIs are installed, omit
`-UseDockerCompose`.

### Run From Existing k6 Summaries

If k6 was already executed separately, pass summary exports:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File scripts/loadtest/loadtest-report.ps1 `
  -EnvFile .env.loadtest `
  -K6SummaryFiles reports/loadtest/run/basic-api.summary.json,reports/loadtest/run/job-worker.summary.json
```

### What The Report Contains

| Area | Collected metrics |
|---|---|
| HTTP/API | RPS, p95 latency, p99 latency, error rate |
| Worker/jobs | job create rate, terminal job throughput from k6 custom metrics |
| Redis | queue depth per stream, DLQ depth, memory, ops/sec, expired keys |
| Postgres | long queries, blocking locks, table sizes, index/seq scan hints |
| Runtime | container CPU/RAM, network I/O, block I/O from Docker stats |

The report also writes a machine-readable summary:

```text
loadtest-summary.json
```

### Thresholds

The runner uses these env values:

```text
LOADTEST_REPORT_MAX_ERROR_RATE=0.05
LOADTEST_REPORT_MAX_P95_MS=1500
LOADTEST_REPORT_MAX_QUEUE_DEPTH=100
LOADTEST_REPORT_MAX_DLQ=0
ALLOW_PRODUCTION_LOADTEST_REPORT=false
```

If a threshold is exceeded, the report lists a finding and promotes the first
finding as the initial bottleneck candidate.

### Safety

- The report refuses `APP_ENV=production` unless explicitly allowed.
- It does not call real AI providers.
- It does not print secrets, raw prompts, VK launch params, provider payloads
  or private artifact URLs.
- It reads operational metrics only.

### Success Criteria

Chapter 9 is green when one report clearly shows:

- RPS;
- p95/p99 latency;
- error rate;
- queue depth;
- worker throughput;
- Postgres slow-query/lock diagnostics;
- Redis memory/ops;
- DLQ count;
- CPU/RAM;
- the first likely bottleneck.

## Chapter 10. Decisions After A Load Run

Goal: turn a load-test report into concrete engineering decisions instead of
guessing which service to scale or rewrite.

### Required Inputs

Use the latest generated report directory:

```text
reports/loadtest/<timestamp>/
```

The decision pass must read at least:

```text
loadtest-summary.json
loadtest-report.md
postgres-diagnostics.md
redis-diagnostics.md
```

Do not use production traffic or provider spend to make Chapter 10 decisions.
If a report was produced against production or real paid providers by mistake,
discard it for capacity planning and rerun against `APP_ENV=loadtest`.

### Decision Areas

For every report, explicitly decide:

| Area | Decision To Record |
|---|---|
| Bottleneck | Which component failed first: API, worker, Redis queue, Postgres, S3, provider, delivery or test data/config |
| Indexes | Whether a slow query/index change is justified by data, or whether more diagnostics are needed |
| Workers | Whether to add worker instances, split worker pools, or keep one worker |
| Queues | Whether text/image/video/delivery queues need separate thresholds or workers |
| Data services | Whether Postgres/Redis/S3 must move out of the app VPS now or can wait |
| User limits | Whether anti-spam, active-job and per-modality limits need tightening |
| Provider quotas | Whether provider-side quotas are the actual bottleneck, if real providers were intentionally tested |

### Default Decision Rules

- If HTTP p95/p99 is low and CPU/RAM are low, do not scale API first.
- If Redis stream depth grows while DLQ stays zero, inspect worker throughput,
  delivery throughput and queue consumers before scaling Redis.
- If `pg_stat_statements` is missing, do not guess slow SQL from table sizes
  alone. Enable it on load/staging and rerun before adding broad indexes.
- If only one job modality fails, treat it as a route/config/provider-path
  issue before treating the whole worker as overloaded.
- If delivery stream grows, consider a dedicated delivery worker/pool before
  increasing generation workers.
- If synthetic users hit anti-spam, separate anti-spam validation from
  throughput validation; both are useful but answer different questions.
- Moving Postgres/Redis/S3 to managed/external services is mandatory before
  serious horizontal scaling, but it should be scheduled from measured pressure,
  not done as a reflex after a small local run.

### Output

Write a decision note next to the report:

```text
reports/loadtest/<timestamp>/loadtest-decisions.md
```

The note must include:

- verdict;
- first bottleneck;
- immediate fixes before the next run;
- scaling decision;
- index decision;
- queue/backpressure decision;
- data-services decision;
- user-limit decision;
- next load-test run plan.

Chapter 10 is complete when the next engineering action is clear and specific.
It does not mean the system is ready for production-scale traffic.
