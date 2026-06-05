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
the mock-backed runtime can start without secrets. For handoff/local real VK
testing, create a real local `.env` next to `.env.example`:

```bash
cp .env.example .env
```

Windows PowerShell:

```powershell
Copy-Item .env.example .env
notepad .env
```

`cmd/api`, `cmd/worker`, and `cmd/migrate` load `.env` automatically when
started from the repository root. OS/CI environment variables override values
from `.env`. The real `.env` is ignored by Git; commit only `.env.example`.

Override these values when needed:

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
| `PROVIDER` | `mock` | Primary provider adapter: `mock` or `openai` |
| `PROVIDER_CHAIN` | value of `PROVIDER` | Comma-separated router/fallback chain, e.g. `openai,mock` |
| `OPENAI_API_KEY` | `` | Required when OpenAI provider/moderation/scanner is enabled |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI API root |
| `OPENAI_TEXT_MODEL` / `OPENAI_IMAGE_MODEL` | OpenAI defaults | OpenAI text/image model codes |
| `OPENAI_IMAGE_SIZE` | `1024x1024` | OpenAI image size |
| `OPENAI_VIDEO_MODEL` / `OPENAI_VIDEO_SECONDS` / `OPENAI_VIDEO_SIZE` | `sora-2` / `4` / `720x1280` | OpenAI video settings |
| `OPENAI_TEXT_PRICE` / `OPENAI_IMAGE_PRICE` / `OPENAI_VIDEO_PRICE` | `1` / `10` / `50` | Internal provider-router cost estimates |
| `MODERATION_PROVIDER` | `keyword` | Output moderation provider: `keyword` or `openai` |
| `OPENAI_MODERATION_MODEL` | `omni-moderation-latest` | OpenAI moderation model |
| `ARTIFACT_SCANNER` | `none` | Artifact scanner: `none` or `openai` |
| `VK_DELIVERY_MODE` | `mock` | VK delivery adapter: `mock` or `real` |
| `VK_ACCESS_TOKEN` / `VK_API_VERSION` | `` / `5.199` | Required for real VK send/upload and API-side `/start` control menu responses |
| `VK_API_BASE_URL` | `https://api.vk.com/method` | VK API method root |
| `VK_WELCOME_ATTACHMENT` | `` | Optional pre-uploaded VK photo/video attachment sent with `/start` menu |
| `VK_MENU_BUTTON_MODE` | `callback` | Inline menu buttons: `callback` hides user echo messages; `text` keeps legacy text-button behavior |
| `VK_UNROUTED_TEXT_MODE` | `reply` | Plain text outside GPT mode: `reply` sends choose-mode menu, `silent` sends nothing, `gpt` preserves legacy text-to-GPT behavior |
| `SIGNED_DELIVERY` / `ARTIFACT_URL_TTL` | `false` / `1h` | Deliver media through signed artifact URLs |
| `ARTIFACT_RETENTION_DAYS` | `0` | Optional S3 lifecycle expiry |
| `PRICES` | `` | Price overrides, e.g. `text_generate=2,image_generate=12` |
| `MAX_JOB_COST` | `0` | Per-job cost cap; `0` disables the cap |
| `STREAM_MAX_LEN` | `100000` | Redis stream max length; `0` disables trimming |
| `WORKER_SHUTDOWN_GRACE` | `30s` | Time allowed to drain in-flight worker handlers |
| `WORKER_METRICS_ADDR` | `:9090` | Worker `/metrics` and `/healthz`; empty disables |
| `MAINTENANCE_INTERVAL` | `1h` | Cleanup cadence for idempotency/outbox/streams |
| `OUTBOX_RETENTION` | `168h` | Retention for terminal outbox events |
| `BILLING_RECONCILIATION_INTERVAL` | `5m` | Balance-vs-ledger reconciliation cadence |
| `BILLING_RECONCILIATION_LIMIT` | `100` | Accounts checked per reconciliation pass |
| `OTEL_TRACES_EXPORTER` | `none` | `none` or `stdout` |
| `OTEL_SERVICE_NAME` | `vk-ai-aggregator` | OpenTelemetry service name prefix |

