# 45. Qwen Image 3.0 — реализация

**Статус 08.09.2026: реализовано и выложено на DEV; пользователь подтвердил успешную генерацию. Фактическое списание APIMart отдельно не проверялось; PROD rollout не выполнялся.**

По выбору пользователя модель подключена напрямую к существующему image pipeline. B0–B3 не являются отдельными предварительными этапами этой задачи; общая переработка транспорта, durable submit intents и metered billing сюда не входит.

## Контракт

- Public ID: `qwen_image_3`, название: `Qwen Image 3.0`.
- Provider: APIMart, точный ID: `qwen-image-3.0`.
- Флаг: `FEATURE_APIMART_QWEN_IMAGE_3_ENABLED`, по умолчанию `false`.
- `POST /v1/images/generations`, ответ `data[0].task_id`, затем `GET /v1/tasks/{task_id}`.
- Первый выпуск: одно изображение, `1K`/`2K`, без переписывания промпта (`prompt_extend=false`). `-pro`, 4K, batch и режимы prompt rewriting недоступны.
- Web использует существующую форму и квадратный размер `1:1`. Новые поля UI не добавлялись. Загрузка референсов в Web не входит в эту задачу.
- Адаптер поддерживает семь документированных соотношений, запись `16x9` и размеры сторон 512–2048. При пикселях без resolution площадь >2 250 000 явно выбирает 2K.
- До трёх референсов; каждый data URL не более 10 MiB. Worker проверяет владельца артефакта и очищает изображение перед отправкой. Негативный промпт передаётся адаптером, если присутствует в доверенном запросе.
- Все URL результата обрабатываются общим сохранением Artifact и модерацией перед выдачей.

[Официальный контракт](https://docs.apimart.ai/ru/api-reference/images/qwen-image-3.0/generation) и [прайс](https://apimart.ai/ru/pricing) прочитаны 08.09.2026. Context7 пока возвращал материал Qwen 2.0; для версии 3.0 использована указанная пользователем официальная страница.

## Цена

Базовая версия: **0.205712 APIMart credits = $0.0205712** за изображение, одинаково 1K/2K; референсы бесплатны. Цена Pro не применяется.

Статический каталог версии 5 хранит точный floor `205712 apimart_credit_micros`. Для нового маршрута используется розничная цена **15 внутренних кредитов**: floor ×3, округлённый вверх до существующего шага 5 кредитов. В snapshot сохраняется точный розничный множитель и cap=15. Действующие тарифы других моделей по сумме не изменены.

Это локальная конфигурация на основании публичного прайса; тариф конкретной группы ключа и фактическое списание не проверены. Активная БД тарифов не изменялась. При `runtime_db` для каждого открываемого качества нужна отдельная включённая запись Qwen. Непроценённые варианты скрыты; если нет ни одного тарифа, скрыта вся модель.

## Выполнено

- [x] Сверен текущий код; использованы существующие submit/poll, resolver, Job snapshots, Artifact, moderation, history и ledger.
- [x] Serializer/capability и очищенный fixture `internal/adapter/provider/apimart/testdata/contracts/qwen_image_3.request.json`.
- [x] Лимиты: 4-й референс, >10 MiB data URL, 4K, неподдерживаемые размеры, Pro, batch и agent+референс отклоняются до HTTP.
- [x] Public ID, registry, отдельный config flag/readiness, варианты качества и статические тарифы.
- [x] Web prepare сохраняет точный provider/model/quality/price только в приватном Job snapshot; публичный ответ не содержит маршрутизацию, ключи и floors.
- [x] Флаг по умолчанию выключен; отсутствие provider config или тарифа скрывает модель.
- [x] Локальный HTTP contract test сначала упал на отсутствии модели, затем прошёл.
- [x] Асинхронный lifecycle с реальным адаптером на `httptest`: сохранение результата, модерация, проверка владельца, capture 15 и повторы generation/poll/delivery без второго submit/capture.
- [x] Блокировка модерацией не публикует результат, не списывает баланс и освобождает резерв. Невалидный ответ/ошибка провайдера нормализуются существующим адаптером.

## Проверки и границы

08.09.2026 пользователь разрешил push и DEV deploy. Перед публикацией изменения
совмещены с `origin/dev-deploy` (`31129c30e`); Qwen явно ограничена `MaxOutputCount=1`
с учётом появившегося в DEV общего batch-параметра. DEV env renderer включает модель
при настроенном APIMart, сохраняет явное выключение и не включает её без ключа.
Полный `go test ./...`, `go vet`, CLI preflight tests и `test-dev-env.sh` пройдены.
Статус внешней выкладки подтверждается результатом GitHub Actions, а не этой записью.

Основные тесты: `TestModel_qwen_image_3`, `TestQwenImageCatalogAndTrustedQuote`, `TestQwenImageReadinessFailsClosed`, `TestQwenImageFlagAndRequiredConfig`, `TestWebQwenImagePrepareUsesPrivateRouteAndServerPrice`, `TestQwenImageAsyncLifecycle`.

```powershell
go test ./internal/adapter/provider/apimart ./internal/service/providermodels ./internal/service/modelcatalog ./internal/service/productcatalog ./internal/service/pricingcatalog ./internal/service/imagegeneration ./internal/platform/config ./internal/adapter/inbound/websession ./internal/service/joborchestrator ./internal/worker ./internal/app/miniapp ./cmd/worker ./cmd/api -count=1
go vet ./internal/adapter/provider/apimart ./internal/service/providermodels ./internal/service/modelcatalog ./internal/service/productcatalog ./internal/service/pricingcatalog ./internal/service/imagegeneration ./internal/platform/config ./internal/adapter/inbound/websession ./internal/service/joborchestrator ./internal/worker ./internal/app/miniapp ./cmd/worker ./cmd/api
```

Проверки повторной отправки покрывают сохранённый provider task и последовательный replay. Окно сбоя процесса между HTTP submit и сохранением task_id остаётся ограничением существующей интеграции; B2 не объявляется выполненным.

- [ ] Доступ рабочего ключа к exact model ID, персональная группа/стоимость.
- [x] Успешная генерация на DEV — подтверждение пользователя после выкладки.
- [x] Включение на DEV: deploy commit `cdc3b17`, [успешный workflow](https://github.com/Fxck-VK/vk_agregator/actions/runs/34233775257).
- [ ] Отдельный аудит фактического списания провайдера.

Результат: оба приведённых набора (`go test` по 13 пакетам и `go vet`) — PASS; `gofmt` выполнен. `golangci-lint` отсутствует в PATH и не запускался. Первый общий запуск содержал ошибочный путь `internal/app/web`; после удаления несуществующего пакета полный набор прошёл.

После локальной реализации пользователь разрешил commit/push и DEV deploy; выкладка выполнена, работа модели подтверждена пользователем. PROD и активные тарифы БД в этой работе не менялись. Frontend-код Qwen отдельно не менялся.

[Общий реестр](README.md) · [Общая основа и отложенные улучшения](00-common-foundation.md)
