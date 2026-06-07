# PROGRESS

## VK welcome banner

Status: **configured locally for the main VK bot panel**.

- The provided НейроХаб PNG banner was uploaded to VK as a message photo and
  wired through local `.env` `VK_WELCOME_ATTACHMENT`.
- The existing menu contract already scopes the attachment to the main
  welcome/menu screens only: `Старт`, `Показать меню` and menu repair.
- Photo/GPT/account/student/video submenu screens do not receive this banner.
- `.env.example` intentionally keeps `VK_WELCOME_ATTACHMENT` empty; each
  environment should use its own pre-uploaded VK attachment string.

---

## VK bot photo UX and delivery

Status: **done for text-to-image prompt mode; manual VK smoke remains user-run**.

- `VK_MENU_IMAGE_ENABLED=true` in `.env.example` exposes the `Создать фото`
  button while preserving per-button feature flags.
- Clicking `Создать фото` stores Redis-backed `photo_text` mode for the VK peer.
  The next plain text creates an `image.generate` Job
  through `joborchestrator`; VK handlers still do not call image providers.
- The API sends `НейроХаб рисует...` as a control placeholder and stores its VK
  `message_id` in `job.Params`.
- Successful image jobs continue through worker -> provider -> Artifact ->
  delivery, and the delivery worker sends the ready image as VK photo media.
- Terminal image provider failures release the reservation before delivery and
  send/edit a short "funds were not charged" notice instead of capturing
  credits.
- `Фото с референсом` is hidden by `VK_MENU_IMAGE_REFERENCE_ENABLED=false`
  until incoming VK photo attachments are saved as owned input Artifacts.
- The current VK bot profile allows 100 free text-to-image attempts per user per
  24h window through `VK_ANTISPAM_IMAGE_DAILY_LIMIT=100` and
  `PRICES=image_generate=0`.
- Mini App code was not changed.

---

## DeepInfra Seedream image adapter

Status: **done for text-to-image; reference-image flow remains follow-up**.

- Added provider-agnostic image request/result contracts in `internal/domain`:
  `ImageGenerationRequest` and `ImageGenerationResult`.
- Image jobs now carry worker-side fields for model, size, aspect ratio,
  reference artifact ids and provider-safe input URLs through the generic
  `ProviderRequest`.
- `cmd/worker` can prefer an image provider with `IMAGE_PROVIDER` and attach
  worker-only `IMAGE_MODEL` / `IMAGE_SIZE` defaults while preserving
  `PROVIDER_CHAIN` fallback.
- OpenAI image generation now respects per-job `model_code` and `size` when
  they are provided by the worker request; mock image routing is wildcarded for
  local/dev fallback.
- DeepInfra now supports text-to-image through the native
  `/v1/inference/{model}` endpoint with `DEEPINFRA_IMAGE_MODEL` defaulting to
  `ByteDance/Seedream-4.5`; the Seedream default size is provider-native `2K`.
- `DEEPINFRA_IMAGE_FALLBACK_MODEL` can name a second DeepInfra image model for
  retryable primary-model submit failures; this fallback is handled only inside
  the worker/provider adapter and is not exposed to VK or Mini App surfaces.
- DeepInfra image responses are normalized into `ImageGenerationResult` and
  then into provider `data:image/...` output URLs so the existing artifact
  storage flow remains unchanged.
- DeepInfra image HTTP failures map to the existing provider error classes
  through the same adapter error taxonomy as text generation.
- VK bot and Mini App surfaces were not changed to call providers directly:
  image generation still goes through Job -> worker -> provider -> Artifact.
- DeepInfra reference-image generation is intentionally fail-closed behind
  `DEEPINFRA_IMAGE_REFERENCE_ENABLED=false` until the provider reference-image
  API contract is verified and wired with artifact ownership/signed URL checks.

---

## Shared VK referral foundation

Status: **done for VK bot backend; Mini App integration remains follow-up**.

- Added shared referral domain objects, repository interface and Postgres
  migration `000007_referrals` with `referral_codes` and `referrals`.
- Each internal user gets one stable public referral code. The code is shared
  by VK Bot and future VK Mini App flows; it does not expose `vk_user_id` or the
  internal UUID.
- VK Bot `/start <code>` and Callback API `ref` values are parsed as control
  flow: they apply the referral relation idempotently, create no billable Job
  and call no provider.
- The VK bot account screen currently shows the "безлимитное общение" note,
  invited count, a single plain-text referral link and `@neirohub_help`.
  The account keyboard keeps only `Назад`; the VK share/open-link button is not
  rendered in the bot UI.
- Referral signup bonuses use `billingservice.Grant`, which writes committed
  ledger top-up entries with idempotency keys. No direct balance mutation was
  added.
- Full Mini App referral account/API screens remain follow-up work over the
  shared `referralservice`/repository.

---

## VK text dialog context

Status: **done**.

- Added Postgres-backed dialog memory for VK `text.ask` jobs:
  `conversations`, `conversation_messages`, `conversation_summaries`.
- `cmd/worker` now saves the current user prompt before provider submit, renders
  a bounded context packet from bot profile + rolling summary + recent messages
  + current request, and saves the assistant answer after provider success.
- Text context is not assembled in VK handlers; they still only create Jobs.
- Defaults are env-configurable: input `1600` estimated tokens, output `800`,
  summary `400`, recent messages `6`, summary thresholds `10` messages or
  `1500` estimated tokens.
- OpenAI and DeepInfra adapters now receive provider max-output token caps when
  configured and return normalized text in `ProviderTaskResult.Text` for dialog
  history persistence while still saving text outputs as Artifacts.
- Summary compaction is local/extractive in this beta to avoid extra billable
  provider calls. Semantic model-based summaries and old-message retention are
  tracked as follow-up work.

---

## VK GPT dialog mode persistence

Status: **done**.

- `Спросить у НейроХаб` now stores selected GPT mode in Redis-backed dialog
  state by `peer_id`.
- Dialog mode survives `cmd/api` restart or switching to another API instance.
- `VK_DIALOG_MODE_TTL` controls retention; default is `1h` and refreshes while
  the user keeps chatting in the selected mode.
- The handler keeps process-local mode only as a cache/fallback. `Назад`,
  `Показать меню` and other menu transitions clear both local and Redis state.
- Active-menu message tracking is still process-local and remains a separate
  follow-up for multi-instance `messages.edit` resilience.

---

## VK bot anti-spam

Status: **done**.

- Added Redis-backed per-`vk_user_id` anti-spam for VK bot intake.
- Limits implemented:
  - all incoming user events: `40/60s`;
  - new users during first `4h`: `30/60s`;
  - billable GPT/text jobs: `3/30s`;
  - new-user GPT/text jobs: `1/15s`;
  - cooldown after violations: `30s`;
  - repeated violations: `5/10m -> 15m` temporary block;
  - active GPT/text jobs: max `2` per user.
- Denied events are acknowledged through the VK control path, the inbound event
  is marked processed, idempotency is completed, and no command/job is created.
- Added env knobs in `.env.example`, runtime docs, unit tests and VK handler
  coverage.

---

## VK bot local operations

Status: **done**.

- Added bot-only PowerShell dev scripts under `scripts/dev/`:
  - `start-bot.ps1` starts Docker dependencies, applies migrations, builds and starts `cmd/api` + `cmd/worker`, starts `cloudflared`, and prints the VK Callback URL.
  - `status-bot.ps1` reports tracked process state, API/worker health, Docker dependency status, and the current callback URL.
  - `stop-bot.ps1` stops API, worker and tunnel processes, with optional `-StopDocker` for local containers.
- Runtime pid/log/url files go to `.runtime/vk-bot/`, which is ignored by Git.
- Scope is VK bot hand testing only; the VK Mini App frontend is not started by these scripts.
- Added optional named Cloudflare Tunnel setup for a stable local VK Callback:
  `scripts/dev/setup-cloudflare-tunnel.ps1` creates/reuses `neiirohub-vk-bot`,
  writes local tunnel config under `.runtime/vk-bot/cloudflared/`, and supports
  `.\scripts\dev\start-bot.ps1 -TunnelMode named` for
  `https://vk.neiirohub.ru/webhooks/vk`.
- `start-bot.ps1 -TunnelMode named` now validates local VK confirmation plus
  public `/health`, and repairs stale Cloudflare DNS routes with
  `cloudflared tunnel route dns --overwrite-dns` before declaring the bot ready.
  Public tunnel diagnostics do not send `VK_SECRET`.
- External prerequisite remains manual: `neiirohub.ru` must be active in
  Cloudflare DNS and registrar NS records must point to Cloudflare.

---

Журнал прогресса по разработке VK AI Aggregator (Go backend, AI Job Processing Platform).
Источник истины по архитектуре — `docs/ARCHITECTURE.md`, инварианты — `AGENTS.md`.

---

## Integration — Mini App + VK bot backend merge

Статус: **завершён**.

### Что сделано

- Принят semantic merge plan между `feature/integration-web-backend` и `feature/vk-miniapp`.
- Сохраняется единый backend platform: VK bot и Mini App идут через `/webhooks/vk` и `/miniapp/*`, но создают jobs через `joborchestrator`.
- Приняты интеграционные решения: frontend sends `model_id`, BFF validates it; `GET /miniapp/artifacts/{id}` fails closed unless job is `succeeded` and output moderation passed; VK control/menu sends остаются documented control-path exception.
- Проверки интеграции зелёные: backend `go build/test/vet`, Mini App `npm ci` + `npm run build`, `git diff --check`.

---

## Step 1 — Domain layer, repository interfaces, initial migration

Статус: **завершён**.

### Что сделано

- Реализован Domain-слой в `internal/domain/` (единый пакет `domain`, плоские файлы):
  - `user.go` — `User`, `Role` (`user`/`moderator`/`admin`), `Status` (`active`/`blocked`/`banned`/`deleted`).
  - `job.go` — `Job`, `Modality`, `OperationType`, `JobStatus` со **всеми статусами из архитектуры**
    (`received` … `refunded`) и стейт-машиной переходов (`jobTransitions`, `CanTransitionTo`, `IsTerminal`).
  - `command.go` — `Command`, `CommandType` (+ `CreatesJob` для отделения биллинговых команд от управляющих).
  - `provider.go` — `ProviderTask`, `ProviderTaskStatus`, `ProviderRequest`, `ProviderTaskRef`,
    `CostEstimate`, `Capability`, `ProviderErrorClass` и **интерфейс `Provider`**
    (`Name`, `Capabilities`, `Estimate`, `Submit`, `Poll`, `Cancel`).
  - `artifact.go` — `Artifact`, `ArtifactVariant`, `MediaType`, `ArtifactKind`, `VariantType`, `ArtifactStatus`.
  - `delivery.go` — `Delivery`, `DeliveryStatus`, `DeliveryType` (с `vk_random_id` для дедупликации отправок).
  - `billing.go` — `CreditAccount`, `LedgerEntry`, `CreditReservation` (+ типы движений и статусы ledger).
- Описаны интерфейсы репозиториев в `internal/domain/repositories.go`:
  `UserRepository`, `JobRepository`, `ArtifactRepository`, `DeliveryRepository`,
  `BillingRepository`, `OutboxRepository`, `IdempotencyRepository`.
  Все методы принимают `context.Context` первым аргументом.
  Добавлены вспомогательные типы (`OutboxEvent`, `IdempotencyRecord`) и доменные ошибки
  (`ErrNotFound`, `ErrConflict`, `ErrInsufficientCredits`).
- Создана первая миграция `migrations/000001_init_schema.up.sql` / `.down.sql`:
  таблицы `users`, `commands`, `jobs`, `provider_tasks`, `artifacts`, `artifact_variants`,
  `deliveries`, `credit_accounts`, `credit_reservations`, `ledger_entries`,
  `outbox_events`, `idempotency_keys`.

### Ключевые решения

- **Единый пакет `domain` с плоскими файлами** вместо подпапок из `ARCHITECTURE.md` — упрощает обмен
  общими типами между сущностями и репозиториями без циклических импортов.
- **UUID-идентификаторы** через `github.com/google/uuid` как идиоматичный выбор; в SQL — `gen_random_uuid()`
  (расширение `pgcrypto`).
- **Стейт-машина job** вынесена в код (`jobTransitions`) — поддерживает инвариант «every job status
  transition is explicit».
- **Биллинг = append-only ledger**: баланс (`balance_cached`) — это проекция, прямая мутация запрещена;
  списания идут только через `ledger_entries` и `credit_reservations`.
- **Идемпотентность** заложена на уровне схемы — уникальные `idempotency_key` у jobs, commands,
  provider_tasks, deliveries, ledger_entries, reservations + таблица `idempotency_keys`.
