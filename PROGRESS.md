# PROGRESS

Журнал прогресса по разработке VK AI Aggregator (Go backend, AI Job Processing Platform).
Источник истины по архитектуре — `docs/ARCHITECTURE.md`, инварианты — `AGENTS.md`.

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
  - любой другой текст (включая неизвестные слэш-команды) → `text.ask` / `text_generate`.
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
  - добавлены env-переменные OpenAI text/image/video/moderation/scanner и `PROVIDER_CHAIN`;
  - обновлены `README.md`, `RUNBOOK.md`, `TESTING.md`, `TASKS.md`, `AGENTS.md`, `AUDIT.md`, `ROADMAP.md`;
  - добавлены unit-тесты OpenAI text/image/video/moderation/scanner, VK upload pipeline, delivery upload и provider fallback.
- **VK inbound attachments**:
  - sticker-only сообщения больше не превращаются в пустой prompt;
  - handler синтезирует text prompt с `sticker_id/product_id`, поэтому стикер проходит через обычный `InboundEvent -> Command -> Job` flow;
  - фото/видео/аудио attachments остаются задачей полноценного input Artifact pipeline.
- **VK product menu**:
  - первичная нижняя VK keyboard содержит только одну кнопку `Старт`;
  - после нажатия `Старт` бот заменяет нижнюю постоянную клавиатуру на одну кнопку `Показать меню`;
  - `Показать меню` хранится как отдельный `show_menu` control-command и открывает VK inline keyboard под welcome-сообщением без повторной переустановки нижней клавиатуры;
  - `Старт`, `/start`, `меню` и `начать` открывают VK inline keyboard под welcome-сообщением в стиле Super GPT;
  - `vkdelivery.HTTPClient` получил `SendMessage` с `keyboard` JSON, поэтому VK API по-прежнему вызывается только из `internal/adapter/delivery/vk`;
  - кнопки `Создать видео`, `Создать фото`, `Спросить у GPT`, `Студентам и школьникам`, `Мой аккаунт`, `Пополнить баланс` классифицируются как control commands и не создают пустые billable jobs;
  - баланс в меню берется через `billingservice.EnsureAccount`, без прямой мутации баланса;
  - опциональный баннер подключается через `VK_WELCOME_ATTACHMENT` как уже готовый VK attachment string;
  - если VK возвращает `error_code=912` из-за выключенных bot features, API повторяет отправку без keyboard, чтобы callback не падал.

### Проверки

- Targeted tests: `go test ./internal/adapter/provider/openai ./internal/adapter/delivery/vk ./internal/adapter/inbound/vk ./internal/service/commandrouter ./internal/worker ./internal/platform/config`.
- Full regression checks выполняются после документационного sync.
- Live VK `/start` smoke: callback returned `ok`, command persisted as `start`,
  zero jobs created, welcome text delivered to VK. After enabling bot features
  in community settings, VK accepts keyboard sends without `error_code=912`.

### Текущие ограничения

- Реальные OpenAI/VK вызовы не прогонялись без пользовательских credentials; нужен live smoke на dev-аккаунтах.
- Второй реальный provider для fallback ещё не добавлен; сейчас fallback можно проверить с `mock`.
- VK control/menu responses пока отправляются напрямую из API через `vkdelivery.ControlClient` с deterministic `random_id`; если на product/control sends распространяем invariant `Every delivery attempt is persisted`, нужен отдельный persisted delivery/outbox flow для таких сообщений.
- Video artifact scanner пока fail-open; полноценный video scan/probe/transcode остаётся Phase 3 media pipeline.
- Нужен resume fix для edge-case: `provider_task=succeeded`, но artifact/result_ready ещё не сохранены после crash.

### Next step

См. актуальный backlog в `TASKS.md`; ближайший фокус — live smoke с реальными
ключами, второй реальный provider для fallback, video media pipeline и worker
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
