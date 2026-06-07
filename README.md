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

The same `cmd/api` binary mounts both user-facing app surfaces: VK text bot at
`/webhooks/vk` and VK Mini App BFF under `/miniapp/*`. Surface wiring lives in
`internal/app/vkbot` and `internal/app/miniapp`; `cmd/api/main.go` is now a thin
bootstrap over shared backend core. Mini App requests verify VK launch params
server-side, create jobs through the shared `joborchestrator`, use backend
billing/idempotency, and fetch artifacts only through owner-checked backend
endpoints.

Real integrations are implemented at adapter level and remain **opt-in**:

- `PROVIDER=openai` enables OpenAI text (`Responses`), image (`Images`) and
  async video (`Videos`) generation.
- `PROVIDER=deepinfra` enables DeepInfra text generation through
  `deepseek-ai/DeepSeek-V4-Flash` (`/chat/completions`) and DeepInfra image
  generation through `ByteDance/Seedream-4.5` (`/v1/inference/{model}`) on
  DeepInfra's native image API. `DEEPINFRA_IMAGE_FALLBACK_MODEL` can name a
  second DeepInfra image model for retryable primary-model failures. Text providers receive an internal
  instruction to answer as `НейроХаб бот`, keep replies concise and under 3000
  characters, and avoid exposing provider/model/backend details; VK delivery
  still chunks longer output as a fallback.
- `PROVIDER_CHAIN=openai,mock` enables the provider router with
  health/circuit-breaker, fallback, cost and observed-latency aware selection.
  `PROVIDER_CHAIN=deepinfra,mock` uses DeepInfra for text/image and mock
  fallback for unsupported or retryable paths.
- `IMAGE_PROVIDER`, `IMAGE_MODEL` and `IMAGE_SIZE` are worker-only image
  generation defaults. They prefer one provider/model for image jobs while
  keeping `PROVIDER_CHAIN` as fallback. Current image-capable adapters are
  mock, OpenAI and DeepInfra Seedream. VK bot and Mini App surfaces still only
  create Jobs and never call image providers directly.
- `VK_DELIVERY_MODE=real` enables VK `messages.send` plus raw photo/video
  artifact upload to VK upload servers before send.
- `cmd/api` can send the VK `/start` НейроХаб menu and inline keyboard through
  the VK delivery adapter when `VK_ACCESS_TOKEN` is configured. The optional
  `VK_WELCOME_ATTACHMENT` env attaches a pre-uploaded VK banner.
- The VK `Создать видео` menu button opens a model picker (`Sora 2`,
  `Kling v2.1`, `Seedance 1`, `Haiuo v0.2`) with a `Назад` control. `Sora 2`
  and `Kling v2.1` open detail screens with description, prompt example,
  instruction link, `Начать генерацию`, `Примеры`, and `Назад`; `Seedance 1`
  opens `Lite` / `Pro`; `Haiuo v0.2` opens `Обычный` / `Fast`. These video
  submenu buttons are control-only until model-specific generation state is
  wired.
- VK menu screens are described through a small declarative registry. `Создать
  фото` skips model selection when only one main image model is available and
  opens the text-to-image instruction screen directly; reference-photo generation
  is hidden by flag until the input artifact flow is ready. `Спросить у НейроХаб`
  opens the active GPT prompt screen and enables text GPT mode for that peer.
  The first `Старт` welcome is personalized with the cached VK first name when
  `VK_ACCESS_TOKEN` allows `users.get`; subsequent menu openings use the regular
  non-personalized welcome.
  Plain text and stickers create `text.ask` jobs only while GPT mode is active
  or when `VK_UNROUTED_TEXT_MODE=gpt` is explicitly configured. In active GPT
  mode, the bot sends `НейроХаб думает...` and the delivery worker edits that same
  VK message to the provider answer. Before sending to VK, text delivery strips
  simple Markdown markers such as `**bold**`, backticks and `*`/`-` list syntax,
  rendering lists as plain `•` bullets. Legacy `VK_UNROUTED_TEXT_MODE=gpt` keeps
  normal text delivery.
