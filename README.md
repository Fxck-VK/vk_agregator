# VK AI Aggregator

Go backend for an **AI Job Processing Platform** that accepts requests from VK,
turns them into billable Jobs, runs them through AI providers, stores the
results as Artifacts and delivers them back to VK — with credit accounting,
idempotency and retry-safe workers throughout.

This is **not** a chatbot: every user request becomes a persisted `Job` that
moves through an explicit state machine. The architecture source of truth is
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md); invariants live in
[`AGENTS.md`](AGENTS.md); the build log is in [`PROGRESS.md`](PROGRESS.md) and
the backlog in [`TASKS.md`](TASKS.md).

## Current status

Current release: **v0.1.3 / Beta integrations foundation**.

The default local runtime is fully runnable with:

- PostgreSQL, Redis and MinIO from `docker-compose.yml`;
- mock AI provider;
- mock VK delivery client;
- in-memory E2E tests for the full VK -> Job -> Provider -> Artifact -> Delivery -> Capture flow.

Production-shaped hardening already exists: transactional outbox relay, atomic
job+reserve+outbox, output moderation, retry budget, DLQ, SSRF protection,
webhook rate limiting, Prometheus metrics, OpenTelemetry trace propagation,
migration checksums, S3 retention, signed artifact URL support, maintenance
cleanup and billing reconciliation metrics.
API metrics are served at `GET /metrics`; worker-local metrics are served at
`WORKER_METRICS_ADDR` (default `:9090`).

Real integrations are implemented at adapter level and remain **opt-in**:

- `PROVIDER=openai` enables OpenAI text (`Responses`), image (`Images`) and
  async video (`Videos`) generation.
- `PROVIDER_CHAIN=openai,mock` enables the provider router with
  health/circuit-breaker, fallback, cost and observed-latency aware selection.
- `VK_DELIVERY_MODE=real` enables VK `messages.send` plus raw photo/video
  artifact upload to VK upload servers before send.
- `cmd/api` can send the VK `/start` Super GPT menu and inline keyboard through
  the VK delivery adapter when `VK_ACCESS_TOKEN` is configured. The optional
  `VK_WELCOME_ATTACHMENT` env attaches a pre-uploaded VK banner.
- The VK `Создать видео` menu button opens a model picker (`Sora 2`,
  `Kling v2.1`, `Seedance 1`, `Haiuo v0.2`) with a `Назад` control; model
  buttons are control-only until model-specific generation state is wired.
- VK menu screens are described through a small declarative registry. `Создать
  фото` skips model selection when only one main image model is available and
  opens the text/reference photo instruction screen directly; `Спросить у GPT`
  opens the active GPT prompt screen.
- `MODERATION_PROVIDER=openai` enables OpenAI output moderation.
- `ARTIFACT_SCANNER=openai` enables OpenAI text/image artifact scanning before
  storage. Video scanning/transcoding is still part of the future media
  pipeline.

Credential-bound live smoke with real OpenAI/VK accounts is still required
before calling this production-ready. The default runtime remains mock-backed.

## End-to-end flow

```
VK webhook ─► InboundEvent ─► User ─► Command ─► Job (queued)
                                                   │
                                          Redis Streams (per modality)
                                                   │
                                     Generation Worker ──► Provider (Submit/Poll)
                                                   │            │
                                          (async) Provider Poll Worker
                                                   │
                                              Artifact (S3/MinIO)
                                                   │
                                          stream:jobs:delivery
                                                   │
                                       Delivery Worker ──► VK send (random_id)
                                                   │
                                          Billing Capture ──► Job: succeeded
```

- **VK never calls a provider.** Inbound only normalizes events into Jobs.
- **Providers are called only inside workers.**
- **Billing is an append-only ledger:** reserve on intake, capture on delivery,
  refund/release on failure. Balance is never mutated without a ledger entry.
