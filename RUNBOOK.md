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
| `VK_APP_ID` | `` | VK Mini App id (informational for BFF/dev setup) |
| `VK_APP_SECRET` | `` | VK Mini App protected key; required in production |
| `MINIAPP_LAUNCH_PARAMS_MAX_AGE` | `1h` | Maximum accepted VK Mini App launch-param age |
| `ADMIN_TOKEN` | `` (empty = open) | Admin API `X-Admin-Token` |
| `WORKER_GROUP` / `WORKER_CONSUMER` | `workers` / hostname | Consumer group identity |
| `APP_ENV` | `development` | `production` enforces fail-closed secrets |
| `MAX_ATTEMPTS` | `3` | Retry budget before dead-lettering |
| `RETRY_BASE_DELAY` / `RETRY_MAX_DELAY` | `500ms` / `30s` | Exponential backoff bounds |
| `MODERATION_EXTRA_TERMS` | `` | Comma-separated extra blocklist terms |
| `WEBHOOK_RATE_LIMIT_RPS` / `WEBHOOK_RATE_LIMIT_BURST` | `20` / `40` | Per-IP webhook rate limit |
| `VK_ANTISPAM_ENABLED` | `true` | Redis-backed per-`vk_user_id` VK bot anti-spam switch |
| `VK_ANTISPAM_MESSAGE_LIMIT` / `VK_ANTISPAM_MESSAGE_WINDOW` | `10` / `60s` | Any VK user events per window: text, stickers and buttons |
| `VK_ANTISPAM_GPT_LIMIT` / `VK_ANTISPAM_GPT_WINDOW` | `3` / `30s` | Billable GPT/text jobs per user window |
| `VK_ANTISPAM_COOLDOWN` | `30s` | Temporary pause after a rate-limit violation |
| `VK_ANTISPAM_VIOLATION_LIMIT` / `VK_ANTISPAM_VIOLATION_WINDOW` | `5` / `10m` | Violations before temporary block |
| `VK_ANTISPAM_BLOCK_DURATION` | `15m` | Temporary block length after repeated spam |
| `VK_ANTISPAM_NEW_USER_AGE` | `4h` | Age window for stricter new-user limits |
| `VK_ANTISPAM_NEW_USER_MESSAGE_LIMIT` | `5` | New-user event limit per message window |
| `VK_ANTISPAM_NEW_USER_GPT_LIMIT` / `VK_ANTISPAM_NEW_USER_GPT_WINDOW` | `1` / `15s` | New-user GPT/text job limit |
| `VK_ANTISPAM_ACTIVE_GPT_JOB_LIMIT` | `2` | Max active text-generation jobs per user before queue protection denies new ones |
| `VK_BOT_TUNNEL_MODE` | `quick` | Local bot tunnel mode for scripts: `quick` or `named` |
| `VK_BOT_TUNNEL_NAME` | `neiirohub-vk-bot` | Cloudflare named tunnel used by `start-bot.ps1 -TunnelMode named` |
| `VK_BOT_TUNNEL_HOSTNAME` | `vk.neiirohub.ru` | Stable public hostname for the local VK Callback API |
| `VK_BOT_TUNNEL_CONFIG` | `.runtime/vk-bot/cloudflared/config.yml` | Optional override for named tunnel config path |
| `PROVIDER` | `mock` | Primary provider adapter: `mock`, `openai`, or `deepinfra` |
| `PROVIDER_CHAIN` | value of `PROVIDER` | Comma-separated router/fallback chain, e.g. `openai,mock` or `deepinfra,mock` |
| `DEEPINFRA_API_KEY` | `` | Required when DeepInfra provider is enabled |
| `DEEPINFRA_BASE_URL` | `https://api.deepinfra.com/v1/openai` | DeepInfra OpenAI-compatible API root |
| `DEEPINFRA_TEXT_MODEL` | `deepseek-ai/DeepSeek-V4-Flash` | DeepInfra text model code |
| `DEEPINFRA_TEXT_PRICE` | `1` | Internal provider-router cost estimate |
| `TEXT_CONTEXT_ENABLED` | `true` | Persist and render compact VK text dialog context in `cmd/worker` |
| `TEXT_CONTEXT_MAX_INPUT_TOKENS` | `1600` | Estimated input budget for rendered dialog context |
| `TEXT_CONTEXT_MAX_OUTPUT_TOKENS` | `800` | Provider max output token cap for text generation when supported |
| `TEXT_CONTEXT_SUMMARY_MAX_TOKENS` | `400` | Max estimated tokens retained in rolling summary |
| `TEXT_CONTEXT_RECENT_MESSAGES_LIMIT` | `6` | Recent unsummarized messages included in provider prompt |
| `TEXT_CONTEXT_SUMMARIZE_AFTER_MESSAGES` / `TEXT_CONTEXT_SUMMARIZE_AFTER_TOKENS` | `10` / `1500` | Thresholds for compacting old turns into summary |
| `MINIAPP_JOB_RATE_LIMIT_RPS` / `MINIAPP_JOB_RATE_LIMIT_BURST` | `1` / `5` | Per-user Mini App `POST /miniapp/jobs` rate limit |
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
| `VK_UNROUTED_TEXT_MODE` | `reply` | Plain text outside GPT mode: `reply` sends a choose-mode hint with the lower menu keyboard, `silent` sends nothing, `gpt` preserves legacy text-to-GPT behavior |
| `VK_DIALOG_MODE_TTL` | `1h` | Redis TTL for active VK peer modes such as `Спросить у НейроХаб`; refreshes while the user keeps chatting |
| `VK_REFERRAL_LINK_BASE` | `` | Base link for the user's single VK referral URL. If it contains `{code}`, that placeholder is replaced; otherwise `ref=<code>` is appended |
| `VK_REFERRAL_SHARE_BASE` | `https://vk.com/share.php` | Reserved base URL for future VK share/open-link flows; the current account screen does not render a share button |
| `REFERRAL_CODE_LENGTH` | `10` | Length for generated stable public referral codes |
| `REFERRAL_REFERRER_SIGNUP_REWARD_CREDITS` | `10` | Signup reward posted to the inviter through billing ledger |
| `REFERRAL_REFERRED_SIGNUP_REWARD_CREDITS` | `0` | Optional signup reward posted to the invited user through billing ledger |
| `VK_MENU_*_ENABLED` | mixed | Per-button VK product menu flags; current bot profile keeps NeuroHub text mode and account/referral visible, while video/image/students/top-up stay hidden without deleting their screens |
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
> unless `VK_SECRET`, `ADMIN_TOKEN`, `VK_APP_SECRET`, and a non-default `VK_CONFIRMATION_TOKEN`
> are set (fail-closed, `AUDIT.md` S1). Both `cmd/api` and `cmd/worker` run the
> same validation. `PROVIDER=openai`, `PROVIDER_CHAIN` containing `openai`,
> `MODERATION_PROVIDER=openai`, or `ARTIFACT_SCANNER=openai` require
> `OPENAI_API_KEY`; `PROVIDER=deepinfra` or `PROVIDER_CHAIN` containing
> `deepinfra` requires `DEEPINFRA_API_KEY`; `VK_DELIVERY_MODE=real` requires
> `VK_ACCESS_TOKEN` in any environment.

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
- **VK anti-spam**: Redis counters per `vk_user_id` limit all user events
  (`10/60s`, new users `5/60s`), billable GPT jobs (`3/30s`, new users
  `1/15s`), repeated violations (`5/10m -> 15m` temporary block), and active
  GPT jobs (`2` per user). Denied events are acknowledged through the VK control
  path and do not create commands/jobs.

