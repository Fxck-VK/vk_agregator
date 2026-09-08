# 40. Midjourney — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** подключить Midjourney через выбранный маршрут APIMart.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [полный реестр](./README.md), [общая основа](./00-common-foundation.md). Статус: план, не реализовано в рамках этого задания.

## Global Constraints

Выполнять после B0–B3 и раздела I общей основы. Для этой модели отдельный флаг; разрешение на её реализацию не означает включение остальных. Provider key, prompt, raw payload и private URLs не записываются в отчёт/fixtures. Старые Jobs используют собственные snapshots. Платные проверки, commit/push/deploy не выполняются при подготовке плана.

## Документация и точное соответствие

- [Карточка Study24](https://study24.ai/chat/midjourney_toe_bot)
- [Документация midjourney/imagine](https://docs.apimart.ai/ru/api-reference/images/midjourney/imagine)
- [Документация midjourney/query](https://docs.apimart.ai/ru/api-reference/images/midjourney/query)
- [Актуальный прайс APIMart](https://apimart.ai/ru/pricing), проверен 08.09.2026.
- [Доступность ID и schema для конкретного ключа](https://docs.apimart.ai/ru/api-reference/texts/models/list).

| Поле | Значение |
| --- | --- |
| Название Study24 | Midjourney |
| Предлагаемый public ID | `midjourney_v7` |
| APIMart model ID | `midjourney` |
| Метод | `POST https://api.apimart.ai/v1/midjourney/generations` |
| Флаг | `FEATURE_APIMART_MIDJOURNEY_V7_ENABLED` |
| Соответствие | Условное: версия/режим уточнены в этом плане; тождество Study24 не доказано |

Study24 не раскрывает версию Midjourney. V7 — предложение; не выдавать за доказанное совпадение. Upscale/variation/reroll — последующие отдельные Jobs с тарифом, не бесплатные кнопки.

Перед включением проверить наличие exact ID в доступном ключу каталоге и цену выбранного варианта.

## Контракт первого выпуска

Отдельный POST /v1/midjourney/generations; version=7 — предлагаемый явный старт. Старт Imagine, без repeat и без сырых --параметров. Сохранять полученную сетку/отдельные изображения без повторной оплаты каждого файла.

Пример тела запроса — основа fixture, не команда на выполнение. URL example.com в референсах обозначают тестовые входы; рабочие ссылки формирует worker из принадлежащих аккаунту артефактов.

```json
{
  "prompt": "A calm mountain lake",
  "version": "7",
  "speed": "relax"
}
```

Асинхронный submit HTTP 200 → data[0].task_id; GET /v1/tasks/{task_id}. Сохранить все result.images[].url в собственное хранилище и провести модерацию до доставки.

## Цена

Imagine relax $0.04504, fast $0.05504, turbo $0.10 за вызов. Версионную комбинацию v7×speed подтвердить.

Это себестоимость APIMart на дату проверки, не утверждённая пользовательская цена. Перед включением зафиксировать точный тариф/единицу/группу в версии price catalog. Quote и резерв определяет backend; итог не превышает принятый резерв. Не полагаться на значения по умолчанию провайдера, если они меняют цену.

## Файлы реализации

Пути от корня репозитория; новые общие файлы создаются по плану основы, существующие изменяются.

- `internal/adapter/provider/apimart/midjourney.go`
- `internal/adapter/provider/apimart/apimart_test.go`
- `internal/service/providermodels/registry.go`
- `internal/service/imagegeneration/resolver.go`
- `internal/service/productcatalog/catalog.go`
- `internal/service/pricingcatalog/static_catalog.go`
- `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx`
- `internal/platform/config/config.go`
- `internal/adapter/provider/apimart/midjourney_test.go`
- Создать fixture `internal/adapter/provider/apimart/testdata/contracts/midjourney_v7.request.json`.
- Создать `internal/adapter/provider/apimart/model_midjourney_v7_test.go` с именем теста `TestModel_midjourney_v7`.

## Порядок реализации

- [ ] **1. Контракт.** В B0 проверить `midjourney`, записать разрешённые поля/операции и очищенный fixture указанного запроса. Для спорной версии сохранить предлагаемое полное название в UI и закрыть rollout до подтверждения выбора. Не выводить статус «подключено» по одному наличию в прайсе.
- [ ] **2. Адаптер.** Реализовать serializer и capability для `midjourney` по указанному JSON; локально проверять лимиты. Разобрать описанную форму результата. В тестовом HTTP server проверить exact endpoint/body; неподдерживаемое значение не должно вызывать HTTP.
- [ ] **3. Каталог и стоимость.** Добавить/обновить public ID `midjourney_v7`, backend readiness и `FEATURE_APIMART_MIDJOURNEY_V7_ENABLED` в config wiring. Создать отдельные ключи всех открываемых вариантов; определить exact rates и reserve/capture fixture. Сохранить model ID и тариф в Job snapshot.
- [ ] **4. Пользовательский поток.** Backend resolver разрешает только зарегистрированный public ID; UI показывает допустимые варианты и серверный quote. Job создаётся под владельцем с idempotency key, результат появляется в его истории. Ошибка модели не переключает на иной ID.
- [ ] **5. Проверки этой модели.** Реализовать перечисленные ниже positive/negative cases; общий lifecycle B2/B3 применяется к этому exact ID. Сначала получить ожидаемое падение нового contract test, затем реализовать маршрут и добиться PASS.
- [ ] **6. Выпуск.** Пройти preflight, тесты и ограниченный отдельно разрешённый canary именно этой конфигурации; сверить provider cost и ledger. До этого `FEATURE_APIMART_MIDJOURNEY_V7_ENABLED` выключен. Отключение запрещает новые submit, но не мешает завершить старые Jobs.

## Приёмочные сценарии

- [ ] speed=relax: один вызов, один резерв, даже если вернулось 4 плитки.
- [ ] repeat=2 и prompt с переопределением --version/--repeat отклоняются.
- [ ] Poll штатно через /v1/tasks/{id}; кнопки действий получать через /v1/midjourney/{id}.
- [ ] `FEATURE_APIMART_MIDJOURNEY_V7_ENABLED=false` → модель недоступна для нового Job; ключ/остаток провайдера не раскрываются клиенту.
- [ ] Повторный Job с тем же idempotency key не создаёт второй платный запрос; result повторно не списывается.
- [ ] Невалидный ответ, provider failure и блокировка модерацией дают контролируемый статус без выдачи непроверенного результата.

## Следующая стадия этой модели

- [ ] Стадия 2: реализовать отдельные Jobs Upscale/Variation/Reroll с собственными тарифами и ссылкой на принадлежащий аккаунту parent task. Получать buttons[].customId через MJ-style query; никогда не принимать произвольный внешний customId от клиента без проверки родителя.

## Команды проверки при реализации

```powershell
go test ./internal/adapter/provider/apimart -run '^TestModel_midjourney_v7$' -count=1
go test ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/worker
```

Для изменённого resolver добавить его package test; при изменении UI выполнить typecheck/lint/test из [общей основы](./00-common-foundation.md). Эти команды предназначены для будущей реализации и не являются заявлением о пройденных сейчас тестах.

**Готовность:** После общей основы изображений; только проверенные ценовые варианты. При незакрытом условии соответствия/стоимости план подготовлен, но модель не готова к включению.
