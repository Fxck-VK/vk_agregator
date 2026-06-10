# TESTING

Operational guide for running the VK AI Aggregator locally and verifying the
full pipeline: `VK webhook → Job → Queue → Worker → Provider → Artifact →
Delivery → Billing Capture`.

## Prerequisites

- Go 1.25+
- Docker + Docker Compose (Postgres, Redis, MinIO)
- `curl` (or any HTTP client)

## Architecture at runtime

Three binaries:

| Binary            | Role                                                        |
| ----------------- | ----------------------------------------------------------- |
| `cmd/migrate`     | Applies SQL migrations from `migrations/`.                  |
| `cmd/api`         | HTTP intake: VK webhook, admin API, `/health`. No provider calls. |
| `cmd/worker`      | Generation / poll / delivery worker pools over Redis Streams. The only place providers are called. |

## Configuration (environment variables)

All have local-dev defaults (see `internal/platform/config/config.go`). For
local handoff, copy the committed template and fill real secrets only in the
ignored `.env` file:

```bash
cp .env.example .env
```

Windows PowerShell:

```powershell
Copy-Item .env.example .env
notepad .env
```

The binaries load `.env` automatically when started from the repository root.
OS/CI environment variables override `.env` values.

| Var                     | Default                                                                                  |
| ----------------------- | ---------------------------------------------------------------------------------------- |
| `HTTP_ADDR`             | `:8080`                                                                                   |
| `DATABASE_URL`          | `postgres://vk_ai_aggregator:vk_ai_aggregator@localhost:5432/vk_ai_aggregator?sslmode=disable` |
| `MIGRATIONS_DIR`        | `migrations`                                                                              |
| `REDIS_ADDR`            | `localhost:6379`                                                                          |
| `S3_ENDPOINT`           | `localhost:9000`                                                                          |
| `S3_ACCESS_KEY`         | `minioadmin`                                                                              |
| `S3_SECRET_KEY`         | `minioadmin`                                                                              |
| `S3_BUCKET`             | `artifacts`                                                                               |
| `VK_CONFIRMATION_TOKEN` | `dev-confirmation`                                                                        |
| `VK_SECRET`             | _(empty = no secret check)_                                                               |
| `VK_APP_ID`             | _(empty)_                                                                                 |
| `VK_APP_SECRET`         | _(empty = dev/mock Mini App signature bypass; required in production)_                    |
| `MINIAPP_LAUNCH_PARAMS_MAX_AGE` | `1h`                                                                             |
| `MINIAPP_JOB_RATE_LIMIT_RPS` / `MINIAPP_JOB_RATE_LIMIT_BURST` | `1` / `5`                                       |
| `ADMIN_TOKEN`           | _(empty = admin API open)_                                                                |
| `PROVIDER`              | `mock`                                                                                    |
| `PROVIDER_CHAIN`        | value of `PROVIDER`                                                                        |
| `DEEPINFRA_API_KEY`     | _(required when DeepInfra provider is enabled)_                                           |
| `DEEPINFRA_TEXT_MODEL`  | `deepseek-ai/DeepSeek-V4-Flash`                                                           |
| `DEEPINFRA_IMAGE_MODEL` | `ByteDance/Seedream-4.5`                                                                  |
| `OPENAI_API_KEY`        | _(required when OpenAI provider/moderation/scanner is enabled)_                           |
| `OPENAI_TEXT_MODEL`     | `gpt-4.1-mini`                                                                             |
| `OPENAI_IMAGE_MODEL`    | `gpt-image-1`                                                                              |
| `OPENAI_VIDEO_MODEL`    | `sora-2`                                                                                   |
| `MODERATION_PROVIDER`   | `keyword`                                                                                  |
| `ARTIFACT_SCANNER`      | `none`                                                                                     |
| `VK_DELIVERY_MODE`      | `mock`                                                                                    |
| `VK_ACCESS_TOKEN`       | _(required when `VK_DELIVERY_MODE=real`; also enables API-side `/start` menu sends)_       |
| `VK_WELCOME_ATTACHMENT` | _(optional VK attachment string for `/start` banner)_                                     |
| `VK_MENU_BUTTON_MODE`   | `callback`                                                                                |
| `VK_UNROUTED_TEXT_MODE` | `reply`                                                                                   |
| `VK_DIALOG_MODE_TTL`    | `1h`                                                                                      |
| `VK_BOT_TUNNEL_MODE`   | `quick` or `named` for local bot dev scripts                                              |
| `VK_BOT_TUNNEL_HOSTNAME` | `vk.neiirohub.ru` for the stable named Cloudflare Tunnel                                |
| `TEXT_CONTEXT_ENABLED` | `true`                                                                                    |
| `TEXT_CONTEXT_MAX_INPUT_TOKENS` / `TEXT_CONTEXT_MAX_OUTPUT_TOKENS` | `1600` / `800`                                      |
| `TEXT_CONTEXT_SUMMARY_MAX_TOKENS` / `TEXT_CONTEXT_RECENT_MESSAGES_LIMIT` | `400` / `6`                                  |
| `VK_MENU_*_ENABLED`     | `true`                                                                                    |
| `VK_ANTISPAM_*`         | enabled; `10/60s` messages, `3/30s` GPT, stricter new-user limits, `2` active GPT jobs    |
| `MAX_ATTEMPTS`          | `3`                                                                                       |
| `SIGNED_DELIVERY`       | `false`                                                                                   |
| `STREAM_MAX_LEN`        | `100000`                                                                                  |
| `WORKER_SHUTDOWN_GRACE` | `30s`                                                                                     |
| `WORKER_METRICS_ADDR`   | `:9090`                                                                                   |
| `OTEL_TRACES_EXPORTER`  | `none` (`stdout` for local trace output)                                                  |