- **JSONB** для свободных payload'ов: `commands.args`, `jobs.params`, `provider_tasks.request/result`,
  `outbox_events.payload`.
- **Outbox pattern** учтён таблицей `outbox_events` и `OutboxRepository` для надёжной публикации событий.
- SQL-имплементаций репозиториев пока **нет** — только интерфейсы (по требованию задачи).

### Проверки

- `go fmt ./...`, `go vet ./internal/domain/...`, `go build ./...` — без ошибок.

---

---

## Step 2 — PostgreSQL repositories, Command Router, integration tests

Статус: **завершён**.

### Что сделано

- Уточнены интерфейсы в `internal/domain/repositories.go`:
  - добавлен `CommandRepository`;
  - провайдер-таски вынесены из `JobRepository` в отдельный `ProviderTaskRepository`
    (+ метод `GetByExternalID` для реконсиляции вебхуков).
- Реализованы PostgreSQL-адаптеры на `pgx/v5` в `internal/adapter/storage/postgres/`:
  - `postgres.go` — пул соединений (`NewPool`), интерфейс `Querier` (общий для `*pgxpool.Pool` и `pgx.Tx`),
    `RunInTx`, нормализация ошибок (`mapError`: no rows → `ErrNotFound`, unique_violation → `ErrConflict`).
  - `user.go`, `job.go`, `command.go`, `provider_task.go`, `artifact.go`, `delivery.go`,
    `billing.go`, `outbox.go`, `idempotency.go` — реализации всех репозиториев.
- Реализован Command Router (`internal/service/commandrouter/`):
  - `/image` → `image.generate`, `/video` → `video.generate`, `/edit` → `image.edit`,
    `/balance`, `/status <id>`, `/cancel <id>`, `/help`;
  - любой другой текст (включая неизвестные слэш-команды) → `text.ask` / `text_generate` на уровне parser; VK inbound дополнительно gate-ит такой текст через active GPT mode / `VK_UNROUTED_TEXT_MODE`.
- Добавлены тесты:
  - unit-тесты роутера (`router_test.go`) — без БД, входят в обычный `go test ./...`;
  - integration-тесты репозиториев (`integration_test.go`) — поднимают схему из миграции 000001
    и проверяют CRUD, переходы статусов job, биллинг (reserve/insufficient/capture/баланс),
    idempotency get-or-create, outbox, artifacts/variants, deliveries.

### Ключевые решения

- **`Querier` вместо конкретного соединения**: репозитории работают и на пуле, и внутри транзакции,
  что позволит писать outbox-событие в той же транзакции, что и изменение состояния.
- **Атомарный биллинг**: `Reserve`/`Capture`/`Release`/`AppendEntry` выполняются через `RunInTx`
  с блокировкой строки аккаунта (`FOR UPDATE`). Резерв — это `pending`-запись в ledger (баланс не двигает),
  доступный баланс = `balance_cached - SUM(active reservations)`; `Capture` списывает (`committed`, баланс −),
  `Release` снимает холд. Прямой мутации баланса без ledger-записи нет.
- **Оптимистичные переходы job**: `UpdateStatus(from → to)` фиксирует прежний статус в `WHERE`,
  рассинхрон → `ErrConflict` (поддержка инварианта «explicit job status transition»).
- **Идемпотентность**: `GetOrCreate` через `INSERT ... ON CONFLICT (key) DO NOTHING RETURNING`;
  уникальные `idempotency_key` мапятся в `ErrConflict`.
- **Outbox drain**: `FetchPending` использует `FOR UPDATE SKIP LOCKED` для конкурентных публишеров.
- **google/uuid + pgx**: используются нативно (через underlying `[16]byte`), JSONB — через `json.RawMessage`.
- **Integration-тесты env-guarded**: запускаются только при заданном `TEST_DATABASE_URL`,
  иначе `t.Skip`, поэтому дефолтный `go test ./...` зелёный без внешней инфраструктуры.

### Проверки

- `gofmt -w .`, `go vet ./...`, `go test ./...` — без ошибок (integration-тесты пропускаются без БД).
- Для запуска integration-тестов:
  `docker compose up -d postgres` и
  `TEST_DATABASE_URL=postgres://vk_ai_aggregator:vk_ai_aggregator@localhost:5432/vk_ai_aggregator?sslmode=disable go test ./...`.

---

## Step 3 — Billing Service, Job Orchestrator, VK Webhook

Статус: **завершён**.

### Что сделано

- **Billing Service** (`internal/service/billingservice/`) поверх `domain.BillingRepository`:
  - `Estimate` по прайс-листу: `text_generate=1`, `image_generate=10`, `image_edit=10`,
    `video_generate=50`, `image_to_video=50` (неизвестная операция → `ErrUnknownOperation`).
  - `EnsureAccount` — создаёт аккаунт со стартовым балансом **1000** test credits (идемпотентно).
  - `Reserve` / `Capture` / `Release` / `Refund` — идемпотентные по job ключи, поверх ledger.
- **Job Orchestrator** (`internal/service/joborchestrator/`):
  - Flow `Command → Estimate → Reserve → Job → Outbox → Queue`.
  - Job + outbox-событие пишутся атомарно через `uow.Manager` (transactional outbox);
    при нехватке кредитов job паркуется в `awaiting_payment` (`ErrInsufficientCredits`),
    при сбое после резерва — компенсирующий `Release`.
  - Идемпотентность по `idempotency_key` job (повторный вызов возвращает существующий job).
- **VK Webhook** (`internal/adapter/inbound/vk/`), `POST /webhooks/vk`:
  - `confirmation` → возврат токена; `message_new` → flow `InboundEvent → User → Command → Job`.
  - Идемпотентность по `vk_event:{group}:{event_id}`, валидация `secret`, быстрый ответ `ok`.
  - **VK handler не вызывает Provider** — только нормализованный intake.
- **Domain**: добавлены `InboundEvent` + `InboundEventRepository`; миграция `000002_inbound_events`.
- **Инфраструктура**: `internal/platform/queue` (контракт `Publisher` + in-memory, маршрутизация
  по модальности в `queue.<modality>.generate`), `internal/platform/uow` (unit of work) с
  PostgreSQL-реализацией `postgres.UnitOfWork`, и in-memory адаптеры всех репозиториев
  (`internal/adapter/storage/memory`) для unit-тестов без БД.

### Ключевые решения

- **Порядок job-перед-reserve**: т.к. `credit_reservations.job_id` имеет FK на `jobs(id)`,
  job создаётся (committed) до резерва; затем статусы `validated → credits_reserved → queued`.
  Логически flow `Estimate → Reserve → Job` сохранён, но строки job предшествует резерву из-за FK.
- **Транзакционность**: job+outbox — в одной транзакции (`uow`); резерв — собственная транзакция
  billing-репозитория. Полная атомарность (job+reserve+outbox в одном tx) отложена до рефакторинга
  `BillingRepository` на приём `Querier` — отмечено как следующий шаг.
- **Стартовый баланс 1000** выдаётся лениво в `EnsureAccount` при первом обращении (в т.ч. из webhook
  при создании пользователя), без отдельного провижининга.
- **In-memory адаптеры** повторяют семантику Postgres (idempotency-конфликты, оптимистичные переходы
  статусов, ledger-баланс), что позволило покрыть сервисы и webhook unit-тестами без внешней инфраструктуры.

### Проверки

- `gofmt -l .` — пусто; `go vet ./...` — чисто; `go test ./...` — зелёный.
- Покрытие тестами: billing (estimate/ensure/reserve/capture/release/refund/insufficient),
  orchestrator (happy path, идемпотентность, нехватка кредитов), VK webhook
  (confirmation, неверный secret, message_new создаёт job, дедуп дубля, control-команда без job).

---

## Step 4 — Redis Streams, Provider Layer (mock), Artifact Service, S3/MinIO

Статус: **завершён**.

### Что сделано

- **Redis Streams + consumer groups** (`internal/adapter/queue/redis`, пакет `redisqueue`):
  - стримы `stream:jobs:text`, `stream:jobs:image`, `stream:jobs:video`,
    `stream:jobs:delivery`, `stream:jobs:provider_poll` (`AllStreams`);
  - `Publisher` реализует `queue.Publisher` (XADD, маршрутизация операции → стрим
    через `StreamForOperation`) + `PublishTo` для delivery/provider_poll;
  - `Consumer`: `EnsureGroups` (XGROUP CREATE MKSTREAM, идемпотентно к BUSYGROUP),
    `Read` (XREADGROUP, at-least-once), `Ack` (XACK); poison-сообщения ацкаются и пропускаются.
- **Provider Layer / MockProvider** (`internal/adapter/provider/mock`) под `domain.Provider`:
  - `Estimate`/`Submit`/`Poll`/`Cancel` + `Capabilities`; поддержка `text_generate`,
    `image_generate`, `video_generate` (прочие операции → ошибка `unsupported_capability`);
  - детерминированный жизненный цикл (`pending → processing → succeeded`,
    `WithCompleteAfterPolls`), выдача `OutputURLs`;
  - инъекция ошибок по ключевым словам в prompt: `mock_timeout` → `provider_timeout`,
    `mock_rate_limit` → `rate_limited`, `mock_provider_error` → `provider_internal_error`.
- **Artifact Service** (`internal/service/artifactservice`):
  - `SaveTextArtifact`, `SaveBytesArtifact`, `SaveRemoteArtifact`;
  - sha256-хеширование, дедуп по `(owner, sha256)`, загрузка в `ObjectStore`,
    запись метаданных через `domain.ArtifactRepository` (статус `ready`);
  - контракты `ObjectStore` и `Downloader` (дефолтный — HTTP с лимитом 256 MiB).
- **S3/MinIO adapter** (`internal/adapter/storage/s3`): `New` (проверка коннекта),
  `EnsureBucket`, `Put`, `PresignedGetURL` на базе `minio-go/v7`.
- **In-memory** `ArtifactRepo` и `ObjectStore` (`internal/adapter/storage/memory`) для unit-тестов.

### Ключевые решения

- **Стрим на модальность**: медленное видео не блокирует быстрый текст; delivery и
  provider_poll — отдельные стримы (продюсятся явно, не из операции).
