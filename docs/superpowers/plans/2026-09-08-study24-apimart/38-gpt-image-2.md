# 38. GPT Image 2 — план сопровождения существующей интеграции

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** сверить контракт и себестоимость уже реализованного GPT Image 2 без повторного подключения.
**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [реестр и актуальная очередь](./README.md), [общая основа](./00-common-foundation.md).
**Статус:** реализовано в коде; остаются сверка и общие доработки. Сверка 08.09.2026, HEAD `cb34ded57`; live-доступность не проверялась.

## Global Constraints

Сохранить public ID, готовый serializer, feature flag и pipeline. Проверка плана не отключает текущую модель и не изменяет runtime/env/цены. Обновления проходят B0 и необходимые B1/B2/B3/I. Старые и подготовленные Jobs сохраняют snapshots. Не включать n>1 или дорогой fallback по умолчанию. Ключи, prompts, raw payload и private URLs не писать в документы/fixtures. Платные canary, commit/push/deploy — вне актуализации.

## Текущий маршрут и источники

| Поле | Реализовано |
| --- | --- |
| Public ID | `gpt_image_2` |
| Provider / model | APIMart / `gpt-image-2` |
| Submit / poll | `POST /v1/images/generations` → `data[0].task_id`; `GET /v1/tasks/{id}` |
| Флаг | `FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED` |
| Варианты | 1K/2K/4K; lowercase resolution; n=1; до 16 refs |

- [Карточка Study24](https://study24.ai/chat/gpt_image).
- [API generation](https://docs.apimart.ai/ru/api-reference/images/gpt-image-2/generation), [schema/metadata](https://docs.apimart.ai/ru/api-reference/texts/models/list), [прайс исходного исследования](https://apimart.ai/ru/pricing).
- Код: `internal/service/providermodels/registry.go`, `internal/adapter/provider/apimart/apimart.go`, `internal/platform/config/config.go`.

Пример тела для сверки, без пользовательского текста:

```json
{"model":"gpt-image-2","prompt":"synthetic fixture","resolution":"1k","size":"1:1","n":1}
```

Нижний регистр resolution, n=1 и optional image_urls уже реализованы. Из исходного исследования: 4K square описан как 2880×2880, не обещать 4096×4096; точные размеры и limits сверить по B0. Поле official_fallback опущено при false.

## Уже выполнено

- [x] Public ID, APIMart mapping, feature flag и readiness существуют.
- [x] Serializer нормализует 1K/2K/4K в 1k/2k/4k и задаёт n=1; submit/poll и errors реализованы.
- [x] Model-specific validation: до 16 refs, лимиты data URL 20 MiB на изображение / 256 MiB суммарно в адаптере; не переносить этот лимит на Pro.
- [x] Price keys трёх качеств, trusted resolver и immutable price snapshot есть.
- [x] Общий Web text-to-image pipeline и backend-тесты существуют; контрактные Go-тесты прошли при сверке.

## Осталось

- [ ] **38.1 — B0.** Подтвердить exact ID/schema, текущий предел refs, sizes и output dimensions. При изменении обновить существующие validation/test cases, не писать второй serializer. Очищенный fixture: `internal/adapter/provider/apimart/testdata/contracts/gpt_image_2.request.json`.
- [ ] **38.2 — цена.** Сверить code-backed floors 0.06/0.12/0.18 APIMart credits (= $0.006/$0.012/$0.018) с исходным исследованием $0.0085/$0.014/$0.021. Runtime DB и персональная группа отдельно; меньшую ставку не принимать автоматически. Подтверждённые изменения — новой версией, без пересчёта старых Jobs.
- [ ] **38.3 — B2.** Durable submit intent/lease и разрешение неопределённого исхода. Отдельно проверить APIMart redelivery и отсутствие второго capture.
- [ ] **38.4 — только при выборе Web refs.** Реализовать upload/API/owner-scoped input artifacts по I. Возможность адаптера принимать refs не означает готовый Web image-to-image.
- [ ] **38.5 — выпуск изменений.** Проверить изменённые варианты, отсутствие молчаливого снижения качества и ограниченный отдельно разрешённый canary с сопоставлением cost/quote/ledger. Существующие флаги заново не создавать.

## Файлы оставшихся изменений

- `internal/adapter/provider/apimart/apimart.go`, `apimart_test.go` — только подтверждённый contract delta; `images.go` пока не существует.
- `internal/service/providermodels/registry.go`, `registry_test.go` — только если изменился подтверждённый limit.
- `internal/service/pricingcatalog/static_catalog.go`, `static_catalog_test.go` и runtime catalog при соответствующем источнике цены.
- B2 — общий durable submit; I — отдельный Web refs поток.

## Приёмка и проверки

- [x] Lowercase resolution и n=1 проверяет `TestSubmitGPTImage2Success`; model-specific refs — `TestSubmitGPTImage2ValidationUsesModelSpecificLimits`, `TestRegistryGPTImage2ReferenceLimitIsIsolated`.
- [ ] Подтверждённая schema/size не обещает результат больше фактического output; n>1 остаётся закрыт.
- [ ] 16-й/17-й ref и отключённый флаг проверены после любого изменения лимитов.
- [ ] Новый тариф не изменяет snapshot существующего Job; повтор не создаёт второй submit/capture.
- [ ] Invalid provider response/moderation rejection не приводит к выдаче непроверенного результата.

```powershell
go test ./internal/adapter/provider/apimart -run 'Test(SubmitGPTImage2|PollCompletedImage|PollImagePolicy)' -count=1
go test ./internal/service/providermodels ./internal/service/imagegeneration ./internal/service/pricingcatalog ./internal/worker
```

При изменении кода дополнительно gofmt/go vet затронутых пакетов; Web проверки — при изменении Web. Локальные результаты сверки и ограничение vitest см. README. Модель исключена из очереди первичного подключения.
