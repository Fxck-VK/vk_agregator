# RUNBOOK — VK AI Aggregator

Operational runbook to bring the project up from zero. A new developer should be
able to follow this top to bottom without extra help.

---

## 1. System Requirements

| Tool | Version | Check |
|------|---------|-------|
| Go | **1.25+** (module targets `go 1.25.0`) | `go version` |
| Docker Engine | **24+** | `docker version` |
| Docker Compose | **v2+** (`docker compose`, not `docker-compose`) | `docker compose version` |
| Git | any recent | `git --version` |

OS: Linux/macOS/Windows. On Windows use Docker Desktop with the engine running.

---

## 2. Installation

```bash
# Clone
git clone <repo-url> vk_agregator
cd vk_agregator

# Dependencies (Go modules)
go mod download

# Build everything (sanity)
go build ./...
```

### Environment configuration

All variables have local-dev defaults (`internal/platform/config/config.go`), so
no `.env` is required for local runs. Override via environment when needed:

| Var | Default | Purpose |
|-----|---------|---------|
| `HTTP_ADDR` | `:8080` | API listen address |
| `DATABASE_URL` | `postgres://vk_ai_aggregator:vk_ai_aggregator@localhost:5432/vk_ai_aggregator?sslmode=disable` | Postgres DSN |
| `MIGRATIONS_DIR` | `migrations` | Migration files dir |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` / `REDIS_DB` | `` / `0` | Redis auth/db |
| `S3_ENDPOINT` | `localhost:9000` | MinIO/S3 endpoint (host:port, no scheme) |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | `minioadmin` / `minioadmin` | Object store creds |
| `S3_BUCKET` | `artifacts` | Artifact bucket (auto-created) |
| `S3_USE_SSL` | `false` | HTTPS to S3 |
| `VK_CONFIRMATION_TOKEN` | `dev-confirmation` | Returned for VK `confirmation` |
| `VK_SECRET` | `` (empty = no check) | VK callback secret |
| `ADMIN_TOKEN` | `` (empty = open) | Admin API `X-Admin-Token` |
| `WORKER_GROUP` / `WORKER_CONSUMER` | `workers` / hostname | Consumer group identity |
| `APP_ENV` | `development` | `production` enforces fail-closed secrets |
| `MAX_ATTEMPTS` | `3` | Retry budget before dead-lettering |
| `RETRY_BASE_DELAY` / `RETRY_MAX_DELAY` | `500ms` / `30s` | Exponential backoff bounds |
| `MODERATION_EXTRA_TERMS` | `` | Comma-separated extra blocklist terms |
| `WEBHOOK_RATE_LIMIT_RPS` / `WEBHOOK_RATE_LIMIT_BURST` | `20` / `40` | Per-IP webhook rate limit |

> Production note: set `APP_ENV=production`; the API then **refuses to start**
> unless `VK_SECRET`, `ADMIN_TOKEN`, and a non-default `VK_CONFIRMATION_TOKEN`
> are set (fail-closed, `AUDIT.md` S1).

### Hardening features (post-release)

- **Output moderation** gates delivery: a blocked prompt sets the job to
  `rejected`, releases the reservation (no capture), and writes a
  `moderation_results` audit row (migration `000003`). Default classifier is
  keyword-based (`MODERATION_EXTRA_TERMS` extends it).
- **DLQ + retry budget**: exhausted/poison tasks go to `stream:jobs:dlq`
  (not auto-consumed) and the job becomes `failed_terminal`. Inspect with
  `docker exec vk-ai-aggregator-redis redis-cli XLEN stream:jobs:dlq`.
- **Metrics**: `GET /metrics` (Prometheus). Note metrics are process-local —
  scrape the API and each worker separately.
- **SSRF**: artifact downloader blocks private/loopback/link-local hosts and
  non-http(s) schemes; optional host allowlist.
- **Rate limit**: per-IP token bucket on `/webhooks/vk` (429 when exceeded).

---

## 3. Infrastructure Startup

```bash
docker compose up -d
docker compose ps          # wait until postgres & redis are healthy; minio Up
```

Services started by `docker-compose.yml`:

| Service | Port | Notes |
|---------|------|-------|
| postgres | `5432` | healthcheck `pg_isready` |
| redis | `6379` | healthcheck `redis-cli ping` |
| minio | `9000` (API), `9001` (console) | console login `minioadmin`/`minioadmin` |

Expected `docker compose ps`:
```
postgres   Up (healthy)
redis      Up (healthy)
minio      Up
```

---

## 4. Migrations

```bash
go run ./cmd/migrate up        # apply all pending
go run ./cmd/migrate status    # verify
```

Expected:
```
applied 000001_init_schema
applied 000002_inbound_events
up complete: 2 migration(s) applied
```
`status` should list every migration as `applied`.

Rollback one step: `go run ./cmd/migrate down`.

Verify in DB:
```bash
docker exec vk-ai-aggregator-postgres psql -U vk_ai_aggregator -d vk_ai_aggregator -c "\dt"
```

---

## 5. API Startup

```bash
go run ./cmd/api               # listens on :8080
```

Serves: `POST /webhooks/vk`, `GET /admin/...`, `GET /health`.

Verify (new terminal):
```bash
curl -s localhost:8080/health
# {"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
```

---

## 6. Worker Startup

A single binary runs **all** worker pools; start it once:

```bash
go run ./cmd/worker
```

It runs these pools over Redis Streams (one consumer group, recovery via `XAUTOCLAIM`):

| Logical worker | Stream consumed | Role |
|----------------|-----------------|------|
| text worker | `stream:jobs:text` | text_generate |
| image worker | `stream:jobs:image` | image_generate / edit |
| video worker | `stream:jobs:video` | video_generate |
| polling worker | `stream:jobs:provider_poll` | poll async provider tasks |
| delivery worker | `stream:jobs:delivery` | Artifact → Delivery → Capture → succeeded |

> Scaling note: run multiple `cmd/worker` instances (each joins the same group) for more throughput. Per-pool isolation is a Phase-2 item (`AUDIT.md` SC2). The worker auto-creates the MinIO bucket and consumer groups on start.

Expected log:
```
{"level":"INFO","msg":"workers started","group":"workers","consumer":"<host>"}
```

---

## 7. Health Checks

| Endpoint | Expected |
|----------|----------|
| `GET /health` | `200` `{"status":"ok","checks":{"postgres":"ok","redis":"ok"}}` |
| `GET /healthz` | same (alias) |
| `GET /metrics` | `200` Prometheus exposition (`vkagg_*` + Go/process) |

`503 {"status":"degraded",...}` means Postgres or Redis is unreachable — see Troubleshooting.

Infra liveness:
```bash
docker compose ps
docker exec vk-ai-aggregator-redis redis-cli ping        # PONG
docker exec vk-ai-aggregator-postgres pg_isready -U vk_ai_aggregator
```

---

## 8. Local Testing

### VK confirmation
```bash
curl -s -X POST localhost:8080/webhooks/vk \
  -H 'Content-Type: application/json' \
  -d '{"type":"confirmation","group_id":1}'
