# Общая основа интеграций APIMart — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** обеспечить безопасное подключение отдельных моделей из [реестра моделей](./README.md), без смешения версий и тарифов.

**Architecture:** текущий путь Job → worker → provider adapter → Artifact → moderation → delivery сохраняется. Каталог разрешает публичный model_id в доверенный snapshot; только worker вызывает APIMart.

**Tech Stack:** Go, PostgreSQL, Redis, object storage, Next.js/TypeScript.

**Spec:** [границы задания и полный список](./README.md). Статус: локальный инструмент B0 реализован; реальные проверки B0 и оставшиеся расширения основы не завершены. По выбору пользователя [Qwen Image 3.0](../../../runbooks/DEV.md#qwen-image-30-configuration) реализована напрямую на существующих компонентах; отдельные B0–B3 не являлись предварительными этапами. Это не закрывает общий B2 и прочие расширения основы.

## Состояние основы после сверки с кодом

Сверка: 08.09.2026, `dev-deploy`, HEAD `cb34ded57`. `[x]` — реализовано в коде; локальные тесты и их ограничения перечислены в README. Пункты `[ ]` описывают оставшуюся работу, а не разрешение включить все модели.

| Блок | Уже есть | Осталось |
| --- | --- | --- |
| B0 | Серверные config/readiness; read-only preflight, projection schema, price-source checks и искусственные fixtures/tests | Внешняя проверка выбранных ID, подтверждённые рабочие сведения о цене/группе и provider cost |
| B1 | HTTP transport, image serializers для Pro/GPT Image 2, Hailuo, legacy submit/poll, url string/array, классификация ошибок | Ограничение response body, точные retry-контракты; новые envelopes/serializers только по выбранной модели |
| B2 | Восстановление по сохранённому provider task и Artifact, sanitization durable result, идемпотентный capture | Intent/lease **до** submit, неопределённый исход после HTTP; durable синхронный текст при реализации T |
| B3 | Exact integer/rational math, versioned static/runtime catalogs, immutable pricing snapshot, reserve/capture | Сверка floors; metered quantities, text/audio keys и settlement при соответствующих модальностях |
| I | Registry/resolver/config, лимиты адаптера, text-to-image Web prepare/activate/history/result | Возможности новых ID и загрузка owner-scoped refs в Web, если выбрана такая операция |
| T/V/C/A | Общий Job/worker/Artifact и существующие отдельные text/video маршруты | Выбранные новые APIMart-контракты и недостающие публичные сценарии |

**Первая очередь:** B0 → B2 + необходимое из B1 → фиксированные image-тарифы B3 → одна новая image-модель. Полные text/audio/video расширения B3, синхронный результат T, Suno polling и versioned Grok response не являются предварительным условием для любого простого image-маршрута.

## Global Constraints

- Модели выбираются независимо. Подключение одной не разрешает включить весь каталог.
- Провайдер APIMart, база `https://api.apimart.ai/v1`. Ключ только в существующей серверной конфигурации; значение не писать в код, документы или fixtures.
- Все флаги новых моделей выключены до проверки API, цены, модерации и сохранения результата. Доступность сайта не доказывает доступность модели конкретному API-ключу.
- Существующие Pro/GPT Image 2 через APIMart и Nano Banana 2 через PoYo не подключать повторно. Проверка плана не меняет их текущие флаги или цены. Миграция Nano Banana 2 выполняется только при отдельном выборе этой миграции.
- Ни переключения на другую модель при ошибке, ни более дорогого official_fallback без отдельного явного выбора.
- Пользовательский интерфейс отправляет public ID, параметры и принадлежащие аккаунту artifact IDs. Provider/model code, цена, роли и URL для вызова формируются сервером.
- Нельзя считать провайдерский nsfw_check заменой существующей модерации: часть проверок у провайдера работает fail-open.
- Не делать commit/push/deploy и платные canary-вызовы в рамках подготовки этого плана.

## B0. Проверка доступности и источника тарифов

**Files (реализовано):** `scripts/providers/apimart-preflight.ps1`; `scripts/providers/preflight/main.go`, `main_test.go`; `scripts/providers/tests/apimart-preflight.Tests.ps1`; `internal/adapter/provider/apimart/testdata/contracts/preflight/` — только искусственные fixtures. **Read:** `internal/platform/config/config.go`. [Запуск и ограничения](../../../runbooks/APIMART_PREFLIGHT.md).

- [x] Создать read-only preflight с обязательным выбранным списком ID; ключ только из окружения. `GET /v1/models?expand=category`; вывод только выбранных ID/category и очищенных результатов проверок, без ключа/headers/баланса/полного ответа аккаунта.
- [x] Реализовать чтение image/video schema, включая отдельный query endpoint для ID с `/`. Сохранять безопасную projection schema_version/operation/endpoint/enums/idempotency/response contract и проверять привязку к exact ID. Отсутствующие сведения о контракте помечать для ручной сверки; chat/audio schema не придумывать.
- [x] Реализовать публичное чтение token pricing **без Authorization** и сравнение `data.pricing.effective_rates` с локальным reference-файлом без повторного применения скидки и float64. Missing/zero различаются; неизвестные units и multi-tier pricing отклоняются.
- [ ] Выполнить реальные запросы metadata/schema для выбранного пользователем списка и проверить документированные ограничения конкретных моделей. Пока проверены только искусственные fixtures; соответствие живым ответам не подтверждено. Chat/audio контракты и отдельно разрешённые canary остаются отдельной проверкой.
- [ ] Сверить каждую выбранную конфигурацию с группой рабочего ключа, единицами тарифа и итоговым provider cost. При расхождении не открывать новый маршрут/вариант; существующее включение не менять автоматически в рамках проверки плана.
- [x] Реализовать JSON-отчёт с очищенными числовыми ставками, ID, датой, публичным источником и единицами/группой. Fixed-price reference не выдаётся за live-тариф. `b0_complete=false` и `manual_checks` явно сохраняют непроверенные персональные условия.
- [ ] После внешней сверки сохранить подтверждённые рабочие сведения о цене/группе. Сейчас добавлены только искусственные reference fixtures; цены рабочего аккаунта не сохранялись.

**Локальная приёмка B0 — выполнена:** тесты покрывают отсутствующий/дублирующийся ID, недоступную/неверную/unbound schema, 401/403/429/5xx, JSON errors, несовпадение rates/unit/group, private metadata, credential echo, перенаправление, размер ответа, имена с `/`, missing/zero и multi-tier rates. PowerShell-тесты запускают CLI с искусственным ключом и проверяют stdout/stderr. Реальных генераций и изменений env/registry/цен нет.

**Проверки реализации:** `go test ./scripts/providers/preflight -count=1`, `go vet ./scripts/providers/preflight`, `pwsh -NoProfile -File scripts/providers/tests/apimart-preflight.Tests.ps1`. Успешный exit code означает только прохождение автоматических проверок; полный B0 пока не закрыт. Реальные API-контракты metadata/pricing сверены по официальной документации, без обращения к рабочему ключу.

Источники: [каталог/schema](https://docs.apimart.ai/ru/api-reference/texts/models/list), [контракт публичного прайса](https://docs.apimart.ai/ru/api-reference/texts/qwen3.8-max/pricing), [прайс](https://apimart.ai/ru/pricing).

## B1. Общий HTTP-клиент и асинхронные задачи

**Modify:** `internal/adapter/provider/apimart/apimart.go`, `apimart_test.go`.
**Create only when needed:** в том же пакете `task_decode.go`, `task_decode_test.go`, `text.go`, `music.go`, `midjourney.go`. `transport.go`, `images.go`, `video.go` — возможное выделение из существующего `apimart.go`, а не отсутствующие реализации. Не делать разделение файлов обязательным условием подключения модели.

- [x] HTTP transport и относительные image/video paths существуют в `apimart.go`; base URL по умолчанию содержит /v1. Существующие serializers проверяют известные model IDs до HTTP.
- [x] Legacy submit `data[0].task_id`, poll `/tasks/{id}`, terminal status и декодирование `url` как строки/массива реализованы. Основание: `TestSubmitGemini3ProImageSuccess`, `TestSubmitGPTImage2Success`, `TestPollCompletedImageReturnsProviderURLWithSanitizedRaw`.
- [ ] При расширении добавить строгую диспетчеризацию operation/modality/model: сейчас всё, кроме image, попадает в video validation. Не использовать Hailuo serializer для новых моделей. Добавить regression case с base URL `/v1`, чтобы исключить `/v1/v1`.
- [ ] Только для модели с versioned-контрактом добавить HTTP 202 `data.id` отдельной структурой. Для Grok 2 EXT посылать `X-APIMart-Response-Version: 2026-07-27`; не выставлять заголовок глобально.
- [ ] При Suno добавить `/music/tasks/{id}` и выбор polling по operation/model snapshot; существующий image/video polling сохранить.
- [ ] Добавить нормализованные usage/cost с явной единицей для settlement выбранной модели. Raw response не хранить; готовый декодер media URLs использовать повторно.
- [ ] HTTP 401/402/403 — управляемая недоступность без бесконечных повторов; 400/422 — ошибка запроса; 429 — Retry-After с пределом; 5xx/timeout после отправки — потенциально неопределённый исход, правила B2.
- [ ] Ограничить response body в `postJSON`/`getJSON` и тестом подтвердить отказ на превышении. Таймаут HTTP уже есть; сохранить его. Ошибки только нормализованные, без prompt, ссылок, response body или auth.

Контрактные fixtures для двух форм submit:

```json
{"code":200,"data":[{"status":"submitted","task_id":"task_fixture_1"}]}
```

```json
{"code":202,"request_id":"request_fixture","data":{"id":"task_fixture_2","object":"generation.task","type":"image","status":"pending","progress":0,"poll_url":"/v1/tasks/task_fixture_2"}}
```

**Проверки:** обе формы дают ровно один ExternalID; пустой/невалидный ID отклоняется; неизвестный внешний poll_url не становится произвольной целью HTTP-запроса. К существующим URL safety проверкам не добавлять обходов.

Источники: [задачи](https://docs.apimart.ai/ru/api-reference/tasks/status), [versioned Grok](https://docs.apimart.ai/ru/api-reference/images/grok-imagine-2.0-ext/generation).

## B2. Повторы и сохранение результата

**Modify:** `internal/worker/generation.go`, `poll.go`, `worker.go`, `internal/domain/provider.go`, `internal/adapter/storage/postgres/provider_task.go`, `internal/adapter/storage/memory/provider_task.go`.
**Create:** `internal/domain/provider_submission.go`, `internal/adapter/storage/postgres/provider_submission.go`, `internal/adapter/storage/memory/provider_submission.go`, `internal/worker/submission_recovery_test.go`; миграция с **следующим свободным номером**, суффикс `apimart_submission_intents`, additive up/down без удаления пользовательских данных.

- [x] `GenerationWorker.Process` возобновляет уже сохранённую provider task; worker восстанавливает сохранённые Artifact и повторяет доставку/capture по текущему контракту. Это покрыто `TestGenerationIdempotentRedelivery`, `TestPollWorkerResumesTerminalProviderTaskWithSavedArtifact`, `TestDeliveryExternalPushCaptureRetryDoesNotRepublish`.
- [x] Durable provider result уже исключает текст, raw и private URLs (`internal/domain/provider.go`). Не ослаблять эту политику ради нового адаптера.

**Незакрытое окно:** `provider.Submit` вызывается до `persistTask`; APIMart хранит submit cache в `map` процесса. Дедупликация Job и восстановление существующей task не закрывают crash после отправки до записи ExternalID. Вводить новую политику с отдельными APIMart-тестами, сохраняя существующие гарантии остальных провайдеров. Текущий `TestProviderSubmitTimeoutBecomesTerminalAndReleasesReservation` не доказывает отсутствие принятой upstream задачи; его контракт нужно сопоставить с новой политикой неопределённого исхода.

- [ ] До платного submit сохранять уникальный intent `(job_id, attempt)`, выбранный provider/model, idempotency key, HMAC отпечаток нормализованного запроса и состояние `prepared/sending/accepted/indeterminate`. Запись intent не содержит prompt, private URL или raw payload.
- [ ] Один retry/redelivery использует тот же intent и ключ. Новый attempt допустим только после подтверждённого terminal failure предыдущего.
- [ ] Для документированной идемпотентности Grok: повтор с тем же телом/ключом; in-progress 409 → ждать Retry-After; reused key с иным телом → локальная ошибка; result_indeterminate → сверка, без нового платного запроса.
- [ ] Для endpoint без доказанной гарантии идемпотентности timeout после отправки не повторять автоматически. Сохранять indeterminate, удерживать резерв по ограниченной процедуре сверки/эскалации; после срока сверки возможный убыток относится на провайдера/оператора, а не второе списание с пользователя. Не держать Job бесконечно и не освобождать резерв одновременно с продолжающимся submit.
- [ ] Добавить lease/блокировку intent: два worker не отправляют один Job параллельно. Проверить падения до HTTP, после ответа до записи task_id, после Artifact до capture.
- [ ] **При T, не до первого image-маршрута:** для синхронного текста добавить в `ProviderTask` поле `ImmediateResult *ProviderTaskResult` с `json:"-"`. APIMart Submit возвращает его только в памяти. Worker сохраняет нормализованный текст как закрытый Artifact до terminal checkpoint; повтор завершает обработку по artifact ID. Не копировать memory-only cache как единственную возможность recovery.
- [ ] Не менять политику `DurableProviderTaskResultJSON`: inline text, raw и private URLs по-прежнему исключены. В durable checkpoint допускаются только artifact IDs и безопасные usage/cost поля.
- [ ] Если процесс упал после фактической текстовой генерации до сохранения Artifact и endpoint не позволяет восстановить ответ, фиксировать неопределённый исход. Exactly-once на внешнем сервисе без его поддержки не обещать.
- [ ] Cancel, который сейчас ничего не отменяет у APIMart, не считать подтверждением отмены платной задачи. Продолжать сверку upstream и блокировать дубли.

**Тесты:** конкурентный redelivery создаёт один submit; crash-window без task_id не вызывает второй; восстановление готового текста не идёт в remote poll; повтор capture идемпотентен; moderation failure не доставляет артефакт. Проверить reservation expiry на фоне незавершённого provider intent.

## B3. Цена, резерв и итоговое списание

**Modify:** `internal/service/pricingcatalog/catalog.go`, `static_catalog.go`, `runtime.go`; `internal/adapter/storage/postgres/runtime_pricing.go`, memory-аналог; `internal/service/joborchestrator/orchestrator.go`; `internal/service/billingservice/service.go`.
**Create:** `internal/service/pricingcatalog/apimart_rates.go`, `apimart_rates_test.go`, `internal/worker/apimart_settlement_test.go`. При изменении DB-контракта — additive миграция с очередным свободным номером.

- [x] `ProductKey` для image/video, точные provider-specific единицы, `math/big` расчёт, static/runtime версии и immutable snapshot существуют. `imagegeneration.Resolver` требует валидную серверную цену; отсутствие цены блокирует модель. Сохранить `TestRuntimeCatalogSnapshotsRemainImmutableAcrossDBPriceChanges` и `TestResolver_UsesTrustedModelAndExactPricingSnapshot`.
- [ ] **Для первой image-модели:** использовать текущий `ImageModelID + Quality` и существующие fixed-price snapshots. Сверить floors с B0; при изменении выпускать новую static/runtime версию без переписывания старых Jobs. Не считать статический прайс фактической активной ценой DEV/PROD без проверки источника runtime catalog.
- [ ] **При text/audio, metered video или новых вариантах:** выполнить расширения ключей/usage/settlement ниже. Не создавать поля/таблицы для всех будущих модальностей в задаче простого изображения с фиксированной ценой.

- [ ] Расширить ProductKey: `TextModelID`, `AudioModelID`, `Variant` (std/pro, audio/silent, generation subtype). Для произвольных количеств хранить в server-owned quote отдельно `InputTokensCap`, `OutputTokensCap`, `InputDurationMillis`, `OutputDurationMillis`, `ReferenceImageCount`, `OutputCount`; не создавать строку SQL для каждого миллисекундного значения.
- [ ] Обновить Normalize/Valid/canonical key/DB uniqueness/копирование snapshots. Старые ключи с пустыми новыми полями сохраняют смысл; старые Jobs используют прежнюю версию.
- [ ] Для платных APIMart text/audio запретить legacy fallback text=0. Отсутствующий/невалидный price snapshot блокирует submit. Прежнюю политику существующего бесплатного text route не менять автоматически.
- [ ] Хранить мелкие ставки точно. Для опубликованных ставок с 7–8 знаками после запятой использовать USD nanos и rational denominator; float64 не применять. Текущий целочисленный AmountCredits в Provider.Estimate не использовать как источник пользовательской цены.

Предлагаемый внутренний тип и контрольные значения:

```go
type ExactRate struct {
    USDNanos int64
    Units    int64
}
type MeteredUsage struct {
    InputTokens       int64
    OutputTokens      int64
    InputDurationMillis int64
    OutputDurationMillis int64
    ReferenceImages   int64
    OutputCount       int64
}
// Пример: Qwen 3.7 Flash, USD за 1 000 000 токенов:
var qwen37Input = ExactRate{USDNanos: 22856800, Units: 1000000}
var qwen37Output = ExactRate{USDNanos: 91428800, Units: 1000000}
```

- [ ] Расчёт через math/big.Rat или checked integer rational: суммировать `rate × quantity / units`; округлять вверх один раз при переводе в внутренние кредиты. Разделять USD, APIMart credits и внутренние кредиты.
- [ ] Прайс APIMart выражает 1 credit как $0.10. Это не внутренний credit проекта. Текущая конверсия/наценка остаётся server-owned политикой; retail цены всех новых моделей сначала оформить как черновик версии.
- [ ] Для текста резерв по полной длине доверенного+пользовательского контекста и максимальному billed output, включая reasoning. Стартовые 1600/800 — наши лимиты; если нельзя доказать output cap, модель не включать. Внепиковую скидку и cache не предполагать до факта.
- [ ] Для видео заранее фиксировать режим, звук, разрешение, входную длину по проверенным media metadata. Для динамической длины Omni резерв на максимальный выход + подтверждённые входные доплаты. Для моделей с token billing посекундный прайс считать оценкой, проверить верхнюю границу.
- [ ] Итоговая себестоимость и пользовательский capture — разные величины. Capture только по принятому quote/snapshot и существующему контракту доставки; сумма не выше резерва. Разницу освобождать идемпотентно; без usage/cost применять определённую политику сверки, а не ноль.
- [ ] Не менять порядок delivery/capture без отдельной проверки текущих гарантий. Один Job с двумя треками или четырьмя плитками не порождает четыре независимых capture.

**Проверки расчёта:** GPT 6 Astra, 1600 input + 800 output = $0.0448; Seedance Mini 480p×5s = $0.0528; Kling v3 std×5s silent=$0.336, audio=$0.504; Suno music-v5.5 с двумя треками=$0.05/вызов. Qwen 3.7 Flash, 1000+1000 = $0.0001142856, без промежуточного округления.

## T. Текстовая основа

**Modify:** `internal/service/providermodels/registry.go`, `internal/service/modelcatalog/catalog.go`, `internal/service/productcatalog/catalog.go`, `internal/adapter/inbound/websession/handler.go`, `internal/worker/worker.go`, `cmd/worker/main.go`.
**Create:** `internal/service/textgeneration/resolver.go`, `resolver_test.go`; `internal/adapter/provider/apimart/text_test.go`.

- [ ] Добавить выбор text model_id в Web message DTO и resolver. Сейчас handler вызывает ResolvePublicModel с пустой строкой; miniAppTextModels сохраняет имя, но не provider/model code. Исправить обе точки, чтобы snapshot всегда указывал APIMart и точный ID.
- [ ] Сохранить прежний default chatgpt alias для уже созданных Jobs; новый ID не является переименованием старой модели.
- [ ] На /v1/chat/completions задавать stream=false явно. Отдельная nonstream-страница показывает /api/v1/chat/completions, поэтому не менять базу URL глобально; выбранный /v1/ маршрут подтверждён общей страницей и Qwen guide.
- [ ] Принимать OpenAI top-level ответ и документированную обёртку code/data; извлекать конечный content и usage. Reasoning_content не показывать как финальный текст.
- [ ] System prompt и TrustedFacts идут доверенными system-сообщениями, dialogcontext.Prepared.Prompt остаётся user. Не переносить пользовательскую историю в system.
- [ ] Для каждого ID отдельно проверить поддерживаемое поле max_tokens/max_completion_tokens и billed reasoning. Не добавлять temperature/tools/vision по предположению. ID в прайсе означает кандидат на вызов, а не подтверждение всех функций.
- [ ] В UI выбрать модель, показать лимит/цену, сохранить выбор на конкретном Job и отобразить результат в той же беседе. Проверить смену модели между сообщениями и запрет доступа к чужой conversation.
- [ ] Название беседы и summarization не должны неожиданно использовать дорогую выбранную модель; их существующий отдельный маршрут не менять.

**UI modify:** `web/platform/src/app/app/chat/[conversationId]/page.tsx`, `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.tsx`; тесты этих поверхностей. Любое расширение Mini App/VK требует сначала чтения их локальных AGENTS.md и самостоятельной проверки adapter DTO.

## I. Основа изображений

**Modify:** `internal/service/imagegeneration/resolver.go`, `internal/service/providermodels/registry.go`, `internal/service/modelcatalog/catalog.go`, `internal/service/productcatalog/catalog.go`; APIMart images/validation; `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx` и `ImageGenerationEditor/ImageGenerationEditor.tsx`.

- [x] Три public IDs, config gates, trusted image resolver и pricing keys уже существуют. Nano Banana 2 использует PoYo, Pro/GPT Image 2 — APIMart. Не создавать повторные public IDs/флаги.
- [x] APIMart serializers уже задают n=1, `resolution` вместо `quality`, lowercase для GPT Image 2 и model-specific лимиты 14/16 refs. `official_fallback` не включается. Сохранить эти тесты при добавлении новых ID.
- [x] Worker сохраняет output как Artifact, применяет moderation/scanning; Web выдаёт owner-checked результат через внутренний маршрут. Базовый pipeline не требуется писать заново.
- [x] Web editor получает модели, `quality_options` и `price_by_quality` с backend; есть prepare/activate/retry/history/result и тесты этих HTTP-путей.
- [ ] Для новых моделей расширять матрицу по ID: MP, 1K, quality и 1.5K — разные поля/единицы. Не переносить текущую нормализацию 1K/2K/4K на несовместимый API.
- [ ] **Если выбран Web image-to-image:** добавить загрузку и owner-scoped input artifact IDs в Web API/DTO/client/editor; сейчас Web prepare принимает только prompt/model_id/image_quality. Проверять refs/MIME/размер/площадь до резерва и повторно перед submit. Затронуть `internal/adapter/inbound/websession/handler.go`, `image_jobs_test.go`, `web/platform/src/lib/web-api/contracts.ts`, соответствующий API client и editor. Поддержка refs в адаптерах не закрывает этот пункт.
- [ ] Для нового маршрута проверить восстановление результата после истечения provider URL по сохранённым Artifact. Не ослаблять owner checks или moderation.
- [ ] Если отдельно выбрана миграция 41, сохранять provider/model/price snapshots всех уже подготовленных и незавершённых Jobs; менять маршрут только при подготовке новых Jobs.

## V. Основа видео и референсов

**Modify:** `internal/service/providermodels/registry.go`, `internal/service/videorouter/catalog.go`, `internal/service/productcatalog/catalog.go`, `internal/domain/video_route.go`, `internal/worker/worker.go`, `internal/worker/media_contracts.go`.
**Create:** `internal/service/videogeneration/resolver.go`, `resolver_test.go`; `web/platform/src/features/video-generation/VideoGenerationPanel.tsx` и тест, если соответствующая форма ещё отсутствует.

- [ ] Добавить отдельный route alias/feature gate для каждой выбранной модели. Старые route enums/Validate должны пропускать только зарегистрированные новые aliases.
- [ ] Типизированные роли входов: first_frame, last_frame, reference_image, reference_video, reference_audio. Ссылки разрешает worker из принадлежащих аккаунту артефактов; модель не получает клиентские произвольные URL.
- [ ] В resolved route snapshot сохранить mode/audio/resolution/duration/generation_type и проверенные входные длительности. Нынешний safeVideoProviderParams не должен отбрасывать эти поля.
- [ ] Реализовать общий UI видео: поля только из backend capabilities, загрузка разрешённых типов, quote, статус Job, результат/ошибка. В интерфейсе нельзя независимо выбрать несовместимые mode/audio/last frame.
- [ ] Native audio, reference video и динамическую длительность добавить в media contract. Проверка звука/частоты кадров/размера не должна отклонять документированный результат из-за старого Hailuo профиля.
- [ ] Первая стадия интеграции — выбранные T2V/I2V комбинации. Следующие режимы подключаются отдельно, с новой ценой, входной проверкой и тестом, без скрытого открытия всех полей APIMart.

## C. Motion Control

После B0–B3 и V: два отдельных public ID, общий provider model ID. Создать обязательные входы image+video, backend media probe для длины, orientation=image/video. Разрешение определяется std/pro; пользователь не задаёт независимую несовместимую resolution. Ни duration, ни цена из клиентского JSON не являются источником расчёта. Два маршрута отличаются mode и тарифом; оба должны иметь отдельный тест маршрутизации.

## A. Музыка

**Modify:** `internal/domain/job.go`, registry/catalog, joborchestrator, pricingcatalog, worker generation/poll/media validation, artifactservice, queue/config wiring.
**Create:** `internal/service/audiogeneration/resolver.go`, `resolver_test.go`, `internal/adapter/provider/apimart/music_test.go`, `web/platform/src/features/audio-generation/AudioGenerationPanel.tsx` и тест.

- [ ] Добавить `OperationAudioMusicGenerate = "audio_music_generate"` в enum/Valid, маршрутизацию, лимиты, очередь и pricing key. Обновить DB CHECK только если такой constraint присутствует; сначала найти его. Не использовать audio_tts.
- [ ] Зарегистрировать audio capability; включить audio в isAsyncMediaJob и соответствующий worker/queue. Все audio outputs сохранять в artifact storage и обычной account history.
- [ ] Suno submit /music/generations; poll /music/tasks/{id}. Прочитать оба документированных envelope при poll: корневой task_id/status с data.result.music и обёртка code/data.
- [ ] Для каждого music[] сохранить отдельный audio Artifact, title, duration и lyrics безопасными нормализованными метаданными; обложку — отдельным artifact при отображении. Не сохранять весь payload/частные ссылки.
- [ ] Обеспечить модерацию текста песни и поддерживаемую проверку аудиорезультата до доставки. Если аудиомодератор в данном контуре отсутствует, readiness=false, а не пропуск проверки.
- [ ] UI: описание/свой текст, instrumental, версия, genre/style только для custom=true, аудиоплеер всех полученных треков и единая цена вызова. Следующие операции Suno (extend/remaster/stems) — отдельные продукты, не открывать случайно.

Источники: [генерация](https://docs.apimart.ai/ru/api-reference/audios/suno/generation), [результат](https://docs.apimart.ai/ru/api-reference/audios/suno/overview).

## Проверки реализации и выпуск одной модели

- [ ] Contract fixtures: exact body/path/headers, positive и negative cases выбранного маршрута, 200/202 и соответствующий poll envelope.
- [ ] Unit/integration: route/price snapshot, ошибки провайдера, повтор Job/capture, crash recovery, чужие reference/conversation/task IDs, moderation rejection, сохранение результата после истечения provider URL.
- [ ] Go: `go test ./internal/adapter/provider/apimart ./internal/service/providermodels ./internal/service/modelcatalog ./internal/service/productcatalog ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/service/billingservice ./internal/worker`; добавить тесты выбранного resolver и storage при их изменении. `go vet` по тем же затронутым пакетам; gofmt только изменённым Go-файлам.
- [ ] Web, если менялся: `npm --prefix web/platform run typecheck`, `npm --prefix web/platform run lint`, `npm --prefix web/platform run test`.
- [ ] Обновить ARCHITECTURE лишь после фактического изменения billing/provider/storage contracts; RUNBOOK/docs/runbooks/DEV.md — при изменении флагов/запуска.
- [ ] Перед включением сделать ограниченный отдельно разрешённый canary с фиксированным бюджетом для выбранной модели; сравнить invoice/cost, quote и ledger. Флаг включить только для прошедших комбинаций. Ошибка выключает новые отправки этой модели, но polling уже принятых задач продолжается.
- [ ] Зафиксировать `git diff --check`, итоговые проверки и `git status --short`. Commit/push только по отдельному прямому запросу.