- **Everything is idempotent:** inbound events, commands, jobs, provider submits,
  artifacts (sha256), deliveries (VK `random_id`) and ledger entries all carry
  idempotency keys.

## Layout

```
cmd/                 entrypoints (api, vk-inbound, worker, provider-webhook, admin-api, migrate)
internal/
  domain/            entities, state machines, repository interfaces (no infra)
  service/           billing, joborchestrator, commandrouter, artifactservice
  worker/            generation, provider-poll and delivery workers + engine
  adapter/
    inbound/vk/      VK Callback API webhook
    inbound/admin/   read-only admin HTTP API
    delivery/vk/     outbound VK client (+ mock)
    provider/mock/   mock AI provider
    provider/openai/ OpenAI generation/moderation/scanning adapters
    queue/redis/     Redis Streams publisher/consumer (consumer groups)
    storage/postgres pgx repositories
    storage/s3/      S3/MinIO object store
    storage/memory/  in-memory repositories (tests/local)
  platform/          queue + unit-of-work contracts
migrations/          SQL migrations
```

## Running locally

Create a local environment file from the committed template:

```bash
cp .env.example .env
# edit .env and fill VK_ACCESS_TOKEN / VK_SECRET / VK_CONFIRMATION_TOKEN if needed
```

On Windows PowerShell:

```powershell
Copy-Item .env.example .env
notepad .env
```

The application loads `.env` automatically when started from the repository
root. The real `.env` is ignored by Git; only `.env.example` is committed.

Start the infrastructure (PostgreSQL, Redis, MinIO):

```bash
docker compose up -d
```

| Service     | Address                | Credentials                 |
|-------------|------------------------|-----------------------------|
| PostgreSQL  | `localhost:5432`       | `vk_ai_aggregator` / same   |
| Redis       | `localhost:6379`       | —                           |
| MinIO (S3)  | `localhost:9000`       | `minioadmin` / `minioadmin` |
| MinIO web   | `localhost:9001`       | `minioadmin` / `minioadmin` |

Apply migrations:

```bash
go run ./cmd/migrate up
```

Start the API (VK webhook, admin API, health endpoint) — listens on `:8080`:

```bash
go run ./cmd/api
```

Start the workers (generation, provider poll, delivery) in a second terminal:

```bash
go run ./cmd/worker
```

Check health:

```bash
curl localhost:8080/health   # {"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
```

See `TESTING.md` for full runtime validation, curl examples and expected
results.

Real adapter modes are opt-in:

```bash
# OpenAI text/image/video generation; requires a real key.
PROVIDER=openai OPENAI_API_KEY=... go run ./cmd/worker

# OpenAI primary with mock fallback through the provider router.
PROVIDER_CHAIN=openai,mock OPENAI_API_KEY=... go run ./cmd/worker

# Real VK messages.send + photo/video upload; requires a real token.
VK_DELIVERY_MODE=real VK_ACCESS_TOKEN=... go run ./cmd/worker

# Real VK /start menu replies from the API; requires a real token.
VK_ACCESS_TOKEN=... go run ./cmd/api

# Real output moderation and text/image artifact scanner.
MODERATION_PROVIDER=openai ARTIFACT_SCANNER=openai OPENAI_API_KEY=... go run ./cmd/worker
```

For production, set `APP_ENV=production` and configure non-default
`VK_SECRET`, `ADMIN_TOKEN` and `VK_CONFIRMATION_TOKEN`. Both `cmd/api` and
`cmd/worker` run fail-closed config validation; `PROVIDER=openai` requires
`OPENAI_API_KEY`, and `VK_DELIVERY_MODE=real` requires `VK_ACCESS_TOKEN` in any
environment.
`PROVIDER_CHAIN`, `MODERATION_PROVIDER=openai` and `ARTIFACT_SCANNER=openai`
also require `OPENAI_API_KEY`.
For a VK welcome banner, set `VK_WELCOME_ATTACHMENT` to a pre-uploaded
attachment string such as `photo-239332376_123_accesskey`.