## Startup commands

```bash
# 0. Local env file
cp .env.example .env

# 1. Infrastructure
docker compose up -d
docker compose ps          # postgres/redis healthy; minio running

# 2. Migrations
go run ./cmd/migrate up
go run ./cmd/migrate status   # all "applied"

# 3. API (terminal A)
go run ./cmd/api

# 4. Worker (terminal B)
go run ./cmd/worker
```

The worker auto-creates the MinIO bucket and the Redis consumer groups on
startup, and reclaims un-acked work (restart recovery).

Real integration smoke commands (credential-bound):

```bash
# OpenAI text/image/video provider.
PROVIDER=openai OPENAI_API_KEY=... go run ./cmd/worker

# Provider router: OpenAI primary, mock fallback.
PROVIDER_CHAIN=openai,mock OPENAI_API_KEY=... go run ./cmd/worker

# DeepInfra DeepSeek-V4-Flash text provider and Seedream image provider.
PROVIDER_CHAIN=deepinfra,mock DEEPINFRA_API_KEY=... go run ./cmd/worker

# API-side VK /start menu responses with keyboard.
VK_ACCESS_TOKEN=... go run ./cmd/api

# VK messages.send + raw photo/video upload.
VK_DELIVERY_MODE=real VK_ACCESS_TOKEN=... go run ./cmd/worker

# OpenAI output moderation and text/image artifact scanner.
MODERATION_PROVIDER=openai ARTIFACT_SCANNER=openai OPENAI_API_KEY=... go run ./cmd/worker
```

## Migration commands

```bash
go run ./cmd/migrate up       # apply all pending
go run ./cmd/migrate down     # roll back the most recent
go run ./cmd/migrate status   # list applied/pending
```

## curl examples

Health:

```bash
curl -s localhost:8080/health
# {"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
```

VK confirmation:

```bash
curl -s -X POST localhost:8080/webhooks/vk \
  -H 'Content-Type: application/json' \
  -d '{"type":"confirmation","group_id":1}'
# dev-confirmation
```

VK text mode + message (creates user → control command → text job):

```bash
curl -s -X POST localhost:8080/webhooks/vk \
  -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"text-mode-1","object":{"message":{"from_id":777,"peer_id":777,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}}'
# ok

curl -s -X POST localhost:8080/webhooks/vk \
  -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"text-1","object":{"message":{"from_id":777,"peer_id":777,"text":"hello world"}}}'
# ok
```

VK `/start` menu (creates user → command, but **no job**):

```bash
curl -s -X POST localhost:8080/webhooks/vk \
  -H 'Content-Type: application/json' \
  -d '{"type":"message_new","group_id":1,"event_id":"menu-1","object":{"message":{"from_id":777,"peer_id":777,"text":"/start"}}}'
# ok
```

