# 37. Nano Banana Pro — план сопровождения существующей интеграции

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** сверить и при необходимости обновить существующий маршрут APIMart, сохранив public ID и готовый pipeline.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [реестр и актуальная очередь](./README.md), [общая основа](./00-common-foundation.md).
**Статус:** реализовано в коде; остаются сверка контракта/цены и общие доработки. Сверка 08.09.2026, HEAD `cb34ded57`; live-доступность не проверялась.

## Global Constraints

Не создавать вторую карточку, флаг или serializer. Существующие флаги/env и тарифы не менять при чтении плана. Новые варианты проходят B0 и необходимые B1/B2/B3/I; переносить на них уже реализованные проверки. Старые и подготовленные Jobs сохраняют provider/model/price snapshots. Ключи, пользовательские prompts, raw payload и private URLs не писать в документы/fixtures. Платные canary, commit/push/deploy не входят в актуализацию.

## Текущий маршрут и источники

| Поле | Реализовано |
| --- | --- |
| Public ID | `nano_banana_pro` |
| Provider / model | APIMart / `gemini-3-pro-image-preview` |
| Submit / poll | `POST /v1/images/generations` → `data[0].task_id`; `GET /v1/tasks/{id}` |
| Флаг | `FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED` |
| Варианты | 1K/2K/4K; n=1; до 14 refs; fallback не включается |

- [Карточка Study24](https://study24.ai/chat/nano_banana_pro).
- [API generation](https://docs.apimart.ai/ru/api-reference/images/gemini-3-pro/generation), [schema/metadata](https://docs.apimart.ai/ru/api-reference/texts/models/list), [прайс исходного исследования](https://apimart.ai/ru/pricing).
- Код: `internal/service/providermodels/registry.go`, `internal/adapter/provider/apimart/apimart.go`, `internal/platform/config/config.go`.

Пример целевого тела для контрактной сверки, без пользовательского текста:

```json
{"model":"gemini-3-pro-image-preview","prompt":"synthetic fixture","resolution":"1K","size":"1:1","n":1}
```

Serializer уже передаёт resolution, n=1 и optional image_urls; поле official_fallback опущено при false. Проверка иного поведения default провайдера относится к B0, а не к заявлению о live-гарантии.

## Уже выполнено

- [x] Public ID, APIMart mapping, feature flag и readiness зарегистрированы.
- [x] Submit/poll, 1K/2K/4K, n=1, reference limit 14 и нормализация ошибок реализованы.
- [x] Есть отдельные price keys для трёх качеств, immutable price snapshot и backend resolver.
- [x] Общий Web text-to-image поток выбора модели/качества/цены, prepare/activate/history/result существует.
- [x] Контрактные проверки в `apimart_test.go` и mapping в `registry_test.go` прошли при сверке.

## Осталось

- [ ] **37.1 — B0.** Проверить exact ID, schema, допустимые поля и персональную группу тарифа. Создать только очищенный fixture `internal/adapter/provider/apimart/testdata/contracts/nano_banana_pro.request.json`, если требуется закрепить подтверждённый контракт.
- [ ] **37.2 — цена.** Сверить существующий floor с исследованием; при подтверждённом изменении выпустить новую price version. Код static catalog содержит 0.40/0.50/0.50 APIMart credits (= $0.04/$0.05/$0.05), исходное исследование — $0.03/$0.03/$0.04. Это расхождение источников, не доказательство активной цены DEV/PROD. Пользовательский тариф не выводить автоматически из меньшего числа.
- [ ] **37.3 — B2.** Закрыть crash-window до сохранения task_id и проверить повтор одного Job без второго платного submit/capture. Готовый adapter cache не заменяет durable intent.
- [ ] **37.4 — только при выборе Web refs.** Выполнить недостающий Web image-to-image пункт I: upload, owner-scoped input artifacts, API/DTO и редактор. Adapter поддерживает refs; Web форма пока только text-to-image.
- [ ] **37.5 — выпуск изменений.** Дополнить существующие тесты лишь выявленными расхождениями. Для изменённой конфигурации после проверок выполнить отдельно разрешённый бюджетный canary; сравнить quote, provider cost и ledger. Отключение новых submit не останавливает polling ранее принятых задач.

## Файлы оставшихся изменений

- Contract/validation при расхождении: `internal/adapter/provider/apimart/apimart.go`, `apimart_test.go`. `images.go` пока не существует и не требуется ради переименования.
- Price: `internal/service/pricingcatalog/static_catalog.go`, `static_catalog_test.go`, runtime catalog по B3 при соответствующем источнике.
- Durable submit/recovery: файлы B2; model registry/resolver/config меняются только если B0 докажет изменение capabilities.
- Web refs — отдельный пункт I; существующий Web pipeline не создавать повторно.

## Приёмка и проверки

- [x] Текущий путь и верхний лимит refs покрыты `TestSubmitGemini3ProImageSuccess`, `TestSubmitGemini3ProImageValidation`, `TestRegistryNanoBananaProUsesAPIMartGemini3ProContract`.
- [ ] При обновлении floor новый Job получает новую версию; подготовленный/старый Job сохраняет принятую.
- [ ] 14 refs принимаются, 15 отклоняются до HTTP; повторная проверка на новой schema при её изменении.
- [ ] Disabled/unready route и moderation rejection не дают новый submit/непроверенный результат.
- [ ] Crash после HTTP до записи task_id, конкурентный redelivery и повтор capture проверены по B2 именно для APIMart.

```powershell
go test ./internal/adapter/provider/apimart -run 'Test(SubmitGemini3ProImage|PollCompletedImage|PollImagePolicy)' -count=1
go test ./internal/service/providermodels ./internal/service/imagegeneration ./internal/service/pricingcatalog ./internal/worker
```

При изменении кода дополнительно gofmt/go vet затронутых пакетов; Web проверки — только при изменении Web. Результаты уже выполненной локальной сверки и отсутствие vitest записаны в README. Модель исключена из очереди первичного подключения.
