# 51. Seedance 2.5 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** подключить Seedance 2.5 через выбранный маршрут APIMart.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [полный реестр](./README.md), [общая основа](./00-common-foundation.md). Статус: план, не реализовано в рамках этого задания.

## Global Constraints

Выполнять после B0–B3 и раздела V общей основы. Для этой модели отдельный флаг; разрешение на её реализацию не означает включение остальных. Provider key, prompt, raw payload и private URLs не записываются в отчёт/fixtures. Старые Jobs используют собственные snapshots. Платные проверки, commit/push/deploy не выполняются при подготовке плана.

## Документация и точное соответствие

- [Карточка Study24](https://study24.ai/chat/seedance25)
- [Документация seedance-2-5/generation](https://docs.apimart.ai/ru/api-reference/videos/seedance-2-5/generation)
- [Актуальный прайс APIMart](https://apimart.ai/ru/pricing), проверен 08.09.2026.
- [Доступность ID и schema для конкретного ключа](https://docs.apimart.ai/ru/api-reference/texts/models/list).

| Поле | Значение |
| --- | --- |
| Название Study24 | Seedance 2.5 |
| Предлагаемый public ID | `seedance_2_5` |
| APIMart model ID | `seedance-2.5` |
| Метод | `POST https://api.apimart.ai/v1/videos/generations` |
| Флаг | `FEATURE_APIMART_SEEDANCE_2_5_ENABLED` |
| Соответствие | Название модели сопоставлено с каталогом/контрактом APIMart; доступ рабочего ключа ещё не проверен |

Прайс содержит также token-тарифы: до запуска сверить правило расчёта token usage и точность посекундной оценки. Edit/extend, MOV и audio-only references — следующая стадия этого плана с отдельными snapshots.

Перед включением проверить наличие exact ID в доступном ключу каталоге и цену выбранного варианта.

## Контракт первого выпуска

4–30 секунд; 480p/720p/1080p; 30 изображений, 10 видео, 10 аудио по полной схеме. Старт T2V/I2V, фиксированные 5/10/15/30 с, MP4; duration=-1 отключён.

Пример тела запроса — основа fixture, не команда на выполнение. URL example.com в референсах обозначают тестовые входы; рабочие ссылки формирует worker из принадлежащих аккаунту артефактов.

```json
{
  "model": "seedance-2.5",
  "prompt": "Плавное движение камеры над горным озером.",
  "resolution": "480p",
  "size": "16:9",
  "duration": 5,
  "generate_audio": true
}
```

Асинхронный submit HTTP 200 → data[0].task_id; GET /v1/tasks/{task_id}. Сохранить result.videos[].url в собственное хранилище и провести модерацию до доставки.

## Цена

Без входного видео: $0.09608 / $0.216 / $0.38488 за секунду 480p/720p/1080p. С входным видео: $0.0576 / $0.1296 / $0.22992 × (вход+выход).

Это себестоимость APIMart на дату проверки, не утверждённая пользовательская цена. Перед включением зафиксировать точный тариф/единицу/группу в версии price catalog. Quote и резерв определяет backend; итог не превышает принятый резерв. Не полагаться на значения по умолчанию провайдера, если они меняют цену.

## Файлы реализации

Пути от корня репозитория; новые общие файлы создаются по плану основы, существующие изменяются.

- `internal/adapter/provider/apimart/video.go`
- `internal/adapter/provider/apimart/apimart_test.go`
- `internal/service/providermodels/registry.go`
- `internal/service/videorouter/catalog.go`
- `internal/service/videogeneration/resolver.go`
- `internal/service/pricingcatalog/apimart_rates.go`
- `internal/worker/worker.go`
- `internal/worker/media_contracts.go`
- `web/platform/src/features/video-generation/VideoGenerationPanel.tsx`
- `internal/platform/config/config.go`
- Создать fixture `internal/adapter/provider/apimart/testdata/contracts/seedance_2_5.request.json`.
- Создать `internal/adapter/provider/apimart/model_seedance_2_5_test.go` с именем теста `TestModel_seedance_2_5`.

## Порядок реализации

- [ ] **1. Контракт.** В B0 проверить `seedance-2.5`, записать разрешённые поля/операции и очищенный fixture указанного запроса. Для спорной версии сохранить предлагаемое полное название в UI и закрыть rollout до подтверждения выбора. Не выводить статус «подключено» по одному наличию в прайсе.
- [ ] **2. Адаптер.** Реализовать serializer и capability для `seedance-2.5` по указанному JSON; локально проверять лимиты. Разобрать описанную форму результата. В тестовом HTTP server проверить exact endpoint/body; неподдерживаемое значение не должно вызывать HTTP.
- [ ] **3. Каталог и стоимость.** Добавить/обновить public ID `seedance_2_5`, backend readiness и `FEATURE_APIMART_SEEDANCE_2_5_ENABLED` в config wiring. Создать отдельные ключи всех открываемых вариантов; определить exact rates и reserve/capture fixture. Сохранить model ID и тариф в Job snapshot.
- [ ] **4. Пользовательский поток.** Backend resolver разрешает только зарегистрированный public ID; UI показывает допустимые варианты и серверный quote. Job создаётся под владельцем с idempotency key, результат появляется в его истории. Ошибка модели не переключает на иной ID.
- [ ] **5. Проверки этой модели.** Реализовать перечисленные ниже positive/negative cases; общий lifecycle B2/B3 применяется к этому exact ID. Сначала получить ожидаемое падение нового contract test, затем реализовать маршрут и добиться PASS.
- [ ] **6. Выпуск.** Пройти preflight, тесты и ограниченный отдельно разрешённый canary именно этой конфигурации; сверить provider cost и ledger. До этого `FEATURE_APIMART_SEEDANCE_2_5_ENABLED` выключен. Отключение запрещает новые submit, но не мешает завершить старые Jobs.

## Приёмочные сценарии

- [ ] 5 с 480p без video=$0.4804.
- [ ] 5 с входа + 5 с выхода 720p=$1.296 по посекундной таблице, не $0.648.
- [ ] 4K отклоняется.
- [ ] `FEATURE_APIMART_SEEDANCE_2_5_ENABLED=false` → модель недоступна для нового Job; ключ/остаток провайдера не раскрываются клиенту.
- [ ] Повторный Job с тем же idempotency key не создаёт второй платный запрос; result повторно не списывается.
- [ ] Невалидный ответ, provider failure и блокировка модерацией дают контролируемый статус без выдачи непроверенного результата.

## Следующая стадия этой модели

- [ ] Стадия 2: reference/edit/extend с omni_reference_task_type; typed roles, проверка суммарной длины и 30/10/10 лимитов, MOV output. Для каждого режима отдельный snapshot; duration=-1 только после надёжного расчёта потолка и возврата разницы.

## Команды проверки при реализации

```powershell
go test ./internal/adapter/provider/apimart -run '^TestModel_seedance_2_5$' -count=1
go test ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/worker
```

Для изменённого resolver добавить его package test; при изменении UI выполнить typecheck/lint/test из [общей основы](./00-common-foundation.md). Эти команды предназначены для будущей реализации и не являются заявлением о пройденных сейчас тестах.

**Готовность:** После общей видеоосновы и прохождения тестов этой конфигурации. При незакрытом условии соответствия/стоимости план подготовлен, но модель не готова к включению.
