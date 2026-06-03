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

## Next step — Step 5

**Worker-пулы и реконсиляция.**

- Воркеры на `redisqueue.Consumer`: dispatch задачи в `domain.Provider`, сохранение
  `ProviderTask`, переходы статусов job, `provider_poll` для асинхронных результатов.
- По результату провайдера: скачивание выходов через `artifactservice.SaveRemoteArtifact`,
  `capture`/`refund` в биллинге, постановка в `stream:jobs:delivery`.
- Рефакторинг `BillingRepository` на `Querier`, чтобы job+reserve+outbox шли одной транзакцией.
