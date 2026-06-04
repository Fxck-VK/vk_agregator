# TASKS

Бэклог и трекинг работ по VK AI Aggregator.

Источник истины по архитектуре — `docs/ARCHITECTURE.md`, строгие инварианты —
`AGENTS.md`, журнал разработки — `PROGRESS.md`, production-аудит — `AUDIT.md`,
план фаз — `ROADMAP.md`.

Текущий релиз: **v0.1.3 / Beta integrations foundation**.

Легенда: `[x]` сделано, `[~]` частично, `[ ]` запланировано.

---

## Done

### Step 1 — Domain, repository interfaces, migration 000001
- [x] Domain-сущности (`User`, `Command`, `Job`, `ProviderTask`, `Artifact`, `Delivery`, `Billing`).
- [x] Стейт-машина job (`jobTransitions`, `CanTransitionTo`, `IsTerminal`).
- [x] Интерфейсы репозиториев + доменные ошибки.
- [x] Миграция `000001_init_schema`.

### Step 2 — PostgreSQL adapters, Command Router
- [x] `pgx/v5` репозитории всех сущностей (+ `Querier`, `RunInTx`, `mapError`).
- [x] Command Router (`/image`, `/video`, `/edit`, `/balance`, `/status`, `/cancel`, `/help`; прочее -> `text_generate`).
- [x] Env-guarded integration-тесты репозиториев.

### Step 3 — Billing, Orchestrator, VK Webhook
- [x] Billing Service (estimate/ensure/reserve/capture/release/refund, старт-баланс 1000).
- [x] Job Orchestrator (`Command -> Estimate -> Reserve -> Job -> Outbox`).
- [x] VK Webhook `POST /webhooks/vk` (confirmation + `message_new`, идемпотентность, без вызова Provider).
- [x] `InboundEvent` + миграция `000002`, `uow.Manager`, `queue.Publisher`, in-memory адаптеры.

### Step 4 — Queue, Providers, Artifacts
- [x] Redis Streams + consumer groups: text/image/video/delivery/provider_poll streams, `Publisher`/`Consumer`.
- [x] `MockProvider` (Estimate/Submit/Poll/Cancel) + ошибки `mock_timeout`/`mock_rate_limit`/`mock_provider_error`.
- [x] Artifact Service (`SaveTextArtifact`/`SaveBytesArtifact`/`SaveRemoteArtifact`, дедуп по sha256).
- [x] S3/MinIO adapter (`minio-go/v7`: EnsureBucket/Put/GetObject/PresignedGetURL/retention).
- [x] In-memory `ArtifactRepo` + `ObjectStore`, unit-тесты провайдера и artifact-сервиса, env-guarded Redis-тест.

### Step 5 — Workers и Provider Poll
- [x] `GenerationWorker` (text/image/video): `Job -> Provider -> ProviderTask -> Artifact -> Delivery Queue`.
- [x] `PollWorker`: `Poll -> Update Status -> Requeue -> Artifact -> Delivery Queue`.
- [x] `Engine` (Read/Ack/Recover через `XAUTOCLAIM`), `Registry`, in-memory `ProviderTaskRepo`.
- [x] Retry safety, idempotency (active-task + per-attempt key), error classification, recovery после рестарта.
- [x] Провайдер вызывается только внутри воркера; unit-тесты sync/async/idempotency/retry/terminal.

### Step 6 — VK Delivery, Admin API, E2E
- [x] `vkdelivery.Client` + `MockClient` (`SendText`/`SendPhoto`/`SendVideo`), deterministic `random_id`, no duplicate sends.
- [x] Delivery worker: `Artifact -> Delivery -> Billing Capture -> Job Success`, идемпотентность доставки.
- [x] `billingservice.CaptureForJob` + `BillingRepository.GetReservationByJob` (postgres + memory).
- [x] Admin API: `GET /admin/jobs`, `/admin/jobs/{id}`, `/admin/users/{id}`, `/admin/deliveries/{id}` (pagination/filters/DTO) + `JobRepository.List`.
- [x] Полный in-memory E2E `VK -> Job -> Queue -> Provider -> Artifact -> Delivery -> Capture`; README + troubleshooting.