- VK photo text mode is now wired in the bot: when `VK_MENU_IMAGE_ENABLED=true`,
  `Создать фото` opens a text-to-image instruction screen and immediately stores
  `photo_text` mode for the peer. The next plain text creates an
  `image.generate` Job, and the API sends `НейроХаб рисует...` while workers
  produce and deliver the image Artifact. The current VK profile gives each
  user 100 free text-to-image attempts per 24h window through
  `VK_ANTISPAM_IMAGE_DAILY_LIMIT=100` and `PRICES=image_generate=0`;
  reference-photo generation stays hidden behind
  `VK_MENU_IMAGE_REFERENCE_ENABLED=false` until input photo artifacts are wired.
- Text-mode dialog context is persisted in Postgres. For each VK peer the
  worker stores user/assistant turns in `conversations`,
  `conversation_messages` and `conversation_summaries`, then sends providers a
  bounded context packet: bot profile, rolling summary, recent messages and the
  current request. Defaults are `TEXT_CONTEXT_MAX_INPUT_TOKENS=1600`,
  `TEXT_CONTEXT_MAX_OUTPUT_TOKENS=800`, summary up to 400 estimated tokens and
  the last 6 messages. The full dialog is never sent to a provider.
- Shared VK referral foundation is implemented in the backend. Each internal
  user receives one stable public referral code in `referral_codes`; accepted
  invitations are stored in `referrals` with source `vk_bot` or `vk_miniapp`.
  The VK bot account screen currently shows the "безлимитное общение" note,
  invited-user count, one plain-text referral link and support handle
  `@neirohub_help`; it does not render a share button.
  `/start <code>` / VK `ref` handling records the relation without creating a
  billable job. Signup rewards are posted through billing ledger entries with
  idempotency keys. A full Mini App referral account/API screen is still a
  follow-up over the same backend service/repository.
- VK inline menu navigation uses a hybrid UX: if the last bot message is the
  active menu, inline button clicks edit it through VK `messages.edit`; pressing
  the persistent lower `Показать меню` button always sends a fresh menu at the
  bottom of the chat. An ordinary first non-payload text/sticker/menu-repair
  contact is treated as onboarding and opens `/start`; after onboarding, a plain user message outside GPT mode with
  the default `VK_UNROUTED_TEXT_MODE=reply` sends the text hint
  `Выберите режим в меню выше или нажмите на кнопку показать меню` with the persistent lower `Показать меню`
  keyboard and does not duplicate the inline menu. Edit failure falls back to a
  normal send. In Beta the active-menu message pointer
  is still process-local to `cmd/api`, while GPT dialog mode is persisted in
  Redis with `VK_DIALOG_MODE_TTL` so it survives API restarts.
- Inline menu buttons default to VK `callback` actions
  (`VK_MENU_BUTTON_MODE=callback`), so button clicks arrive as `message_event`
  and do not add user echo messages to the chat. Set
  `VK_MENU_BUTTON_MODE=text` to return to legacy text buttons. The persistent
  lower `Показать меню` button stays a text button.
  Stale inline `show_menu` callbacks after a GPT answer are acknowledged but do
  not send a fresh welcome/menu message; users reopen the menu through the
  persistent lower `Показать меню` button.
- `VK_UNROUTED_TEXT_MODE` controls ordinary text outside GPT mode after
  onboarding: `reply` (default) sends `Выберите режим в меню выше или нажмите на кнопку показать меню` with the
  lower `Показать меню` keyboard, `silent` records the command but sends
  nothing, and `gpt` preserves the old behavior where any text becomes a GPT
  job. Typed repair phrases like `меню`, `нет меню`, `нет кнопки` and
  `где меню` reopen the welcome menu and repair the lower keyboard.
- VK bot anti-spam is Redis-backed and enabled by default. It counts any user
  event (text, stickers and buttons), applies stricter limits for new users,
  separately limits billable GPT requests, temporarily blocks repeated
  violators, and prevents one user from enqueueing more than two active GPT
  jobs. Denied events are acknowledged and do not create commands or jobs.
- Every VK product-menu button has an env feature flag (`VK_MENU_*_ENABLED`).
  Disabled buttons are hidden from new keyboards, while stale payload clicks
  from older messages fall back to the current main menu instead of opening a
  hidden section.
  Current bot-facing env profile keeps `VK_MENU_GPT_ENABLED=true` and
  `VK_MENU_ACCOUNT_ENABLED=true` visible in the main menu; video, image,
  students and top-up sections stay implemented but hidden behind flags.
