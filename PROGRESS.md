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

## Next step — Step 3

**Job Orchestrator и VK Inbound Gateway.**

Ожидаемый объём:

- `joborchestrator.CreateJob`: валидация команды, резерв кредитов через `BillingRepository`,
  создание job и запись outbox-события в одной транзакции, возврат job id.
- VK Inbound handler: парсинг raw-события, валидация группы/секрета, idempotency по `vk_event:*`,
  сохранение inbound-события и постановка команды в очередь, быстрый ответ VK.
- Mock-провайдер под интерфейс `domain.Provider` и каркас worker-пула.
