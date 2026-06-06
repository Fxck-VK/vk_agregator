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
- [x] Command Router (`/image`, `/video`, `/edit`, `/balance`, `/status`, `/cancel`, `/help`; прочее -> `text_generate` at parser level, with VK inbound gated by active GPT mode / `VK_UNROUTED_TEXT_MODE`).
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
- [x] `.env.example` added as the committed handoff template; local `.env` is ignored by Git and loaded automatically from the repository root.
- [x] `README.md` describes `v0.1.3 / Beta integrations foundation`, default mock runtime and opt-in real integrations.
- [x] `RUNBOOK.md` reflects 3 migrations, DLQ/retry budget, migration checksums and real adapter modes.
- [x] `TESTING.md` no longer calls real adapters stubs and documents current limitations.
- [x] `ROADMAP.md` aligns Phase 2 with actual remaining Beta work.
- [x] `PROGRESS.md` includes Step 8 / v0.1.2 hardening.
- [x] `AUDIT.md` distinguishes fixed hardening/integrations from credential-bound live-smoke follow-ups.
- [x] `AGENTS.md` includes current release status and documentation DoD.
- [x] Bot-only local dev scripts (`scripts/dev/start-bot.ps1`, `status-bot.ps1`, `stop-bot.ps1`) automate VK bot startup without starting the VK Mini App frontend.

### Production hardening follow-up
- [x] `cmd/worker` calls `cfg.Validate()` and fails closed for production, real provider and real VK delivery modes.
- [x] `gofmt -l .` is clean.
- [x] OpenTelemetry trace context is propagated by `traceparent` from VK intake through outbox/Redis to provider, artifact and delivery spans.
- [x] Worker shutdown stops reading new Redis entries first, then drains in-flight handlers with `WORKER_SHUTDOWN_GRACE`.
- [x] Maintenance cleanup covers expired `idempotency_keys`, old terminal `outbox_events` and Redis stream backlog trimming.
- [x] Balance-vs-ledger reconciliation runs periodically and exports `vkagg_billing_mismatches`.

