# APIMart metadata preflight (R0/B0)

Статус 08.09.2026: локальная утилита и тесты реализованы. Реальные model IDs,
рабочая группа ключа и provider cost в этой задаче не проверялись.

## Назначение и запуск

Из корня репозитория, с установленными PowerShell и Go из проекта:

```powershell
pwsh -NoProfile -File scripts/providers/apimart-preflight.ps1 -ModelId 'gemini-3-pro-image-preview,gpt-image-2'
```

Это пример выбора, а не зафиксированная успешная внешняя проверка. Список
`-ModelId` обязателен, допускает массив PowerShell или строку с ID через запятую;
не более 64 уникальных ID. Имена сравниваются точно, включая регистр.

Ключ читается только из уже подготовленного окружения процесса
`APIMART_API_KEY`. Скрипт не загружает `.env`/`dev.env`, не печатает значения
переменных и не меняет их. `APIMART_BASE_URL` может отсутствовать либо содержать
`https://api.apimart.ai/v1` (завершающий `/` допустим). Другой host, HTTP, userinfo
или query отклоняются до отправки ключа. Redirects не выполняются.

Единственные сетевые операции:

- `GET /v1/models?expand=category`, Authorization — только для этого официального origin.
- Для image/video: `GET /v1/models/{id}/schema`; для ID с `/` —
  `GET /v1/model-schema?model=...`.
- Для chat и явно указанных token-price моделей:
  `GET https://api.apimart.ai/api/pricing/model?model=...` **без Authorization**.

Генерация, polling генераций, balance/account endpoints, обновление каталога,
ledger и runtime flags отсутствуют. Таймаут каждого запроса — 20 секунд,
ответ ограничен 2 MiB; автоматических повторов нет.

## Локальные сведения о цене

У публичного token pricing API читается только `data.pricing.effective_rates`;
скидка повторно не применяется. Числа разбираются без float64. Если требуется
сверка с ранее проверенным тарифом, передать JSON-файл:

```powershell
pwsh -NoProfile -File scripts/providers/apimart-preflight.ps1 -ModelId 'gpt-image-2' -ExpectedPricingPath '.runtime/apimart-price-reference.json'
```

Формат файла: объект `models` с массивом записей. Каждая запись содержит:

| Поле | Смысл |
| --- | --- |
| `model_id` | Exact model ID |
| `unit` | `usd_per_image`, `usd_per_second`, `usd_per_call` или `usd_per_million_tokens` |
| `group` | Название тарифной группы без идентификаторов аккаунта |
| `source_url` | Публичный прайс APIMart либо exact public model pricing URL; без приватных query-параметров |
| `checked_at` | Дата проверки источника `YYYY-MM-DD`; будущая дата запрещена |
| `rates` | Числовые ставки по вариантам или token dimensions; отсутствие ставки не означает ноль |

Полный **искусственный** пример находится в
[expected-pricing.json](../../internal/adapter/provider/apimart/testdata/contracts/preflight/expected-pricing.json).
Его ID и значения предназначены для тестов; не использовать их как проверенные
тарифы реальных моделей. В рабочих reference-файлах нельзя сохранять ключи,
account IDs, баланс, prompts или provider payloads.

Для token-модели проверяются точное совпадение единицы, группы и набора ставок с
публичным API. Для fixed-price модели локальный файл фиксирует только ссылку,
дату, единицу и числовую ставку; утилита **не подтверждает** содержимое прайса
или его применение к ключу. Без reference-файла fixed pricing имеет статус
`fixed_price_source_required`. Реальные справочные ставки не добавлены в рамках
этой реализации.

## Интерпретация отчёта

Stdout содержит очищенный JSON; его можно сохранить в отдельный файл отчёта.
В нём только выбранные ID/category, projection schema и ценовые сведения.
Данные других моделей, ownership, account/balance, raw errors, примеры и
описания схем не экспортируются. Text/prompt const/enum также не экспортируются.

- `checks_passed`: автоматические проверки доступных metadata/reference прошли;
  CLI завершается с кодом 0.
- `incomplete`: отсутствует fixed-price source, есть только общая `base` schema
  либо отсутствуют пригодные сведения об idempotency/response contract.
- `blocked`: отсутствует ID/ключ, schema неверна или недоступна, тариф/единица
  не совпадает, HTTP/JSON/transport ошибочен. Данные из ошибки не выводятся.

В обоих последних случаях PowerShell wrapper возвращает ненулевой код.
Прямая Go-утилита использует 1 для blocked и 2 для incomplete; `go run`
нормализует оба ненулевых результата в 1.

**`b0_complete` всегда false:** public pricing не подтверждает тариф группы
рабочего ключа или фактическое списание. `manual_checks` отдельно сохраняет эти
условия, а для chat/audio — необходимость контрактной проверки без model schema.
`checks_passed` не разрешает включение модели. Полный B0 закрывается после
отдельной проверки выбранной конфигурации и источников цены.

Schema projection сохраняет operation/method/endpoint/schema_version,
properties, required/enum, числовые границы, items и anyOf/oneOf/allOf.
Принимаются документированные корневой контракт, `data` и `parameters`.
Контракт должен быть связан с выбранной моделью через id/model или model
const/enum. Сейчас image/video routes проверяются по
`image_generation`/`video_generation` и `/v1/images/generations`/
`/v1/videos/generations`; другой протокол требует отдельной сверки, а не
молчаливого признания совместимости. Idempotency/response contract выводятся
только через список безопасных полей. Projection не заменяет полную API/schema
документацию и окончательную серверную проверку.

TokenPricingV2 с `tier_count > 1` либо неизвестными единицами/измерениями
завершается ошибкой; утилита не сводит многоуровневую цену к одной ставке.
Missing optional rate не дополняется нулём; явный ноль сохраняется.

## Локальные проверки

```powershell
go test ./scripts/providers/preflight -count=1
go vet ./scripts/providers/preflight
pwsh -NoProfile -File scripts/providers/tests/apimart-preflight.Tests.ps1
```

Go-тесты исполняют настоящий reader на искусственном HTTP transport. Тесты
PowerShell запускают реальные дочерние CLI с искусственным ключом и запрещённым
URL, поэтому не обращаются к провайдеру. Fixtures расположены в
`internal/adapter/provider/apimart/testdata/contracts/preflight/` и не содержат
реальных ответов аккаунта. Утилита не подключена к API/worker startup или deploy.

Источники API: [metadata/schema](https://docs.apimart.ai/ru/api-reference/texts/models/list),
[public pricing](https://docs.apimart.ai/ru/api-reference/texts/qwen3.8-max/pricing).
Контракты прочитаны 08.09.2026 из официальных Markdown-страниц; Context7 не вернул
нужных примеров metadata/pricing. Live-ответы здесь не заявляются проверенными.