### Step 7 — Runnable entrypoints и live smoke
- [x] `internal/platform/config`: env-конфигурация с local-dev defaults.
- [x] `cmd/migrate`: `up`/`down`/`status`, `schema_migrations`, checksum tracking.
- [x] `cmd/api`: VK webhook, Admin API, `/health`, `/healthz`, `/metrics`.
- [x] `cmd/worker`: generation/poll/delivery workers, consumer groups, bucket setup, recovery.
- [x] `TESTING.md`: запуск, миграции, curl-примеры, happy path, hardening checks.
- [x] Live-валидация against Postgres/Redis/MinIO: text/image/video jobs reach `succeeded`.

### v0.1.2 hardening
- [x] Output moderation перед delivery (`moderationservice`, `moderation_results`, миграция `000003`).
- [x] DLQ + hard retry budget для provider/download/delivery failures (`stream:jobs:dlq`).
- [x] Outbox relay: `outbox_events` -> Redis Streams -> mark published.
- [x] Atomic job creation + reserve + outbox through transaction-bound `BillingRepository`.
- [x] Fail-closed config validation for production in API startup.
- [x] SSRF protection in artifact downloader; optional egress allowlist.
- [x] Per-IP webhook rate limit.
- [x] Prometheus metrics (`/metrics`, `vkagg_*` counters/histograms).
- [x] Migration checksums and transactional migration apply/rollback.
- [x] S3 retention, signed artifact URLs and scanner hook.
- [x] Price overrides through `PRICES` and per-job cap through `MAX_JOB_COST`.
- [x] OpenAI image provider adapter behind `PROVIDER=openai` (requires real key).
- [x] Real VK `messages.send` adapter behind `VK_DELIVERY_MODE=real` (requires real token).

### Documentation sync
- [x] `README.md` describes `v0.1.3 / Beta integrations foundation`, default mock runtime and opt-in real integrations.
- [x] `RUNBOOK.md` reflects 3 migrations, DLQ/retry budget, migration checksums and real adapter modes.
- [x] `TESTING.md` no longer calls real adapters stubs and documents current limitations.
- [x] `ROADMAP.md` aligns Phase 2 with actual remaining Beta work.
- [x] `PROGRESS.md` includes Step 8 / v0.1.2 hardening.
- [x] `AUDIT.md` distinguishes fixed hardening/integrations from credential-bound live-smoke follow-ups.
- [x] `AGENTS.md` includes current release status and documentation DoD.

### Production hardening follow-up
- [x] `cmd/worker` calls `cfg.Validate()` and fails closed for production, real provider and real VK delivery modes.
- [x] `gofmt -l .` is clean.
- [x] OpenTelemetry trace context is propagated by `traceparent` from VK intake through outbox/Redis to provider, artifact and delivery spans.
- [x] Worker shutdown stops reading new Redis entries first, then drains in-flight handlers with `WORKER_SHUTDOWN_GRACE`.
- [x] Maintenance cleanup covers expired `idempotency_keys`, old terminal `outbox_events` and Redis stream backlog trimming.
- [x] Balance-vs-ledger reconciliation runs periodically and exports `vkagg_billing_mismatches`.

### Real integrations
- [x] Реальный OpenAI provider покрывает text (`/responses`), image (`/images/generations`) и async video (`/videos`, poll, content download) behind `PROVIDER=openai`.
- [x] Provider router выбирает capable provider по health/circuit breaker, fallback chain, estimated cost и observed latency; `PROVIDER_CHAIN=openai,mock` включает fallback.
- [x] Реальный VK delivery покрывает `messages.send` и upload pipeline для raw photo/video artifacts в VK upload servers (`photos.getMessagesUploadServer` -> upload -> `photos.saveMessagesPhoto`, `video.save` -> upload).
- [x] Реальный output moderation provider включается через `MODERATION_PROVIDER=openai` и пишет verdict в существующий `moderation_results` flow.
- [x] Реальный artifact scanner включается через `ARTIFACT_SCANNER=openai`; scanner проверяет text/image bytes до storage, video остается задачей полноценного media pipeline.
- [x] VK inbound распознает sticker-only сообщения и превращает их в text job prompt, чтобы стикеры не терялись как пустой текст.
- [x] VK onboarding keyboard: первичная нижняя кнопка `Старт` запускает welcome flow; после `Старт` нижняя постоянная клавиатура заменяется на одну кнопку `Показать меню`; `Показать меню` использует отдельный `show_menu` control-command и открывает Super GPT inline menu без повторной переустановки нижней клавиатуры. Кнопки меню не создают пустые billable jobs без промпта; при VK `error_code=912` есть fallback на текст без keyboard.