### Real integrations
- [x] DeepInfra provider supports `deepseek-ai/DeepSeek-V4-Flash` text generation through the OpenAI-compatible `/chat/completions` endpoint behind `PROVIDER=deepinfra` / `PROVIDER_CHAIN=deepinfra,mock`.
- [x] Text providers receive an internal instruction in addition to the user's prompt: answer as `НейроХаб бот`, keep replies concise (`<= 3000 characters`), and do not reveal provider/model/backend details; VK delivery still splits long answers as a fallback.
- [x] Mock-aware downloader supports provider `data:` URLs, so `PROVIDER_CHAIN=deepinfra,mock` can store DeepInfra text outputs before VK delivery.
- [x] Реальный OpenAI provider покрывает text (`/responses`), image (`/images/generations`) и async video (`/videos`, poll, content download) behind `PROVIDER=openai`.
- [x] Provider router выбирает capable provider по health/circuit breaker, fallback chain, estimated cost и observed latency; `PROVIDER_CHAIN=openai,mock` включает fallback.
- [x] Реальный VK delivery покрывает `messages.send` и upload pipeline для raw photo/video artifacts в VK upload servers (`photos.getMessagesUploadServer` -> upload -> `photos.saveMessagesPhoto`, `video.save` -> upload).
- [x] Реальный output moderation provider включается через `MODERATION_PROVIDER=openai` и пишет verdict в существующий `moderation_results` flow.
- [x] Реальный artifact scanner включается через `ARTIFACT_SCANNER=openai`; scanner проверяет text/image bytes до storage, video остается задачей полноценного media pipeline.
- [x] VK inbound распознает sticker-only сообщения и превращает их в text prompt; prompt становится `text.ask` job только когда включен GPT text mode или legacy `VK_UNROUTED_TEXT_MODE=gpt`, поэтому стикеры не теряются как пустой текст и не создают случайные jobs вне режима.
- [x] VK onboarding keyboard: первичная нижняя кнопка `Старт` запускает welcome flow; первый обычный non-payload текст/стикер/menu-repair контакт пользователя также открывает onboarding `/start`, а явные slash generation-команды и payload-кнопки не перехватываются; после `Старт` нижняя постоянная клавиатура заменяется на одну кнопку `Показать меню`; нижняя `Показать меню` использует отдельный `show_menu` control-command и всегда отправляет свежее НейроХаб inline menu вниз без повторной переустановки нижней клавиатуры. Кнопки меню не создают пустые billable jobs без промпта; при VK `error_code=912` есть fallback на текст без keyboard.
- [x] VK video menu: кнопка `🎬 Создать видео` открывает inline-экран `Выбери модель для генерации:` с кнопками `Sora 2`, `Kling v2.1`, `Seedance 1`, `Haiuo v0.2` и `⬅️ Назад`; `Sora 2`/`Kling v2.1` открывают detail-экраны с описанием, примером prompt, ссылкой на инструкцию, `Начать генерацию`, `Примеры`, `Назад`; `Seedance 1` открывает `Lite`/`Pro`, `Haiuo v0.2` открывает `Обычный`/`Fast`; все video submenu кнопки пока control-only и не создают billable job.
- [x] VK menu registry: control-экраны описаны декларативно; `🖼️ Создать фото` при одной основной модели сразу открывает инструкцию с режимами `Фото по тексту` / `Фото с референсом`, а `💬 Спросить у НейроХаб` открывает active-сообщение без лишнего выбора модели.
- [x] VK student menu: `🎁 Студентам и школьникам` открывает учебный экран с кнопками `Решальник задач`, `Генерация презентаций (скоро)`, `Создание рефератов (скоро)`, `Ответы на вопросы` и `Назад`; все кнопки пока control-only.
- [x] VK active menu UX: inline menu navigation edits the current menu message via `messages.edit`; plain user text outside GPT mode keeps the previous menu usable and with default `VK_UNROUTED_TEXT_MODE=reply` sends `Выберите режим в меню выше или нажмите на кнопку показать меню` plus the lower `Показать меню` keyboard instead of duplicating the inline menu or creating a billable job. Typed repair phrases `меню`, `нет меню`, `нет кнопки`, `где меню` restore the lower keyboard and reopen the welcome menu. Edit failures fall back to a normal send.
- [x] VK callback menu buttons: inline menu can run with `VK_MENU_BUTTON_MODE=callback`, processing VK `message_event` without user echo messages; `VK_MENU_BUTTON_MODE=text` keeps the legacy text-button fallback. Persistent lower `Показать меню` remains text.
- [x] VK callback button ack: every `message_event` is acknowledged through blank `messages.sendMessageEventAnswer`, so VK client button loading spinner stops after a click.
- [x] VK unrouted text gating: `Спросить у НейроХаб` stores Redis-backed GPT mode for the peer; ordinary text/stickers outside GPT mode are configurable via `VK_UNROUTED_TEXT_MODE=reply|silent|gpt` and do not create jobs by default. Old text label `Спросить у GPT` remains a compatible alias.
- [x] VK GPT pending UX: after `Спросить у НейроХаб`, the next text/sticker sends `НейроХаб думает...`; when the text job is delivered, delivery worker edits that same VK message with the provider answer instead of posting a second bot message. Text delivery formats simple provider Markdown into VK plain text (`**`/backticks/headings stripped, `*`/`-` lists rendered as `•`). Long text answers are split into deterministic follow-up chunks so VK `error_code=914` does not leave the placeholder stuck. Legacy `VK_UNROUTED_TEXT_MODE=gpt` still uses normal text delivery.
- [x] VK first-start personalization: the first `Старт` can fetch the VK first name once via `users.get`, cache it on the user row, send `👋 <name>, добро пожаловать в НейроХаб!`, and then use regular non-personalized welcome text for later menus.
- [x] VK menu feature flags: every main and nested product-menu button has a `VK_MENU_*_ENABLED` env flag; disabled buttons are hidden from new keyboards, stale disabled payload clicks fall back to the current main menu, and no jobs are created.
- [x] VK bot anti-spam: Redis-backed per-`vk_user_id` limits for all incoming user events (`10/60s`, new users `5/60s`), separate GPT job limits (`3/30s`, new users `1/15s`), cooldown replies, repeated-violation temporary blocks (`5/10m -> 15m`), and max 2 active GPT jobs per user before queue protection denies new requests. Anti-spam denials acknowledge the inbound event and do not create commands/jobs.
- [x] VK GPT dialog mode persistence: selected `Спросить у НейроХаб` mode is stored in Redis under peer-scoped dialog state with `VK_DIALOG_MODE_TTL`, so ordinary text keeps routing to GPT after `cmd/api` restart or API instance switch.
- [x] VK text dialog context v1: `cmd/worker` persists user/assistant turns in Postgres (`conversations`, `conversation_messages`, `conversation_summaries`), sends providers a bounded context packet instead of full history, caps text output via provider request when supported, and keeps context assembly out of VK handlers.
- [x] Shared VK referral foundation: Postgres `referral_codes` / `referrals`, one stable public code per internal user, idempotent `/start <code>` / VK `ref` handling in the bot, account screen with `безлимитное общение`, invited count/plain referral link, `@neirohub_help` and `Назад` only, and signup rewards through billing ledger entries. Mini App code was not changed; backend service/repository are ready for a future Mini App referral endpoint.
- [x] VK inbound retry hardening: duplicate `inbound_events.idempotency_key` loads the existing inbound row before status updates, preventing `mark inbound processed: domain: entity not found` and repeated VK retries after partial processing.