---

## 3. VK Bot One-Command Startup

For local hand testing of the VK bot on Windows, prefer the bot-only dev
scripts:

```powershell
.\scripts\dev\start-bot.ps1
.\scripts\dev\status-bot.ps1
.\scripts\dev\stop-bot.ps1
```

`start-bot.ps1` performs the VK bot runtime startup sequence:

- starts Docker dependencies: PostgreSQL, Redis and MinIO;
- applies `cmd/migrate up`;
- builds and starts `cmd/api` and `cmd/worker`;
- starts a `cloudflared` quick tunnel to the API;
- prints the VK Callback URL:

```text
https://<random>.trycloudflare.com/webhooks/vk
```

Use that URL in VK Callback API settings and confirm the server. The scripts do
not start the VK Mini App frontend; Mini App development remains separate.
Runtime pid/log/url files are stored in `.runtime/vk-bot/` and ignored by Git.

For local development with a stable VK Callback URL, use a named Cloudflare
Tunnel instead of the quick tunnel:

```powershell
.\scripts\dev\setup-cloudflare-tunnel.ps1 -Login
.\scripts\dev\start-bot.ps1 -TunnelMode named
```

Default stable callback:

```text
https://vk.neiirohub.ru/webhooks/vk
```

The setup script creates/reuses the `neiirohub-vk-bot` tunnel, writes its local
config to `.runtime/vk-bot/cloudflared/config.yml`, and creates the Cloudflare
DNS route for `vk.neiirohub.ru`. The hostname works only after `neiirohub.ru`
is added to Cloudflare and the registrar NS records point to Cloudflare.

