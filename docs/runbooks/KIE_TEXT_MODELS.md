# Текстовые модели через KIE и APIMart

Дата: 2026-09-09. Статус: реализовано локально, по умолчанию выключено.
Каталог содержит 11 платных моделей и прежний чат. Платные запросы, миграция
на рабочей БД и деплой не выполнялись. Новых Markdown-планов моделей нет.

## Провайдеры и тарифы

Цены проверены в открытых пользователем кабинетах [KIE](https://kie.ai/pricing)
и [APIMart](https://apimart.ai/ru/pricing). USD за 1 млн обычных входных /
выходных токенов, без tools и скидок за кэш. Для десяти выбранных маршрутов
опубликованные тарифы KIE ниже APIMart. Fable 5.1 не найдена в проверенном
каталоге KIE; для неё используется APIMart. Это выбор среди проверенных
предложений, а не гарантия минимальной цены на всём рынке.

| Модель | Провайдер | Вход / выход, USD | Кредитов за ограниченный ответ |
| --- | --- | --- | --- |
| GPT-5.5 | KIE | 1.40 / 8.40 | 20 |
| Claude Opus 4.7 | KIE | 1.425 / 7.150 | 20 |
| Gemini 3.1 Pro | KIE | 0.50 / 3.50 | 10 |
| Claude Opus 4.8 | KIE | 2 / 10 | 25 |
| GPT 5.6 Terra | KIE | 0.56 / 3.36 | 10 |
| GPT 6 Astra | KIE | 2.8 / 14 | 35 |
| Claude Opus 5 | KIE | 2 / 10 | 25 |
| Gemini 3.7 Flash | KIE | 0.225 / 1.125 | 5 |
| Claude Fable 5.1 | APIMart | 8 / 40 | 90 |
| Claude Fable 5 | KIE | 4 / 20 | 45 |
| Gemini 3.6 Flash | KIE | 0.225 / 1.125 | 5 |

Цена фиксирована за ответ, а не пересчитывается по фактическому usage.
Расчёт: максимум 8192 входных + 2048 выходных токенов, себестоимость ×3,
один внутренний кредит = $0.005, округление вверх до 5 кредитов.
Кредиты провайдеров имеют другую единицу: KIE $0.005, APIMart $0.1.

Вход включает историю, системное сообщение и backend facts. Рабочий запрос
ограничен 7680 байтами UTF-8 для prompt + facts; 512 токенов зарезервированы
для системного сообщения и оформления ролей. Длинный контекст отклоняется
до HTTP, без обрезания запроса пользователя. Старый чат и генерация заголовков
продолжают использовать прежний маршрут.

## Точные контракты

KIE base URL: `https://api.kie.ai`. APIMart base URL: `https://api.apimart.ai/v1`.
Endpoint в таблице добавляется к соответствующему base URL.

| Public ID | Upstream model | POST endpoint | Документация |
| --- | --- | --- | --- |
| `gpt_5_5` | `gpt-5-5` | `/codex/v1/responses` | [KIE](https://docs.kie.ai/cn/market/chat/gpt-5-5) |
| `claude_opus_4_7` | `claude-opus-4-7` | `/claude/v1/messages` | [KIE](https://docs.kie.ai/market/claude/claude-opus-4-7) |
| `gemini_3_1_pro` | `gemini-3.1-pro` | `/gemini-3.1-pro/v1/chat/completions` | [KIE](https://docs.kie.ai/cn/market/gemini/gemini-3-1-pro) |
| `claude_opus_4_8` | `claude-opus-4-8` | `/claude/v1/messages` | [KIE](https://docs.kie.ai/market/claude/claude-opus-4-8) |
| `gpt_5_6_terra` | `gpt-5-6-terra` | `/codex/v1/responses` | [KIE](https://docs.kie.ai/cn/market/chat/gpt-5-6-terra) |
| `gpt_6_astra` | `gpt-6-astra` | `/codex/v1/responses` | [KIE](https://docs.kie.ai/cn/market/chat/gpt-6-astra) |
| `claude_opus_5` | `claude-opus-5` | `/claude/v1/messages` | [KIE](https://docs.kie.ai/market/claude/claude-opus-5) |
| `gemini_3_7_flash` | `gemini-3-7-flash-openai` | `/gemini-3-7-flash-openai/v1/chat/completions` | [KIE](https://docs.kie.ai/cn/market/gemini/gemini-3-7-flash-openai) |
| `claude_fable_5_1` | `claude-fable-5.1` | `/chat/completions` | [APIMart](https://docs.apimart.ai/ru/api-reference/texts/general/chat-completions) |
| `claude_fable_5` | `claude-fable-5` | `/claude/v1/messages` | [KIE](https://docs.kie.ai/market/claude/cluade-fable-5) |
| `gemini_3_6_flash` | `gemini-3-6-flash-openai` | `/gemini-3-6-flash-openai/v1/chat/completions` | [KIE](https://docs.kie.ai/40573330e0) |

Написание `cluade` в ссылке Fable 5 соответствует текущему URL документации.
Название Terra на странице цен отличается пунктуацией от API model ID.
Версии Gemini не подменяются друг другом; для 3.6 и 3.7 выбраны отдельные
OpenAI-совместимые endpoints, а не native streamGenerateContent.
KIE Gemini 3.1 Pro и APIMart Gemini 3.1 Pro Preview имеют разные IDs;
тождество внутреннего revision не подтверждено.

Все запросы: Bearer auth, `stream:false`, одно текстовое сообщение пользователя,
отдельный доверенный контекст. Без вложений, поиска, tools и публичного streaming.
Из Responses берутся только assistant/output_text, из Messages только text,
из Chat Completions только единственный assistant message. Reasoning не публикуется.
Gemini получает `messages[].content` как массив блоков `{type:"text", text:...}`
для обеих ролей и явный `include_thoughts:false`. APIMart получает строковый
content ролей system/user. Redirect запрещён. Ошибки не содержат raw response,
prompt, URL с ключами или заголовок Authorization.

**До включения:** в проверенных схемах KIE не подтверждены `max_output_tokens`
для GPT и `max_tokens` для Gemini, включая ограничение оплачиваемого reasoning.
Адаптер отправляет эти поля как кандидаты контракта. `include_thoughts:false`
управляет возвратом рассуждений, но не доказывает ограничение их стоимости.
У Claude документирован `max_tokens`, но top-level `system` не указан в схеме.
У APIMart `max_tokens` описан общим контрактом Chat Completions; точный model ID
Fable 5.1 найден в каталоге цен. Его лимиты и usage требуют проверки на этой модели.
Пустой ответ, отсутствующий/превышенный usage и неизвестный исход завершаются
ошибкой без повторного платного submit. Отказ после превышения usage сам по себе
не предотвращает уже понесённые расходы: эффективность лимитов проверяется до rollout.

## Реализация и восстановление

- `internal/adapter/provider/textapi/`: общий ограниченный HTTP-вызов и четыре
  формата запроса/ответа. `kie/` задаёт KIE; `apimart/` делегирует только текст.
  Синхронные ответы не сохраняются в памяти APIMart media idempotency cache.
- `providermodels`, `modelcatalog`, `productcatalog`, `textgeneration`:
  отдельные public IDs, точные маршруты, readiness и серверный resolver.
- `pricingcatalog/text.go`: тарифы версии 11. Неизменяемые caps и стоимость
  сохраняются в Job snapshot; migration `000052` добавляет TextModelID в DB pricing.
- `config`, `cmd/worker`: регистрация явно включённых моделей. Платные модели
  не участвуют в выборе провайдера для default chat.
- `joborchestrator/paid_text.go`, `worker/paid_text.go`,
  `worker/apimart_image_submission.go`: проверка цены/резерва и восстановление.
- Web и Mini App: серверный каталог, выбор модели, цена перед отправкой,
  сохранение выбора при повторе. Provider IDs и цены от клиента не принимаются.
  Общие Mini App `/jobs` и `/estimate` используют ту же проверенную цену.

Worker создаёт уникальную заявку `kie_text_submit:<job_id>` или
`apimart_text_submit:<job_id>` в `provider_tasks` до HTTP. Единственный успешный
создатель выполняет платный вызов. Текст сохраняется как приватный Artifact
и связывается с Job до terminal checkpoint. После сбоя worker использует Artifact,
повторно проверяет moderation и завершает Job без нового запроса к провайдеру.
Если HTTP мог быть принят, но Artifact не сохранён, после bounded timeout
резерв освобождается; автоматического нового submit нет. Возможная оплата
провайдеру за потерянный ответ остаётся расходом платформы и требует сверки.
Capture выполняет существующий account-history delivery ровно один раз.

## Включение

1. Подтвердить недостающие ограничения из раздела контрактов. Проверить каждый
   включаемый маршрут: маленький output cap, русский prompt, usage с reasoning,
   system/facts, ошибки, отсутствие повторного submit. Локальные HTTP fixtures
   не заменяют контрольную генерацию на оплачиваемом аккаунте.
2. Выполнить migration `000052` обычным deploy-процессом, если её ещё нет.
   При DB pricing добавить нужные цены в новую проверенную версию каталога;
   не редактировать исторические Job. Static pricing уже содержит все 11 цен.
   Без цены модель не попадает в публичный список.
3. KIE: `KIE_API_KEY`, `KIE_BASE_URL=https://api.kie.ai`, `KIE_PROVIDER_ENABLED=true`.
   После проверки лимитов включаемых маршрутов: `KIE_TEXT_LIMITS_VERIFIED=true`.
4. APIMart Fable 5.1: `APIMART_API_KEY`, `APIMART_BASE_URL=https://api.apimart.ai/v1`,
   `APIMART_PROVIDER_ENABLED=true`, после проверки `APIMART_TEXT_LIMITS_VERIFIED=true`.
   Верификация текста APIMart независима от KIE и флагов media моделей.
5. Включить только нужные индивидуальные флаги из таблицы ниже. Все новые bool
   по умолчанию false. `PROVIDER_CHAIN` менять не требуется: отдельные text routes
   регистрируются после штатной цепочки. Запуск с выбранной моделью без ключа
   или верификации отклоняется. Ключи задаются только в серверном окружении.
6. Проверить authenticated `GET /web/v1/text-models` и `/miniapp/text-models`,
   затем одну генерацию, Artifact moderation и одну capture-запись.
   API/worker используют одинаковую конфигурацию и каталог цен.

| Модель | Флаг |
| --- | --- |
| GPT-5.5 | `FEATURE_TEXT_GPT_5_5_ENABLED` |
| Claude Opus 4.7 | `FEATURE_TEXT_CLAUDE_OPUS_4_7_ENABLED` |
| Gemini 3.1 Pro | `FEATURE_TEXT_GEMINI_3_1_PRO_ENABLED` |
| Claude Opus 4.8 | `FEATURE_TEXT_CLAUDE_OPUS_4_8_ENABLED` |
| GPT 5.6 Terra | `FEATURE_TEXT_GPT_5_6_TERRA_ENABLED` |
| GPT 6 Astra | `FEATURE_TEXT_GPT_6_ASTRA_ENABLED` |
| Claude Opus 5 | `FEATURE_TEXT_CLAUDE_OPUS_5_ENABLED` |
| Gemini 3.7 Flash | `FEATURE_TEXT_GEMINI_3_7_FLASH_ENABLED` |
| Claude Fable 5.1 | `FEATURE_TEXT_CLAUDE_FABLE_5_1_ENABLED` |
| Claude Fable 5 | `FEATURE_TEXT_CLAUDE_FABLE_5_ENABLED` |
| Gemini 3.6 Flash | `FEATURE_TEXT_GEMINI_3_6_FLASH_ENABLED` |

Для rollback выключить индивидуальные флаги, вернуть предыдущий runtime image.
Аддитивную схему оставить: down отказывается выполняться при наличии текстовых цен.
Уже сохранённые тексты могут быть завершены без обращения к провайдеру.

Для runtime pricing использовать `floor_unit=usd_micros`, `quality=''`,
`resolution=''`, `duration_sec=0`, остальные model/route dimensions пустые.
Точные floors рассчитывает `pricingcatalog/text.go` целочисленно с округлением
вверх до одного USD micro. Формула snapshot multiplier:
numerator = retail × 1000000, denominator = floor × 200;
caps равны retail и floor соответственно. Она сохраняет округлённый итог ×3
до пяти кредитов без float math.

## Проверки

Есть локальные проверки точных API-контрактов и тарифов, независимых readiness
флагов, недопустимых public IDs, подмены provider/цены клиентом, сохранения резерва
при повторе Mini App и Web-маршрутов всех моделей. Для всех 11 моделей тестируется
потеря checkpoint после ответа, восстановление Artifact без второго запроса
и однократный capture. Проверяется отказ от redirects и автоматического retry POST.

Локальные проверки расширения: `go test` по 16 затронутым пакетам и `go vet`
проходят; Web — 136 тестов переписки, lint, typecheck и production build;
Mini App — 36 тестов, lint, typecheck и production build. `git diff --check`
проходит. Сборка Web требует доступа к Google Fonts.
Дополнительный reviewer не запустился: модель `gpt-5.5`, требуемая AGENTS.md
для subagent, недоступна. Проведена самостоятельная проверка маршрутизации,
резервов, модерации, восстановления и публичных DTO.

Реальный оплачиваемый API, PostgreSQL migration и визуальный smoke с включёнными
платными моделями остаются непроверенными. PostgreSQL integration tests требуют
отдельной тестовой БД (`TEST_DATABASE_URL`). Коммит, push и deploy не выполнялись.