Expected: command type `start`, no queued job, no billing reservation. If
`cmd/api` is running with `VK_ACCESS_TOKEN`, the peer receives the НейроХаб
welcome text and VK inline keyboard. `VK_WELCOME_ATTACHMENT` may point at a
pre-uploaded VK banner attachment. For a user whose `welcome_name_sent_at` is
empty and whose VK profile can be fetched, the first `Старт` should say
`<first_name>, добро пожаловать в НейроХаб`; later `Старт` / `Показать меню`
responses should use the regular welcome without the name.
Clicking `🎬 Создать видео` should return `Выбери модель для генерации:` with
`Sora 2`, `Kling v2.1`, `Seedance 1`, `Haiuo v0.2`, and `⬅️ Назад`; these
button presses are control commands and should not enqueue jobs.
Clicking `Sora 2` or `Kling v2.1` should open a detail screen with model
description, example prompt, instruction link, `😀 Начать генерацию`,
`ℹ️ Примеры`, and `⬅️ Назад`. Clicking `Seedance 1` should open `Lite` / `Pro`;
clicking `Haiuo v0.2` should open `Обычный` / `Fast`. These nested buttons are
also control commands and should not enqueue jobs.
Clicking `🖼️ Создать фото` should return the daily-free-attempt photo
instruction screen directly with `Фото по тексту`, `Фото с референсом`, and
`⬅️ Назад`; these mode buttons are also control commands. Clicking
`💬 Спросить у НейроХаб` should return `НейроХаб активен` and wait for the next
plain user message; that next text or sticker should create a `text.ask` job.
In active GPT mode, the bot should first send `НейроХаб думает...`; after the
provider result is ready, that same VK message should be edited to the answer
instead of sending a second bot message. This placeholder/edit UX is only for
the button-enabled GPT mode, not for legacy `VK_UNROUTED_TEXT_MODE=gpt`. If the
answer is too long for one VK message, the edited placeholder should contain
the first chunk and the remaining chunks should be sent as deterministic
follow-up text messages. Text answers should be VK plain text: generated
Markdown markers like `**`, backticks and raw `*` list prefixes should not be
visible; list items should appear as `•` bullets.
Plain text outside GPT mode is controlled by `VK_UNROUTED_TEXT_MODE`: `reply`
(default) sends only `Выберите режим в меню выше или нажмите на кнопку показать меню` and creates no job, `silent`
creates no job and sends nothing, `gpt` preserves legacy any-text-to-GPT
behavior.
Clicking `🎁 Студентам и школьникам` should return the study submenu with
`Решальник задач`, `Генерация презентаций (скоро)`,
`Создание рефератов (скоро)`, `❓ Ответы на вопросы`, and `⬅️ Назад`; these
button presses must not enqueue jobs.
For live VK UX, click several inline menu buttons in a row: the bot should edit
the active menu message instead of posting a new bot message each time. Press
the persistent lower `Показать меню` button: the bot should send a fresh menu at
the bottom instead of editing the old one. For a brand-new user, ordinary first
non-payload text/sticker/menu-repair input should open onboarding/welcome instead of replying with a hint.
For an onboarded user, send plain text outside GPT mode: with the default
`reply` setting, the bot should post `Выберите режим в меню выше или нажмите на кнопку показать меню` with the
lower `Показать меню` keyboard, should not attach an inline keyboard, and should
still create no job. Typing `нет меню`, `нет кнопки`, `где меню` or `меню`
should send a fresh welcome menu and restore the lower keyboard. Active-menu
state is process-local to the running API
instance, while GPT dialog mode is Redis-backed and survives API restarts for
`VK_DIALOG_MODE_TTL`.
With `VK_MENU_BUTTON_MODE=callback`, inline menu clicks should not appear as
user messages in the chat. Make sure VK Callback API has callback-button events
(`message_event`) enabled. To verify legacy fallback, set
`VK_MENU_BUTTON_MODE=text`, restart `cmd/api`, and confirm button labels are sent
as user messages again.
The clicked callback button should not keep spinning: `cmd/api` acknowledges
every `message_event` with blank `messages.sendMessageEventAnswer` before
editing/sending the menu.
Feature flag smoke: set `VK_MENU_STUDENTS_ENABLED=false`, restart `cmd/api`,
open `Показать меню`, and confirm `🎁 Студентам и школьникам` is absent while
other main buttons remain. Re-enable it with `true`. The same pattern applies
to all `VK_MENU_*_ENABLED` flags in `.env.example`.

