# 53. Gemini Omni 1.1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** подключить Gemini Omni 1.1 через выбранный маршрут APIMart.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [полный реестр](./README.md), [общая основа](./00-common-foundation.md). Статус: план, не реализовано в рамках этого задания.

## Global Constraints

Выполнять после B0–B3 и раздела V общей основы. Для этой модели отдельный флаг; разрешение на её реализацию не означает включение остальных. Provider key, prompt, raw payload и private URLs не записываются в отчёт/fixtures. Старые Jobs используют собственные snapshots. Платные проверки, commit/push/deploy не выполняются при подготовке плана.

## Документация и точное соответствие

- [Карточка Study24](https://study24.ai/chat/gemini_omni1)
- [Документация omni-flash-ext/generation](https://docs.apimart.ai/ru/api-reference/videos/omni-flash-ext/generation)
- [Документация gemini-omni-1.1-flash/generation](https://docs.apimart.ai/ru/api-reference/videos/gemini-omni-1.1-flash/generation)
- [Актуальный прайс APIMart](https://apimart.ai/ru/pricing), проверен 08.09.2026.
- [Доступность ID и schema для конкретного ключа](https://docs.apimart.ai/ru/api-reference/texts/models/list).

| Поле | Значение |
| --- | --- |
| Название Study24 | Gemini Omni 1.1 |
| Предлагаемый public ID | `gemini_omni_1_1_flash_ext` |
| APIMart model ID | `gemini-omni-1.1-flash-ext` |
| Метод | `POST https://api.apimart.ai/v1/videos/generations` |
| Флаг | `FEATURE_APIMART_GEMINI_OMNI_1_1_FLASH_EXT_ENABLED` |
| Соответствие | Условное: версия/режим уточнены в этом плане; тождество Study24 не доказано |

В Study24 название без Flash/EXT. Это документированный более дешёвый кандидат, не доказанное тождество. Официальный gemini-omni-1.1-flash имеет другой контракт; не включать автоматическую замену.

Перед включением проверить наличие exact ID в доступном ключу каталоге и цену выбранного варианта.

## Контракт первого выпуска

Длительность 4/6/8/10 с, generation_type frame/reference; 1 или 3 изображения, не 2. Старт 720p/1080p, T2V/I2V. video_urls и duration взаимоисключающие.

Пример тела запроса — основа fixture, не команда на выполнение. URL example.com в референсах обозначают тестовые входы; рабочие ссылки формирует worker из принадлежащих аккаунту артефактов.

```json
{
  "model": "gemini-omni-1.1-flash-ext",
  "prompt": "Плавное движение камеры над горным озером.",
  "resolution": "720p",
  "aspect_ratio": "16:9",
  "duration": 4
}
```

Асинхронный submit HTTP 200 → data[0].task_id; GET /v1/tasks/{task_id}. Сохранить result.videos[].url в собственное хранилище и провести модерацию до доставки.

## Цена

720p/1080p: 4/6/8/10 с стоят $0.25/$0.30/$0.35/$0.40 за вызов. 4K: $0.75/$0.80/$0.85/$0.90. 360p: $0.15/$0.175/$0.20/$0.225.

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
- Создать fixture `internal/adapter/provider/apimart/testdata/contracts/gemini_omni_1_1_flash_ext.request.json`.
- Создать `internal/adapter/provider/apimart/model_gemini_omni_1_1_flash_ext_test.go` с именем теста `TestModel_gemini_omni_1_1_flash_ext`.

## Порядок реализации

- [ ] **1. Контракт.** В B0 проверить `gemini-omni-1.1-flash-ext`, записать разрешённые поля/операции и очищенный fixture указанного запроса. Для спорной версии сохранить предлагаемое полное название в UI и закрыть rollout до подтверждения выбора. Не выводить статус «подключено» по одному наличию в прайсе.
- [ ] **2. Адаптер.** Реализовать serializer и capability для `gemini-omni-1.1-flash-ext` по указанному JSON; локально проверять лимиты. Разобрать описанную форму результата. В тестовом HTTP server проверить exact endpoint/body; неподдерживаемое значение не должно вызывать HTTP.
- [ ] **3. Каталог и стоимость.** Добавить/обновить public ID `gemini_omni_1_1_flash_ext`, backend readiness и `FEATURE_APIMART_GEMINI_OMNI_1_1_FLASH_EXT_ENABLED` в config wiring. Создать отдельные ключи всех открываемых вариантов; определить exact rates и reserve/capture fixture. Сохранить model ID и тариф в Job snapshot.
- [ ] **4. Пользовательский поток.** Backend resolver разрешает только зарегистрированный public ID; UI показывает допустимые варианты и серверный quote. Job создаётся под владельцем с idempotency key, результат появляется в его истории. Ошибка модели не переключает на иной ID.
- [ ] **5. Проверки этой модели.** Реализовать перечисленные ниже positive/negative cases; общий lifecycle B2/B3 применяется к этому exact ID. Сначала получить ожидаемое падение нового contract test, затем реализовать маршрут и добиться PASS.
- [ ] **6. Выпуск.** Пройти preflight, тесты и ограниченный отдельно разрешённый canary именно этой конфигурации; сверить provider cost и ledger. До этого `FEATURE_APIMART_GEMINI_OMNI_1_1_FLASH_EXT_ENABLED` выключен. Отключение запрещает новые submit, но не мешает завершить старые Jobs.

## Приёмочные сценарии

- [ ] 5 с отклоняется; 4 с 1080p=$0.25 за весь вызов.
- [ ] 2 изображения отклоняются; video_urls+duration отклоняется.
- [ ] `FEATURE_APIMART_GEMINI_OMNI_1_1_FLASH_EXT_ENABLED=false` → модель недоступна для нового Job; ключ/остаток провайдера не раскрываются клиенту.
- [ ] Повторный Job с тем же idempotency key не создаёт второй платный запрос; result повторно не списывается.
- [ ] Невалидный ответ, provider failure и блокировка модерацией дают контролируемый статус без выдачи непроверенного результата.

## Следующая стадия этой модели

- [ ] Стадия 2: 360p и 4K с отдельными тарифами; затем 3-image reference. Video reference без duration открыть только после подтверждения правил VIDREF и верхней границы длины.

## Команды проверки при реализации

```powershell
go test ./internal/adapter/provider/apimart -run '^TestModel_gemini_omni_1_1_flash_ext$' -count=1
go test ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/worker
```

Для изменённого resolver добавить его package test; при изменении UI выполнить typecheck/lint/test из [общей основы](./00-common-foundation.md). Эти команды предназначены для будущей реализации и не являются заявлением о пройденных сейчас тестах.

**Готовность:** После общей видеоосновы и прохождения тестов этой конфигурации. При незакрытом условии соответствия/стоимости план подготовлен, но модель не готова к включению.