Useful options:

```powershell
.\scripts\dev\start-bot.ps1 -NoTunnel       # local API/worker only
.\scripts\dev\start-bot.ps1 -SkipDocker     # reuse already-running containers
.\scripts\dev\start-bot.ps1 -SkipMigrate    # skip migration step
.\scripts\dev\start-bot.ps1 -TunnelMode named  # stable vk.neiirohub.ru callback
.\scripts\dev\start-bot.ps1 -TunnelProtocol quic  # default is http2
.\scripts\dev\stop-bot.ps1 -StopDocker      # stop app processes and containers
```

The manual sections below are still useful for debugging individual steps.

---

## 4. Infrastructure Startup

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

## 5. Migrations

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

## 6. API Startup

```bash
go run ./cmd/api               # listens on :8080
```

Serves: `POST /webhooks/vk`, `/miniapp/*`, `GET /admin/...`,
`GET /metrics`, `GET /health`, and `GET /healthz`.

API wiring map:

- `cmd/api/main.go` is the thin bootstrap: config validation, tracing,
  Postgres/Redis clients, shared core construction, route mounting, health,
  metrics and graceful shutdown.
- `internal/app/api` wires shared backend-core repositories/services used by
  surfaces.
- `internal/app/vkbot` wires the VK text bot surface for `/webhooks/vk`.
- `internal/app/miniapp` wires the Mini App BFF surface for `/miniapp/*`.
- `internal/adapter/inbound/vk` and `internal/adapter/inbound/miniapp` contain
  the actual inbound/BFF handlers and DTOs.

To add a VK bot command or menu behavior, start in
`internal/adapter/inbound/vk` and the related command/router/service tests.
Only update `internal/app/vkbot` when the command needs a new wiring dependency
or feature flag.

Do not put provider calls, pricing truth, balance mutation, job state truth or
moderation decisions in app surfaces. Those stay in `internal/service`,
`internal/worker`, provider adapters and storage.

Verify (new terminal):
```bash
curl -s localhost:8080/health
# {"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
```

---

## 7. Worker Startup

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

# DeepInfra DeepSeek-V4-Flash text generation with mock fallback for other
# modalities.
PROVIDER_CHAIN=deepinfra,mock DEEPINFRA_API_KEY=... go run ./cmd/worker

# API-side VK /start menu responses with keyboard.
VK_ACCESS_TOKEN=... go run ./cmd/api

# Real VK messages.send plus raw photo/video upload to VK upload servers.
VK_DELIVERY_MODE=real VK_ACCESS_TOKEN=... go run ./cmd/worker