- **Consumer groups + ручной ACK**: at-least-once семантика, pending-список для рекавери;
  воркеры должны быть retry-safe (инвариант #5).
- **`ObjectStore` как структурный интерфейс** в `artifactservice`: S3-адаптер удовлетворяет
  его без обратной зависимости адаптера на сервис (адаптер не знает о бизнес-слое).
- **Дедуп артефактов** по контент-хешу делает запись идемпотентной (инвариант: media → Artifact).
- **Env-guarded Redis-тест**: запускается только при `TEST_REDIS_ADDR`, иначе `t.Skip` —
  дефолтный `go test ./...` зелёный без Redis.

### Проверки

- `gofmt -w .`, `go vet ./...`, `go test ./...` — зелёные (Redis-интеграционка пропускается без `TEST_REDIS_ADDR`).
- Для Redis-интеграции: `docker compose up -d redis` и
  `TEST_REDIS_ADDR=localhost:6379 go test ./internal/adapter/queue/redis/...`.

---

## Step 5 — Workers (text/image/video) и Provider Poll Worker

Статус: **завершён**.

### Что сделано

- **Worker-пакет** (`internal/worker`) — единственное место вызова провайдера (инвариант:
  VK-хендлеры и оркестратор провайдеров не вызывают):
  - `GenerationWorker` (один тип на все модальности; стрим определяет, какие job он видит):
    flow `Job → Provider.Submit → ProviderTask → Poll → Artifact → stream:jobs:delivery`.
    Синхронный результат завершается сразу; асинхронный — статус `provider_processing/pending`
    и постановка в `stream:jobs:provider_poll`.
  - `PollWorker`: flow `Poll → Update Status → Requeue (пока running) → Artifact → Delivery Queue`.
  - `Engine` поверх `redisqueue` (`Reader`-интерфейс): `Read → handle → Ack` (ацк только успешно
    обработанных), `Recover` через `XAUTOCLAIM` (рекавери после рестарта), `Run`-цикл.
  - `Registry` для выбора провайдера (`ForOperation` — статический дефолт, `ForName` — для
    реконсиляции по сохранённому `ProviderTask.Provider`).
- **Redis Consumer**: добавлен `AutoClaim` (XAUTOCLAIM) для перехвата зависших pending-сообщений.
- **Mock.Error**: метод `ProviderErrorClass()` — воркер классифицирует ошибку без зависимости
  от пакета провайдера.
- **In-memory** `ProviderTaskRepo` (`internal/adapter/storage/memory`) для unit-тестов.

### Ключевые решения

- **Retry safety**: handler возвращает `nil` только для полностью обработанных задач (успех,
  терминальный fail, requeue) — тогда сообщение ацкается; при инфраструктурной ошибке возвращается
  `error`, сообщение остаётся в pending и переобрабатывается (at-least-once + рекавери через `Recover`).
- **Idempotency**: перед сабмитом проверяется активный `ProviderTask` (pending/processing/succeeded) —
  повторная доставка не сабмитит повторно; ключ сабмита `provider_submit:{job}:{attempt}` (per-attempt),
  артефакты дедуп по sha256, id артефактов добавляются в job без дублей.
- **Error classification**: `ProviderErrorClass → retryable?` (`rate_limited`/`timeout`/`overloaded`/
  `internal`/`output_download_failed` → retryable). Retryable → `failed_retryable → queued` + повторная
  постановка в стрим операции (кап `maxProviderAttempts=3`); иначе `failed_terminal`.
- **Recovery после рестарта**: `Engine.Recover` (`XAUTOCLAIM` с `minIdle`) забирает доставленные,
  но не ацкнутые сообщения из PEL и переобрабатывает их при старте.
- **Переходы строго по стейт-машине job** (`queued → dispatching_provider → provider_submitted →
  provider_processing/pending → provider_succeeded → result_ready`), `setStatus` идемпотентен
  (no-op, если статус уже целевой).

### Проверки

- `gofmt -l .` — пусто; `go vet ./...` — чисто; `go test ./...` — зелёный.
- Покрытие: sync-успех (delivery enqueue), async-flow через poll-воркер, идемпотентная повторная
  доставка (без повторного сабмита), терминальная ошибка, retryable-ошибка (requeue) и переход в
  terminal по достижении лимита попыток, no-op для неизвестного job.

---

## Step 6 — VK Delivery, Admin API, E2E

Статус: **завершён**.

### Что сделано

- **VK Delivery client** (`internal/adapter/delivery/vk`, пакет `vkdelivery`) — единственное место
  вызова VK `messages.send`:
  - интерфейс `Client` с `SendText` / `SendPhoto` / `SendVideo`;
  - `MockClient` — детерминированные `MessageID`, дедуп по `random_id` (повтор не шлёт второе
    сообщение, `Duplicate=true`), `FailNext` для тестов retry;
  - `DeterministicRandomID(key)` — стабильный неотрицательный `random_id` из ключа доставки.
- **Delivery worker** (`internal/worker/delivery.go`), стрим `stream:jobs:delivery`:
  flow `Artifact → Delivery → Billing Capture → Job Success`.
  - Идемпотентность доставки: одна строка `delivery` на job (ключ `delivery:{job}`),
    дедуп через `GetByIdempotencyKey` + `ErrConflict`-reload; `random_id` детерминирован →
    нет дублей отправок; capture идемпотентен (`CaptureForJob`).
  - Переходы `result_ready → delivering → succeeded`; при сбое отправки — `retrying` и возврат
    ошибки (сообщение остаётся pending для ретрая).
- **Billing**: добавлен `CaptureForJob` (lookup резервации по job + идемпотентный capture) и
  контракт `BillingRepository.GetReservationByJob` (postgres + memory).
- **Admin API** (`internal/adapter/inbound/admin`), read-only, DTO-ответы:
  - `GET /admin/jobs` (фильтры `status`/`user_id`/`operation`, пагинация `limit`/`offset`,
    `has_more` через выборку `limit+1`), `GET /admin/jobs/{id}`, `GET /admin/users/{id}`
    (с балансом), `GET /admin/deliveries/{id}`;
  - опциональная авторизация по `X-Admin-Token`.
  - Контракт `JobRepository.List(filter, limit, offset)` + `domain.JobFilter` (postgres + memory).
- **E2E** (`internal/worker/e2e_test.go`, `TestEndToEnd`): полный сценарий
  `VK → Job → Queue → Provider → Artifact → Delivery → Capture` на in-memory адаптерах.
- **In-memory** `DeliveryRepo` и `ObjectStore.GetObject`; **S3** `GetObject`.
- **README.md** с обзором, схемой flow, запуском, Admin API и разделом **Troubleshooting**.

### Ключевые решения

- **Capture при доставке**: кредиты захватываются в delivery-воркере после успешной отправки
  (`reserve` на интейке → `capture` на доставке), баланс двигается только через ledger.
- **Deterministic random_id** вместо случайного — ретраи доставки переиспользуют тот же id,
  и VK сам подавляет дубль (инвариант: every delivery deduplicated).
- **Контракты расширены по необходимости**: `GetReservationByJob` (capture без проброса
  reservation-id через очередь) и `JobRepository.List` (админ-листинг) — без упрощения архитектуры.
- **Admin отдаёт DTO**, а не доменные структуры, и не выполняет мутаций.

### Проверки

- `gofmt -l .` — пусто; `go vet ./...` — чисто; `go test ./...` — зелёный.
- Покрытие: vk mock (dedup/fail), delivery worker (success+capture, идемпотентная повторная
  доставка без двойного списания, текстовая доставка, ретрай при сбое send), admin
  (пагинация/фильтры/404/400/auth), полный E2E.

---

## Step 7 — Runnable entrypoints + MVP smoke check (готово)

- **`internal/platform/config`**: загрузка конфигурации из env с локальными дефолтами
  (Postgres/Redis/MinIO/VK/Admin/worker group).
- **`cmd/migrate`**: применение/откат SQL-миграций (`up`/`down`/`status`) с таблицей
  `schema_migrations`.
- **`cmd/api`**: HTTP-интейк — VK webhook (`/webhooks/vk`), Admin API (`/admin/...`),
  `/health` (пинг Postgres+Redis, 503 при недоступности). Провайдеры не вызываются.
- **`cmd/worker`**: пулы воркеров (generation text/image/video, poll, delivery) поверх
  Redis Streams; авто-создание бакета MinIO и consumer-групп, recovery через AutoClaim.
- **`TESTING.md`**: prerequisites, запуск, миграции, curl-примеры, ожидаемые результаты,
  troubleshooting.

### Проверки

- `go build ./...`, `gofmt -w`, `go vet ./...` — чисто; `go test ./...` — зелёный.
- Полный бизнес-флоу + идемпотентность + failure-сценарии валидируются in-memory
  E2E (`internal/worker/TestEndToEnd`) и воркер/vk тестами.
- Live-валидация против реальных Postgres/Redis/MinIO (docker compose) пройдена:
  `/health` 200; полный E2E для text/image/video — job `succeeded`, артефакт в MinIO,
  delivery `sent`, capture закоммичен (1/10/50); стримы стабильны (нет бесконечных retry).

### Найдено и исправлено при live-прогоне

- **NOT NULL по UUID[]-колонкам** (`commands.attachment_artifact_ids`,
  `jobs.input/output_artifact_ids`): pgx кодировал nil-срез как SQL NULL, перекрывая
  `DEFAULT '{}'`. Webhook падал с 500. Фикс — `uuidArray()` приводит nil к пустому срезу.
- **Бесконечный re-enqueue**: mock-провайдер отдаёт `mock://` output URL, который реальный
  HTTP-downloader не может скачать → `handleFailure` бесконечно переочередял job. Фикс —
  `mock.NewDownloader()` (резолвит `mock://` в реальные байты), подключён в `cmd/worker`
  через `artifactservice.WithDownloader`.

---

## Step 8 — v0.1.2 production hardening

Статус: **завершён как MVP+ hardening**.

### Что сделано

- **Output moderation**:
  - добавлен `moderationservice` с интерфейсом `Moderator`;
  - добавлен audit trail `moderation_results` и миграция `000003`;
  - generation/poll worker блокирует delivery, если moderation verdict не позволяет выдачу;
  - при блокировке job получает `rejected`, reservation release выполняется без capture.
- **DLQ и retry budget**:
  - retry budget теперь охватывает submit/poll/download/delivery phases;
  - exhausted tasks уходят в `stream:jobs:dlq`;
  - бесконечный re-enqueue для download/provider/delivery failures закрыт.
- **Outbox relay**:
  - orchestrator больше не публикует job напрямую в Redis;
  - `event.job.queued` пишется в `outbox_events`;
  - `service/outboxrelay` публикует pending events в Redis Streams и помечает их published.
- **Atomic billing**:
  - `BillingRepository` может работать standalone или transaction-bound через `Querier`;
  - job creation + reserve + outbox выполняются в одной транзакции.
- **Security / hardening**:
  - production / real-mode config validation в `cmd/api` и `cmd/worker`;
  - SSRF protection в artifact downloader;
  - per-IP webhook rate limiting;
  - Prometheus metrics at `/metrics`.
- **Operational hardening**:
  - OpenTelemetry trace context propagation через `traceparent` от VK intake до worker pipeline;
  - graceful worker drain через `WORKER_SHUTDOWN_GRACE`;
  - maintenance cleanup для `idempotency_keys`, terminal `outbox_events` и Redis Stream backlog;
  - balance-vs-ledger reconciliation metric `vkagg_billing_mismatches`.
- **Storage / migrations / cost controls**:
  - migration checksums и transactional apply/rollback;
  - S3 retention lifecycle, signed artifact URLs, scanner hook;
  - `PRICES` overrides и `MAX_JOB_COST`.
- **Real adapter starts**:
  - OpenAI image-generation adapter behind `PROVIDER=openai`;
  - real VK `messages.send` adapter behind `VK_DELIVERY_MODE=real`.

### Проверки

- `go test ./...`, `go vet ./...`, `docker compose config` — зелёные на `v0.1.2`.
- Unit/in-memory coverage включает:
  - VK inbound idempotency;
  - billing reserve/capture/release;
  - job orchestrator;
  - provider mock/OpenAI adapter;
  - Redis Streams;
  - output moderation;
  - DLQ/retry budget;
  - delivery idempotency/capture;
  - full in-memory E2E `VK → Job → Queue → Provider → Artifact → Delivery → Capture`.

### Текущие ограничения после v0.1.2

- OpenAI adapter покрывал только image generation.
- Real VK adapter покрывал только `messages.send`; upload raw photo/video artifacts в VK upload servers ещё был нужен.
- Output moderation была keyword-based; real moderation provider и artifact scanner оставались follow-up.
- Нужен resume fix для edge-case: `provider_task=succeeded`, но artifact/result_ready ещё не сохранены после crash.

---

## Step 9 — v0.1.3 real integrations foundation

Статус: **завершён как adapter-level foundation; live smoke с реальными ключами ещё нужен**.

### Что сделано

- **OpenAI provider**:
  - text generation через `/responses`;
  - image generation через `/images/generations`, включая `url` и `b64_json`;
  - async video generation через `/videos`, polling `/videos/{id}` и download `/videos/{id}/content`;
  - output нормализуется в Artifact-compatible URLs, включая `data:` URLs для inline bytes.
- **DeepInfra provider**:
  - `deepseek-ai/DeepSeek-V4-Flash` text generation is wired through DeepInfra's OpenAI-compatible `/chat/completions` endpoint;
  - `ByteDance/Seedream-4.5` text-to-image generation is wired through DeepInfra's native `/v1/inference/{model}` endpoint;
  - `PROVIDER=deepinfra`, `IMAGE_PROVIDER=deepinfra` or `PROVIDER_CHAIN=deepinfra,mock` enables DeepInfra where the selected modality is supported;
  - the adapter returns normalized `data:text/plain` / `data:image/...` outputs and maps DeepInfra HTTP failures into internal provider error classes.
  - text providers now receive an internal instruction alongside the user prompt: answer as `НейроХаб бот`, stay concise (`<= 3000 characters`) and do not reveal provider/model/backend details; VK delivery still chunks longer outputs as a fallback.
  - follow-up fix: the mock-aware downloader now decodes provider `data:` URLs, so `PROVIDER_CHAIN=deepinfra,mock` can store DeepInfra text outputs before VK delivery.
- **Provider router**:
  - `PROVIDER_CHAIN` задаёт ordered fallback chain;
  - router проверяет capabilities, estimated cost, observed latency и circuit-breaker health;
  - retryable submit failures (`rate_limited`, `timeout`, `overloaded`, `internal`) пробуют следующий provider в той же попытке;
  - persisted `ProviderTask.Provider` хранит фактического provider, принявшего задачу.
- **VK media delivery**:
  - `vkdelivery.HTTPClient` теперь реализует `MediaUploader`;
  - photo flow: `photos.getMessagesUploadServer` → multipart upload → `photos.saveMessagesPhoto`;
  - video flow: `video.save` → multipart upload;
  - delivery worker загружает raw artifact bytes в VK и отправляет canonical `photo...` / `video...` attachment через `messages.send`.
- **Moderation / scanner**:
  - `MODERATION_PROVIDER=openai` включает OpenAI moderation вместо keyword-only moderator;
  - `ARTIFACT_SCANNER=openai` проверяет text/image artifact bytes до storage;
  - video scanning/transcode остаётся частью будущего media pipeline.
- **Config/docs/tests**:
  - added `.env.example` and automatic local `.env` loading through `internal/platform/config`; real OS/CI env still wins over `.env`;
  - добавлены env-переменные OpenAI text/image/video/moderation/scanner, DeepInfra text и `PROVIDER_CHAIN`;
  - обновлены `README.md`, `RUNBOOK.md`, `TESTING.md`, `TASKS.md`, `AGENTS.md`, `AUDIT.md`, `ROADMAP.md`;
  - добавлены unit-тесты OpenAI text/image/video/moderation/scanner, DeepInfra text, VK upload pipeline, delivery upload и provider fallback.
- **VK inbound attachments**:
  - sticker-only сообщения больше не превращаются в пустой prompt;
  - handler синтезирует text prompt с `sticker_id/product_id`; prompt проходит в `text.ask` job только при активном GPT text mode или legacy `VK_UNROUTED_TEXT_MODE=gpt`, поэтому стикер не теряется и не создает случайный billable job вне режима;
  - фото/видео/аудио attachments остаются задачей полноценного input Artifact pipeline.
- **VK product menu**:
  - current bot-facing menu profile shows NeuroHub text mode and account/referral: `VK_MENU_GPT_ENABLED=true`, `VK_MENU_ACCOUNT_ENABLED=true`, while video, image, students and top-up remain implemented but hidden by `VK_MENU_*_ENABLED=false`;
  - menu flow переведен на декларативный `menuScreen` registry: каждый control-command указывает текст, inline keyboard, необходимость баланса и optional welcome attachment;
  - первичная нижняя VK keyboard содержит только одну кнопку `Старт`;
  - после нажатия `Старт` бот заменяет нижнюю постоянную клавиатуру на одну кнопку `Показать меню`;
  - `Показать меню` хранится как отдельный `show_menu` control-command: нижняя persistent-кнопка всегда отправляет свежий VK inline menu вниз без повторной переустановки нижней клавиатуры, а inline-переходы внутри меню продолжают редактировать active menu message;
  - первый `Старт` пользователя делает one-time VK `users.get`, кеширует `vk_first_name` / `vk_last_name` в `users` и отправляет именное welcome-сообщение; `welcome_name_sent_at` фиксирует, что последующие `Старт` / `Показать меню` должны идти обычным welcome без имени;
  - первый обычный non-payload текст/стикер/menu-repair контакт пользователя принудительно открывает onboarding `/start`, чтобы новый пользователь не застревал без нижней кнопки; явные slash generation-команды и payload-кнопки не перехватываются;
  - `Старт`, `/start`, `меню`, `нет меню`, `нет кнопки`, `где меню` и `начать` открывают VK inline keyboard под коротким welcome-сообщением НейроХаб; typed repair-фразы дополнительно восстанавливают нижнюю persistent-кнопку `Показать меню`;
  - `Создать видео` теперь открывает отдельный inline-экран `Выбери модель для генерации:` с моделями `Sora 2`, `Kling v2.1`, `Seedance 1`, `Haiuo v0.2` и кнопкой `Назад`;
  - `Sora 2` и `Kling v2.1` открывают detail-экраны с описанием, prompt-примером, ссылкой на инструкцию и кнопками `Начать генерацию`, `Примеры`, `Назад`;
  - `Seedance 1` открывает выбор `Lite` / `Pro`, а `Haiuo v0.2` открывает выбор `Обычный` / `Fast`;
  - кнопки выбора video-модели и вложенных video submenu записываются как control commands и не создают billable jobs до подключения model-specific generation state;
  - `Создать фото` при одной основной модели пропускает выбор модели, сразу включает `photo_text` mode и показывает text-to-image инструкцию только с кнопкой `Назад`; `Фото по тексту` и `Фото с референсом` скрыты флагами до необходимости отдельных selection paths;
  - `Спросить у НейроХаб` открывает active-сообщение `НейроХаб активен` без создания job и включает Redis-backed GPT text mode для `peer_id`; следующий обычный текст/стикер пользователя проходит через `text.ask` flow; старый text-label `Спросить у GPT` остается совместимым alias;
  - в активном GPT mode handler сначала отправляет `НейроХаб думает...`, сохраняет `vk_placeholder_message_id` в `job.Params`, а delivery worker при текстовом результате редактирует это сообщение через VK `messages.edit`; перед отправкой text delivery приводит простой provider Markdown к VK plain text (`**`, backticks, heading hashes убираются, `*`/`-` списки становятся `•`); длинные ответы режутся на follow-up chunks с детерминированными `random_id`, чтобы VK `error_code=914` не оставлял placeholder зависшим; legacy `VK_UNROUTED_TEXT_MODE=gpt` остается обычной текстовой доставкой без placeholder;
  - `Студентам и школьникам` открывает учебное подменю: `Решальник задач`, `Генерация презентаций (скоро)`, `Создание рефератов (скоро)`, `Ответы на вопросы`, `Назад`;
  - `vkdelivery.HTTPClient` получил `SendMessage` с `keyboard` JSON, поэтому VK API по-прежнему вызывается только из `internal/adapter/delivery/vk`;
  - `vkdelivery.HTTPClient` получил `EditMessage` поверх VK `messages.edit`, а `ControlClient` теперь покрывает и send, и edit для product/control меню;
  - `vkdelivery.KeyboardButton` получил `ActionType`, поэтому inline menu можно рендерить как VK `callback` или legacy `text` без переписывания payload;
  - `VK_MENU_BUTTON_MODE=callback` стал дефолтом для inline menu: нажатия приходят как VK `message_event` и не добавляют пользовательские echo-сообщения в чат; `VK_MENU_BUTTON_MODE=text` возвращает прежнее поведение;
  - добавлены `VK_MENU_*_ENABLED` feature flags для каждой основной и вложенной product-menu кнопки: disabled buttons скрываются из новых keyboard, а stale payload от старого сообщения падает обратно в актуальное главное меню без создания job;
  - handler хранит process-local active menu по `peer_id`, а GPT dialog mode хранится в Redis: кнопочные payload-переходы редактируют текущий menu message, обычный пользовательский текст вне GPT mode оставляет предыдущее меню доступным выше, а другой control-экран сбрасывает GPT mode;
  - `VK_UNROUTED_TEXT_MODE=reply` стал дефолтом для обычного текста вне GPT mode после onboarding: handler записывает `unknown` command, не создает Job и отправляет hint `Выберите режим в меню выше или нажмите на кнопку показать меню` с нижней persistent-кнопкой `Показать меню`, но без дублирования inline keyboard; `silent` молчит, `gpt` возвращает legacy any-text-to-GPT behavior;
  - handler обрабатывает `message_event` как control-only inbound event: сохраняет inbound/command, но не создает Job и не дергает provider;
  - каждый `message_event` подтверждается blank `messages.sendMessageEventAnswer` через `vkdelivery.ControlClient`, чтобы VK-клиент снимал loading spinner с callback-кнопки;
  - если VK не разрешает edit текущего menu message, API логирует warn, очищает active menu и делает fallback на обычный `messages.send`;
  - кнопки `Создать видео`, `Создать фото`, `Спросить у НейроХаб`, `Студентам и школьникам`, `Мой аккаунт`, `Пополнить баланс` классифицируются как control commands и не создают пустые billable jobs;
  - баланс в меню берется через `billingservice.EnsureAccount`, без прямой мутации баланса;
  - опциональный баннер подключается через `VK_WELCOME_ATTACHMENT` как уже готовый VK attachment string;
  - если VK возвращает `error_code=912` из-за выключенных bot features, API повторяет отправку без keyboard, чтобы callback не падал.

### Проверки

- Targeted tests: `go test ./internal/adapter/provider/openai ./internal/adapter/delivery/vk ./internal/adapter/inbound/vk ./internal/service/commandrouter ./internal/worker ./internal/platform/config`.
- DeepInfra targeted tests: `go test ./internal/adapter/provider/deepinfra ./internal/platform/config`.
- DeepInfra delivery regression: `go test ./internal/adapter/provider/mock ./internal/service/artifactservice ./internal/worker`.
- Added VK menu UX coverage: `EditMessage` request shape, mock edit semantics, active-menu edit, lower `Показать меню` fresh send, first-contact onboarding, typed menu repair phrases, and plain-message hint behavior with lower keyboard repair.
- Added callback menu coverage: callback keyboard JSON, `VK_MENU_BUTTON_MODE` config validation, `message_event` command processing, no-job invariant, and legacy text-button mode.
- Added callback ack coverage: real `messages.sendMessageEventAnswer` request shape, mock answer recording, and inbound `message_event` acknowledgement.
- Added unrouted text coverage: default choose-mode hint with lower keyboard repair/no-job, `silent` no-response mode, legacy `gpt` mode, GPT button enabling text jobs, menu transitions clearing GPT mode, and sticker-to-text job only inside GPT mode.
- Added VK text delivery formatting coverage: simple provider Markdown markers are stripped and list items are rendered as `•` bullets before VK send/edit.
- Added menu feature flag coverage: hidden main buttons, hidden nested video buttons, disabled stale payload fallback, and env loading for `VK_MENU_*_ENABLED`.
- Added VK inbound retry regression: duplicate `inbound_events.idempotency_key` now reloads the saved inbound row before `SetStatus`, so VK retries do not fail with `mark inbound processed: domain: entity not found`.
- Full regression checks выполняются после документационного sync.
- Live VK `/start` smoke: callback returned `ok`, command persisted as `start`,
  zero jobs created, welcome text delivered to VK. After enabling bot features
  in community settings, VK accepts keyboard sends without `error_code=912`.

### Текущие ограничения

- Реальные OpenAI/DeepInfra/VK вызовы требуют credential-bound live smoke на dev-аккаунтах; unit-тесты используют mock HTTP servers.
- Второй реальный provider для text fallback добавлен через DeepInfra; реальные image/video fallback providers остаются follow-up.
- VK control/menu responses пока отправляются напрямую из API через `vkdelivery.ControlClient` с deterministic `random_id`; если на product/control sends распространяем invariant `Every delivery attempt is persisted`, нужен отдельный persisted delivery/outbox flow для таких сообщений.
- Active-menu tracking пока хранится в памяти процесса `cmd/api`; после рестарта API или при multi-instance балансировке меню может отправиться новым сообщением. GPT dialog mode уже хранится в Redis и переживает restart/API instance switch.
- Video artifact scanner пока fail-open; полноценный video scan/probe/transcode остаётся Phase 3 media pipeline.
- Нужен resume fix для edge-case: `provider_task=succeeded`, но artifact/result_ready ещё не сохранены после crash.

### Next step

См. актуальный backlog в `TASKS.md`; ближайший фокус — live smoke с реальными
ключами, реальные image/video fallback providers, video media pipeline и worker
resume hardening.

---

## Шаг 10 — VK Mini App: BFF, фронт, монохромный UI, фикс биллинга

### Что сделано

- **BFF `/miniapp/*`** в `cmd/api` (адаптер `internal/adapter/inbound/miniapp`):
  `POST /miniapp/jobs`, `GET /miniapp/jobs`, `GET /miniapp/jobs/{id}`,
  `GET /miniapp/balance`. Хендлеры не вызывают провайдеров — только
  `joborchestrator` + существующий биллинг-путь (reserve/capture поверх ledger).
  Ownership: задачи доступны только своему `vk_user_id` (иначе 404).
- **Проверка подписи launch-параметров** (`sign.go`, HMAC-SHA256 по спецификации
  VK). При заданном `VK_APP_SECRET` подпись проверяется по-настоящему:
  невалидная/просроченная/без подписи → `401` без деталей, dev-обход
  (`X-VK-User-ID`) отключается. В production пустой `VK_APP_SECRET` =
  fail-closed на старте.
- **Фронт `web/miniapp`** (React 18 + VKUI 8 + VK Bridge): экраны список задач,
  создание (текст/фото/видео), детали с авто-обновлением, баланс. Монохромный
  ч/б стиль через переопределение токенов VKUI (`theme.css`); дешёвые/цветные
  эмодзи убраны, иконки — лаконичные моно из `@vkontakte/icons`.
- **VK Tunnel** (`@vkontakte/vk-tunnel`) + npm-скрипт `tunnel` для запуска
  локального фронта внутри VK.
- **Фикс биллинга (AUDIT B1a):** стартовый грант 1000 теперь создаётся
  committed-проводкой в ledger атомарно с созданием аккаунта; миграция
  `000004` бэкоффилит открывающие проводки для существующих аккаунтов. Воркер
  больше не пишет `billing balance mismatch`.

### Проверки

- `go build ./...`, `go test ./...` — зелёные; `tsc --noEmit` и
  `npm run build` в `web/miniapp` — без ошибок.
- Live (mock): `/health`=200; create→list→detail→balance прошёл через воркер
  (`queued → succeeded`, артефакт создан, баланс 1000→999).
- Подпись: с заданным секретом invalid/missing/dev-bypass → `401`;
  валидный accept-путь покрыт `TestHandler_ValidSign`.
- Биллинг: post-migration реконсиляция — 0 mismatch; у нового аккаунта есть
  запись `opening balance grant`.

### Текущее ограничение

- VK Tunnel требует интерактивной OAuth-авторизации в браузере, поэтому https-URL
  туннеля выдаётся только после ручного логина (`npm run tunnel` → открыть
  ссылку → подтвердить → Enter). Этот URL затем вставляется в настройки
  приложения на dev.vk.com.

---

## Step (доп.) — Mini App в iframe VK: dev-туннель и mixed-content

Статус: **завершён**.

### Что сделано

- **VK Tunnel на техработах с 02.10.2025** → перешли на `cloudflared` (обходной
  путь, рекомендованный VK).
- **`web/miniapp/vite.config.ts`, секция `server`:** `host: true`,
  `allowedHosts: true` (принимает меняющийся домен `*.trycloudflare.com`, URL
  нигде не хардкодится), `hmr: { clientPort: 443, protocol: 'wss' }` (HMR через
  https-туннель), proxy `/miniapp` и `/api` → `http://localhost:8080`. Так
  фронт ходит к бэку через тот же origin — mixed-content под https устранён.
- **API-клиент фронта** уже использует относительные пути (`BASE_URL` пустой в
  dev) — изменений не потребовалось.
- **Launch params**: фронт читает `window.location.search` и шлёт в заголовке
  `X-Launch-Params`; бэк проверяет подпись при заданном `VK_APP_SECRET`, иначе
  dev-fallback по `X-VK-User-ID`. Оба пути проверены.

### Проверки

- Live (mock, через эндпоинты бэка): `GET /miniapp/balance`=1000,
  `POST /miniapp/jobs` (`queued → succeeded`, артефакт создан),
  `GET /miniapp/jobs` и detail — данные отдаются, баланс 1000→999.
- Гейты: `go test ./...` — зелёные; `tsc --noEmit`=0; `npm run build` — без
  ошибок.

### Ручной шаг оператора

- `cloudflared tunnel --protocol http2 --url http://localhost:5173` → вставить
  выданный https-URL в dev.vk.com → «Версия для vk.com» → «URL для разработки».

---

## Step (доп.) — Mini App: чат-интерфейс

Статус: **завершён** (фронт).

### Что сделано

- Переход с таб-экранов на **чат** (`src/chat/`: `ChatScreen`, `Composer`,
  `MessageBubble`, `types.ts`). Слои: `api/client.ts` (DTO = `miniapp.JobDTO`),
  `hooks/useBridge.ts` (тема VK + `VKWebAppGetUserInfo` только для UI),
  `ui/` (примитивы без сети).
- `@vkontakte/vkui` и `@vkontakte/icons` удалены из зависимостей.
- Поллинг: интервал 2с, макс. 90 итераций, остановка при unmount/терминальном
  статусе; `patchMessage` по id.
- `artifactUrl` — только UUID из `output_artifact_ids`; `fetchArtifactText` для
  text_generate (ожидает `GET /miniapp/artifacts/{id}` на бэке).

### Проверки

- `npx tsc --noEmit` и `npm run build` в `web/miniapp` — без ошибок.

---

## Step (доп.) — Mini App: восстановление чат-фронта из HEAD

Статус: **завершён**.

### Что было сломано

- Рабочее дерево было грязным после ручной чистки: ожидались пропавшие импорты
  `./hooks/useBridge` и `./chat/ChatScreen`, а legacy `src/panels/` остался на
  диске пустой директорией. Источник истины — уже закоммиченная чат-архитектура
  в `feature/vk-miniapp`.

### Что сделано

- `web/miniapp/src/**` восстановлен из `HEAD`: `main.tsx`, `App.tsx`,
  `api/client.ts`, `hooks/useBridge.ts`, `ui/theme.css`, `ui/ui.tsx`,
  `chat/types.ts`, `chat/MessageBubble.tsx`, `chat/Composer.tsx`,
  `chat/ChatScreen.tsx`.
- Проверено, что в `web/miniapp/src` нет ссылок на `panels`/`screens`, VKUI,
  `dangerouslySetInnerHTML`, `eval`, `new Function` или `console.log`.

### Проверки

- `npx tsc --noEmit` — без ошибок.
- `npm run build` — без ошибок.
- `npm run lint` отсутствует в `package.json`; ручная проверка неиспользуемых
  legacy-ссылок выполнена поиском.

---

## Step (доп.) — Mini App: hardening чат-фронта после ручной чистки

Статус: **завершён**.

### Что было сломано

- В IDE были симптомы битых импортов `./hooks/useBridge` и
  `./chat/ChatScreen` после ручной чистки. Диагностика показала, что целевые
  файлы уже есть на диске и в `HEAD`, `src/panels/` остался только пустой
  legacy-каталог, а `tsc` не выдаёт TS2307/TS6133.

### Что сделано

- Удалён пустой legacy-каталог `web/miniapp/src/panels`; ссылок на
  `panels`/`screens` в `web/miniapp/src` нет.
- `useBridge.ts`: подписка `bridge.subscribe` теперь снимается через
  `bridge.unsubscribe(handler)` в cleanup.
- `ChatScreen.tsx`: `patchMessage` мемоизирован, polling делает первый запрос
  сразу, затем ждёт 2 секунды между итерациями; лимит и stop-on-unmount
  сохранены.

### Проверки

- `npx tsc --noEmit` — без вывода, exit 0.
- `npm run build` — без ошибок/предупреждений.
- `npm run lint` не настроен в `package.json`; ручной поиск опасных и legacy
  паттернов (`panels`, `screens`, VKUI, `console`, `dangerouslySetInnerHTML`,
  `eval`, `new Function`) — без совпадений.

---

## Step (доп.) — Mini App: история чатов, модальности и графитовая тема

Статус: **завершён**.

### Что сделано

- Добавлена локальная история чатов в `localStorage` (`vk_miniapp_chats_v1`):
  `src/chat/store.ts`, `src/hooks/useChats.ts`, шторка `src/chat/ChatList.tsx`.
- `Composer` теперь поддерживает выбор модальности (`Текст`, `Фото`, `Видео`) и
  dropdown модели для выбранной модальности.
- `ChatScreen` использует `useChats`, список чатов, заголовок активного чата и
  действие «Новый чат».
- `theme.css` переведён на графитовую тёмную тему `#1A1A1D` и дополнен стилями
  `segment`, `model-select`, `drawer`.

### Проверки

- `npx tsc --noEmit` — без вывода, exit 0.
- `npm run build` — без ошибок/предупреждений.
- `npm run lint` отсутствует в `package.json`; ручная проверка legacy/опасных
  паттернов в `web/miniapp/src` — без проблем.

---

## Step (доп.) — Mini App: фиксация UI-итерации и аудит фронта

Статус: **завершён**.

### Что сделано

- Зафиксирована UI-итерация Mini App: история чатов в `localStorage`
  (`vk_miniapp_chats_v1`), выбор модальности (`Текст`/`Фото`/`Видео`) и
  dropdown модели.
- Тёмная тема Mini App переведена на графитовую палитру `#1A1A1D`.
- Исправлен видимый scrollbar в composer textarea: прокрутка сохранена, но
  нативный scrollbar скрыт для WebKit/Firefox/старого Edge.
- Добавлен `docs/AUDIT.md` с аудитом безопасности, утечек и оптимизации новых
  фронтенд-фич.

### Проверки

- `npx tsc --noEmit` — без вывода, exit 0.
- `npm run build` — без ошибок/предупреждений.

---

## Step (доп.) — Mini App: аудит безопасности/архитектуры + бэклог фиксов

Статус: **завершён** (ревью только-чтение; код по находкам не правился).

### Что сделано

- Проведён глубокий read-only аудит безопасности и архитектуры всего репозитория
  (Go-бэкенд + React-фронтенд Mini App). Отчёт — `docs/REVIEW.md`: сводная таблица
  по разделам, находки с severity (`Critical/High/Medium/Low`),
  файл:строка, суть и рекомендация, ТОП-5 приоритетов.
- Подтверждён корректный периметр: fail-closed в проде (`config.Validate`
  требует `VK_APP_SECRET`/`ADMIN_TOKEN`), ownership-проверки на job/artifact/balance,
  параметризованный SQL, append-only ledger с `FOR UPDATE` и идемпотентными
  записями (двойного списания нет), таймауты внешних вызовов (OpenAI 120s/30s),
  graceful drain воркеров, cleanup поллинга и `bridge.unsubscribe` на фронте.
- Зафиксирован **резолв** ранее заявленного рассинхрона фронт↔бэк: роут
  `GET /miniapp/artifacts/{id}` существует (`handler.go:75`, ownership +
  `Cache-Control: private`), текст отдаётся как `text/plain` артефакт
  (`artifactservice/service.go:102-103`) и читается фронтом
  (`client.ts:107-118`); поля `result_text` в DTO нет намеренно.
- Главные находки вынесены в `TASKS.md` → раздел «Бэклог по аудиту» как отдельные
  задачи (High: rate-limiting на `/miniapp/*`; Medium: fail-closed `vk_ts`,
  проброс модели в API, мягкая деградация `getArtifact` при недоступности S3;
  Low→Medium: развязать `mountedRef` и перезапуск эффекта; и т.д.). Сейчас **не**
  исправлялись.
- Зафиксирована UI-итерация Mini App (уже в коммитах ветки): история чатов в
  `localStorage` (`vk_miniapp_chats_v1`), выбор модальности `Текст/Фото/Видео` +
  dropdown модели, графитовая тема `#1A1A1D`, скрытие нативного scrollbar в
  composer textarea.

### Проверки

- `npx tsc --noEmit` — без вывода, exit 0.
- `npm run build` — без ошибок (vite 8, `dist` собран, gzip JS ~66.7 kB).

---

## Step (доп.) — Mini App: job intake rate limiting

Статус: **завершён**.

### Что сделано

- `POST /miniapp/jobs` теперь rate-limited после проверки launch params, с ключом
  `miniapp_job:<verified vk_user_id>`.
- Добавлены отдельные настройки `MINIAPP_JOB_RATE_LIMIT_RPS` и
  `MINIAPP_JOB_RATE_LIMIT_BURST`; webhook limit не переиспользуется.
- При превышении лимита BFF возвращает безопасный `429` и `Retry-After`; новый
  job при rate limit не создаётся.

### Проверки

- `go test ./internal/adapter/inbound/miniapp ./internal/platform/config` — exit 0.
- `go test ./...` — exit 0.

---

## PR-2 — Mini App: remove obsolete VK Tunnel tooling

Статус: **завершён**.

### Что сделано

- Удалены `@vkontakte/vk-tunnel`, npm-скрипт `tunnel` и obsolete
  `web/miniapp/vk-tunnel-config.json`.
- Dev tunnel documentation нормализована на `cloudflared` /
  `*.trycloudflare.com`; backend calls остаются same-origin через Vite proxy.

### Проверки

- `npm install` — exit 0.
- `npm run build` — exit 0.

---

## PR-3 — Mini App: submit idempotency and API errors

Статус: **завершён**.

### Что сделано

- Frontend `createJob` sends `X-Idempotency-Key` generated per submit attempt.
- Submit flow has a minimal in-flight guard beyond disabled button UX.
- API errors are normalized into safe user-facing messages, including network
  failures and `429` with `Retry-After` metadata.

### Проверки

- `npm run build` — exit 0.

---

## PR-4 — Mini App: fail-closed vk_ts

Статус: **завершён**.

### Что сделано

- `VerifyLaunchParams` rejects missing, invalid, future, or expired `vk_ts`
  whenever `MINIAPP_LAUNCH_PARAMS_MAX_AGE` is enabled.
- `POST /miniapp/jobs` now fails safely at auth middleware before job creation
  when `vk_ts` cannot be trusted.
- Added handler coverage that verifies safe `401` response and no job creation.

### Проверки

- `go test ./internal/adapter/inbound/miniapp ./internal/platform/config` — exit 0.

---

## PR-5 — Mini App: backend model_id contract

Статус: **завершён**.

### Что сделано

- `POST /miniapp/jobs` accepts optional `model_id` and validates it by operation
  against the Mini App backend whitelist.
- Unsupported or cross-operation model IDs return safe `400` before user,
  billing or job creation.
- Supported `model_id` is stored in normalized job params; job API responses do
  not expose model selector/model_id.

### Проверки

- `go test ./internal/adapter/inbound/miniapp ./internal/platform/config` — exit 0.

---

## PR-7 — Mini App: cost estimate before submit

Статус: **завершён**.

### Что сделано

- Добавлен BFF endpoint `POST /miniapp/estimate` с тем же launch-param auth и
  per-user rate limiting, что и create-job путь.
- Endpoint принимает `operation`, `prompt`, optional `model_id`, валидирует
  operation/model по Mini App whitelist, переиспользует
  `billingservice.Estimate` и не создаёт job, reservation или ledger entries.
- Ответ отдаёт только backend-owned данные: `operation`, `model_id`,
  `cost_estimate`, `balance_credits`, `enough_credits`; provider, prompt и
  user details не раскрываются.
- Mini App frontend вызывает estimate с debounce при изменении prompt/model,
  показывает стоимость до submit и предупреждение при `enough_credits=false`.
  Если estimate временно недоступен, submit не блокируется; решение
  зафиксировано в `DECISIONS.md`.

### Проверки

- `go test ./internal/adapter/inbound/miniapp ./internal/service/billingservice` — exit 0.
- `go test ./...` — exit 0.
- `npm run build` в `web/miniapp` — exit 0.

---

## PR-8 — Mini App: result and artifact UX

Статус: **завершён**.

### Что сделано

- Добавлен frontend `ResultCard` для bot/result сообщений: карточка
  «Готовый VK-пост» вместо обычного chat bubble.
- Text result отображается как plain text с `white-space: pre-wrap`; copy button
  копирует только текст, без HTML.
- Image/video preview использует только backend artifact route через
  `artifactUrl(id)` с UUID validation; artifact URL не сохраняется в
  `localStorage`.
- Добавлены loading/skeleton state, safe error/fallback state и retry action,
  который создаёт новый job через существующий `createJob` flow с тем же
  prompt/operation/model.

### Проверки

- `npm run build` в `web/miniapp` — exit 0.
- Поиск `dangerouslySetInnerHTML`, `innerHTML`, `eval`, `new Function`,
  `markdown`, `marked` в `web/miniapp/src` — без совпадений.

---

## PR-9 - Mini App: history reload recovery and local retention

Статус: **завершён**.

### Что сделано

- Mini App on startup calls `GET /miniapp/jobs`, restores non-terminal jobs and
  resumes polling after reload. Locally remembered terminal jobs are restored as
  lightweight UI shells and resolved from backend artifacts only when allowed.
- `localStorage` schema for `vk_miniapp_chats_v1` now stores only
  `job_id`, `operation_type`, `status` and `created_at`, with a 7-day TTL and
  max 50 entries. Prompt bodies, generated text, artifact IDs/URLs, balance,
  launch params and provider details are not persisted.
- Legacy or suspicious local history containing sensitive-looking keys is
  cleared on initialization with a warning that does not include field values.
- Polling keeps one active poller and one timeout per job, clears timers on
  terminal states and component unmount, and does not create duplicate pollers
  for the same `job_id`.
- Added clear local history action and a privacy note explaining that only job
  metadata is stored locally; backend job history is not deleted.

### Проверки

- `npm run build` в `web/miniapp` — exit 0.
- Поиск localStorage/sensitive/XSS patterns in Mini App frontend scope reviewed:
  persisted history contains only allowed metadata and no raw HTML rendering was
  added.

---

## PR-10 - Mini App: workflow and chat mode redesign

Статус: **завершён**.

### Что сделано

- Добавлен явный переключатель режимов `Chat` / `Workflow`; выбранный режим
  хранится как UI preference `vk_miniapp_mode_v1`, не влияет на billing/job
  semantics и не останавливает polling активных jobs.
- Chat mode сохраняет текущий chat-like UX: drawer истории, composer,
  idempotent `createJob`, backend estimate в composer и безопасный render
  результата через `ResultCard`.
- Workflow mode реализует экраны `Home`, `Generate`, `Status`, `Result`,
  `History`: backend balance, быстрые сценарии, model/operation selector,
  backend estimate перед submit, timeline статусов, VK post preview и история
  jobs из backend.
- `ResultCard` переведён в VK-post-preview: аватар/имя сообщества, plain-text
  текст или media только через backend artifact route, copy/retry и safe
  fallback без `innerHTML`.
- Theme CSS получил design tokens для spacing/radius/color/motion, light/dark
  через переменные, semantic colors, touch targets и reduced-motion fallback.
- ADR mode switching и ADR design direction зафиксированы в `DECISIONS.md`.

### Проверки

- `npm run build` в `web/miniapp` — exit 0.
- `go build ./...` — exit 0.
- Поиск `dangerouslySetInnerHTML`, `innerHTML`, `eval`, `new Function`,
  sensitive/localStorage patterns в Mini App frontend scope reviewed.

---

## PR-11 - Mini App: VKUI compatibility research ADR

Статус: **завершён**.

### Что сделано

- Проверена VKUI совместимость с текущим frontend stack: React `19.2.7`,
  React DOM `19.2.7`, VKUI `8.2.1`.
- `npm info @vkontakte/vkui` показал peer dependencies
  `react: ^18.2.0 || ^19.0.0`, `react-dom: ^18.2.0 || ^19.0.0`; downgrade
  React до 18 не требуется.
- Временная установка `@vkontakte/vkui --save-dev` использовалась только для
  research и не сохранена в репозитории.
- Изолированный prototype с `Button`, `Input`, `Panel`, `PanelHeader`,
  `Tabbar`, `TabbarItem` и `vkui.css` собрался, но показал большой bundle
  impact: baseline `254.06 kB` raw / `77.85 kB gzip`, VKUI prototype
  `661.27 kB` raw / `132.42 kB gzip`, delta `+407.21 kB` raw /
  `+54.57 kB gzip`.
- DX note: `TabbarItem` в VKUI `8.2.1` использует children для label, не старый
  `text` prop.
- ADR outcome: `C - hybrid`, без blind migration; VKUI не добавлен в
  production dependencies.

### Проверки

- `npm run build` baseline — exit 0.
- `npm run build` после временной devDependency без импортов — exit 0.
- `npm run build` isolated VKUI prototype — exit 0.
- `npm audit --json` после временной установки VKUI — 0 vulnerabilities.
- Финальный `npm run build` текущего кода после удаления VKUI/prototype —
  exit 0.

---

## PR-13 - Mini App API hang hotfix

Status: **completed**.

### What changed

- Step 0 confirmed that Mini App `POST /miniapp/jobs` is already async:
  the handler calls `joborchestrator.CreateJob` and returns the DTO without a
  provider call. VK text bot intake uses the same orchestrator path.
- Worker provider `Submit` and `Poll` calls now run under a bounded
  per-call context timeout. Deadline errors are normalized as
  `provider_timeout` and use the existing retry/backoff policy.
- Terminal provider failures release reserved credits before the job is moved
  to `failed_terminal`; billing remains append-only through the existing
  reservation release path.
- Regression tests cover terminal provider failure release, exhausted retry
  release and stuck submit timeout handling.

### Checks

- `go test ./internal/worker` - exit 0.
- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `npm run build` in `web/miniapp` - exit 0.

---

## PR-13.1 - Mini App DeepSeek e2e smoke and fixes

Status: **completed**.

### What changed

- Verified the real Mini App text job flow against DeepSeek through the
  DeepInfra provider adapter (`PROVIDER=deepinfra`, provider chain forced to
  `deepinfra` for smoke).
- Fixed Mini App text `model_id` validation so `deepseek-v4-flash` is accepted
  and persisted in job params without exposing it in `JobDTO`.
- Happy path smoke: `POST /miniapp/jobs` returned in 68 ms, job reached
  `succeeded` in 5.1 s, provider task was `deepinfra` with the DeepSeek model,
  artifact access was owner-scoped, credits were captured once and idempotent
  repeat returned the same job.
- Failure path smoke: unreachable DeepInfra endpoint with one attempt returned
  a job in 55 ms, reached `failed_terminal` with `provider_timeout` in 1.0 s,
  released the reservation once and did not capture credits.

### Checks

- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `npm run build` in `web/miniapp` - exit 0.

---

## PR-14 - Mini App VKUI hybrid base primitives

Status: **completed**.

### What changed

- Added `@vkontakte/vkui` `8.2.1` as a production dependency for the Mini App.
- Wrapped the app root with VKUI `ConfigProvider`, `AdaptivityProvider` and
  `AppRoot`; VK light/dark appearance is bridged through the existing
  `data-scheme` token path.
- Migrated base controls to VKUI: primary/secondary buttons, model/status
  selects, prompt textareas, root panel and the top-level `Chat` / `Workflow`
  `Tabbar`.
- Kept custom signature UX: workflow shell, quick scenario cards, backend job
  rows, `ResultCard`, VK post preview and status timeline.
- Preserved backend-owned decisions: no BFF contract changes, no client-side
  billing/status source of truth, no provider calls from Mini App.

### Checks

- `npm run build` in `web/miniapp` - exit 0. Hybrid bundle:
  `695.18 kB` raw / `142.18 kB gzip`; delta vs PR-14 baseline:
  `+440.69 kB` raw / `+64.04 kB gzip`.
- `go build ./...` - exit 0.
- `npm audit` in `web/miniapp` - 0 vulnerabilities.

---

## PR-15 - Mini App chat parity with VK text bot

Status: **completed**.

### STEP 0 contract

- VK text bot routes conversational text through `commandrouter` and
  `joborchestrator.CreateJob`; VK inbound does not call providers directly.
- Active GPT mode is process-local by peer and uses the same async worker path
  with the `GPT думает...` placeholder.
- DeepInfra/DeepSeek persona and "do not reveal provider/model/backend" rule
  live inside the provider adapter system prompt, not in Mini App frontend.
- Mini App already used `/miniapp/jobs` and `joborchestrator`, but chat UI/API
  exposed selectable text model IDs.

### What changed

- Added `POST /miniapp/chat/messages` for Mini App chat. It verifies launch
  params, rate-limits the verified user, creates a backend-owned
  `text_generate` job through `joborchestrator`, and keeps submit async.
- Mini App text model branding is now the fixed public alias `ChatGPT`.
  Legacy DeepSeek text model IDs are accepted only for compatibility and are
  normalized to `chatgpt` before persistence/API output.
- Added process-local BFF chat context keyed by verified VK user. Context is
  capped, prompt bodies are not stored in `localStorage`, and assistant
  context is appended only after backend `succeeded` plus moderated text
  artifact access.
  Superseded by PR-18.3/18.4/18.5: Mini App chat now uses durable
  `source=miniapp` conversations in Postgres and the BFF no longer keeps
  process-local prompt/answer memory.
- Chat mode frontend now sends text through `/miniapp/chat/messages`, shows
  only `ChatGPT`, keeps safe React text rendering and preserves polling.

### Checks

- `go test ./internal/adapter/inbound/miniapp` - exit 0.
- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `npm run build` in `web/miniapp` - exit 0.
- Credential-bound DeepInfra smoke through `POST /miniapp/chat/messages`:
  job reached `succeeded`, response model name was `ChatGPT`, one text artifact
  was readable through the Mini App artifact route, and the generated text did
  not contain DeepSeek/DeepInfra/provider/model details. The local smoke
  wrapper itself returned non-zero after force-stopping temporary `go run`
  API/worker processes during cleanup; no smoke assertion failed.

---

## PR-16.1 - Mini App 3-tab navigation shell

Status: **completed**.

### STEP 0 context

- Branch: `fastlife_dev`, base/merge-base `e1d5c45`; PR-14 `b2b16a9` and
  PR-15 `2c9bdfa` are present.
- Reused PR-14 VKUI primitives (`Tabbar`, `TabbarItem`, `Button`, `Textarea`,
  `Panel`) and PR-15 ChatGPT chat flow.
- Existing PR-10 Workflow remains the Create surface; no backend/BFF contracts
  changed.

### What changed

- Replaced the two-tab `Chat` / `Workflow` mode switch with a bottom VKUI
  `Создать` / `Чат` / `Настройки` tab shell.
- `Чат` is the default center tab. The selected tab is saved as
  `vk_miniapp_active_tab_v1`, a UI-only localStorage preference.
- `ChatScreen` stays mounted and inactive panels are hidden with CSS, so active
  job polling and UI state survive tab switches.
- `Настройки` is a placeholder with title and "soon" copy only.
- ADR-010 documents the 3-tab navigation decision and PR-16.1-16.4 split.

### Checks

- `npm run build` in `web/miniapp` - exit 0.
- `go build ./...` - exit 0.

---

## Mini App Create/chat UX polish - 2026-06-06

Status: **completed**.

### What changed

- Chat header is now rendered only on the center `Чат` tab. The Create and
  Settings tabs no longer show the `AI` avatar/header strip or chat-history
  button.
- Create starts with a plain vertical service list: `Создать фото`,
  `Создать видео`. The `Создать пост` entry is temporarily disabled in this
  tab; text generation remains available through Chat/VK bot flows.
- The old Create-post preview was removed from the Create flow. Final result
  rendering still uses safe React text/media rendering for backend artifacts.
- Backend contracts, billing, artifact access and polling ownership were not
  changed.

### Checks

- `npm run build` in `web/miniapp` - exit 0.
- Browser smoke on the ngrok Mini App - Create tab has no chat header and
  service choices are limited to photo/video.

---

## Mini App status/polling UX fix - 2026-06-06

Status: **completed**.

### What changed

- Status timeline was tightened for mobile: smaller aligned markers, compact
  rows, lighter connector line and reduced title sizing on the status screen.
- Added frontend auto-resume polling for every non-terminal job already present
  in Mini App state. This covers HMR/reload/history cases where the backend job
  has progressed but the active status screen was left showing an old queued
  state.
- Checked local runtime aggregates: recent jobs are reaching terminal
  `succeeded`, and outbox `event.job.created` / `event.job.queued` rows are
  published. No backend/provider/billing code changed.

### Checks

- `npm run build` in `web/miniapp` - exit 0.

---

## PR-16.2 - Mini App chat threads and history sheet

Status: **completed**.

### STEP 0 context

- PR-16.1 is present on `fastlife_dev`: the Chat tab is a mounted panel inside
  the bottom 3-tab shell, so polling refs survive tab switches.
- PR-15 chat submits use `POST /miniapp/chat/messages`; the BFF accepts
  `conversation_id` as an opaque restricted string. Client UUIDs are accepted,
  while empty `conversation_id` maps to backend `default`.
- The backend has no conversation list/read endpoint. Conversation context is
  still process-local in `cmd/api`, so restart or scale-out can lose context.
  Superseded by PR-18.3/18.4/18.5: authenticated conversation list/history
  endpoints now make backend durable conversation history the source of truth.

### What changed

- Chat threads now have an active id. The migrated/default dialog keeps id
  `default`; new dialogs use client-generated UUIDs and are sent as
  `conversation_id`.
- The old side drawer state is reused as a top history sheet opened by tapping
  the chat title. It shows thread title, session-only last-message preview,
  last activity, new-dialog and clear-local-history actions.
- Local thread persistence moved to `vk_miniapp_threads_v1` and stores only
  `id`, `title` and `last_activity_at`. Legacy/suspicious chat history is
  cleared without logging values.
- The typing indicator is tied to pending job/poll state and disappears only
  after the backend job reaches a terminal state.
- ADR-011 documents thread storage, default migration, graceful degradation and
  the missing durable backend conversation endpoint.

### Checks

- `npm run build` in `web/miniapp` - exit 0.
- `go build ./...` - exit 0.

---

## PR-16.3 - Mini App Create tab generation segment

Status: **completed**.

### STEP 0 context

- PR-16.1 is present on `fastlife_dev`: `Создать` reuses the mounted PR-10
  `WorkflowMode` inside the 3-tab shell.
- Supported Mini App operations are `text_generate`, `image_generate` and
  `video_generate` in backend `operationMeta`; frontend `MODALITIES` mirrors
  only those operations. No BFF discovery endpoint exists.
- PR-10 workflow already contains estimate, status timeline, result screen,
  History and `ResultCard` / VK post preview.

### What changed

- Added a top VKUI `SegmentedControl` for generation type selection in the
  Create tab. It uses only supported backend operations from `MODALITIES`.
- Kept the existing Generate -> Status -> Result flow, backend estimate
  debounce and `enough_credits=true` submit gating.
- History remains available in the Create tab; PR-9 reload recovery still uses
  `GET /miniapp/jobs` and was not moved into local state.
- Switching the operation segment changes only draft modality/model state and
  does not clear `activeJobId`, `jobs` or the `ChatScreen` polling owner.
- The VK post preview is more prominent on the result screen: `ResultCard` is
  full-width in Create result and keeps text as React text plus media through
  backend artifact routes only.

### Checks

- `npm run build` in `web/miniapp` - exit 0.
- `go build ./...` - exit 0.

---

## PR-16.3.1 - Mini App Create choice screen and chat history button

Status: **completed**.

### STEP 0 context

- Continued directly after PR-16.3 (`259639c`) on `fastlife_dev`.
- Reused known PR-16.3 files only: `WorkflowMode`, chat history panel trigger,
  `ResultCard` and `theme.css`. Backend/BFF contracts were not changed.

### What changed

- Removed the top Create operation segment. Create now opens on two large
  cards: `Создать фото`, `Создать видео`.
- The cards route into the existing PR-10 Generate -> Status -> Result flow:
  photo uses `image_generate`, video uses `video_generate`. The previous
  Create-post path is disabled for now; Chat/VK bot text generation remains
  separate.
- Estimate/gating remains backend-owned through `POST /miniapp/estimate`;
  submit still requires `enough_credits=true`.
- Create history is scoped to the selected type by filtering backend jobs by
  operation. The general all-types Create history is deferred to Settings
  PR-16.4.
- Chat thread history now opens from an explicit header icon button. Tapping
  the chat title no longer opens the panel.

### Checks

- `npm run build` in `web/miniapp` - exit 0.
- `go build ./...` - exit 0.

---

## PR-16.4 - Mini App Settings tab and brand-driven polish

Status: **completed**.

### What changed

- Settings is now a real tab: theme preference (`system` / `light` / `dark`),
  backend balance display, payment-history placeholder, privacy note and local
  history clear action.
- Settings polish update: theme choices no longer show explanatory copy, balance
  is presented as a dedicated account card with refresh and top-up actions, and
  payment/history sections are collapsible lists to keep the tab within mobile
  bounds.
- Added summary generation history in Settings, sourced from backend jobs and
  filterable by all/post/photo/video. Create keeps only operation-scoped
  workflow history.
- Theme preference is stored only as `vk_miniapp_theme_v1`; balance, job state
  and billing remain backend-owned.
- Applied brand-driven design tokens from the provided community banner/avatar:
  accessible cyan action accent, violet and pink secondary accents, light/dark
  neutral scales and VKUI scheme synchronization.
- Replaced the lower tabbar letter markers with simple CSS icons; no decorative
  emoji/sticker controls were added.

### Backend dependency

- Mini App has `GET /miniapp/balance`, but no read-only payment or ledger
  history endpoint. Settings shows a safe placeholder and tracks this as a
  separate backend follow-up.
- Mini App has no top-up/payment-intent endpoint yet. The Settings top-up button
  does not mutate balance locally; the documented backend follow-up is a shared
  Mini App/VK bot payment-intent flow that appends committed `topup` ledger
  entries only after trusted payment confirmation.

### Checks

- `npm run build` in `web/miniapp` - exit 0.
- `go build ./...` - exit 0.

---

## Integration merge - fastlife_dev into feature/integration-web-backend

Status: **completed locally, pending push at merge time**.

Date: 2026-06-06

### Branches

- Target before merge: `feature/integration-web-backend` at `44df8d4`.
- Source before merge: `fastlife_dev` at `f5f4873`.
- Backup branch: `backup/pre-merge-integration-web-backend`.
- Merge base: `e1d5c45`.

### Conflict resolution

- `TASKS.md`: combined both follow-up sets instead of choosing a side:
  colleague VK bot/domain/live-smoke items and Mini App payment/top-up/thread
  backend dependencies are all retained.
- `internal/worker/worker_test.go`: merged the test harnesses so worker tests
  keep both VK text dialog context coverage and Mini App provider timeout /
  reservation release coverage.
- Auto-merged files were reviewed manually:
  - `cmd/api/main.go` keeps `/webhooks/vk`, admin/health/metrics and
    `/miniapp/*` wiring plus VK control/profile, referral, dialog state,
    anti-spam and Mini App BFF deps.
  - `internal/worker/worker.go` and `generation.go` keep text context
    preparation/completion and provider Submit/Poll timeouts with terminal
    reservation release.
  - `internal/service/billingservice/service.go` remains append-only through
    repository ledger/reservation methods; estimate balance reads do not write.
  - config and migrations were union-reviewed; migrations `000005`-`000007`
    are from the VK bot branch and have no Mini App numbering conflict.

### Checks

- conflict marker scan - clean.
- `gofmt -l .` - exit 0.
- `go build ./...` - exit 0.
- `go test ./...` - exit 0.
- `npm --prefix web/miniapp run build` - exit 0.

---

## PR-17.4 - Simplify API bootstrap wiring

Status: **completed**.

### STEP 0 context

- After PR-17.2 and PR-17.3, `cmd/api/main.go` no longer owned VK bot or
  Mini App surface internals, but `main()` still mixed repository/service
  construction with app-surface wiring and route mounting.
- Safe simplification was limited to bootstrap shape: keep behavior, env keys,
  health/admin, rate limits and route map unchanged.

### What changed

- Added `internal/app/api/core.go` as a bootstrap-only helper that groups shared
  backend-core repositories and services for app surfaces.
- `cmd/api/main.go` now calls `apiapp.NewSharedCore(pool, cfg)`, wires
  `vkbot` and `miniapp` modules, mounts routes, and keeps health/admin/metrics
  in the API binary.
- Final route map is unchanged: `/webhooks/vk`, `/admin/`, `/miniapp/`,
  `/metrics`, `/health`, `/healthz`.
- Provider execution remains worker-owned through job flow; billing setup still
  uses `billingservice` and ledger-backed repositories.

### Checks

- `gofmt -w cmd/api/main.go internal/app/api/core.go` - exit 0.
- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `npm --prefix web/miniapp run build` - exit 0.
- `gofmt -l internal/adapter/inbound/miniapp/handler.go internal/adapter/inbound/miniapp/dto.go internal/adapter/inbound/miniapp/conversation_id.go internal/adapter/inbound/miniapp/handler_test.go` - clean.
- `git grep -nE "^(<<<<<<<|=======|>>>>>>>)"` - clean.
- `git diff --check` - exit 0; only markdown CRLF normalization warnings.

---

## PR-18 planning - Durable shared chat context

Status: **planned**.

### Finding

- VK text bot context is already durable through `internal/service/dialogcontext`,
  Postgres conversations/messages/summaries and worker-owned prompt rendering.
- Mini App chat currently passes `conversation_id`, but the BFF also keeps
  recent turns in a process-local `internal/adapter/inbound/miniapp`
  conversation store. That state can be lost on API restart or scale-out.
- The current durable conversation key `user_id + vk_peer_id` is sufficient for
  VK bot private/group peers but not sufficient for multiple Mini App threads
  owned by the same VK user.

### Plan

- Added ADR-017 in `DECISIONS.md`: Mini App must not call VK bot, and VK bot
  context logic must not be copied into Mini App. Both surfaces should use a
  shared durable chat/conversation core while provider calls stay worker-owned.
- Added `docs/CHAT_CONTEXT_REFACTOR_LOG.md` as the running log for this fix.
- Added `TEMP_PR18_DURABLE_CHAT_CORE_PROMPTS.md` with copy/paste prompts for
  PR-18.1 through PR-18.5.
- Added PR-18 backlog items to `TASKS.md`.

### Checks

- Docs/planning only. Runtime checks not run at planning time.

---

## PR-18.1 - Durable conversation identity foundation

Status: **completed**.

### STEP 0 context

- Existing `conversations` schema used `user_id + vk_peer_id` as the only
  active conversation identity. That remains correct for VK bot, but cannot
  represent multiple Mini App threads for one verified VK user.
- Mini App runtime behavior is not switched in this PR.

### What changed

- Added migration `000008_conversation_sources` with `source`,
  `external_thread_id`, VK-bot active unique index, Mini App/source-thread
  active unique index and source-list index.
- Extended `domain.Conversation` with `Source` and `ExternalThreadID`.
- Added `domain.ConversationRef`.
- Extended `domain.ConversationRepository` with explicit reference lookup,
  owner lookup and list-by-source methods.
- Updated Postgres and memory repositories.
- Added memory repository tests for VK bot backward compatibility and Mini App
  thread isolation.

### Checks

- `go test ./internal/adapter/storage/memory ./internal/service/dialogcontext` - exit 0.
- `go test ./internal/adapter/storage/postgres` - exit 0.
- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `git grep -nE "^(<<<<<<<|=======|>>>>>>>)"` - clean.
- `git diff --check` - clean.

---

## PR-18.2 - Explicit conversation references in worker/dialogcontext

Status: **completed**.

### STEP 0 context

- `dialogcontext.Prepare` and `dialogcontext.Complete` previously keyed text
  memory through the VK bot fallback identity `user_id + vk_peer_id`.
- `worker.buildRequest` already calls `dialogcontext.Prepare` before provider
  submission and persists a durable `conversation_id` into `job.Params`.
- `worker.saveDialogAnswer` already calls `dialogcontext.Complete` after text
  provider success when a durable `conversation_id` is present.

### What changed

- Added explicit text-job conversation params support:
  `conversation_source`, `external_thread_id` and durable `conversation_id`.
- `dialogcontext.Prepare` now prefers explicit `source=miniapp` plus
  `external_thread_id` when present, while VK bot jobs without explicit params
  still use `user_id + vk_peer_id`.
- `dialogcontext.Complete` can save assistant answers for explicit durable
  conversations as well as VK bot fallback conversations.
- The worker preserves `conversation_source` and `external_thread_id` when it
  patches `job.Params` with the durable `conversation_id`.
- Invalid or empty explicit refs degrade to a plain prompt with no conversation
  context, avoiding accidental cross-thread context mixing.

### Compatibility and isolation

- VK bot behavior remains backward compatible.
- Mini App BFF behavior is not switched yet; that remains PR-18.3.
- Added tests for Mini App thread A/B isolation, VK bot vs Mini App isolation
  for the same backend user, invalid explicit ref degradation, and worker param
  preservation.

### Checks

- `go test ./internal/service/dialogcontext ./internal/worker` - exit 0.
- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `gofmt -l internal/service/dialogcontext/service.go internal/service/dialogcontext/service_test.go internal/worker/worker.go internal/worker/worker_test.go` - clean.
- `git grep -nE "^(<<<<<<<|=======|>>>>>>>)"` - clean.
- `git diff --check` - exit 0; only markdown CRLF normalization warnings.

---

## PR-18.3 - Mini App uses durable chat context

Status: **completed**.

### STEP 0 context

- Mini App chat used `internal/adapter/inbound/miniapp/conversation.go` as a
  process-local prompt-prefix memory store.
- `createChatMessage` used the store to prepend recent Mini App turns to the
  prompt before creating a text job.
- `getJob` used `captureChatResult` to read a completed text artifact and put
  the assistant answer back into the process-local store.

### What changed

- Removed the process-local Mini App chat memory store.
- Kept only safe `conversation_id` normalization for the BFF contract.
- `POST /miniapp/chat/messages` now creates a text job with raw current prompt
  plus explicit durable chat params:
  - `conversation_source=miniapp`;
  - `external_thread_id=<normalized conversation_id>`;
  - public `model_name=ChatGPT`.
- Initial `conversation_id` in job params is left empty so the worker can patch
  the durable backend conversation UUID after `dialogcontext.Prepare`.
- `conversation_id=""` remains backward compatible as `default`.

### Compatibility and isolation

- Mini App auth, rate limiting, billing reservation and job creation still run
  through the same BFF/orchestrator path.
- Mini App BFF does not call providers and no longer stores prompt/answer
  history in process memory.
- VK bot context remains unchanged.

### Checks

- `go test ./internal/adapter/inbound/miniapp` - exit 0.
- `go test ./internal/adapter/inbound/miniapp ./internal/service/dialogcontext ./internal/worker` - exit 0.
- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `npm --prefix web/miniapp run build` - exit 0.

---

## PR-18.4 - Durable Mini App chat list and history

Status: **completed**.

### STEP 0 context

- Frontend chat state used local `Chat` objects and `localStorage` metadata key
  `vk_miniapp_threads_v1`; legacy `vk_miniapp_chats_v1` was already cleared on
  load.
- Backend durable conversation repo already had owner-scoped Mini App thread
  lookup and message listing through `ConversationRepository`.

### What changed

- Added authenticated Mini App BFF endpoints:
  - `GET /miniapp/chat/conversations`;
  - `GET /miniapp/chat/conversations/{id}/messages`.
- Wired `ConversationRepository` from shared API core into the Mini App app
  surface.
- Endpoint responses are scoped to the verified backend user and expose only
  product-level thread/message DTOs: opaque thread id, title, timestamps,
  preview, `user`/`bot` role and message text.
- Frontend `api/client.ts` now reads conversation list and active-thread
  messages from the backend.
- Chat UI now treats backend list/history as source of truth and keeps only the
  active thread id in `localStorage` under `vk_miniapp_active_thread_v1`.
- Old local thread/message keys are removed when encountered.

### Security and compatibility

- Both endpoints require existing VK launch-param auth.
- Invalid thread ids return safe 400; missing/not-owned threads return safe
  404.
- `localStorage` no longer stores prompt/answer text or thread metadata.
- No provider calls were added to Mini App BFF; jobs still go through
  `joborchestrator` and worker-owned provider execution.

### Checks

- `go test -count=1 ./internal/adapter/inbound/miniapp` - exit 0.
- `go test ./internal/adapter/inbound/miniapp ./internal/adapter/storage/postgres ./internal/adapter/storage/memory` - exit 0.
- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `npm --prefix web/miniapp run build` - exit 0.
- `gofmt -l cmd/api/main.go internal/app/api/core.go internal/app/miniapp/module.go internal/adapter/inbound/miniapp/handler.go internal/adapter/inbound/miniapp/dto.go internal/adapter/inbound/miniapp/handler_test.go` - clean.
- `git grep -nE "^(<<<<<<<|=======|>>>>>>>)"` - clean.
- `git diff --check` - exit 0; only line-ending normalization warnings.

---

## PR-18.5 - Shared durable chat context cleanup and verification

Status: **completed**.

### What was verified

- VK bot and Mini App chat both use the durable shared conversation core:
  `conversations`, `conversation_messages`, `conversation_summaries`,
  `internal/service/dialogcontext` and worker-owned prompt rendering.
- No process-local Mini App prompt/answer store remains in
  `internal/adapter/inbound/miniapp`.
- Provider calls still occur only from the worker flow; `cmd/api`, VK inbound
  and Mini App BFF handlers remain job/control surfaces.
- Mini App frontend local storage is limited to active thread/tab/theme UI
  state plus legacy cache cleanup; prompt bodies, generated answers, job ids,
  artifact ids/URLs, launch params, tokens, balance and provider details are
  not persisted.
- Public Mini App chat model output remains `ChatGPT`.

### Cleanup/docs

- Marked old Mini App process-local chat-memory notes as historical or
  superseded in audit/progress/ADR/review/integration docs, including the old
  frontend audit.
- Updated architecture and runbook docs with the final shared durable chat
  context contract and Mini App conversation list/history smoke checks.
- Updated task tracking to mark PR-18.5 complete.

### Checks

- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `npm --prefix web/miniapp run build` - exit 0.
- `git grep -nE "^(<<<<<<<|=======|>>>>>>>)"` - clean.
- `git diff --check` - exit 0; only line-ending normalization warnings.

---

## PR-17.5 - Document app surface architecture

Status: **completed**.

### What changed

- Updated `docs/ARCHITECTURE.md` with a current implementation addendum for app
  surfaces over shared backend core: `internal/app/vkbot`,
  `internal/app/miniapp`, `internal/app/api`, `cmd/api` bootstrap and
  `cmd/worker` provider/job execution ownership.
- Updated `RUNBOOK.md` with API wiring locations, where to add VK bot commands
  and Mini App BFF endpoints, and a smoke checklist for both entrances.
- Updated `README.md` layout/current-status notes to point future agents at
  app-surface modules.
- Marked PR-17.2 through PR-17.5 complete in `TASKS.md`.

### Architecture notes

- VK bot and Mini App remain app surfaces only. They may wire handlers and
  dependencies, but provider calls, pricing, balance mutation, job status truth,
  moderation and artifact ownership remain in backend core.
- `cmd/api/main.go` remains bootstrap plus route mounting; `cmd/worker` remains
  the provider/artifact/moderation/delivery/billing-capture owner.

### Checks

- `go test ./...` - exit 0.
- `go build ./...` - exit 0.
- `npm --prefix web/miniapp run build` - exit 0.

### Notes

- No real VK/DeepInfra live smoke was run during the merge because it would
  require credential-bound runtime and VK/tunnel/domain setup. The merge keeps
  both runtime paths wired for follow-up smoke on `neiirohub.ru` or an approved
  dev tunnel.

---

## PR-17.1 - App surface refactor ADR

Status: **completed**.

### What changed

- Added ADR-016 to document VK text bot and Mini App as app surfaces above the
  shared backend core.
- Defined target modules `internal/app/vkbot` and `internal/app/miniapp`.
- Kept backend source-of-truth responsibilities in shared core:
  `internal/domain`, `internal/service`, `internal/worker`, provider adapters,
  storage, delivery, artifact and moderation components.
- Added PR-17.2 through PR-17.5 backlog items to `TASKS.md`.
- Preserved the temporary copy-paste prompt queue in
  `TEMP_PR17_SURFACE_REFACTOR_PROMPTS.md` for the remaining PR-17 steps.

### Checks

- Docs-only change. Runtime build/test were intentionally not run because no
  Go, frontend, env or migration files changed.

---

## PR-17.2 - Extract VK bot API module

Status: **completed**.

### STEP 0 context

- `cmd/api/main.go` previously owned VK delivery control/profile client setup,
  Redis-backed dialog mode, Redis-backed anti-spam, referral service wiring,
  `vkinbound.NewHandler` and `vkMenuFeatures`.
- Shared backend-core deps remain outside the surface module: repositories,
  billing service, job orchestrator, command router, Redis client, config and
  logger.

### What changed

- Added `internal/app/vkbot/module.go` as the VK text bot app-surface wiring
  module.
- Moved VK control/profile client setup, dialog state, anti-spam, referral
  service construction, inbound handler construction and menu feature mapping
  out of `cmd/api/main.go`.
- `cmd/api/main.go` now creates shared core deps and mounts the returned VK
  handler at `/webhooks/vk`; admin, Mini App, metrics and health routes are
  unchanged.
- Runtime behavior is intended to stay unchanged: VK inbound still creates
  Jobs through `joborchestrator`; provider calls remain worker-owned and
  billing/referral rewards still go through `billingservice` ledger methods.

### Checks

- `go test ./internal/adapter/inbound/vk ./internal/service/antispam ./internal/service/dialogstate ./internal/service/referralservice` - exit 0.
- `go test ./internal/app/vkbot` - exit 0.
- `go build ./...` - exit 0.
- `go test ./...` - exit 0.

---

## PR-17.3 - Extract Mini App API module

Status: **completed**.

### STEP 0 context

- `cmd/api/main.go` previously owned Mini App BFF wiring: S3 object-store setup
  for artifact reads, Mini App per-user rate limiter, `miniappapi.NewHandler`
  and deps for users/jobs/artifacts/moderation/billing/orchestrator/logger.
- Shared backend-core deps remain outside the surface module: repositories,
  billing service, job orchestrator, config, logger and process lifecycle.

### What changed

- Added `internal/app/miniapp/module.go` as the Mini App app-surface wiring
  module.
- Moved Mini App S3 object reader setup, Mini App rate limiter construction and
  `miniappapi.NewHandler` construction out of `cmd/api/main.go`.
- `cmd/api/main.go` now creates shared core deps and mounts the returned Mini
  App handler at `/miniapp/`; VK bot, admin, metrics and health routes are
  unchanged.
- BFF contracts are unchanged. The underlying `internal/adapter/inbound/miniapp`
  route map still exposes estimate, chat messages, jobs, balance and artifact
  endpoints.

### Checks

- `go test ./internal/adapter/inbound/miniapp` - exit 0.
- `go test ./internal/app/miniapp` - exit 0.
- `go build ./...` - exit 0.
- `go test ./...` - exit 0.
- `npm --prefix web/miniapp run build` - exit 0.
