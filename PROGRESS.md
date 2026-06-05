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
  - добавлены env-переменные OpenAI text/image/video/moderation/scanner и `PROVIDER_CHAIN`;
  - обновлены `README.md`, `RUNBOOK.md`, `TESTING.md`, `TASKS.md`, `AGENTS.md`, `AUDIT.md`, `ROADMAP.md`;
  - добавлены unit-тесты OpenAI text/image/video/moderation/scanner, VK upload pipeline, delivery upload и provider fallback.
- **VK inbound attachments**:
  - sticker-only сообщения больше не превращаются в пустой prompt;
  - handler синтезирует text prompt с `sticker_id/product_id`; prompt проходит в `text.ask` job только при активном GPT text mode или legacy `VK_UNROUTED_TEXT_MODE=gpt`, поэтому стикер не теряется и не создает случайный billable job вне режима;
  - фото/видео/аудио attachments остаются задачей полноценного input Artifact pipeline.
- **VK product menu**:
  - menu flow переведен на декларативный `menuScreen` registry: каждый control-command указывает текст, inline keyboard, необходимость баланса и optional welcome attachment;
  - первичная нижняя VK keyboard содержит только одну кнопку `Старт`;
  - после нажатия `Старт` бот заменяет нижнюю постоянную клавиатуру на одну кнопку `Показать меню`;
  - `Показать меню` хранится как отдельный `show_menu` control-command: нижняя persistent-кнопка всегда отправляет свежий VK inline menu вниз без повторной переустановки нижней клавиатуры, а inline-переходы внутри меню продолжают редактировать active menu message;
  - `Старт`, `/start`, `меню` и `начать` открывают VK inline keyboard под welcome-сообщением в стиле Super GPT;
  - `Создать видео` теперь открывает отдельный inline-экран `Выбери модель для генерации:` с моделями `Sora 2`, `Kling v2.1`, `Seedance 1`, `Haiuo v0.2` и кнопкой `Назад`;
  - `Sora 2` и `Kling v2.1` открывают detail-экраны с описанием, prompt-примером, ссылкой на инструкцию и кнопками `Начать генерацию`, `Примеры`, `Назад`;
  - `Seedance 1` открывает выбор `Lite` / `Pro`, а `Haiuo v0.2` открывает выбор `Обычный` / `Fast`;
  - кнопки выбора video-модели и вложенных video submenu записываются как control commands и не создают billable jobs до подключения model-specific generation state;
  - `Создать фото` при одной основной модели пропускает выбор модели и сразу показывает инструкцию по `Фото по тексту` / `Фото с референсом` с кнопками режимов и `Назад`;
  - `Спросить у GPT` открывает active-сообщение `SUPER GPT активен` без создания job и включает process-local GPT text mode для `peer_id`; следующий обычный текст/стикер пользователя проходит через `text.ask` flow;
  - `Студентам и школьникам` открывает учебное подменю: `Решальник задач`, `Генерация презентаций (скоро)`, `Создание рефератов (скоро)`, `Ответы на вопросы`, `Назад`;
  - `vkdelivery.HTTPClient` получил `SendMessage` с `keyboard` JSON, поэтому VK API по-прежнему вызывается только из `internal/adapter/delivery/vk`;
  - `vkdelivery.HTTPClient` получил `EditMessage` поверх VK `messages.edit`, а `ControlClient` теперь покрывает и send, и edit для product/control меню;
  - `vkdelivery.KeyboardButton` получил `ActionType`, поэтому inline menu можно рендерить как VK `callback` или legacy `text` без переписывания payload;
  - `VK_MENU_BUTTON_MODE=callback` стал дефолтом для inline menu: нажатия приходят как VK `message_event` и не добавляют пользовательские echo-сообщения в чат; `VK_MENU_BUTTON_MODE=text` возвращает прежнее поведение;
  - добавлены `VK_MENU_*_ENABLED` feature flags для каждой основной и вложенной product-menu кнопки: disabled buttons скрываются из новых keyboard, а stale payload от старого сообщения падает обратно в актуальное главное меню без создания job;
  - handler хранит process-local active menu и dialog mode по `peer_id`: кнопочные payload-переходы редактируют текущий menu message, обычный пользовательский текст вне GPT mode оставляет предыдущее меню доступным выше, а другой control-экран сбрасывает GPT mode;
  - `VK_UNROUTED_TEXT_MODE=reply` стал дефолтом для обычного текста вне GPT mode: handler записывает `unknown` command, не создает Job и отправляет text-only hint `Выберите режим в меню выше.` без дублирования inline keyboard; `silent` молчит, `gpt` возвращает legacy any-text-to-GPT behavior;
  - handler обрабатывает `message_event` как control-only inbound event: сохраняет inbound/command, но не создает Job и не дергает provider;
  - каждый `message_event` подтверждается blank `messages.sendMessageEventAnswer` через `vkdelivery.ControlClient`, чтобы VK-клиент снимал loading spinner с callback-кнопки;
  - если VK не разрешает edit текущего menu message, API логирует warn, очищает active menu и делает fallback на обычный `messages.send`;
  - кнопки `Создать видео`, `Создать фото`, `Спросить у GPT`, `Студентам и школьникам`, `Мой аккаунт`, `Пополнить баланс` классифицируются как control commands и не создают пустые billable jobs;
  - баланс в меню берется через `billingservice.EnsureAccount`, без прямой мутации баланса;
  - опциональный баннер подключается через `VK_WELCOME_ATTACHMENT` как уже готовый VK attachment string;
  - если VK возвращает `error_code=912` из-за выключенных bot features, API повторяет отправку без keyboard, чтобы callback не падал.

