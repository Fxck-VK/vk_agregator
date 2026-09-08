# 56. Google Veo 3.1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** подключить Google Veo 3.1 через выбранный маршрут APIMart.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [полный реестр](./README.md), [общая основа](./00-common-foundation.md). Статус: план, не реализовано в рамках этого задания.

## Global Constraints

Выполнять после B0–B3 и раздела V общей основы. Для этой модели отдельный флаг; разрешение на её реализацию не означает включение остальных. Provider key, prompt, raw payload и private URLs не записываются в отчёт/fixtures. Старые Jobs используют собственные snapshots. Платные проверки, commit/push/deploy не выполняются при подготовке плана.

## Документация и точное соответствие

- [Карточка Study24](https://study24.ai/chat/google_veo3)
- [Документация veo3/generation](https://docs.apimart.ai/ru/api-reference/videos/veo3/generation)
- [Актуальный прайс APIMart](https://apimart.ai/ru/pricing), проверен 08.09.2026.
- [Доступность ID и schema для конкретного ключа](https://docs.apimart.ai/ru/api-reference/texts/models/list).

| Поле | Значение |
| --- | --- |
| Название Study24 | Google Veo 3.1 |
| Предлагаемый public ID | `veo_3_1_fast` |
| APIMart model ID | `veo3.1-fast` |
| Метод | `POST https://api.apimart.ai/v1/videos/generations` |
| Флаг | `FEATURE_APIMART_VEO_3_1_FAST_ENABLED` |
| Соответствие | Условное: версия/режим уточнены в этом плане; тождество Study24 не доказано |

Study24 не уточняет режим. Предлагаемый старт — явно названный Veo 3.1 Fast. Lite/Quality добавить как отдельные доступные варианты с собственными ключами цены и лимитами.

Перед включением проверить наличие exact ID в доступном ключу каталоге и цену выбранного варианта.

## Контракт первого выпуска

Фиксированные 8 с; Fast: frame 1–2 изображения или reference до 3; 720p/1080p/4K. Lite не поддерживает image_urls/generation_type; Quality не поддерживает reference.

Пример тела запроса — основа fixture, не команда на выполнение. URL example.com в референсах обозначают тестовые входы; рабочие ссылки формирует worker из принадлежащих аккаунту артефактов.

```json
{
  "model": "veo3.1-fast",
  "prompt": "Плавное движение камеры над горным озером.",
  "duration": 8,
  "resolution": "720p",
  "aspect_ratio": "16:9",
  "official_fallback": false
}
```

Асинхронный submit HTTP 200 → data[0].task_id; GET /v1/tasks/{task_id}. Сохранить result.videos[].url в собственное хранилище и провести модерацию до доставки.

## Цена

Fast $0.14 за 8 с (4K $0.64); Quality $1.00 (4K $1.50); Lite $0.07 (4K $0.57). Это разные режимы, не цена одной одинаковой конфигурации.

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
- Создать fixture `internal/adapter/provider/apimart/testdata/contracts/veo_3_1_fast.request.json`.
- Создать `internal/adapter/provider/apimart/model_veo_3_1_fast_test.go` с именем теста `TestModel_veo_3_1_fast`.

## Порядок реализации

- [ ] **1. Контракт.** В B0 проверить `veo3.1-fast`, записать разрешённые поля/операции и очищенный fixture указанного запроса. Для спорной версии сохранить предлагаемое полное название в UI и закрыть rollout до подтверждения выбора. Не выводить статус «подключено» по одному наличию в прайсе.
- [ ] **2. Адаптер.** Реализовать serializer и capability для `veo3.1-fast` по указанному JSON; локально проверять лимиты. Разобрать описанную форму результата. В тестовом HTTP server проверить exact endpoint/body; неподдерживаемое значение не должно вызывать HTTP.
- [ ] **3. Каталог и стоимость.** Добавить/обновить public ID `veo_3_1_fast`, backend readiness и `FEATURE_APIMART_VEO_3_1_FAST_ENABLED` в config wiring. Создать отдельные ключи всех открываемых вариантов; определить exact rates и reserve/capture fixture. Сохранить model ID и тариф в Job snapshot.
- [ ] **4. Пользовательский поток.** Backend resolver разрешает только зарегистрированный public ID; UI показывает допустимые варианты и серверный quote. Job создаётся под владельцем с idempotency key, результат появляется в его истории. Ошибка модели не переключает на иной ID.
- [ ] **5. Проверки этой модели.** Реализовать перечисленные ниже positive/negative cases; общий lifecycle B2/B3 применяется к этому exact ID. Сначала получить ожидаемое падение нового contract test, затем реализовать маршрут и добиться PASS.
- [ ] **6. Выпуск.** Пройти preflight, тесты и ограниченный отдельно разрешённый canary именно этой конфигурации; сверить provider cost и ledger. До этого `FEATURE_APIMART_VEO_3_1_FAST_ENABLED` выключен. Отключение запрещает новые submit, но не мешает завершить старые Jobs.

## Приёмочные сценарии

- [ ] duration=5 отклоняется.
- [ ] Fast 8 с 720p=$0.14 за вызов, не $0.14/с.
- [ ] Lite с изображением отклоняется, а не подменяется Fast.
- [ ] `FEATURE_APIMART_VEO_3_1_FAST_ENABLED=false` → модель недоступна для нового Job; ключ/остаток провайдера не раскрываются клиенту.
- [ ] Повторный Job с тем же idempotency key не создаёт второй платный запрос; result повторно не списывается.
- [ ] Невалидный ответ, provider failure и блокировка модерацией дают контролируемый статус без выдачи непроверенного результата.

## Следующая стадия этой модели

- [ ] Стадия 2: отдельные варианты Lite и Quality с точными model IDs veo3.1-lite/veo3.1-quality, ценами и ограничениями. Для Fast включить frame/reference только после тестов каждого режима. Extend — отдельная операция с ценой.

## Команды проверки при реализации

```powershell
go test ./internal/adapter/provider/apimart -run '^TestModel_veo_3_1_fast$' -count=1
go test ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/worker
```

Для изменённого resolver добавить его package test; при изменении UI выполнить typecheck/lint/test из [общей основы](./00-common-foundation.md). Эти команды предназначены для будущей реализации и не являются заявлением о пройденных сейчас тестах.

**Готовность:** После общей видеоосновы и прохождения тестов этой конфигурации. При незакрытом условии соответствия/стоимости план подготовлен, но модель не готова к включению.