> Production note: set `APP_ENV=production`; the API then **refuses to start**
> unless `VK_SECRET`, `ADMIN_TOKEN`, and a non-default `VK_CONFIRMATION_TOKEN`
> are set (fail-closed, `AUDIT.md` S1). Both `cmd/api` and `cmd/worker` run the
> same validation. `PROVIDER=openai`, `PROVIDER_CHAIN` containing `openai`,
> `MODERATION_PROVIDER=openai`, or `ARTIFACT_SCANNER=openai` require
> `OPENAI_API_KEY`; `VK_DELIVERY_MODE=real` requires `VK_ACCESS_TOKEN` in any
> environment.

### Hardening features (post-release)

- **Output moderation** gates delivery: a blocked prompt sets the job to
  `rejected`, releases the reservation (no capture), and writes a
  `moderation_results` audit row (migration `000003`). Default classifier is
  keyword-based (`MODERATION_EXTRA_TERMS` extends it); `MODERATION_PROVIDER=openai`
  switches the check to OpenAI moderation.
- **DLQ + retry budget**: exhausted/poison tasks go to `stream:jobs:dlq`
  (not auto-consumed) and the job becomes `failed_terminal`. Inspect with
  `docker exec vk-ai-aggregator-redis redis-cli XLEN stream:jobs:dlq`.
- **Metrics**: `GET /metrics` (Prometheus). Note metrics are process-local —
  scrape the API and each worker separately.
- **Tracing**: `OTEL_TRACES_EXPORTER=stdout` enables OpenTelemetry stdout spans;
  trace context is propagated through outbox/Redis with `traceparent`.
- **Maintenance**: worker runs cleanup for expired `idempotency_keys`, old
  terminal `outbox_events`, Redis Stream trimming, and billing reconciliation.
  Billing mismatch count is exported as `vkagg_billing_mismatches`.
- **Artifact scanning**: `ARTIFACT_SCANNER=openai` scans text/image artifact
  bytes before storage; video scan/transcode remains a media-pipeline follow-up.
- **SSRF**: artifact downloader blocks private/loopback/link-local hosts and
  non-http(s) schemes; optional host allowlist. Provider data URLs are accepted
  for normalized OpenAI text/image/video outputs.
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
applied 000003_moderation_results
up complete: 3 migration(s) applied
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

> Scaling note: run multiple `cmd/worker` instances (each joins the same group)
> for more throughput. Per-pool isolation via `WORKER_POOLS` is still a Beta
> follow-up (`AUDIT.md` SC2). The worker auto-creates the MinIO bucket and
> consumer groups on start.

Real adapter modes are opt-in:

```bash
# OpenAI text/image/video generation.
PROVIDER=openai OPENAI_API_KEY=... go run ./cmd/worker

# OpenAI primary with mock fallback through router/circuit breaker.
PROVIDER_CHAIN=openai,mock OPENAI_API_KEY=... go run ./cmd/worker

# API-side VK /start menu responses with keyboard.
VK_ACCESS_TOKEN=... go run ./cmd/api

# Real VK messages.send plus raw photo/video upload to VK upload servers.
VK_DELIVERY_MODE=real VK_ACCESS_TOKEN=... go run ./cmd/worker

# Real output moderation and text/image artifact scanning.
MODERATION_PROVIDER=openai ARTIFACT_SCANNER=openai OPENAI_API_KEY=... go run ./cmd/worker
```

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

### VK /start menu
```bash
curl -s -X POST localhost:8080/webhooks/vk \
  -H 'Content-Type: application/json' \
  -d '{"type":"message_new","group_id":1,"event_id":"menu-1","object":{"message":{"from_id":777,"peer_id":777,"text":"/start"}}}'
# -> ok
```