### Проверки

- Targeted tests: `go test ./internal/adapter/provider/openai ./internal/adapter/delivery/vk ./internal/adapter/inbound/vk ./internal/service/commandrouter ./internal/worker ./internal/platform/config`.
- Added VK menu UX coverage: `EditMessage` request shape, mock edit semantics, active-menu edit, lower `Показать меню` fresh send, and plain-message text-only hint behavior.
- Added callback menu coverage: callback keyboard JSON, `VK_MENU_BUTTON_MODE` config validation, `message_event` command processing, no-job invariant, and legacy text-button mode.
- Added callback ack coverage: real `messages.sendMessageEventAnswer` request shape, mock answer recording, and inbound `message_event` acknowledgement.
- Added unrouted text coverage: default text-only choose-mode hint/no-job, `silent` no-response mode, legacy `gpt` mode, GPT button enabling text jobs, menu transitions clearing GPT mode, and sticker-to-text job only inside GPT mode.
- Added menu feature flag coverage: hidden main buttons, hidden nested video buttons, disabled stale payload fallback, and env loading for `VK_MENU_*_ENABLED`.
- Full regression checks выполняются после документационного sync.
- Live VK `/start` smoke: callback returned `ok`, command persisted as `start`,
  zero jobs created, welcome text delivered to VK. After enabling bot features
  in community settings, VK accepts keyboard sends without `error_code=912`.

### Текущие ограничения

- Реальные OpenAI/VK вызовы не прогонялись без пользовательских credentials; нужен live smoke на dev-аккаунтах.
- Второй реальный provider для fallback ещё не добавлен; сейчас fallback можно проверить с `mock`.
- VK control/menu responses пока отправляются напрямую из API через `vkdelivery.ControlClient` с deterministic `random_id`; если на product/control sends распространяем invariant `Every delivery attempt is persisted`, нужен отдельный persisted delivery/outbox flow для таких сообщений.
- Active-menu tracking и GPT dialog mode пока хранятся в памяти процесса `cmd/api`; после рестарта API или при multi-instance балансировке меню может отправиться новым сообщением, а пользователь может потерять выбранный GPT mode. Перед production-scale нужен persisted conversation/menu state.
- Video artifact scanner пока fail-open; полноценный video scan/probe/transcode остаётся Phase 3 media pipeline.
- Нужен resume fix для edge-case: `provider_task=succeeded`, но artifact/result_ready ещё не сохранены после crash.

### Next step

См. актуальный backlog в `TASKS.md`; ближайший фокус — live smoke с реальными
ключами, второй реальный provider для fallback, video media pipeline и worker
resume hardening.
