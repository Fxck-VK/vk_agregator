# 41. Nano Banana 2 — отдельный план миграции PoYo → APIMart

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** при отдельном выборе миграции перевести новые Jobs существующей Nano Banana 2 на подтверждённый маршрут APIMart.
**Architecture:** public ID сохраняется; provider/model/price фиксируются в доверенном Job snapshot; worker завершает каждый Job через его исходного провайдера.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [реестр и актуальная очередь](./README.md), [общая основа](./00-common-foundation.md).
**Статус:** модель реализована через PoYo. Миграция APIMart не реализована и исключена из обязательной первой очереди. Сверка 08.09.2026, HEAD `cb34ded57`.

## Global Constraints

Наличие модели не требует смены провайдера. Выполнять оставшиеся пункты только при выборе миграции 41, после необходимых B0/B1/B2/B3/I. Не создавать новую карточку/общий feature flag и не менять текущий PoYo route в рамках подготовки плана. Сохранить уже подготовленные, ожидающие активации и незавершённые Jobs, включая их тарифы; PoYo остаётся доступен worker для их завершения. Новые APIMart Jobs не должны скрыто откатываться на PoYo или official_fallback. Ключи, prompts, raw payload и private URLs не писать в fixtures/docs. Commit/push/deploy и платные canary — вне актуализации.

## Существующее подключение и целевой маршрут

| Поле | Сейчас | При выбранной миграции |
| --- | --- | --- |
| Public ID | `nano_banana_2` | Сохранить |
| Provider/model | PoYo / `nano-banana-2-new` | APIMart / `gemini-3.1-flash-image-preview` |
| References | PoYo edit `nano-banana-2-new-edit`, до 14 refs | Подтверждённый image_urls contract, до 14 refs по исследованию |
| Флаг модели | `FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED` | Сохранить; обновить provider readiness для новых Jobs |
| Qualities | 1K/2K/4K | 1K/2K/4K после проверки тарифа |
| Новый submit/poll | PoYo adapter | `POST /v1/images/generations`; `GET /v1/tasks/{id}` |

- [Карточка Study24](https://study24.ai/chat/nano_banana_2).
- [APIMart generation](https://docs.apimart.ai/ru/api-reference/images/gemini-3.1-flash/generation), [schema/metadata](https://docs.apimart.ai/ru/api-reference/texts/models/list), [прайс исходного исследования](https://apimart.ai/ru/pricing).
- Текущий код: `internal/service/providermodels/registry.go`, `internal/adapter/provider/poyo/poyo.go`, `internal/platform/config/config.go`, `internal/service/pricingcatalog/static_catalog.go`.

Пример целевого контракта; он пока не реализован в APIMart adapter:

```json
{"model":"gemini-3.1-flash-image-preview","prompt":"synthetic fixture","resolution":"1K","size":"1:1","n":1}
```

По исследованию: HTTP 200 → data[0].task_id; n=1, refs≤14, search/official fallback выключены, 0.5K не открывать без тарифа. Использовать exact ID после B0, не переключать aliases по догадке.

## Уже выполнено

- [x] Public ID, feature flag, PoYo readiness, качества и reference limit зарегистрированы.
- [x] PoYo generation/edit serializers и polling реализованы; reference-вход использует отдельный edit ID.
- [x] Тарифы трёх качеств, image resolver, snapshots и общий Web text-to-image поток существуют.
- [x] `TestRegistryNanoBanana2UsesPoyoSourceContract`, `TestSubmitNanoBanana2TextOnlyUsesGenerationModel` и `TestSubmitNanoBanana2ReferencesUseEditModelAndImageURLs` прошли в составе локальных package tests.

## Осталось только при выборе миграции

- [ ] **41.1 — B0.** Подтвердить `gemini-3.1-flash-image-preview`, schema/refs и тариф рабочей группы; записать очищенный fixture `internal/adapter/provider/apimart/testdata/contracts/nano_banana_2.request.json`.
- [ ] **41.2 — APIMart adapter.** Добавить этот exact ID/capability/validation и serializer на существующий transport. Проверить exact request, 14/15 refs, n=1, resolution и legacy submit/poll. PoYo adapter сохранить для старых Jobs.
- [ ] **41.3 — routing и цена новых Jobs.** Согласованно обновить registry, provider readiness/config validation и новую price version. Исследование APIMart: $0.015/$0.02/$0.025 за 1K/2K/4K. Текущие PoYo floors и retail tariffs не являются ставкой APIMart. Проверить, что отсутствие APIMart config/price закрывает новые Jobs.
- [ ] **41.4 — совместимость.** Job, подготовленный до переключения, при поздней активации использует прежний PoYo/model/price snapshot; уже submitted Job продолжает PoYo polling. Новый Job использует APIMart. Не менять маршрут только по текущему public ID при восстановлении.
- [ ] **41.5 — надёжность.** Применить B2 к новому APIMart route: timeout/crash/replay не создают новый платный submit без разрешённой процедуры. Повтор capture идемпотентен.
- [ ] **41.6 — выпуск миграции.** После тестов — отдельно разрешённый canary выбранной конфигурации, сверка cost/quote/ledger. Rollback меняет только выбор для ещё не подготовленных Jobs, сохраняя adapters/polling для обеих групп.
- [ ] **41.7 — Web refs, если нужны.** Отдельно выполнить I для загрузки owner-scoped input artifacts; миграция backend не добавляет Web upload автоматически.

## Файлы миграции

- `internal/adapter/provider/apimart/apimart.go`, `apimart_test.go`; новый `model_nano_banana_2_test.go` для сценариев нового APIMart ID.
- `internal/service/providermodels/registry.go`, `registry_test.go`, `internal/platform/config/config.go`, `config_test.go`.
- `internal/service/pricingcatalog/static_catalog.go`, `static_catalog_test.go`; runtime price version — по B3.
- `internal/service/imagegeneration/resolver_test.go`, `internal/adapter/inbound/websession/image_jobs_test.go`, `internal/worker/worker_test.go` — late activation, snapshot/replay и завершение PoYo Jobs.
- Registry consumers/modelcatalog/productcatalog менять только если их wiring требует этого; новую Web карточку не создавать.

## Приёмка и проверки

- [ ] Подготовленный PoYo Job после миграции активируется с прежними условиями; старый submitted Job poll-ится через PoYo.
- [ ] Новый Job snapshot содержит APIMart/exact ID/новую price version; API не принимает provider/price от клиента.
- [ ] Disabled/unready/missing price запрещают новые Jobs, не мешая старому polling.
- [ ] 15 refs отклоняются до submit; output сохраняется как собственный moderated Artifact.
- [ ] Миграция и rollback не создают второй submit/capture и не переносят уже зарезервированные Jobs.

```powershell
go test ./internal/adapter/provider/apimart ./internal/adapter/provider/poyo ./internal/service/providermodels ./internal/platform/config
go test ./internal/service/imagegeneration ./internal/service/pricingcatalog ./internal/adapter/inbound/websession ./internal/worker
```

Новый `TestModel_nano_banana_2` запускать после его добавления, не считать отсутствие такого теста успешной проверкой. При изменении кода — gofmt/go vet затронутых пакетов. До отдельного выбора этого плана Nano Banana 2 остаётся существующей интеграцией PoYo.