---

## VK Mini App (Step 10)

- [x] BFF `/miniapp/*` в `cmd/api` (`internal/adapter/inbound/miniapp`): create/list/get job + balance, переиспользуют `joborchestrator` и существующий биллинг-путь, провайдеры не вызываются.
- [x] Проверка подписи launch-параметров (HMAC-SHA256 по VK-спеке): при заданном `VK_APP_SECRET` подпись валидируется реально, invalid/expired/missing → 401 без деталей, dev-обход отключается; `vk_user_id` только из проверенных параметров.
- [x] Production fail-closed при пустом `VK_APP_SECRET`.
- [x] Ownership: задачи доступны только своему `vk_user_id`.
- [x] Фронт `web/miniapp` (React + VKUI + VK Bridge): монохромный ч/б UI без дешёвых эмодзи, моно-иконки; экраны список/создание/детали/баланс.
- [x] VK Tunnel (`@vkontakte/vk-tunnel`) + npm-скрипт `tunnel` для запуска внутри VK.
- [x] Dev-туннель через `cloudflared` (VK Tunnel на техработах с 02.10.2025): `vite.config.ts` `server` — `host: true`, `allowedHosts: true`, `hmr.protocol: wss`/`clientPort: 443`, proxy `/miniapp`+`/api` → `:8080`; mixed-content под https устранён, домен туннеля не хардкодится. E2E (mock) через прокси-эндпоинты проверен.
- [x] Фикс биллинга (AUDIT B1a): стартовый грант 1000 создаётся committed-проводкой в ledger атомарно; миграция `000004` бэкоффилит открывающие проводки; mismatch устранён.
- [ ] Получить https-URL `cloudflared` (`cloudflared tunnel --protocol http2 --url http://localhost:5173`) и вписать его в dev.vk.com → «Версия для vk.com» → «URL для разработки». Ручной шаг оператора — URL меняется каждый запуск.

---

## Current Gaps / Known Follow-Ups

### Integration validation / next providers
- [ ] Live smoke с реальными `OPENAI_API_KEY` и `VK_ACCESS_TOKEN`: text/image/video generation, VK photo/video upload, moderation allow/block.
- [ ] Подключить production-баннер к `/start` через `VK_WELCOME_ATTACHMENT` или отдельный upload flow.
- [x] Bot features включены в настройках сообщений VK-сообщества; VK начал принимать keyboard без `error_code=912`.
- [ ] Перевести VK control/menu responses в persisted delivery/outbox, если product/control sends должны строго попадать под invariant `Every delivery attempt is persisted`.
- [ ] Добавить второго реального provider для fallback не только на mock: Google/Gemini image или Kling/video.
- [ ] Расширить VK inbound/media pipeline для photo/video/audio attachments: сохранять входящие вложения как input Artifacts, ffmpeg probe/transcode, malware scan, VK-ready variants.

### Worker reliability
- [ ] Закрыть resume edge-case: если `provider_task.status=succeeded`, но artifact/result_ready еще не сохранены после crash, poll/generation worker должен восстановить pipeline из сохраненного result.
- [ ] Добавить admin tooling для DLQ inspect/replay.
- [ ] Добавить выбор worker pools через env/flag (`WORKER_POOLS=text,image,delivery`) для независимого масштабирования.

### Product / Phase 3+
- [ ] Kling/video provider + async webhook receiver (`cmd/provider-webhook`).
- [ ] Media pipeline: download, scan, ffmpeg transcode, VK-ready variants.
- [ ] Pricing rules table, daily/user/provider/global spend caps, budget alerts.
- [ ] Backups/restore drills, staging, CI/CD, deployment manifests.
