# 44. Grok Image 2.0 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** подключить Grok Image 2.0 через выбранный маршрут APIMart.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [полный реестр](./README.md), [общая основа](./00-common-foundation.md). Статус: план, не реализовано в рамках этого задания.

## Global Constraints

Выполнять после B0–B3 и раздела I общей основы. Для этой модели отдельный флаг; разрешение на её реализацию не означает включение остальных. Provider key, prompt, raw payload и private URLs не записываются в отчёт/fixtures. Старые Jobs используют собственные snapshots. Платные проверки, commit/push/deploy не выполняются при подготовке плана.

## Документация и точное соответствие

- [Карточка Study24](https://study24.ai/chat/grok_image2)
- [Документация grok-imagine-2.0-ext/generation](https://docs.apimart.ai/ru/api-reference/images/grok-imagine-2.0-ext/generation)
- [Актуальный прайс APIMart](https://apimart.ai/ru/pricing), проверен 08.09.2026.
- [Доступность ID и schema для конкретного ключа](https://docs.apimart.ai/ru/api-reference/texts/models/list).

| Поле | Значение |
| --- | --- |
| Название Study24 | Grok Image 2.0 |
| Предлагаемый public ID | `grok_image_2_0` |
| APIMart model ID | `grok-imagine-2.0-ext` |
| Метод | `POST https://api.apimart.ai/v1/images/generations` |
| Флаг | `FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED` |
| Соответствие | Название модели сопоставлено с каталогом/контрактом APIMart; доступ рабочего ключа ещё не проверен |

Дополнительные layer/region-edit имеют отдельную документацию и не входят в базовую генерацию. Эту модель включать после решения ценового конфликта.

**Блокировка включения:** противоречие цен должно быть разрешено до продаж.

## Контракт первого выпуска

Только text-to-image; resolution=quality, n=1 на старте. Заголовок X-APIMart-Response-Version: 2026-07-27. Ответ HTTP 202 data.id; запрещены image_urls, quality, stream и response_format≠url.

Пример тела запроса — основа fixture, не команда на выполнение. URL example.com в референсах обозначают тестовые входы; рабочие ссылки формирует worker из принадлежащих аккаунту артефактов.

```json
{
  "model": "grok-imagine-2.0-ext",
  "prompt": "Спокойное горное озеро.",
  "resolution": "quality",
  "size": "1:1",
  "n": 1,
  "response_format": "url"
}
```

Дополнительные заголовки: `X-APIMart-Response-Version: 2026-07-27` и стабильный `Idempotency-Key` для одного intent.

Асинхронный submit HTTP 202 → data.id; GET /v1/tasks/{task_id}. Сохранить все result.images[].url в собственное хранилище и провести модерацию до доставки.

## Цена

Конфликт: прайс $0.015/изображение, документация $0.08/изображение. Рабочий тариф не назначать до сверки группы/модели.

Это себестоимость APIMart на дату проверки, не утверждённая пользовательская цена. Перед включением зафиксировать точный тариф/единицу/группу в версии price catalog. Quote и резерв определяет backend; итог не превышает принятый резерв. Не полагаться на значения по умолчанию провайдера, если они меняют цену.

## Файлы реализации

Пути от корня репозитория; новые общие файлы создаются по плану основы, существующие изменяются.

- `internal/adapter/provider/apimart/images.go`
- `internal/adapter/provider/apimart/apimart_test.go`
- `internal/service/providermodels/registry.go`
- `internal/service/imagegeneration/resolver.go`
- `internal/service/productcatalog/catalog.go`
- `internal/service/pricingcatalog/static_catalog.go`
- `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx`
- `internal/platform/config/config.go`
- Создать fixture `internal/adapter/provider/apimart/testdata/contracts/grok_image_2_0.request.json`.
- Создать `internal/adapter/provider/apimart/model_grok_image_2_0_test.go` с именем теста `TestModel_grok_image_2_0`.

## Порядок реализации

- [ ] **1. Контракт.** В B0 проверить `grok-imagine-2.0-ext`, записать разрешённые поля/операции и очищенный fixture указанного запроса. Для спорной версии сохранить предлагаемое полное название в UI и закрыть rollout до подтверждения выбора. Не выводить статус «подключено» по одному наличию в прайсе.
- [ ] **2. Адаптер.** Реализовать serializer и capability для `grok-imagine-2.0-ext` по указанному JSON; локально проверять лимиты. Разобрать описанную форму результата. В тестовом HTTP server проверить exact endpoint/body; неподдерживаемое значение не должно вызывать HTTP.
- [ ] **3. Каталог и стоимость.** Добавить/обновить public ID `grok_image_2_0`, backend readiness и `FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED` в config wiring. Создать отдельные ключи всех открываемых вариантов; определить exact rates и reserve/capture fixture. Сохранить model ID и тариф в Job snapshot.
- [ ] **4. Пользовательский поток.** Backend resolver разрешает только зарегистрированный public ID; UI показывает допустимые варианты и серверный quote. Job создаётся под владельцем с idempotency key, результат появляется в его истории. Ошибка модели не переключает на иной ID.
- [ ] **5. Проверки этой модели.** Реализовать перечисленные ниже positive/negative cases; общий lifecycle B2/B3 применяется к этому exact ID. Сначала получить ожидаемое падение нового contract test, затем реализовать маршрут и добиться PASS.
- [ ] **6. Выпуск.** Пройти preflight, тесты и ограниченный отдельно разрешённый canary именно этой конфигурации; сверить provider cost и ledger. До этого `FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED` выключен. Отключение запрещает новые submit, но не мешает завершить старые Jobs.

## Приёмочные сценарии

- [ ] HTTP 202 с data.id распознаётся как созданная задача.
- [ ] 409 in-progress повторяется только тем же ключом после Retry-After; indeterminate не вызывает новый платный submit.
- [ ] Цена из конфликта блокирует продажу; референсы отвергаются.
- [ ] `FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED=false` → модель недоступна для нового Job; ключ/остаток провайдера не раскрываются клиенту.
- [ ] Повторный Job с тем же idempotency key не создаёт второй платный запрос; result повторно не списывается.
- [ ] Невалидный ответ, provider failure и блокировка модерацией дают контролируемый статус без выдачи непроверенного результата.

## Следующая стадия этой модели

- [ ] Стадия 2: layer/region-edit только по отдельной документации и price key. Сначала снять блокировку базовой цены $0.015 против $0.08; не включать автопереход на официальный grok-imagine-image-2.0.

## Команды проверки при реализации

```powershell
go test ./internal/adapter/provider/apimart -run '^TestModel_grok_image_2_0$' -count=1
go test ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/worker
```

Для изменённого resolver добавить его package test; при изменении UI выполнить typecheck/lint/test из [общей основы](./00-common-foundation.md). Эти команды предназначены для будущей реализации и не являются заявлением о пройденных сейчас тестах.

**Готовность:** После общей основы изображений; только проверенные ценовые варианты. При незакрытом условии соответствия/стоимости план подготовлен, но модель не готова к включению.