# Real output moderation and text/image artifact scanning.
MODERATION_PROVIDER=openai ARTIFACT_SCANNER=openai OPENAI_API_KEY=... go run ./cmd/worker
```

Text provider adapters add an internal instruction to the user's prompt: answer
as `НейроХаб бот`, stay concise (`<= 3000` characters), and do not reveal
provider/model/backend details. This reduces VK overlong-message failures,
while delivery still chunks longer text outputs if the provider ignores the
instruction.

Expected log:
```
{"level":"INFO","msg":"workers started","group":"workers","consumer":"<host>"}
```

---

## 8. Health Checks

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

## 9. Local Testing

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
When `cmd/api` has `VK_ACCESS_TOKEN`, it sends the НейроХаб welcome text with
a VK inline keyboard under the message. Set `VK_WELCOME_ATTACHMENT` to a
pre-uploaded VK attachment string if the welcome message should include a
banner image. On the first `Старт` for a user, the API tries one `users.get`
lookup through `vkdelivery.UserProfileClient`, caches `vk_first_name` /
`vk_last_name` on the user row, sends `👋 <name>, добро пожаловать в НейроХаб!`,
and records `welcome_name_sent_at`; later `Старт` / `Показать меню` responses
use the regular welcome without the name.
Clicking `🎬 Создать видео` opens the video model picker with `Sora 2`,
`Kling v2.1`, `Seedance 1`, `Haiuo v0.2`, and `⬅️ Назад`. `Sora 2` and
`Kling v2.1` open detail screens with description, prompt example, instruction
link, `😀 Начать генерацию`, `ℹ️ Примеры`, and `⬅️ Назад`. `Seedance 1` opens
`Seedance 1 Lite` / `Seedance 1 Pro`; `Haiuo v0.2` opens `Haiuo v0.2 Обычный`
/ `Haiuo v0.2 Fast`. These video submenu buttons are control-only for now and
must not create billable jobs.
Clicking `🖼️ Создать фото` opens the photo instruction screen directly because
there is one main image model in the VK UX. It shows `Фото по тексту`,
`Фото с референсом`, and `⬅️ Назад`; those mode buttons are control-only until
stateful image mode selection is wired. Clicking `💬 Спросить у НейроХаб` sends the
`НейроХаб активен` prompt screen, stores Redis-backed GPT mode for that peer,
and also does not enqueue a job. The next plain text or sticker from the same
peer becomes a `text.ask` job; the API sends `НейроХаб думает...`, stores that VK
message id in `job.Params`, and the delivery worker edits the same message to
the final provider answer when the text artifact is delivered. If the answer is
too long for one VK message, the first chunk replaces the placeholder and the
remaining chunks are sent as follow-up messages with deterministic `random_id`.
Before sending/editing text in VK, delivery converts simple provider Markdown to
VK plain text: `**bold**` markers, backticks and heading hashes are stripped,
while `*` / `-` list items become `•` bullets.
Opening another menu screen clears GPT mode. Legacy `VK_UNROUTED_TEXT_MODE=gpt`
keeps normal text delivery without this placeholder/edit UX.
Clicking `🎁 Студентам и школьникам` opens the study submenu:
`Решальник задач`, `Генерация презентаций (скоро)`,
`Создание рефератов (скоро)`, `❓ Ответы на вопросы`, and `⬅️ Назад`.
Those buttons are control-only until the corresponding scenario state is wired.
Clicking `👤 Мой аккаунт` opens the account/referral screen. The handler reads
the billing projection through `billingservice.EnsureAccount`, ensures one
stable referral code for the user, counts accepted invitations, and renders the
"безлимитное общение" note, invited count, plain-text VK referral link and
`@neirohub_help`. The account keyboard currently keeps only `⬅️ Назад`; no `↗️ Поделиться` /
`open_link` button is rendered. The link is built from `VK_REFERRAL_LINK_BASE`;
use a template such as `https://vk.com/write-239332376?ref={code}` or a base URL
where the API can append `ref=<code>`. `/start <code>` and VK Callback `ref`
params apply the referral as `source=vk_bot`, do not create a billable job, and
post signup bonuses only through idempotent ledger top-up entries. A full Mini
App referral account/API screen is still a follow-up, but the same
`referralservice` and Postgres tables already support
`source=vk_miniapp`.

Text dialog memory is built in `cmd/worker`, not in the VK webhook. For VK
`text.ask` jobs with `vk_peer_id`, the worker writes the user prompt and
assistant answer to Postgres (`conversations`, `conversation_messages`,
`conversation_summaries`), then renders a bounded provider prompt from bot
profile, rolling summary, recent messages and the current request. The system
prompt that says the assistant is NeuroHub remains inside provider adapters and
stays above dialog history. Summary compaction is local/extractive in this
beta; no extra billable provider call is made just to summarize old turns.

