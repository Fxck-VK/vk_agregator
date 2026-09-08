# 68. Suno — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** подключить Suno через выбранный маршрут APIMart.

**Architecture:** backend resolver → доверенный Job snapshot → worker → APIMart → Artifact → moderation → delivery.
**Tech Stack:** Go/PostgreSQL/Redis + Web UI.
**Spec:** [полный реестр](./README.md), [общая основа](./00-common-foundation.md). Статус: план, не реализовано в рамках этого задания.

## Global Constraints

Выполнять после B0–B3 и раздела A общей основы. Для этой модели отдельный флаг; разрешение на её реализацию не означает включение остальных. Provider key, prompt, raw payload и private URLs не записываются в отчёт/fixtures. Старые Jobs используют собственные snapshots. Платные проверки, commit/push/deploy не выполняются при подготовке плана.

## Документация и точное соответствие

- [Карточка Study24](https://study24.ai/chat/suno)
- [Документация suno/generation](https://docs.apimart.ai/ru/api-reference/audios/suno/generation)
- [Документация suno/overview](https://docs.apimart.ai/ru/api-reference/audios/suno/overview)
- [Актуальный прайс APIMart](https://apimart.ai/ru/pricing), проверен 08.09.2026.
- [Доступность ID и schema для конкретного ключа](https://docs.apimart.ai/ru/api-reference/texts/models/list).

| Поле | Значение |
| --- | --- |
| Название Study24 | Suno |
| Предлагаемый public ID | `suno_v5_5` |
| APIMart model ID | `suno` |
| Метод | `POST https://api.apimart.ai/v1/music/generations` |
| Флаг | `FEATURE_APIMART_SUNO_V5_5_ENABLED` |
| Соответствие | Условное: версия/режим уточнены в этом плане; тождество Study24 не доказано |

В Study24 точная версия не раскрыта. v5.5 — предлагаемый явный старт. TTS не использовать вместо операции генерации музыки.

Перед включением проверить наличие exact ID в доступном ключу каталоге и цену выбранного варианта.

## Контракт первого выпуска

version обязателен. custom=false: prompt — описание, title/style игнорируются; custom=true: prompt — текст песни, style — жанр. Сохранить все data.result.music[] как отдельные аудиоартефакты.

Пример тела запроса — основа fixture, не команда на выполнение. URL example.com в референсах обозначают тестовые входы; рабочие ссылки формирует worker из принадлежащих аккаунту артефактов.

```json
{
  "model": "suno",
  "version": "v5.5",
  "custom": false,
  "instrumental": true,
  "prompt": "Спокойная инструментальная композиция с пианино."
}
```

Асинхронный submit даёт task_id; poll /v1/music/tasks/{task_id}; все data.result.music[] сохранить как audio Artifacts. Цена относится к одному вызову, а не числу треков.

## Цена

$0.05 за вызов music-v5.5. Оплата за вызов, не за каждый возвращённый трек; обычно возвращаются две версии, число проверять по факту.

Это себестоимость APIMart на дату проверки, не утверждённая пользовательская цена. Перед включением зафиксировать точный тариф/единицу/группу в версии price catalog. Quote и резерв определяет backend; итог не превышает принятый резерв. Не полагаться на значения по умолчанию провайдера, если они меняют цену.

## Файлы реализации

Пути от корня репозитория; новые общие файлы создаются по плану основы, существующие изменяются.

- `internal/domain/job.go`
- `internal/adapter/provider/apimart/music.go`
- `internal/adapter/provider/apimart/music_test.go`
- `internal/service/providermodels/registry.go`
- `internal/service/audiogeneration/resolver.go`
- `internal/service/pricingcatalog/apimart_rates.go`
- `internal/worker/generation.go`
- `internal/worker/poll.go`
- `web/platform/src/features/audio-generation/AudioGenerationPanel.tsx`
- `internal/platform/config/config.go`
- Создать fixture `internal/adapter/provider/apimart/testdata/contracts/suno_v5_5.request.json`.
- Создать `internal/adapter/provider/apimart/model_suno_v5_5_test.go` с именем теста `TestModel_suno_v5_5`.

## Порядок реализации

- [ ] **1. Контракт.** В B0 проверить `suno`, записать разрешённые поля/операции и очищенный fixture указанного запроса. Для спорной версии сохранить предлагаемое полное название в UI и закрыть rollout до подтверждения выбора. Не выводить статус «подключено» по одному наличию в прайсе.
- [ ] **2. Адаптер.** Реализовать serializer и capability для `suno` по указанному JSON; локально проверять лимиты. Разобрать описанную форму результата. В тестовом HTTP server проверить exact endpoint/body; неподдерживаемое значение не должно вызывать HTTP.
- [ ] **3. Каталог и стоимость.** Добавить/обновить public ID `suno_v5_5`, backend readiness и `FEATURE_APIMART_SUNO_V5_5_ENABLED` в config wiring. Создать отдельные ключи всех открываемых вариантов; определить exact rates и reserve/capture fixture. Сохранить model ID и тариф в Job snapshot.
- [ ] **4. Пользовательский поток.** Backend resolver разрешает только зарегистрированный public ID; UI показывает допустимые варианты и серверный quote. Job создаётся под владельцем с idempotency key, результат появляется в его истории. Ошибка модели не переключает на иной ID.
- [ ] **5. Проверки этой модели.** Реализовать перечисленные ниже positive/negative cases; общий lifecycle B2/B3 применяется к этому exact ID. Сначала получить ожидаемое падение нового contract test, затем реализовать маршрут и добиться PASS.
- [ ] **6. Выпуск.** Пройти preflight, тесты и ограниченный отдельно разрешённый canary именно этой конфигурации; сверить provider cost и ledger. До этого `FEATURE_APIMART_SUNO_V5_5_ENABLED` выключен. Отключение запрещает новые submit, но не мешает завершить старые Jobs.

## Приёмочные сценарии

- [ ] Нет version → локальная ошибка, submit не вызывается.
- [ ] Два трека одного Job дают один capture на вызов и два результата.
- [ ] Poll идёт на /v1/music/tasks/{id}; все audio_url скачиваются, lyrics проходят модерацию.
- [ ] `FEATURE_APIMART_SUNO_V5_5_ENABLED=false` → модель недоступна для нового Job; ключ/остаток провайдера не раскрываются клиенту.
- [ ] Повторный Job с тем же idempotency key не создаёт второй платный запрос; result повторно не списывается.
- [ ] Невалидный ответ, provider failure и блокировка модерацией дают контролируемый статус без выдачи непроверенного результата.

## Следующая стадия этой модели

- [ ] Стадия 2: custom=true с lyrics/title/style и instrumental; валидировать поля по режиму. Extend/remaster/stems/voice не входят в исходную генерацию: для каждого нужна отдельная операция/цена и parent artifact ownership.

## Команды проверки при реализации

```powershell
go test ./internal/adapter/provider/apimart -run '^TestModel_suno_v5_5$' -count=1
go test ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/joborchestrator ./internal/worker
```

Для изменённого resolver добавить его package test; при изменении UI выполнить typecheck/lint/test из [общей основы](./00-common-foundation.md). Эти команды предназначены для будущей реализации и не являются заявлением о пройденных сейчас тестах.

**Готовность:** После новой операции audio_music_generate, аудио-хранилища/доставки и тарифа. При незакрытом условии соответствия/стоимости план подготовлен, но модель не готова к включению.
