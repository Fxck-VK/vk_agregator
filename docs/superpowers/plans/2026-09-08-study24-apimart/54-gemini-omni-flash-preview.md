# 54. Gemini Omni Flash — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** подключить Gemini Omni Flash через выбранный маршрут APIMart.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [полный реестр](./README.md), [общая основа](./00-common-foundation.md). Статус: план, не реализовано в рамках этого задания.

## Global Constraints

Выполнять после B0–B3 и раздела V общей основы. Для этой модели отдельный флаг; разрешение на её реализацию не означает включение остальных. Provider key, prompt, raw payload и private URLs не записываются в отчёт/fixtures. Старые Jobs используют собственные snapshots. Платные проверки, commit/push/deploy не выполняются при подготовке плана.

## Документация и точное соответствие

- [Карточка Study24](https://study24.ai/chat/gemini_omni_flash)
- [Документация gemini-omni-flash-preview/generation](https://docs.apimart.ai/ru/api-reference/videos/gemini-omni-flash-preview/generation)
- [Актуальный прайс APIMart](https://apimart.ai/ru/pricing), проверен 08.09.2026.
- [Доступность ID и schema для конкретного ключа](https://docs.apimart.ai/ru/api-reference/texts/models/list).

| Поле | Значение |
| --- | --- |
| Название Study24 | Gemini Omni Flash |
| Предлагаемый public ID | `gemini_omni_flash_preview` |
| APIMart model ID | `gemini-omni-flash-preview` |
| Метод | `POST https://api.apimart.ai/v1/videos/generations` |
| Флаг | `FEATURE_APIMART_GEMINI_OMNI_FLASH_PREVIEW_ENABLED` |
| Соответствие | Условное: версия/режим уточнены в этом плане; тождество Study24 не доказано |

Используется Preview — показать явно. Входное видео и многораундовое редактирование открывать после расчёта входной стоимости; старт T2V/I2V.

Перед включением проверить наличие exact ID в доступном ключу каталоге и цену выбранного варианта.

## Контракт первого выпуска

720p, 24fps, звук; результат 3–10 с. В документации нет поля duration: не отправлять и не показывать фиктивный выбор секунд. До 16 изображений; 1 видео; video_urls исключает extend_from_task_id.

Пример тела запроса — основа fixture, не команда на выполнение. URL example.com в референсах обозначают тестовые входы; рабочие ссылки формирует worker из принадлежащих аккаунту артефактов.

```json
{
  "model": "gemini-omni-flash-preview",
  "prompt": "Плавное движение камеры над горным озером.",
  "resolution": "720p",
  "aspect_ratio": "16:9"
}
```

Асинхронный submit HTTP 200 → data[0].task_id; GET /v1/tasks/{task_id}. Сохранить result.videos[].url в собственное хранилище и провести модерацию до доставки.

## Цена

$0.088 за секунду 720p вывода. Для переменной длины до 10 с предварительный потолок $0.88 без входных доплат.

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
- Создать fixture `internal/adapter/provider/apimart/testdata/contracts/gemini_omni_flash_preview.request.json`.
- Создать `internal/adapter/provider/apimart/model_gemini_omni_flash_preview_test.go` с именем теста `TestModel_gemini_omni_flash_preview`.

## Порядок реализации

- [ ] **1. Контракт.** В B0 проверить `gemini-omni-flash-preview`, записать разрешённые поля/операции и очищенный fixture указанного запроса. Для спорной версии сохранить предлагаемое полное название в UI и закрыть rollout до подтверждения выбора. Не выводить статус «подключено» по одному наличию в прайсе.
- [ ] **2. Адаптер.** Реализовать serializer и capability для `gemini-omni-flash-preview` по указанному JSON; локально проверять лимиты. Разобрать описанную форму результата. В тестовом HTTP server проверить exact endpoint/body; неподдерживаемое значение не должно вызывать HTTP.
- [ ] **3. Каталог и стоимость.** Добавить/обновить public ID `gemini_omni_flash_preview`, backend readiness и `FEATURE_APIMART_GEMINI_OMNI_FLASH_PREVIEW_ENABLED` в config wiring. Создать отдельные ключи всех открываемых вариантов; определить exact rates и reserve/capture fixture. Сохранить model ID и тариф в Job snapshot.
- [ ] **4. Пользовательский поток.** Backend resolver разрешает только зарегистрированный public ID; UI показывает допустимые варианты и серверный quote. Job создаётся под владельцем с idempotency key, результат появляется в его истории. Ошибка модели не переключает на иной ID.
- [ ] **5. Проверки этой модели.** Реализовать перечисленные ниже positive/negative cases; общий lifecycle B2/B3 применяется к этому exact ID. Сначала получить ожидаемое падение нового contract test, затем реализовать маршрут и добиться PASS.
- [ ] **6. Выпуск.** Пройти preflight, тесты и ограниченный отдельно разрешённый canary именно этой конфигурации; сверить provider cost и ledger. До этого `FEATURE_APIMART_GEMINI_OMNI_FLASH_PREVIEW_ENABLED` выключен. Отключение запрещает новые submit, но не мешает завершить старые Jobs.

## Приёмочные сценарии

- [ ] JSON не содержит duration.
- [ ] Выход 8 с без входных доплат=$0.704; резерв допускает 10 с.
- [ ] Расширение по чужому task_id запрещено.
- [ ] `FEATURE_APIMART_GEMINI_OMNI_FLASH_PREVIEW_ENABLED=false` → модель недоступна для нового Job; ключ/остаток провайдера не раскрываются клиенту.
- [ ] Повторный Job с тем же idempotency key не создаёт второй платный запрос; result повторно не списывается.
- [ ] Невалидный ответ, provider failure и блокировка модерацией дают контролируемый статус без выдачи непроверенного результата.

## Следующая стадия этой модели

- [ ] Стадия 2: video edit/extend_from_task_id с проверкой владельца исходной задачи, её версии и расхода входных секунд. Нельзя считать каждый раунд чистым T2V.

## Команды проверки при реализации

```powershell
go test ./internal/adapter/provider/apimart -run '^TestModel_gemini_omni_flash_preview$' -count=1
go test ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/worker
```

Для изменённого resolver добавить его package test; при изменении UI выполнить typecheck/lint/test из [общей основы](./00-common-foundation.md). Эти команды предназначены для будущей реализации и не являются заявлением о пройденных сейчас тестах.

**Готовность:** После общей видеоосновы и прохождения тестов этой конфигурации. При незакрытом условии соответствия/стоимости план подготовлен, но модель не готова к включению.
