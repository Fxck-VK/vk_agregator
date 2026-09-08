# 34. Deepseek R1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** подключить Deepseek R1 через выбранный маршрут APIMart.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [полный реестр](./README.md), [общая основа](./00-common-foundation.md). Статус: план, не реализовано в рамках этого задания.

## Global Constraints

Выполнять после B0–B3 и раздела T общей основы. Для этой модели отдельный флаг; разрешение на её реализацию не означает включение остальных. Provider key, prompt, raw payload и private URLs не записываются в отчёт/fixtures. Старые Jobs используют собственные snapshots. Платные проверки, commit/push/deploy не выполняются при подготовке плана.

## Документация и точное соответствие

- [Карточка Study24](https://study24.ai/chat/a24_deepseek_r1)
- [Документация general/chat-completions](https://docs.apimart.ai/ru/api-reference/texts/general/chat-completions)
- [Актуальный прайс APIMart](https://apimart.ai/ru/pricing), проверен 08.09.2026.
- [Доступность ID и schema для конкретного ключа](https://docs.apimart.ai/ru/api-reference/texts/models/list).

| Поле | Значение |
| --- | --- |
| Название Study24 | Deepseek R1 |
| Предлагаемый public ID | `deepseek_r1` |
| APIMart model ID | `deepseek-r1` |
| Метод | `POST https://api.apimart.ai/v1/chat/completions` |
| Флаг | `FEATURE_APIMART_DEEPSEEK_R1_ENABLED` |
| Соответствие | Условное: версия/режим уточнены в этом плане; тождество Study24 не доказано |

Study24 не раскрывает ревизию R1. В прайсе отдельно есть deepseek-r1-0528 ($0.408 / $1.23216); не подменять deepseek-r1 этой более дешёвой ревизией автоматически.

Перед включением проверить наличие exact ID в доступном ключу каталоге и цену выбранного варианта.

## Контракт первого выпуска

Первый выпуск: текстовый ввод, один ответ, stream=false, до 800 токенов вывода и консервативный бюджет входа 1600 токенов с учётом системного контекста. Это наши стартовые лимиты, а не заявленный максимум модели. temperature, tools, reasoning_effort, вложения и cache-control не отправлять до проверки контракта конкретного ID.

Пример тела запроса — основа fixture, не команда на выполнение. URL example.com в референсах обозначают тестовые входы; рабочие ссылки формирует worker из принадлежащих аккаунту артефактов.

```json
{
  "model": "deepseek-r1",
  "stream": false,
  "messages": [
    {
      "role": "user",
      "content": "Ответь одним коротким предложением."
    }
  ],
  "max_tokens": 800
}
```

Синхронный ответ Chat Completions: конечный content + числовой usage. Worker получает transient ImmediateResult и сохраняет закрытый текстовый Artifact до фиксации завершения. Remote /tasks для текста не использовать.

## Цена

$0.448 / $1.792 за 1 млн входных / выходных токенов (обычный контекст, без инструментов и скидки кэша).

Это себестоимость APIMart на дату проверки, не утверждённая пользовательская цена. Перед включением зафиксировать точный тариф/единицу/группу в версии price catalog. Quote и резерв определяет backend; итог не превышает принятый резерв. Кэш, внепиковую скидку и инструменты не предполагать; учитывать billable reasoning и ограничения контекста.

## Файлы реализации

Пути от корня репозитория; новые общие файлы создаются по плану основы, существующие изменяются.

- `internal/adapter/provider/apimart/text.go`
- `internal/adapter/provider/apimart/text_test.go`
- `internal/service/providermodels/registry.go`
- `internal/service/textgeneration/resolver.go`
- `internal/service/modelcatalog/catalog.go`
- `internal/service/pricingcatalog/apimart_rates.go`
- `internal/adapter/inbound/websession/handler.go`
- `web/platform/src/app/app/chat/[conversationId]/page.tsx`
- `internal/platform/config/config.go`
- Создать fixture `internal/adapter/provider/apimart/testdata/contracts/deepseek_r1.request.json`.
- Создать `internal/adapter/provider/apimart/model_deepseek_r1_test.go` с именем теста `TestModel_deepseek_r1`.

## Порядок реализации

- [ ] **1. Контракт.** В B0 проверить `deepseek-r1`, записать разрешённые поля/операции и очищенный fixture указанного запроса. Для спорной версии сохранить предлагаемое полное название в UI и закрыть rollout до подтверждения выбора. Не выводить статус «подключено» по одному наличию в прайсе.
- [ ] **2. Адаптер.** Реализовать serializer и capability для `deepseek-r1` по указанному JSON; локально проверять лимиты. Разобрать описанную форму результата. В тестовом HTTP server проверить exact endpoint/body; неподдерживаемое значение не должно вызывать HTTP.
- [ ] **3. Каталог и стоимость.** Добавить/обновить public ID `deepseek_r1`, backend readiness и `FEATURE_APIMART_DEEPSEEK_R1_ENABLED` в config wiring. Создать отдельные ключи всех открываемых вариантов; определить exact rates и reserve/capture fixture. Сохранить model ID и тариф в Job snapshot.
- [ ] **4. Пользовательский поток.** Backend resolver разрешает только зарегистрированный public ID; UI показывает допустимые варианты и серверный quote. Job создаётся под владельцем с idempotency key, результат появляется в его истории. Ошибка модели не переключает на иной ID.
- [ ] **5. Проверки этой модели.** Реализовать перечисленные ниже positive/negative cases; общий lifecycle B2/B3 применяется к этому exact ID. Сначала получить ожидаемое падение нового contract test, затем реализовать маршрут и добиться PASS.
- [ ] **6. Выпуск.** Пройти preflight, тесты и ограниченный отдельно разрешённый canary именно этой конфигурации; сверить provider cost и ledger. До этого `FEATURE_APIMART_DEEPSEEK_R1_ENABLED` выключен. Отключение запрещает новые submit, но не мешает завершить старые Jobs.

## Приёмочные сценарии

- [ ] Запрос с public_id=deepseek_r1 резолвится строго в APIMart/deepseek-r1.
- [ ] При входе 1000 и выходе 1000 токенов себестоимость равна $0.00224; применение скидки второй раз запрещено.
- [ ] Ответ без choices[0].message.content или с некорректным usage не должен пройти как успешная бесплатная генерация.
- [ ] `FEATURE_APIMART_DEEPSEEK_R1_ENABLED=false` → модель недоступна для нового Job; ключ/остаток провайдера не раскрываются клиенту.
- [ ] Повторный Job с тем же idempotency key не создаёт второй платный запрос; result повторно не списывается.
- [ ] Невалидный ответ, provider failure и блокировка модерацией дают контролируемый статус без выдачи непроверенного результата.

## Команды проверки при реализации

```powershell
go test ./internal/adapter/provider/apimart -run '^TestModel_deepseek_r1$' -count=1
go test ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/worker
```

Для изменённого resolver добавить его package test; при изменении UI выполнить typecheck/lint/test из [общей основы](./00-common-foundation.md). Эти команды предназначены для будущей реализации и не являются заявлением о пройденных сейчас тестах.

**Готовность:** После общей текстовой основы и контрактной проверки конкретного ID. При незакрытом условии соответствия/стоимости план подготовлен, но модель не готова к включению.