## Admin API

Read-only operator endpoints (DTO responses, pagination, filters). If an admin
token is configured, send it via the `X-Admin-Token` header.

| Endpoint                      | Description                                   |
|-------------------------------|-----------------------------------------------|
| `GET /admin/jobs`             | List jobs. Query: `status`, `user_id`, `operation`, `limit` (≤100), `offset` |
| `GET /admin/jobs/{id}`        | Single job                                    |
| `GET /admin/users/{id}`       | Single user (includes credit balance)         |
| `GET /admin/deliveries/{id}`  | Single delivery attempt                       |

List responses are wrapped as `{ "items": [...], "pagination": { "limit", "offset", "count", "has_more" } }`.

Example:

```bash
curl "http://localhost:8080/admin/jobs?status=succeeded&limit=20" -H "X-Admin-Token: $ADMIN_TOKEN"
```

## Testing

```bash
go test ./...          # unit + in-memory E2E; external infra is skipped
gofmt -l .             # formatting check (should print nothing)
go vet ./...
```

Integration tests are environment-guarded so the default run is green without
infrastructure:

```bash
# PostgreSQL repository integration tests
TEST_DATABASE_URL="postgres://vk_ai_aggregator:vk_ai_aggregator@localhost:5432/vk_ai_aggregator?sslmode=disable" go test ./internal/adapter/storage/postgres/...

# Redis Streams integration tests
TEST_REDIS_ADDR=localhost:6379 go test ./internal/adapter/queue/redis/...
```

The full pipeline is covered by an in-memory end-to-end test:
`internal/worker/e2e_test.go` (`TestEndToEnd`) drives VK → Job → Provider →
Artifact → Delivery → Capture without any external services.

## Troubleshooting

**`go test ./...` shows packages as `[no test files]`.**
Expected for `domain`, `platform/*`, `storage/memory` and `storage/s3` — they
hold types/adapters exercised indirectly by other packages' tests.

**Postgres or Redis integration tests are skipped.**
They only run when `TEST_DATABASE_URL` / `TEST_REDIS_ADDR` are set. Start the
stack with `docker compose up -d` and export the variables shown above.

**`missing go.sum entry` after pulling new code.**
Run `go mod download` (or `go mod tidy`) to fetch new dependencies.

**A job is stuck in `queued` / `provider_processing`.**
Nothing is consuming the stream, or the poll worker is not running. Ensure the
worker process is up and that consumer groups exist (the worker creates them on
start). On restart, the engine reclaims un-acked entries via `XAUTOCLAIM`, so
in-flight jobs resume automatically.

**A job reached `awaiting_payment`.**
The user's available balance is below the operation price. New accounts start
with 1000 test credits; check the balance via `GET /admin/users/{id}`.

**A job is `failed_retryable` but never progresses.**
Retryable provider errors (`rate_limited`, `provider_timeout`,
`provider_overloaded`, `provider_internal_error`) re-queue up to 3 attempts,
then become `failed_terminal`. Inspect `error_code` on the job DTO.

**VK shows duplicate messages.**
It shouldn't: deliveries use a deterministic `random_id` derived from the
delivery idempotency key, and VK deduplicates repeats. If you see duplicates,
verify the delivery worker is not generating a fresh `random_id` per attempt.

**`/start` sends text but no keyboard.**
Enable bot features in the VK community message settings. VK returns
`error_code=912` when keyboards are disabled; the API falls back to welcome text
without keyboard so the callback still succeeds.

**Provider was called outside a worker.**
That violates a core invariant. Providers must only be invoked from
`internal/worker`. VK inbound and the orchestrator must never import a provider.

**MinIO `connectivity check` fails on startup.**
Confirm MinIO is up (`docker compose ps`), the endpoint is host:port without a
scheme, and the access/secret keys match the compose configuration.