---

## VK Mini App (Step 10)

- [x] BFF `/miniapp/*` в `cmd/api` (`internal/adapter/inbound/miniapp`): create/list/get job + balance, переиспользуют `joborchestrator` и существующий биллинг-путь, провайдеры не вызываются.
- [x] Проверка подписи launch-параметров (HMAC-SHA256 по VK-спеке): при заданном `VK_APP_SECRET` подпись валидируется реально, invalid/expired/missing → 401 без деталей, dev-обход отключается; `vk_user_id` только из проверенных параметров.
- [x] Production fail-closed при пустом `VK_APP_SECRET`.
- [x] Ownership: задачи доступны только своему `vk_user_id`.
- [x] Фронт `web/miniapp` (React + VK Bridge, без VKUI): чат-интерфейс (`chat/`), ч/б `theme.css`, слои api/hooks/ui/chat; `X-Launch-Params` из URL; поллинг задач ≥2с с лимитом; медиа только через `artifactUrl` (UUID).
- [x] История чатов Mini App: `localStorage` ключ `vk_miniapp_chats_v1`, шторка `ChatList`, `useChats`, заголовок активного чата и «Новый чат».
- [x] Выбор модальности и модели: сегмент `Текст/Фото/Видео` и dropdown модели в `Composer`, связка с `operation` для `/miniapp/jobs`.
- [x] Графитовая тема Mini App: тёмная палитра `#1A1A1D`, стили `segment`, `model-select`, `drawer`.
- [x] Composer textarea: скрыт нативный scrollbar при сохранении внутренней прокрутки.
- [x] Frontend audit: `docs/AUDIT.md` описывает безопасность, утечки и оптимизацию новых Mini App фич.
- [x] Восстановление `web/miniapp/src/**` из `HEAD` после ручной чистки: целевая чат-структура на месте, legacy `panels`/`screens` не импортируются, `tsc` и `build` зелёные.
- [x] Hardening чат-фронта: cleanup для `bridge.subscribe` через `bridge.unsubscribe`, polling без стартовой задержки и без размножения таймеров, `patchMessage` по id мемоизирован.
- [x] Бэкенд: `GET /miniapp/artifacts/{id}` отдаёт байты артефакта с ownership-проверкой (`art.OwnerUserID == user.ID`), `job.status == succeeded` и passed output moderation guard; `Cache-Control: private`; текст приходит как `text/plain`, фронт читает его через `fetchArtifactText`. Зависит от доступности S3 в `cmd/api` (см. бэклог аудита).
- [x] Frontend submit hardening: `POST /miniapp/jobs` sends stable per-submit `X-Idempotency-Key`; API/network errors normalize to safe user-facing messages; duplicate in-flight submits are guarded.
- [x] Mini App API: frontend sends supported `model_id` only for backend-validated operations. Text chat is branded as public `ChatGPT`; legacy DeepSeek text IDs are normalized to `chatgpt` before persistence/API output and are not exposed in Mini App UI/DTO.
- [x] Mini App estimate before submit: `POST /miniapp/estimate` возвращает backend-owned `cost_estimate`, `balance_credits` и `enough_credits` без создания job/резерва/ledger; фронт показывает стоимость и предупреждение до submit.
- [x] Mini App result UX: результат показывается карточкой «Готовый VK-пост» с plain-text copy, retry action и image/video preview только через backend artifact route.
- [x] Mini App history reload recovery: running jobs восстанавливаются через `GET /miniapp/jobs`, локальная история хранит только `job_id`, `operation_type`, `status`, `created_at` за 7 дней, есть clear local history и privacy note.
- [x] Mini App PR-10 redesign: явные `Chat` / `Workflow` режимы, workflow screens `Home -> Generate -> Status -> Result -> History`, status timeline, VK post preview, design tokens и ADR mode/design direction.
- [x] Mini App PR-14 VKUI hybrid: `@vkontakte/vkui` `8.2.1` added as production dependency; app root uses `ConfigProvider`/`AdaptivityProvider`/`AppRoot`; base controls use VKUI `Button`, `NativeSelect`, `Textarea`, `Panel`, `Tabbar`; custom workflow shell, result preview and status timeline remain custom.
- [x] Mini App PR-16.1 navigation shell: bottom VKUI `Tabbar` with `Создать` / `Чат` / `Настройки`, default center `Чат`, UI-only active tab preference `vk_miniapp_active_tab_v1`; Chat and Workflow stay mounted as tab panels so polling survives tab switches.
- [x] Mini App PR-16.2 chat threads: active `conversation_id` is the thread id (`default` for migrated legacy context, UUID for new dialogs), history opens as a top sheet from the chat title, and `localStorage` keeps only `id` / `title` / `last_activity_at` thread metadata.
- [x] Mini App PR-16.3 Create tab: top VKUI operation segment for supported backend operations (`text_generate`, `image_generate`, `video_generate`), existing estimate/status/result/history workflow preserved, VK post preview made the signature result surface without new URL sources or unsafe rendering.
- [x] Mini App Create UX revision: Create currently exposes only `Создать фото` / `Создать видео`; `Создать пост` is temporarily disabled in this tab, while text generation remains in Chat/VK bot flows. History is scoped per selected operation type; chat thread history opens from an explicit header icon button.
- [x] Mini App PR-16.4 Settings and design polish: Settings contains theme preference, backend balance, summary generation history with type filter, local-history privacy/clear controls and a payment-history placeholder; design tokens now use the provided cyan/violet/pink brand palette.
- [x] Obsolete VK Tunnel tooling removed: `@vkontakte/vk-tunnel`, npm `tunnel` script and `web/miniapp/vk-tunnel-config.json`; dev tunnel path normalized to `cloudflared` / `*.trycloudflare.com`.
- [x] Dev-туннель через `cloudflared` (VK Tunnel на техработах с 02.10.2025): `vite.config.ts` `server` — `host: true`, `allowedHosts: true`, `hmr.protocol: wss`/`clientPort: 443`, proxy `/miniapp`+`/api` → `:8080`; mixed-content под https устранён, домен туннеля не хардкодится. E2E (mock) через прокси-эндпоинты проверен.
- [x] Фикс биллинга (AUDIT B1a): стартовый грант 1000 создаётся committed-проводкой в ledger атомарно; миграция `000004` бэкоффилит открывающие проводки; mismatch устранён.
- [ ] Получить https-URL `cloudflared` (`cloudflared tunnel --protocol http2 --url http://localhost:5173`) и вписать его в dev.vk.com → «Версия для vk.com» → «URL для разработки». Ручной шаг оператора — URL меняется каждый запуск.

