# 43. Grok Imagine 1.5 — интеграция

**Статус 08.09.2026:** реализовано локально по запросу пользователя на существующей основе Qwen. Пользователь разрешил DEV deploy; выпуск выполняется через GitHub Actions. Live-генерация ещё не проверена. Общие B0–B3 отложены; их завершение не заявляется.

## Контракт и цена

- [Документация](https://docs.apimart.ai/ru/api-reference/images/grok-imagine/generation) и [прайс](https://apimart.ai/ru/pricing), проверены 08.09.2026.
- Public ID: `grok_image_1_5`; UI: **Grok Imagine 1.5**.
- Канонический ID: `grok-imagine-1.5-apimart`; `grok-imagine-1.5-ext` — alias. Старое предположение плана об основном ID заменено.
- Флаг: `FEATURE_APIMART_GROK_IMAGE_1_5_ENABLED`, application default `false`.
- `POST /v1/images/generations`, HTTP 200, `data[0].task_id`; затем `GET /v1/tasks/{id}`.
- Форматы: `1:1`, `16:9`, `9:16`, `3:2`, `2:3`.
- Первый выпуск: `n=1`, не более **одного** референса. Старый лимит «5» исправлен по текущей документации.
- Публичное качество `standard` — единственный ключ тарифа. Поле `resolution` провайдеру не передаётся, выбора 1K/2K в UI нет.

Fixture: `internal/adapter/provider/apimart/testdata/contracts/grok_image_1_5.request.json`.

Статический каталог версии **6**: public floor $0.015 за изображение = 0.15 APIMart credits = `150000 apimart_credit_micros`; пользовательская цена и cap **10 внутренних кредитов**. Backend определяет quote и сохраняет тариф в Job snapshot; существующие задания сохраняют свои тарифы.

Версия Study24 остаётся нераскрытой. Интегрирована выбранная пользователем версия APIMart 1.5; тождество внутреннему маршруту Study24 не утверждается.

## Реализация и проверки

Существующий resolver передаёт доверенные параметры worker. Worker отправляет запрос, опрашивает задачу, сохраняет Artifact и проводит модерацию до выдачи в историю владельца. Reserve/capture/release остаются ledger-based. Provider/model code, ключ и внутренний тариф не выдаются клиенту.

Backend поддерживает принадлежащий аккаунту reference Artifact. Загрузка референсов в Web не добавлена: текущая Web-форма генерирует из текста. UI получает разрешённые форматы из серверного каталога и выбирает допустимый формат при смене модели.

- [x] Сверены код, документация и маршрут; добавлен очищенный fixture.
- [x] Capability, serializer, валидация полей и существующий async polling.
- [x] Registry, config/readiness, отдельный флаг, public catalog, backend resolver и тариф.
- [x] Web catalog/prepare, допустимые форматы, цена 10.
- [x] VK bot: меню всех шести моделей укладывается в лимит клавиатуры; выбор Grok создаёт один Job с резервом 10 кредитов и без `resolution`. Добавлен тест повторного входящего события. Живая генерация в ВК остаётся непроверенной.
- [x] Два референса, batch > 1, 1K/2K, недопустимые форматы и поля отклоняются до HTTP.
- [x] Невалидный ответ не принимается; повторный Job не отправляется и не списывается повторно.
- [x] Worker lifecycle: модерация, хранение, owner check, capture и возврат резерва при блокировке.
- [x] DEV renderer включает модель при настроенном APIMart и ключе, сохраняет явное false.
- [x] `go test ./...`, `go vet ./...`, `gofmt`, DEV env script tests.

Основные файлы: `internal/adapter/provider/apimart/grok_image.go`, `model_grok_image_test.go`, `internal/service/productcatalog/grok_image_test.go`, `internal/adapter/inbound/websession/grok_image_test.go`, `internal/worker/qwen_image_test.go`, registry/config/pricing/resolver и Web image-generation.

Целевые тесты: `TestModel_grok_image_1_5`, `TestGrokImageCatalogAndPricing`, `TestGrokImageReadiness`, `TestGrokImageFlagsAndRequiredConfig`, `TestWebGrokImageCatalogAndPrepare`, `TestGrokImageAsyncLifecycle`.

- [x] Web: lint, typecheck, полный npm test (742 теста + 4 asset tests), дополнительный тест смены формата; production build. Первая сборка не смогла загрузить Google Fonts в sandbox, повтор с доступом к сети прошёл.
- [x] Документация и git diff --check. Проверка docker compose config выполнена на пустом тестовом env.

Настроенный golangci-lint отсутствует в окружении; вместо него выполнен go vet. Браузерный smoke и сборка Docker-образа отдельно не выполнялись.

## Осталось

- [x] Подготовлен выпуск через dev-deploy по разрешению пользователя. Итог выкладки и smoke проверяется в GitHub Actions.
- [ ] Реальная генерация рабочим ключом, результат и фактическое списание.
- [ ] Персональная группа/себестоимость провайдера.

При локальной реализации платные вызовы и изменение действующих env/DB-тарифов не выполнялись. Последующая выкладка разрешена пользователем; её результат подтверждает workflow Deploy DEV. Окно падения процесса после HTTP до сохранения task ID остаётся общей задачей B2; in-memory replay не доказывает durable submit intent.

[Реестр](README.md) · [Общая основа](00-common-foundation.md) · [DEV](../../../runbooks/DEV.md)