Inline menu navigation is hybrid: while the last bot message is still the
active menu, inline button clicks edit that message through VK `messages.edit`
instead of adding new bot messages. The persistent lower `Показать меню` button
always sends a fresh menu at the bottom of the chat. An ordinary first
non-payload text/sticker/menu-repair contact is treated as onboarding and opens
`/start`. After onboarding, with default `VK_UNROUTED_TEXT_MODE=reply`, plain text outside GPT mode records an
`unknown` command and sends `Выберите режим в меню выше или нажмите на кнопку показать меню` with the lower
`Показать меню` keyboard instead of duplicating the inline menu or creating a
billable job; `silent` records it without a response, and `gpt` restores the
legacy any-text-to-GPT behavior. Typed repair phrases like `меню`, `нет меню`,
`нет кнопки` and `где меню` reopen the welcome menu and repair the lower
keyboard. If VK rejects an edit, the API falls back to sending a new menu
message.
By default, inline menu buttons use VK `callback` actions
(`VK_MENU_BUTTON_MODE=callback`), so clicking `Создать видео`, `Назад`, etc.
does not create a user message in the chat. VK Callback API must have the
`message_event` / callback-button event type enabled. To return to the old
behavior where button labels are sent as user messages, set
`VK_MENU_BUTTON_MODE=text` and restart `cmd/api`.
If a stale inline `show_menu` callback arrives after a GPT answer has cleared
the active menu, the API only acknowledges it and does not send a new welcome
menu. The persistent lower `Показать меню` text button remains the explicit way
to send a fresh menu at the bottom of the chat.
For every callback-button click, the API sends a blank
`messages.sendMessageEventAnswer` through `vkdelivery.ControlClient`; this is
what clears the loading spinner in the VK client.
Each product-menu button is guarded by a boolean env flag. Account/top-up
default to `false`; the other menu flags default to `true`. Set a flag to
`false` and restart `cmd/api` to hide the button. Main menu flags:
`VK_MENU_VIDEO_ENABLED`, `VK_MENU_IMAGE_ENABLED`, `VK_MENU_GPT_ENABLED`,
`VK_MENU_STUDENTS_ENABLED`, `VK_MENU_ACCOUNT_ENABLED`,
`VK_MENU_TOP_UP_ENABLED`. Nested flags:
`VK_MENU_VIDEO_SORA2_ENABLED`, `VK_MENU_VIDEO_SORA2_START_ENABLED`,
`VK_MENU_VIDEO_SORA2_EXAMPLES_ENABLED`, `VK_MENU_VIDEO_KLING21_ENABLED`,
`VK_MENU_VIDEO_KLING21_START_ENABLED`,
`VK_MENU_VIDEO_KLING21_EXAMPLES_ENABLED`,
`VK_MENU_VIDEO_SEEDANCE1_ENABLED`, `VK_MENU_VIDEO_SEEDANCE1_LITE_ENABLED`,
`VK_MENU_VIDEO_SEEDANCE1_PRO_ENABLED`, `VK_MENU_VIDEO_HAIUO02_ENABLED`,
`VK_MENU_VIDEO_HAIUO02_STANDARD_ENABLED`, `VK_MENU_VIDEO_HAIUO02_FAST_ENABLED`,
`VK_MENU_IMAGE_TEXT_ENABLED`, `VK_MENU_IMAGE_REFERENCE_ENABLED`,
`VK_MENU_STUDENTS_SOLVER_ENABLED`, `VK_MENU_STUDENTS_PRESENTATION_ENABLED`,
`VK_MENU_STUDENTS_REPORT_ENABLED`, `VK_MENU_STUDENTS_QA_ENABLED`.
If a user clicks a disabled stale button from an older message, the handler
falls back to the current main menu and still creates no job.