Expected: inbound event + command are persisted, no billable job is created.
When `cmd/api` has `VK_ACCESS_TOKEN`, it sends the Super GPT welcome text with
a VK inline keyboard under the message. Set `VK_WELCOME_ATTACHMENT` to a
pre-uploaded VK attachment string if the welcome message should include a
banner image.
Clicking `🎬 Создать видео` opens the video model picker with `Sora 2`,
`Kling v2.1`, `Seedance 1`, `Haiuo v0.2`, and `⬅️ Назад`. These model buttons
are control-only for now and must not create billable jobs.
Clicking `🖼️ Создать фото` opens the photo instruction screen directly because
there is one main image model in the VK UX. It shows `Фото по тексту`,
`Фото с референсом`, and `⬅️ Назад`; those mode buttons are control-only until
stateful image mode selection is wired. Clicking `💬 Спросить у GPT` sends the
`SUPER GPT активен` prompt screen, sets process-local GPT mode for that peer,
and also does not enqueue a job. The next plain text or sticker from the same
peer becomes a `text.ask` job; opening another menu screen clears GPT mode.
Clicking `🎁 Студентам и школьникам` opens the study submenu:
`Решальник задач`, `Генерация презентаций (скоро)`,
`Создание рефератов (скоро)`, `❓ Ответы на вопросы`, and `⬅️ Назад`.
Those buttons are control-only until the corresponding scenario state is wired.
Inline menu navigation is hybrid: while the last bot message is still the
active menu, button clicks edit that message through VK `messages.edit` instead
of adding new bot messages. If the user sends plain text, the active menu
pointer is cleared. With default `VK_UNROUTED_TEXT_MODE=reply`, plain text
outside GPT mode records an `unknown` command and sends a fresh choose-mode menu
instead of creating a billable job; `silent` records it without a response, and
`gpt` restores the legacy any-text-to-GPT behavior. If VK rejects an edit, the
API falls back to sending a new menu message.
By default, inline menu buttons use VK `callback` actions
(`VK_MENU_BUTTON_MODE=callback`), so clicking `Создать видео`, `Назад`, etc.
does not create a user message in the chat. VK Callback API must have the
`message_event` / callback-button event type enabled. To return to the old
behavior where button labels are sent as user messages, set
`VK_MENU_BUTTON_MODE=text` and restart `cmd/api`.
For every callback-button click, the API sends a blank
`messages.sendMessageEventAnswer` through `vkdelivery.ControlClient`; this is
what clears the loading spinner in the VK client.

### VK message → full pipeline
```bash
# enable text/GPT mode (control command, no job)
curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"text-mode-1","object":{"message":{"from_id":777,"peer_id":777,"text":"💬 Спросить у GPT","payload":"{\"command\":\"menu.text\"}"}}}'
# text job after GPT mode is active
curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"text-1","object":{"message":{"from_id":777,"peer_id":777,"text":"hello world"}}}'
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

**VK menu / keyboard**
- `/start` records command but no keyboard appears: make sure `cmd/api` has
  `VK_ACCESS_TOKEN` and the community has bot features enabled in VK community
  message settings. VK returns `error_code=912` when keyboards are disabled; the
  API falls back to sending the welcome text without keyboard.
- Menu clicks keep creating new bot messages: this is expected after the user
  has sent plain text outside GPT mode and `VK_UNROUTED_TEXT_MODE=reply` posted
  a fresh choose-mode menu, after an API restart, or after an edit rejection from
  VK. Active-menu/dialog-mode tracking is process-local in the current Beta
  implementation.
- Callback menu buttons do nothing: enable the VK Callback API event type for
  callback-button clicks (`message_event`) and confirm `VK_MENU_BUTTON_MODE` is
  `callback`. If you need a quick fallback, set `VK_MENU_BUTTON_MODE=text` and
  restart `cmd/api`.
- Callback button keeps spinning: check `api-live.log` for
  `vk message_event answer failed`. VK requires `messages.sendMessageEventAnswer`
  for every callback click; menu edit/send alone is not enough.
- Banner is absent: set `VK_WELCOME_ATTACHMENT` to an already uploaded VK
  attachment string (`photo...`, `video...`). The API does not upload the banner
  image itself yet.

**Migrations**
- Partial/failed migration: re-run `go run ./cmd/migrate status`. The runner
  records SHA-256 checksums and applies each migration file with its
  `schema_migrations` row in one transaction. If checksum drift is reported,
  restore the migration file to the applied contents or perform an explicit
  migration.
- Reset local DB (DESTRUCTIVE): `docker compose down -v && docker compose up -d` then re-run migrations.

**Queue**
- Stream length keeps growing with no progress: check worker logs for
  `handler failed`; inspect the offending job's `error_code`.
- Poison/retry-exhausted tasks are routed to `stream:jobs:dlq`; inspect with
  `docker exec vk-ai-aggregator-redis redis-cli XLEN stream:jobs:dlq`.
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
- Releases are tagged (current: `v0.1.3`). To roll back code:
  `git checkout <previous-tag>` and redeploy.

**Data**
- If a bad migration corrupted data, restore Postgres from the latest backup (§10), then redeploy the matching app version. Artifacts in MinIO are immutable/content-addressed and generally need no rollback.

**Verify after rollback**
- `/health` = 200; `migrate status` matches the deployed version; send a smoke webhook and confirm a job reaches `succeeded`.