---

## Бэклог по аудиту (`docs/REVIEW.md`)

Полный аудит безопасности/архитектуры — `docs/REVIEW.md` (read-only ревью от
2026-06-04). Код по аудиту **не правился**; ниже — приоритезированный бэклог
фиксов. Не исправлять «заодно» — отдельными задачами.

- [x] **[High] Rate-limiting на `/miniapp/*`** (`cmd/api/main.go:158`). Сейчас
  per-IP лимитер навешен только на `/webhooks/vk`; `POST /miniapp/jobs` создаёт
  биллируемые Job без ограничения частоты. Обернуть `miniapp.Routes()` в
  `ratelimit` (ключ по `vk_user_id`/IP, отдельные RPS/Burst, минимум на `POST /jobs`).
  Fixed for `POST /miniapp/jobs`: verified `vk_user_id` key, separate env RPS/Burst,
  safe `429` + `Retry-After`, deterministic tests.
- [x] **[Medium] Fail-closed проверка `vk_ts`** (`internal/adapter/inbound/miniapp/sign.go`).
  Реализовано: при `maxAge > 0` пустой, битый, future или expired `vk_ts` отклоняется до job creation; клиент получает безопасный `401`.
- [x] **[Medium] Проброс выбора модели на бэкенд**.
  Frontend sends supported `model_id`; backend contract реализован: `POST /miniapp/jobs` принимает optional `model_id`, валидирует по operation whitelist, сохраняет only supported/normalized values in job params и не раскрывает selector/model_id в job API responses. Mini App chat uses public `ChatGPT` alias; real DeepSeek provider/model details stay behind worker/provider config. Worker/provider routing по выбранной модели остаётся отдельным follow-up.