### VK message → full pipeline
```bash
# enable text/GPT mode (control command, no job)
curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"text-mode-1","object":{"message":{"from_id":777,"peer_id":777,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}}'
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

### App surface smoke checklist

Use this checklist after changing app-surface wiring or shared backend core:

- API starts with `go run ./cmd/api`; `/health`, `/healthz` and `/metrics`
  respond.
- Worker starts with `go run ./cmd/worker`; provider calls happen only from
  worker flows, never from `cmd/api` or `internal/app/*`.
- VK text bot entrance: POST a VK `confirmation` event and one `/start` or
  GPT-mode message to `/webhooks/vk`; the webhook returns quickly, inbound
  idempotency prevents duplicate jobs for repeated event ids, and billable text
  prompts create jobs through `joborchestrator`.
- Mini App entrance: call `/miniapp/balance`, `/miniapp/estimate` and
  `/miniapp/jobs` with valid dev launch params or real VK launch params; auth
  and rate limiting remain enforced, and estimate does not create a job,
  reserve credits or write ledger entries.
- Job completion path: a queued job reaches a terminal state through
  `cmd/worker`; output artifact ownership is checked by
  `GET /miniapp/artifacts/{id}` and billing capture/release/refund is ledger
  backed.
- Public model naming remains product-safe: user-visible Mini App/VK chat copy
  says `ChatGPT` where applicable and does not reveal DeepInfra/DeepSeek model
  ids.

---

## 10. Troubleshooting

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
  has sent plain text after an API restart, after the active menu pointer was
  lost, or after an edit rejection from VK. With `VK_UNROUTED_TEXT_MODE=reply`,
  plain text outside GPT mode should only post `Выберите режим в меню выше или нажмите на кнопку показать меню`
  with the lower `Показать меню` keyboard, without duplicating the inline menu
  keyboard. Active-menu tracking is process-local in the current Beta
  implementation, while GPT dialog mode is Redis-backed and survives API
  restarts for `VK_DIALOG_MODE_TTL`.
- Same welcome/menu appears again later without a visible user action: check
  `api-live.log` for `mark inbound processed`. VK retries webhook events when
  the API returns `500`; duplicate inbound idempotency keys must reload the
  existing inbound row before status updates.
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
- Bot replies `Слишком много сообщений...`: VK anti-spam denied the event by
  `vk_user_id`. Check `VK_ANTISPAM_*` settings and Redis keys
  `rate:vk:user:<id>:messages`, `rate:vk:user:<id>:gpt`,
  `spam:vk:user:<id>:violations`, `block:vk:user:<id>`.
- Bot replies `У вас уже есть активный запрос`: the user already has the
  configured number of active text-generation jobs. Wait for delivery or inspect
  `/admin/jobs?user_id=<uuid>&operation=text_generate`.

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

## 11. Backup & Recovery

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

## 12. Deployment Order

Start in dependency order; stop in reverse:

1. **Infrastructure**: Postgres → Redis → MinIO (wait for healthy).
2. **Migrations**: `migrate up` (must complete before app starts).
3. **API**: `cmd/api` (verify `/health` = 200).
4. **Workers**: `cmd/worker` (verify `workers started`; consumer groups auto-created).
5. **Smoke test**: send a `message_new` webhook; confirm job `succeeded`.

Shutdown order: Workers → API → (optionally) Infrastructure.

---

## 13. Rollback Procedure

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

---

## 14. VK Mini App BFF

The Mini App backend-for-frontend (BFF) is mounted by `cmd/api` through
`internal/app/miniapp`. It listens on the `/miniapp/*` path prefix and
authenticates every request using VK launch-params signature verification
(HMAC-SHA256).

To add a Mini App endpoint, edit `internal/adapter/inbound/miniapp` first and
add tests there. Only update `internal/app/miniapp` when the endpoint needs a
new shared dependency. Do not calculate prices, trust balance, call providers or
serve artifact bytes from frontend state.

### New environment variables

| Variable | Default | Required |
|---|---|---|
| `VK_APP_SECRET` | `""` (skip check in dev) | **Yes in production (fail-closed)** |
| `VK_APP_ID` | `""` | Used by the tunnel; informational for the BFF |
| `MINIAPP_LAUNCH_PARAMS_MAX_AGE` | `1h` | Optional |
| `MINIAPP_JOB_RATE_LIMIT_RPS` / `MINIAPP_JOB_RATE_LIMIT_BURST` | `1` / `5` | Optional |

When `VK_APP_SECRET` is set the launch-params HMAC-SHA256 signature is verified
for real: invalid, missing, or expired (`vk_ts` older than
`MINIAPP_LAUNCH_PARAMS_MAX_AGE`) params return `401` with no detail, and the dev
`X-VK-User-ID` bypass is disabled. When `VK_APP_SECRET` is empty the signature
check is skipped (dev/mock convenience) but `vk_user_id` is still required. In
**production** an empty `VK_APP_SECRET` fails startup (fail-closed).
Job creation through `POST /miniapp/jobs` has a separate per-verified-user
in-memory token bucket; exceeded limits return `429` with `Retry-After`.
Artifact bytes from `GET /miniapp/artifacts/{id}` are served only for the
verified owner after the producing job is `succeeded` and output moderation has
allowed the artifact; otherwise the BFF returns `404`.

### Local development without real VK

```powershell
# Start infrastructure + API in mock mode:
docker compose up -d
go run ./cmd/migrate up
. .\.env.ps1
go run ./cmd/api

# Call BFF endpoints directly (X-VK-User-ID header accepted in dev mode
# when VK_APP_SECRET is not set):
curl -s http://localhost:8080/miniapp/balance -H "X-Launch-Params: vk_user_id=777"
curl -s -X POST http://localhost:8080/miniapp/jobs \
  -H "Content-Type: application/json" \
  -H "X-Launch-Params: vk_user_id=777" \
  -d '{"operation":"text_generate","prompt":"hello world"}'
```

### Frontend dev server

```powershell
cd web\miniapp
npm install
npm run dev
# → http://localhost:5173/?vk_user_id=777
```

The Vite proxy routes `/miniapp/*` to `http://localhost:8080`.

### Open the Mini App inside VK via an HTTPS tunnel (`localhost.run`)

VK WebView requires HTTPS. For local Mini App dev, prefer **`localhost.run`**
(`https://<random>.lhr.life`). It avoids the free **ngrok** interstitial that
VK iframe cannot pass (Network shows `error.js` instead of `main.tsx`).

**One command (Windows):**

```powershell
powershell -ExecutionPolicy Bypass -File .\start-miniapp-ngrok.ps1 -NoWait
```

Starts API + worker + Vite, opens an SSH tunnel to `localhost.run`, prints the
public URL. Logs: `%TEMP%\vkagg-miniapp-ngrok\`.

Stop:

```powershell
powershell -ExecutionPolicy Bypass -File .\start-miniapp-ngrok.ps1 -StopOnly
```

**Manual tunnel** (API, worker and `npm run dev` already running):

```powershell
ssh -o StrictHostKeyChecking=no -R 80:127.0.0.1:5173 nokey@localhost.run
```

Paste `https://....lhr.life` into **dev.vk.com → your app → Версия для vk.com →
"URL для разработки"**. The URL changes when the SSH session ends — update VK
settings after each restart.

Vite proxies `/miniapp/*` to `http://127.0.0.1:8080`, so one tunnel URL serves
both frontend and BFF (same-origin, no mixed content).

**Stable alternative (Cloudflare named tunnel):**

```powershell
.\scripts\dev\setup-miniapp-cloudflare-route.ps1
# then run cloudflared with .runtime/vk-bot/cloudflared/config.yml
# app.neiirohub.ru -> http://localhost:5173
```

**Legacy / bot-only:** `cloudflared tunnel --protocol http2 --url http://localhost:5173`
still works for quick smoke, but `*.trycloudflare.com` is less reliable for VK
WebView than `*.lhr.life` in local practice.

### Production

1. Set `VK_APP_SECRET` to the app's Protected Key from the VK Mini App settings.
2. Build the frontend: `cd web/miniapp && npm run build`
3. Host `web/miniapp/dist/` as the Mini App's static URL in VK admin.
4. The BFF runs as part of the existing `cmd/api` binary — no extra process.
