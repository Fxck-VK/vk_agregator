# 44. Grok Imagine 2.0 Ext — интеграция

**Статус 08.09.2026:** реализовано локально по запросу пользователя на существующей основе Qwen. Пользователь разрешил DEV deploy; выпуск выполняется через GitHub Actions. Live-генерация ещё не проверена. Пользователь выбрал прайс $0.015 → 10 кредитов. Общие B0–B3 отложены.

## Контракт

- [Документация](https://docs.apimart.ai/ru/api-reference/images/grok-imagine-2.0-ext/generation) и [прайс](https://apimart.ai/ru/pricing), проверены 08.09.2026.
- Public ID: `grok_image_2_0`; UI: **Grok Imagine 2.0**; provider ID: `grok-imagine-2.0-ext`.
- Флаг: `FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED`, application default `false`.
- Только text-to-image, первый выпуск `n=1`, `resolution=quality`, `response_format=url`.
- `POST /v1/images/generations` с `X-APIMart-Response-Version: 2026-07-27`, `Accept: application/json`, стабильным `Idempotency-Key`.
- Submit HTTP 202, `data.id`; затем `GET /v1/tasks/{id}`. URL результата разбираются в форме строки и массива.
- UI: `1:1`, `16:9`, `9:16`, `2:3`, `3:2`, `3:4`, `4:3`. Adapter также принимает документированные пиксельные alias, не выводит из них качество.
- Публичное качество `standard` — фиксированный ключ цены; UI не обещает 1K/2K. Provider `quality`, `style`, refs, `stream=true`, base64 output и batch > 1 отвергаются.

Fixture: `internal/adapter/provider/apimart/testdata/contracts/grok_image_2_0.request.json`. Layer/region-edit и другие Grok provider IDs не входят в выпуск.

## Источник тарифа

Прайс APIMart показывает **$0.015**, страница API — **$0.08** за изображение. Пользователь 08.09.2026 явно выбрал **прайс $0.015 → 10 внутренних кредитов**. Это утверждённый выбор источника, а не подтверждение фактического списания рабочей группы.

Статический каталог версии **6**: `150000 apimart_credit_micros` = 0.15 APIMart credits; retail/cap **10**. Quote, резерв и capture определяются серверным snapshot. Действующие DB-тарифы автоматически не переписываются. Другой фактический provider cost не увеличивает принятый пользователем резерв.

## Безопасная отправка и обработка

Serializer отправляет только разрешённые поля. При `409 idempotency_in_progress`, сетевом сбое, 429 или 5xx adapter делает до трёх попыток с одинаковыми body/key/version в пределах 30 секунд; перед повтором соблюдает `Retry-After` (при отсутствии — 1 секунда). Если ожидание не помещается в deadline, новая попытка не начинается.

`idempotency_result_indeterminate`, неизвестный conflict, исчерпанные попытки или неоднозначный успешный ответ дают `provider_submit_indeterminate`. Worker завершает задание без автоматического нового платного intent и освобождает резерв. `idempotency_key_reused` отклоняется как invalid request.

Обычный успех проходит существующие async Job → Artifact → moderation → owner-checked history → ledger capture. Повторная обработка сохранённой задачи не создаёт повторное списание. Durable pre-submit intent и восстановление после падения процесса между HTTP и записью task ID остаются общей задачей B2.

## Выполнено локально

- [x] Документированный контракт, exact serializer/capability и fixture.
- [x] HTTP 202 / data.id, polling и нормализация результата.
- [x] Отдельный config flag, readiness, registry, public catalog и backend resolver.
- [x] Цена 10 по выбранному источнику, versioned Job snapshot, без автоподмены модели.
- [x] Референсы, неподдерживаемые поля/форматы, 1K/2K и batch > 1 отклоняются до HTTP.
- [x] 409 replay после Retry-After, одинаковые body/key/version, остановка при indeterminate.
- [x] Web catalog/prepare: форматы и цена с сервера, приватные поля не выдаются.
- [x] Worker lifecycle: модерация, owner checks, хранение, однократный capture, возврат резерва при отказе/неопределённом submit.
- [x] DEV renderer включает модель при настроенном APIMart и ключе, сохраняет явное false.
- [x] `go test ./...`, `go vet ./...`, `gofmt`, DEV env script tests.

Основные файлы: `internal/adapter/provider/apimart/grok_image.go`, `model_grok_image_test.go`, `internal/domain/provider.go`, `internal/worker/worker.go`, `internal/worker/qwen_image_test.go`, registry/config/pricing/resolver и Web image-generation.

Целевые тесты: `TestModel_grok_image_2_0`, `TestGrok20SafeSubmitReplay`, `TestGrokImageCatalogAndPricing`, `TestGrokImageReadiness`, `TestWebGrokImageCatalogAndPrepare`, `TestGrokImageAsyncLifecycle`.

- [x] Web: lint, typecheck, полный npm test (742 теста + 4 asset tests), дополнительный тест смены формата; production build. Первая сборка не смогла загрузить Google Fonts в sandbox, повтор с доступом к сети прошёл.
- [x] Документация и git diff --check. Проверка docker compose config выполнена на пустом тестовом env.

Настроенный golangci-lint отсутствует в окружении; вместо него выполнен go vet. Браузерный smoke и сборка Docker-образа отдельно не выполнялись.

## Осталось

- [x] Подготовлен выпуск через dev-deploy по разрешению пользователя. Итог выкладки и smoke проверяется в GitHub Actions.
- [ ] Реальная генерация рабочим ключом и проверка результата.
- [ ] Фактическая себестоимость, выбранный прайс и пользовательский ledger.
- [ ] Следующая отдельная задача: layer/region-edit, если будет запрошена.

При локальной реализации платные вызовы и изменение действующих env/DB-тарифов не выполнялись. Последующая выкладка разрешена пользователем; её результат подтверждает workflow Deploy DEV.

[Реестр](README.md) · [Общая основа](00-common-foundation.md) · [DEV](../../../runbooks/DEV.md)