Referral smoke: set `VK_MENU_ACCOUNT_ENABLED=true` and configure
`VK_REFERRAL_LINK_BASE` for the VK community, for example
`https://vk.com/write-239332376`. Open `👤 Мой аккаунт`; the response should
show the `безлимитное общение` note, invited count, one stable referral link,
`Поддержка: @neirohub_help`, and only the `⬅️ Назад` inline button. It should
not render `↗️ Поделиться` / `open_link`, balance or completed-generation rows
in the account screen. Send `/start <code>` from a different VK user or
deliver a Callback API `ref=<code>` param; the handler should persist one
`referrals` row with `source=vk_bot`, create no job, and post signup bonuses
through `ledger_entries` with `reason` containing `referral`.
Repeating the same referral start must not duplicate the relation or reward.
A full Mini App referral account/API screen is not part of this smoke yet.

Image / video jobs (slash commands):

```bash
# image
... "text":"/image a red cat" ...
# video
... "text":"/video a flying car" ...
```

Idempotency check — re-send the **same** `event_id`; no second job is created:

```bash
curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"text-1","object":{"message":{"from_id":777,"peer_id":777,"text":"hello world"}}}'
# ok  (deduped)
```

Admin API (add `-H "X-Admin-Token: $ADMIN_TOKEN"` if set):

```bash
curl -s 'localhost:8080/admin/jobs?limit=20'
curl -s 'localhost:8080/admin/jobs?status=succeeded&operation=text_generate'
curl -s localhost:8080/admin/jobs/<job_id>
curl -s localhost:8080/admin/users/<user_id>
curl -s localhost:8080/admin/deliveries/<delivery_id>
```

## Hardening checks (moderation, DLQ, metrics)

```bash
# VK anti-spam: send more than the configured per-user message limit.
for i in $(seq 1 11); do
  curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
    -d "{\"type\":\"message_new\",\"event_id\":\"spam-$i\",\"object\":{\"message\":{\"from_id\":9010,\"peer_id\":9010,\"text\":\"menu\"}}}"
done
# -> webhook still returns ok; after the limit the VK control path should send
#    "Слишком много сообщений. Попробуйте через N секунд" and no command/job is created.

docker exec vk-ai-aggregator-redis redis-cli TTL rate:vk:user:9010:messages
docker exec vk-ai-aggregator-redis redis-cli GET spam:vk:user:9010:violations

# Moderation REJECT: a banned term blocks delivery
curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"mod-1","object":{"message":{"from_id":9001,"peer_id":9001,"text":"please generate nsfw content"}}}'
# -> job ends in status "rejected", cost_captured = 0, reservation released,
#    one moderation_results row (decision=block), no VK send.

# DLQ + retry budget: a poison provider error is dead-lettered (no infinite loop)
curl -s -X POST localhost:8080/webhooks/vk -H 'Content-Type: application/json' \
  -d '{"type":"message_new","event_id":"dlq-1","object":{"message":{"from_id":9002,"peer_id":9002,"text":"mock_provider_error"}}}'
# -> job ends in status "failed_terminal", cost_captured = 0
docker exec vk-ai-aggregator-redis redis-cli XLEN stream:jobs:dlq   # >= 1

# Metrics (process-local: scrape api and each worker separately)
curl -s localhost:8080/metrics | grep vkagg_

# Inspect moderation audit / DLQ in Postgres
docker exec vk-ai-aggregator-postgres psql -U vk_ai_aggregator -d vk_ai_aggregator \
  -c "SELECT job_id, stage, decision, provider FROM moderation_results ORDER BY created_at DESC LIMIT 5;"
```

Expected: moderation **pass** delivers+captures as the happy path; moderation
**reject** → `rejected` (no charge); poison error → `failed_terminal` + a DLQ
entry. In production (`APP_ENV=production`) the API refuses to start without
`VK_SECRET`/`ADMIN_TOKEN`/`VK_CONFIRMATION_TOKEN`.

## Expected results (happy path)

1. `message_new` → HTTP `200 ok`.
2. User row created (first contact), command row created, billing reservation
   created, job created with status `queued`.