- [ ] **[Medium] Мягкая деградация `getArtifact` при недоступности S3**
  (`cmd/api/main.go:88-92`, `handler.go:369-373`). Сейчас при сбое подключения к
  S3 `objectStore == nil` и роут молча отдаёт `503`, хотя Job успешен. В проде —
  считать S3 обязательной зависимостью (падать/алертить) либо явно отражать
  недоступность артефактов в UI; задокументировать связность `api ↔ S3`.
- [ ] **[Low→Medium] Развязать `mountedRef` и перезапуск эффекта**
  (`web/miniapp/src/chat/ChatScreen.tsx:177-231`). Главный `useEffect` завязан на
  `chats.length` и сбрасывает `mountedRef.current = false` при каждом перезапуске,
  смешивая «размонтирован» и «эффект перезапущен». Держать флаг mount/unmount в
  отдельном `useEffect(() => {...}, [])`.
- [ ] **[Low] Constant-time сравнение `ADMIN_TOKEN`**
  (`internal/adapter/inbound/admin/handler.go:61`) — заменить `!=` на
  `subtle.ConstantTimeCompare`/`hmac.Equal`.
- [ ] **[Low] Составной индекс `jobs (user_id, created_at DESC)`** под `ListByUser`
  (сейчас отдельные индексы `user_id` и `status`; сортировка по `created_at`).
- [ ] **[уточнить] CORS-политика** — зависит от модели развёртывания (same-origin
  proxy vs прямой доступ). Не подтверждается кодом, требует решения.
- [x] **[уточнить] Retention/шифрование контента в `localStorage`**
  Fixed in PR-9: `vk_miniapp_chats_v1` keeps only `job_id`, `operation_type`,
  `status`, `created_at` for 7 days; prompt bodies, generated text and artifact
  URLs are not persisted, and clear local history removes only local UI state.

---

## Current Gaps / Known Follow-Ups

### PR-17 app surface refactor

- [x] PR-17.2: Extract VK bot API wiring into `internal/app/vkbot` without
  changing VK callback/menu/dialog/anti-spam/referral behavior. Keep
  `joborchestrator`, `billingservice`, provider adapters, workers, domain and
  storage in shared backend core.
- [x] PR-17.3: Extract Mini App BFF wiring into `internal/app/miniapp` without
  changing `/miniapp/*` contracts, VK launch-param auth, rate limiting,
  estimate, jobs, chat, balance or artifact ownership behavior.
- [x] PR-17.4: Simplify `cmd/api/main.go` into thin bootstrap: config,
  tracing, DB/Redis/S3, shared repos/services, app module mounting,
  admin/health/metrics and graceful shutdown.
