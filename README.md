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
    queue/redis/     Redis Streams publisher/consumer (consumer groups)
    storage/postgres pgx repositories
    storage/s3/      S3/MinIO object store
    storage/memory/  in-memory repositories (tests/local)
  platform/          queue + unit-of-work contracts
migrations/          SQL migrations
```

## Running locally

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
curl "http://localhost:8081/admin/jobs?status=succeeded&limit=20" -H "X-Admin-Token: $ADMIN_TOKEN"
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

**Provider was called outside a worker.**
That violates a core invariant. Providers must only be invoked from
`internal/worker`. VK inbound and the orchestrator must never import a provider.

**MinIO `connectivity check` fails on startup.**
Confirm MinIO is up (`docker compose ps`), the endpoint is host:port without a
scheme, and the access/secret keys match the compose configuration.
