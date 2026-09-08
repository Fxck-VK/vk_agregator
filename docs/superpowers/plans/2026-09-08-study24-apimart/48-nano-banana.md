# 48. Nano Banana — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** подключить Nano Banana через выбранный маршрут APIMart.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [полный реестр](./README.md), [общая основа](./00-common-foundation.md). Статус: план, не реализовано в рамках этого задания.

## Global Constraints

Выполнять после B0–B3 и раздела I общей основы. Для этой модели отдельный флаг; разрешение на её реализацию не означает включение остальных. Provider key, prompt, raw payload и private URLs не записываются в отчёт/fixtures. Старые Jobs используют собственные snapshots. Платные проверки, commit/push/deploy не выполняются при подготовке плана.

## Документация и точное соответствие

- [Карточка Study24](https://study24.ai/chat/google_image)
- [Документация gemini-2.5-flash/generation](https://docs.apimart.ai/ru/api-reference/images/gemini-2.5-flash/generation)
- [Актуальный прайс APIMart](https://apimart.ai/ru/pricing), проверен 08.09.2026.
- [Доступность ID и schema для конкретного ключа](https://docs.apimart.ai/ru/api-reference/texts/models/list).

| Поле | Значение |
| --- | --- |
| Название Study24 | Nano Banana |
| Предлагаемый public ID | `nano_banana` |
| APIMart model ID | `gemini-2.5-flash-image-preview` |
| Метод | `POST https://api.apimart.ai/v1/images/generations` |
| Флаг | `FEATURE_APIMART_NANO_BANANA_ENABLED` |
| Соответствие | Название модели сопоставлено с каталогом/контрактом APIMart; доступ рабочего ключа ещё не проверен |

Документированный alias nano-banana-ext. Не подменять этой моделью Nano Banana 2.

Перед включением проверить наличие exact ID в доступном ключу каталоге и цену выбранного варианта.

## Контракт первого выпуска

Только 1K, n=1; до 14 референсов; prompt до 1000 символов. official_fallback=false.

Пример тела запроса — основа fixture, не команда на выполнение. URL example.com в референсах обозначают тестовые входы; рабочие ссылки формирует worker из принадлежащих аккаунту артефактов.

```json
{
  "model": "gemini-2.5-flash-image-preview",
  "prompt": "Спокойное горное озеро.",
  "resolution": "1K",
  "size": "1:1",
  "n": 1
}
```

Асинхронный submit HTTP 200 → data[0].task_id; GET /v1/tasks/{task_id}. Сохранить все result.images[].url в собственное хранилище и провести модерацию до доставки.

## Цена

$0.0125 за изображение 1K.

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
- Создать fixture `internal/adapter/provider/apimart/testdata/contracts/nano_banana.request.json`.
- Создать `internal/adapter/provider/apimart/model_nano_banana_test.go` с именем теста `TestModel_nano_banana`.

## Порядок реализации

- [ ] **1. Контракт.** В B0 проверить `gemini-2.5-flash-image-preview`, записать разрешённые поля/операции и очищенный fixture указанного запроса. Для спорной версии сохранить предлагаемое полное название в UI и закрыть rollout до подтверждения выбора. Не выводить статус «подключено» по одному наличию в прайсе.
- [ ] **2. Адаптер.** Реализовать serializer и capability для `gemini-2.5-flash-image-preview` по указанному JSON; локально проверять лимиты. Разобрать описанную форму результата. В тестовом HTTP server проверить exact endpoint/body; неподдерживаемое значение не должно вызывать HTTP.
- [ ] **3. Каталог и стоимость.** Добавить/обновить public ID `nano_banana`, backend readiness и `FEATURE_APIMART_NANO_BANANA_ENABLED` в config wiring. Создать отдельные ключи всех открываемых вариантов; определить exact rates и reserve/capture fixture. Сохранить model ID и тариф в Job snapshot.
- [ ] **4. Пользовательский поток.** Backend resolver разрешает только зарегистрированный public ID; UI показывает допустимые варианты и серверный quote. Job создаётся под владельцем с idempotency key, результат появляется в его истории. Ошибка модели не переключает на иной ID.
- [ ] **5. Проверки этой модели.** Реализовать перечисленные ниже positive/negative cases; общий lifecycle B2/B3 применяется к этому exact ID. Сначала получить ожидаемое падение нового contract test, затем реализовать маршрут и добиться PASS.
- [ ] **6. Выпуск.** Пройти preflight, тесты и ограниченный отдельно разрешённый canary именно этой конфигурации; сверить provider cost и ledger. До этого `FEATURE_APIMART_NANO_BANANA_ENABLED` выключен. Отключение запрещает новые submit, но не мешает завершить старые Jobs.

## Приёмочные сценарии

- [ ] 1001 символ отклоняется до submit.
- [ ] 2K/4K не принимаются; 14 референсов допустимы.
- [ ] `FEATURE_APIMART_NANO_BANANA_ENABLED=false` → модель недоступна для нового Job; ключ/остаток провайдера не раскрываются клиенту.
- [ ] Повторный Job с тем же idempotency key не создаёт второй платный запрос; result повторно не списывается.
- [ ] Невалидный ответ, provider failure и блокировка модерацией дают контролируемый статус без выдачи непроверенного результата.

## Команды проверки при реализации

```powershell
go test ./internal/adapter/provider/apimart -run '^TestModel_nano_banana$' -count=1
go test ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/worker
```

Для изменённого resolver добавить его package test; при изменении UI выполнить typecheck/lint/test из [общей основы](./00-common-foundation.md). Эти команды предназначены для будущей реализации и не являются заявлением о пройденных сейчас тестах.

**Готовность:** После общей основы изображений; только проверенные ценовые варианты. При незакрытом условии соответствия/стоимости план подготовлен, но модель не готова к включению.