- [x] PR-17.5: Update architecture and runbook docs so future agents know where
  to add VK bot commands, Mini App BFF endpoints and shared backend-core logic.

### PR-18 durable shared chat context

- [x] PR-18.1: Add durable conversation identity foundation for shared chat
  context: `source`, Mini App opaque thread id, repository methods and indexes,
  while keeping VK bot `user_id + vk_peer_id` behavior backward compatible.
- [x] PR-18.2: Teach worker/dialogcontext to prefer explicit conversation
  references from text job params and preserve VK bot fallback. Introduce a
  small shared chat job contract/facade only if it avoids duplication without
  calling providers.
- [x] PR-18.3: Switch Mini App `/miniapp/chat/messages` from process-local BFF
  context to durable shared chat core. Remove prompt-prefix memory from the
  BFF and keep `conversation_id="" -> default` compatibility.
- [ ] PR-18.4: Add authenticated Mini App conversation list/history endpoints
  and make the frontend treat backend history as source of truth while keeping
  only active thread/UI preferences in localStorage.
- [ ] PR-18.5: Cleanup and verify shared chat context rollout: no Mini App
  process-local context, no provider calls outside worker, no prompt/answer text
  in localStorage, public model alias remains `ChatGPT`.

### Integration validation / next providers
- [~] Stable local VK Callback URL: `scripts/dev/setup-cloudflare-tunnel.ps1`
  and `start-bot.ps1 -TunnelMode named` are implemented for
  `https://vk.neiirohub.ru/webhooks/vk`; manual Cloudflare DNS activation and
  registrar NS switch are still required before the hostname works.
- [ ] Mini App payment history endpoint: add a read-only `/miniapp/payments` or ledger-history BFF endpoint with auth/rate limiting so Settings can show real payment history instead of the PR-16.4 placeholder.
- [ ] Mini App/VK bot top-up backend flow: add an authenticated, rate-limited and idempotent payment-intent endpoint for Mini App top-ups, connect VK `Пополнить баланс` to the same intent/link flow, and append committed `topup` ledger entries only after trusted payment confirmation.
- [ ] Mini App backend conversations: superseded by the PR-18 durable shared
  chat context plan above. PR-16.2 currently degrades to safe local metadata
  only; backend process-local context can be lost on API restart or scale-out.
- [ ] Live smoke with `DEEPINFRA_API_KEY`: GPT text mode should return DeepSeek-V4-Flash output through the normal Job -> Artifact -> Delivery flow.
- [ ] Add production retention/archival job for old `conversation_messages` before large-scale rollout; keep compact summaries and recent hot turns only.
- [ ] Replace local/extractive dialog summary compaction with a dedicated cheap summarizer job/model if semantic summaries become necessary.
- [ ] Add full Mini App referral endpoint/UI over the shared `referralservice` so VK Bot and VK Mini App use the same code/link/reward state beyond the current experimental share bridge.
- [ ] Live smoke с реальными `OPENAI_API_KEY` и `VK_ACCESS_TOKEN`: text/image/video generation, VK photo/video upload, moderation allow/block.
- [ ] Подключить production-баннер к `/start` через `VK_WELCOME_ATTACHMENT` или отдельный upload flow.
- [x] Bot features включены в настройках сообщений VK-сообщества; VK начал принимать keyboard без `error_code=912`.
- [ ] В VK Callback API включить event type для callback-кнопок (`message_event`) перед live-тестом callback menu mode.
- [ ] Перевести VK control/menu responses в persisted delivery/outbox, если product/control sends должны строго попадать под invariant `Every delivery attempt is persisted`.
- [ ] Вынести active-menu tracking из памяти `cmd/api` в persisted conversation state перед multi-instance deploy, чтобы `messages.edit` переживал рестарты и балансировку. GPT dialog mode уже вынесен в Redis через `VK_DIALOG_MODE_TTL`.
- [~] Добавить второго реального provider для fallback не только на mock: DeepInfra text is implemented; Google/Gemini image или Kling/video остаются follow-up.
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
