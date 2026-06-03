# TASKS

Бэклог и трекинг работ по VK AI Aggregator. Источник истины по архитектуре —
`docs/ARCHITECTURE.md`, инварианты — `AGENTS.md`, журнал — `PROGRESS.md`.

Легенда: `[x]` сделано, `[~]` частично, `[ ]` запланировано.

---

## Done

### Step 1 — Domain, repository interfaces, migration 000001
- [x] Domain-сущности (`user`, `job`, `command`, `provider`, `artifact`, `delivery`, `billing`).
- [x] Стейт-машина job (`jobTransitions`, `CanTransitionTo`, `IsTerminal`).
- [x] Интерфейсы репозиториев + доменные ошибки.
- [x] Миграция `000001_init_schema`.

### Step 2 — PostgreSQL adapters, Command Router
- [x] `pgx/v5` репозитории всех сущностей (+ `Querier`, `RunInTx`, `mapError`).
- [x] Command Router (`/image /video /edit /balance /status /cancel /help`, прочее → `text_generate`).
- [x] Env-guarded integration-тесты репозиториев.

### Step 3 — Billing, Orchestrator, VK Webhook
- [x] Billing Service (estimate/ensure/reserve/capture/release/refund, старт-баланс 1000).
- [x] Job Orchestrator (`Command → Estimate → Reserve → Job → Outbox → Queue`).
- [x] VK Webhook `POST /webhooks/vk` (confirmation + message_new, идемпотентность, без вызова Provider).
- [x] `InboundEvent` + миграция `000002`, `uow.Manager`, `queue.Publisher` (in-memory), in-memory адаптеры.

### Step 4 — Queue, Providers, Artifacts
- [x] Redis Streams + consumer groups: 5 стримов, `Publisher`/`Consumer` (XADD/XREADGROUP/XACK).
- [x] `MockProvider` (Estimate/Submit/Poll/Cancel) + ошибки `mock_timeout`/`mock_rate_limit`/`mock_provider_error`.
- [x] Artifact Service (`SaveTextArtifact`/`SaveBytesArtifact`/`SaveRemoteArtifact`, дедуп по sha256).
- [x] S3/MinIO adapter (`minio-go/v7`: EnsureBucket/Put/PresignedGetURL).
- [x] In-memory `ArtifactRepo` + `ObjectStore`, unit-тесты провайдера и artifact-сервиса, env-guarded Redis-тест.

### Step 5 — Workers и Provider Poll
- [x] `GenerationWorker` (text/image/video): `Job → Provider → ProviderTask → Artifact → Delivery Queue`.
- [x] `PollWorker`: `Poll → Update Status → Requeue → Artifact → Delivery Queue`.
- [x] `Engine` (Read/Ack/Recover через `XAUTOCLAIM`), `Registry`, in-memory `ProviderTaskRepo`.
- [x] Retry safety, idempotency (active-task + per-attempt key), error classification, recovery после рестарта.
- [x] Провайдер вызывается только внутри воркера; unit-тесты sync/async/idempotency/retry/terminal.

---

## Next — Step 6: VK Delivery worker, биллинг по результату, outbox relay
- [ ] Delivery-воркер на `stream:jobs:delivery`: upload+send в VK, дедуп по `vk_random_id`, `result_ready → delivering → succeeded`.
- [ ] `capture` при успехе / `refund` при терминальном fail (lookup reservation по job).
- [ ] Outbox relay: drain `outbox_events` → publish в стримы.
- [ ] Рефакторинг `BillingRepository` на `Querier` (job+reserve+outbox в одной транзакции).

## Backlog
- [ ] VK delivery adapter (`internal/adapter/delivery/vk`): upload + send, дедуп по `vk_random_id`.
- [ ] Реальные provider-адаптеры (OpenAI/Google/Kling/Runway) через нормализацию.
- [ ] Модерация перед выдачей результата (инвариант #15).
- [ ] Outbox relay (drain `outbox_events` → publish в стримы).
- [ ] Observability: метрики очередей/воркеров, трейсинг по `correlation_id`.