- `Студентам и школьникам` opens a study submenu with task solving,
  presentations/reports placeholders, question answering, and back navigation.
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
  app/
    api/             bootstrap helper for shared API core deps
    vkbot/           VK text bot surface wiring
    miniapp/         VK Mini App BFF surface wiring
  domain/            entities, state machines, repository interfaces (no infra)
  service/           billing, joborchestrator, commandrouter, artifactservice
  worker/            generation, provider-poll and delivery workers + engine
  adapter/
    inbound/vk/      VK Callback API webhook
    inbound/admin/   read-only admin HTTP API
    delivery/vk/     outbound VK client (+ mock)
    provider/mock/   mock AI provider
    provider/deepinfra/ DeepInfra text/image-generation adapter
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

### VK bot one-command startup

For local hand testing of the VK bot on Windows, use the bot-only dev scripts:

```powershell
.\scripts\dev\start-bot.ps1
.\scripts\dev\status-bot.ps1
.\scripts\dev\stop-bot.ps1
```

`start-bot.ps1` starts PostgreSQL, Redis and MinIO, applies migrations, builds
and starts `cmd/api` and `cmd/worker`, starts a `cloudflared` quick tunnel
using `--protocol http2`, waits for public `/health`, and prints the VK
Callback URL:

```text
https://<random>.trycloudflare.com/webhooks/vk
```

The scripts are intentionally scoped to the VK bot runtime. They do not start
the VK Mini App frontend. Runtime pid/log/url files are written under
`.runtime/vk-bot/` and are ignored by Git.

### VK Mini App local dev (HTTPS via `*.lhr.life`)

For opening the Mini App inside VK during UI work, use the dev launcher
(tunnel = `localhost.run`, not ngrok):

```powershell
powershell -ExecutionPolicy Bypass -File .\start-miniapp-ngrok.ps1 -NoWait
```

Paste the printed `https://....lhr.life` URL into **dev.vk.com → URL для
разработки**. Stop with `-StopOnly`. Details: `web/miniapp/README.md` and
`RUNBOOK.md`.

For a stable local VK Callback URL, configure a named Cloudflare Tunnel once:

```powershell
.\scripts\dev\setup-cloudflare-tunnel.ps1 -Login
.\scripts\dev\start-bot.ps1 -TunnelMode named
```

The default named-tunnel hostname is:

```text
https://vk.neiirohub.ru/webhooks/vk
```

This requires the `neiirohub.ru` DNS zone to be active in Cloudflare. The
tunnel config and credentials stay under `.runtime/vk-bot/` and the user's
Cloudflare profile; they are not committed.
On named-tunnel startup, `start-bot.ps1` verifies the local VK confirmation
response and the public `/health` endpoint. If the hostname is attached to a
stale Cloudflare DNS record, the script repairs the route with
`cloudflared tunnel route dns --overwrite-dns` and retries before reporting the
bot as ready. The VK secret is used only against the local callback URL, not in
public tunnel diagnostics.

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

# DeepInfra DeepSeek-V4-Flash text generation and Seedream image generation.
PROVIDER_CHAIN=deepinfra,mock DEEPINFRA_API_KEY=... go run ./cmd/worker

# Prefer DeepInfra Seedream for image jobs while preserving provider-chain
# fallback.
IMAGE_PROVIDER=deepinfra DEEPINFRA_IMAGE_MODEL=ByteDance/Seedream-4.5 DEEPINFRA_IMAGE_FALLBACK_MODEL=stabilityai/sdxl-turbo IMAGE_SIZE=2K PROVIDER_CHAIN=deepinfra,mock DEEPINFRA_API_KEY=... go run ./cmd/worker

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
`OPENAI_API_KEY`, `PROVIDER=deepinfra` or `IMAGE_PROVIDER=deepinfra` requires
`DEEPINFRA_API_KEY`, and `VK_DELIVERY_MODE=real` requires `VK_ACCESS_TOKEN` in
any environment. `PROVIDER_CHAIN`, `IMAGE_PROVIDER`,
`MODERATION_PROVIDER=openai` and `ARTIFACT_SCANNER=openai` also require the
corresponding provider key when they include/enable that provider.
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