# -> dev-confirmation
```

### VK message → full pipeline
```bash
# text
curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"evt-1","object":{"message":{"from_id":777,"peer_id":777,"text":"hello world"}}}'
# image
curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"evt-2","object":{"message":{"from_id":777,"peer_id":777,"text":"/image a red cat"}}}'
# video
curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"evt-3","object":{"message":{"from_id":777,"peer_id":777,"text":"/video a flying car"}}}'
# each -> ok
```

### Expected result (within ~seconds)
Query the admin API or DB:
```bash
curl -s 'localhost:8080/admin/jobs?limit=10'

docker exec vk-ai-aggregator-postgres psql -U vk_ai_aggregator -d vk_ai_aggregator -At -F' | ' \
  -c "SELECT operation_type, status, cost_reserved, cost_captured FROM jobs ORDER BY created_at DESC LIMIT 3;"
# text_generate  | succeeded | 1  | 1
# image_generate | succeeded | 10 | 10
# video_generate | succeeded | 50 | 50
```
A succeeded job has: an output artifact (in MinIO), a `deliveries` row with status `sent`, and a `capture` ledger entry.

Idempotency: re-POST the same `event_id` → `ok`, no new job/charge/send.

Failure injection (mock): include `mock_timeout`, `mock_rate_limit`, or `mock_provider_error` in the text to simulate retryable/terminal provider errors.

### Automated (no infra)
```bash
go test ./...                                    # full suite + in-memory E2E
go test ./internal/worker/ -run TestEndToEnd -v  # full VK→…→Capture
```

---

## 9. Troubleshooting

**PostgreSQL**
- `/health` 503 `postgres: down` or `migrate: connect`: container not ready → `docker compose ps`; wait for `(healthy)`; check `DATABASE_URL`.
- Inspect: `docker exec vk-ai-aggregator-postgres psql -U vk_ai_aggregator -d vk_ai_aggregator -c "select 1"`.
- Logs: `docker compose logs postgres`.

**Redis**
- `/health` 503 `redis: down`: `docker exec vk-ai-aggregator-redis redis-cli ping` should return `PONG`; check `REDIS_ADDR`.
- Stream depth: `docker exec vk-ai-aggregator-redis redis-cli XLEN stream:jobs:text`.
- Pending/consumers: `redis-cli XINFO GROUPS stream:jobs:text`.

**MinIO**
- Worker fails at `s3 connect`/`ensure bucket`: confirm MinIO is up, `S3_ENDPOINT` is `host:port` (no scheme), creds match compose.
- Console: http://localhost:9001 (`minioadmin`/`minioadmin`); objects live under `artifacts/`.

**Workers**
- Jobs stuck in `queued`: worker not running or wrong `REDIS_ADDR` → start `cmd/worker`; check log `workers started`.
- Job `failed_terminal`: inspect `error_code` on the job (`/admin/jobs/{id}`); for mock, check for trigger keywords in the prompt.
- After crash: pending entries are auto-reclaimed via `XAUTOCLAIM` on next start.

**Migrations**
- Partial/failed migration: re-run `go run ./cmd/migrate status`; a file failing mid-way may need manual cleanup (runner is not yet per-file transactional — `AUDIT.md` D1). Fix DDL, then re-run `up`.
- Reset local DB (DESTRUCTIVE): `docker compose down -v && docker compose up -d` then re-run migrations.

**Queue**
- Stream length keeps growing with no progress: indicates a retry loop — check worker logs for `handler failed`; inspect the offending job's `error_code`. (Hard retry budget/DLQ is Phase-2, `AUDIT.md` R1/Q1.)
- Drain check: `redis-cli XLEN <stream>` stable over time = no loop.

---

## 10. Backup & Recovery

**PostgreSQL (source of truth)**
```bash
# Backup
docker exec vk-ai-aggregator-postgres pg_dump -U vk_ai_aggregator vk_ai_aggregator > backup_$(date +%F).sql
# Restore
cat backup_YYYY-MM-DD.sql | docker exec -i vk-ai-aggregator-postgres psql -U vk_ai_aggregator -d vk_ai_aggregator
```

**MinIO (artifacts)**
- Back up the `artifacts` bucket with `mc mirror local/artifacts <dest>` (or volume snapshot). Artifacts are content-addressed (sha256), so re-stores are idempotent.

**Redis (queues/cache)**
- Treated as ephemeral: it holds in-flight stream entries, not the source of truth. On loss, in-flight jobs are recoverable from Postgres state (re-enqueue from job status). Enable AOF/RDB persistence for smoother restarts (compose already mounts a data volume).

**Recovery principle**
- Postgres = durable truth; MinIO = durable artifacts; Redis = rebuildable. Restore Postgres + MinIO; queues self-heal as workers reprocess from persisted job/provider-task state.

---

## 11. Deployment Order

Start in dependency order; stop in reverse:

1. **Infrastructure**: Postgres → Redis → MinIO (wait for healthy).
2. **Migrations**: `migrate up` (must complete before app starts).
3. **API**: `cmd/api` (verify `/health` = 200).
4. **Workers**: `cmd/worker` (verify `workers started`; consumer groups auto-created).
5. **Smoke test**: send a `message_new` webhook; confirm job `succeeded`.

Shutdown order: Workers → API → (optionally) Infrastructure.

---

## 12. Rollback Procedure

**Application (api/worker)**
1. Redeploy the previous image/build (binaries are stateless).
2. If the new release added a migration, roll it back **before** starting the old binary:
   ```bash
   go run ./cmd/migrate down     # rolls back the most recent migration
   go run ./cmd/migrate status
   ```
3. Restart API then workers (Deployment Order steps 3–5).

**Release tag**
- Releases are tagged (e.g. `v0.1.0`). To roll back code: `git checkout <previous-tag>` and redeploy.

**Data**
- If a bad migration corrupted data, restore Postgres from the latest backup (§10), then redeploy the matching app version. Artifacts in MinIO are immutable/content-addressed and generally need no rollback.

**Verify after rollback**
- `/health` = 200; `migrate status` matches the deployed version; send a smoke webhook and confirm a job reaches `succeeded`.