3. Worker consumes the job, calls the mock provider, creates an artifact, and
   enqueues delivery.
4. Delivery worker sends to VK (mock), captures the reservation, sets job
   `succeeded`.
5. `GET /admin/jobs/<id>` → `status: "succeeded"` with `output_artifact_ids`.

## Automated smoke / regression test

The full pipeline is exercised in-memory (no infra required) and is the fastest
way to validate behavior end-to-end:

```bash
go test ./...                                   # whole suite
go test ./internal/worker/ -run TestEndToEnd -v # full VK→…→Capture flow

go test ./internal/adapter/inbound/miniapp ./internal/platform/config
cd web/miniapp && npm ci && npm run build
```

Covered: business flow, webhook/delivery/capture idempotency, provider timeout
/ rate-limit / internal-error classification, retry + terminal transitions, and
restart recovery (consumer-group AutoClaim).

Postgres and Redis integration tests run only when their env vars are set:

```bash
TEST_DATABASE_URL="$DATABASE_URL" go test ./internal/adapter/storage/postgres/...
TEST_REDIS_ADDR="localhost:6379" go test ./internal/adapter/queue/redis/...
```

## Troubleshooting

| Symptom                                   | Cause / fix                                                                 |
| ----------------------------------------- | --------------------------------------------------------------------------- |
| `/health` returns 503                     | Postgres or Redis unreachable. `docker compose ps`; check `DATABASE_URL` / `REDIS_ADDR`. |
| `migrate: connect` error                  | Postgres not ready yet. Wait for `docker compose ps` healthy, retry.        |
| Jobs stay `queued`                        | Worker not running, or pointing at a different Redis. Start `cmd/worker`; check `REDIS_ADDR`. |
| `s3 connectivity check` / bucket error    | MinIO not up or wrong creds. Check `S3_ENDPOINT` / keys; console at `:9001`. |
| Duplicate VK events create duplicate jobs | Ensure VK sends a stable `event_id`; dedup keys are derived from it.         |
| `/start` sends text but no keyboard        | Enable bot features in VK community message settings; VK returns `error_code=912` when keyboards are disabled. |
| Provider always fails                     | Mock provider injects errors on prompts containing `mock_timeout`, `mock_rate_limit`, `mock_provider_error`. Use a normal prompt. |
| `409`/conflict on retry                   | Expected idempotency guard; the operation already succeeded — safe to ignore. |

## Notes / known limitations (MVP)

- Default local runs use the **mock** AI provider and **mock** VK delivery.
- `PROVIDER=openai` enables real OpenAI text/image/video adapters. Live tests
  require a real key and may incur provider cost.
- `PROVIDER=deepinfra` enables real DeepInfra text generation through
  `deepseek-ai/DeepSeek-V4-Flash` and text-to-image through
  `ByteDance/Seedream-4.5`; live tests require a real key and may incur
  provider cost. DeepInfra reference-image generation is still disabled by
  `DEEPINFRA_IMAGE_REFERENCE_ENABLED=false`; keep `mock` or another capable
  provider in `PROVIDER_CHAIN` for video jobs. The mock-aware downloader also
  accepts provider `data:` URLs used by DeepInfra text and image outputs.
- `PROVIDER_CHAIN=openai,mock` exercises router/fallback/circuit breaker logic
  with OpenAI primary and mock fallback.
  `PROVIDER_CHAIN=deepinfra,mock` uses DeepInfra for text/image and mock
  fallback for unsupported/retryable paths.
- `VK_DELIVERY_MODE=real` enables real VK `messages.send` plus generated
  photo/video upload into VK attachment ids.
- VK `/start` menu replies are sent by `cmd/api` through
  `vkdelivery.ControlClient` when `VK_ACCESS_TOKEN` is set. Button clicks are
  control commands and do not create billable jobs without a prompt.
- `MODERATION_PROVIDER=openai` and `ARTIFACT_SCANNER=openai` require
  `OPENAI_API_KEY`; artifact scanning currently covers text/image bytes, while
  video scan/transcode remains part of the future media pipeline.
- `docker compose` brings up Postgres, Redis, MinIO; the app binaries run on the
  host (no app Dockerfile yet).
- `cmd/api` and `cmd/worker` both validate production secrets fail-closed.
  Real OpenAI/DeepInfra/VK modes require their credentials even in local runs.
