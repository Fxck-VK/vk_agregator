# Передача коллеге: 103 модели, DEV-проверки и исправления видео

Status: active
Дата сверки: 7 октября 2026. Репозиторий: https://github.com/Fxck-VK/vk_agregator.

Сверено с Git, чатами «Проверь интеграцию моделей» и «Починить видео модели
на сайте», таблицей и безопасными метаданными DEV-заданий. Результаты живых
попыток и незакоммиченный runtime hotfix отделены от опубликованного кода.

## Ветка и состав передачи

Ветка передачи — `fastlife_dev`. Родитель коммита документа:
`f329e8f2d320468734baaec3e7b7904b6c173481`.
Перед подготовкой remote `fastlife_dev` была на `2d714d65`, remote
`dev-deploy` — на `f329e8f2`. Первая является предком второй; ветка передачи
синхронизирована fast-forward с опубликованным DEV-кодом.

| Диапазон / коммит | Что входит |
| --- | --- |
| `6dd08963..2d714d65` | Исправления контрактов моделей и preflight: 73 файла |
| `e28cc56b` | Модели, валидация, готовность worker, исправления зависимостей |
| `2d714d65` | Routing-ссылки и проверка документации до DEV push |
| `2d714d65..f329e8f2` | Уже опубликованные регистрация, вход, пароли и резервная почта; сохранены в передаче |
| Новый коммит передачи | Этот документ, архив прежней передачи, INDEX и routing state |

[Diff моделей](https://github.com/Fxck-VK/vk_agregator/compare/6dd08963...2d714d65).
[Последующие опубликованные изменения](https://github.com/Fxck-VK/vk_agregator/compare/2d714d65...f329e8f2).

Для новой ветки, предварительно сохранив свою незакоммиченную работу:

```sh
git fetch origin
git switch -c codex/merge-model-integration-20261007 origin/fastlife_dev
```

В своей существующей рабочей ветке после fetch выполнить `git merge
origin/fastlife_dev`. Не cherry-pick повторно уже включённые коммиты.
При конфликтах сохранить новые account/email изменения. Production/main
не входит в передачу; force push и копирование папки поверх чужой работы не нужны.

## Исправления моделей в опубликованном коде

Каталог: **103 записи — 46 text, 20 image, 31 video, 6 audio**.
Количество включает отключённые кандидаты и не означает 103 успешных live теста.
Ранний отчёт про 11 моделей относился к старой копии.

- Nano Banana 2/Pro и GPT Image 2: один результат; неверное количество
  отклоняется до HTTP. Недокументированный `n` у PoYo Nano Banana 2 удалён.
- GPT Image 2: 15 референсов, до 20 MiB каждый, 256 MiB суммарно. Реестр,
  адаптер, каталог, frontend и ранняя проверка owned artifacts согласованы.
- Nano Banana Pro: обработанный PNG/JPEG до 20 MiB, согласованно с web upload.
  Mini App использует лимит референсов выбранной модели вместо общего потолка 4.
- Unicode-границы промптов: Seedream 4.5 — до 3000, Runway Gen4.5 — до 1800,
  Seedance Fast — 3–2000. Проверка до Job, с объяснением в UI.
- Runway: разрешение выбирает провайдер; поле не отправляется, контрол скрыт
  в web/Mini App. Сохранён default тариф; другие варианты отклоняются до
  оценки/Job, старые queued запросы — адаптером до HTTP.
- Nano Banana 2: закрыты неподдерживаемые крайние ratios. Seedream adapter
  ограничивает output максимумом 15; UI сохраняет более узкий лимит.
- Pending Vidu metadata ограничены реализованными PNG/JPEG и 20 MiB.
- Enabled routes требуют провайдера в реальном наборе worker. Флаги, ключи,
  pricing и admission остаются обязательными.
- Исправлены зависимости/preflight: Go/build images 1.25.14,
  OpenTelemetry 1.45.0, compress 1.18.7, npm/lint dependencies.
  Правила Next lint сохранены и проверяются contract test.

Опорные файлы: `internal/adapter/provider/apimart/apimart.go`,
`internal/adapter/provider/poyo/poyo.go`, `internal/platform/config/config.go`,
`internal/service/providermodels/media_prompt_limits.go`,
`internal/service/productcatalog/workspace.go`,
`internal/adapter/inbound/websession/conversation_upload.go`,
`internal/adapter/inbound/websession/conversation_media.go`,
`web/platform/src/features/conversations/use-chat-attachments.ts`,
`web/platform/src/features/models/generation-options.tsx`.

Legacy admission не обновлялся ради обхода проверок. DEV smoke не повышает
production admission. Provider calls остались в worker; auth, ownership,
moderation, ledger и idempotency сохранены.

## Проверки и прежний деплой

Для `2d714d65` история чата/отчёт за 3 октября фиксируют: полные Go tests/vet,
onboarding/preview checks на 103 ID, 1624 platform-теста (4 пропуска окружения),
lint/typecheck/build/packaging/assets, 64 Mini App-теста, 7 Admin-тестов,
npm audits, symbol govulncheck и pinned Trivy HIGH/CRITICAL.
Это результаты той версии; документационный коммит не повторяет эти наборы.

- [CI 2d714d65](https://github.com/Fxck-VK/vk_agregator/actions/runs/37121554170) и
  [Docker Images](https://github.com/Fxck-VK/vk_agregator/actions/runs/37121554155):
  success; head SHA повторно проверен 7 октября.
- [Прежний Deploy DEV/smoke](https://github.com/Fxck-VK/vk_agregator/actions/runs/37121950222):
  success. Run имеет собственный head SHA; исходный SHA и image tag
  `sha-2d714d65dabeeb940ab9ccbbfd9c0b7e7d208542` проверены в логах предыдущего
  этапа. Run не доказывает текущий runtime DEV.
- [CI f329e8f2](https://github.com/Fxck-VK/vk_agregator/actions/runs/37253152172) и
  [Docker Images](https://github.com/Fxck-VK/vk_agregator/actions/runs/37253152178):
  success; head SHA проверен 7 октября.

Исходники перед исправлениями сохранены в backup вне Git, прежняя Mini App
правка — в patch/stash. Не применять их поверх итогового кода.
Посторонняя папка `output/` основной копии сохранена.

## Живые проверки 7 октября

Scope пользователя: одна попытка на модель, полная цена до 50 токенов,
без повторных платных запросов и проверки скачивания.
**48 новых UI-попыток: 35 результатов, 13 ошибок, списано 610 токенов.**
По всем 13 ошибкам списание 0. Пять попыток не создали Job: четыре
музыкальные подготовки и один запуск Vidu Q3 Pro.
У успешных результатов проверены отображение, история после обновления
и однократное ledger-списание; другие настройки/референсы этим не подтверждены.

| Модель | Результат конкретной попытки |
| --- | --- |
| Wan 3.0, Grok Imagine 1.5 Video, Seedance 2.0 Mini, Vidu Q3 Turbo | `media_provider_output_invalid`; резерв освобождён |
| Imagen 4.0 | `provider_internal_error` |
| GPT-5 Chat Latest, DeepSeek R1 250528 | `provider_model_unavailable` |
| DeepSeek V3 0324 | `invalid_request` |
| Vidu Q3 Pro | Ошибка UI; Job/списание в окне попытки отсутствуют, код ответа UI не зафиксирован |
| Suno V6 / Wild / Mini, Lyria 3.5 | HTTP 401 при prepare; активации/Job не было, native generation не проверена |

Музыкальный 401 вероятно связан с access session: последующее чтение
истории вернуло HTTP 200. Автоматического повторения записи не было.
Видеоошибки — факты запросов, а не доказанный диагноз. В другом чате исправлена
общая проблема видео, но эти четыре модели не получили успешного повторного
запуска; условие одной попытки остаётся в силе.

PixVerse V6: успешный **360p / 1 секунда за 10**; дефолт за 75 не запускался.
Vidu Q3 Turbo: один запуск **540p / 1 секунда за 20**, ошибка; дефолт за 145
не запускался. Цена карточки не равна полной цене конфигурации.
DEV default: одно активное видео на пользователя; пять вкладок не обходят лимит.

Изображения: 20 записей, 18 доступны, 16 успешных по пользовательским и агентским
отметкам. Nano Banana 2: более ранний серверный успех со списанием 50,
но preview не открывается и повторный просмотр 7 октября это подтвердил.
Эта попытка не входит в новые 48. Imagen 4.0 — ошибка провайдера.
Midjourney V7 и FLUX.2 Pro недоступны. Новых непроверенных доступных
image-моделей до 50 токенов нет.

`Проверка_103_моделей_DEV.xlsx` — локальный артефакт чата, передать отдельно:
в Git его нет. Сохранены пользовательские отметки, 103 ID, dropdown, фильтры,
формулы и лист «Сценарии». D — доступность, E — результат, F — история,
G — скачивание, H — списание, I — итог, J — ошибка, K — Job ID,
L — цена, M — списание, N — model ID.
48 новых строк и рассчитанные итоги независимо сверены с журналом.
Исходная таблица сохранена в backup, длинные комментарии читаются с переносами.

Отдельные операции DEV: разово выданы 5000 тестовых токенов через append-only
идемпотентный grant, одна ledger-запись проверена; настроен и проверен локальный
SSH-доступ. Это не Git-миграции. Не повторять начисление и не передавать ключи
или credentials с файлом мерджа.

## Незакоммиченный пакет видео: нужен отдельный перенос

Чат «Починить видео модели на сайте», worktree `c2ec/vk_agregator`,
detached HEAD `f329e8f2`: **18 файлов, 12 modified + 6 untracked**.
Они не входят в published SHA и не попадут к коллеге через fetch.
Исходная рабочая копия при подготовке передачи не менялась.

- `internal/service/mediaprobe/probe.go`, `probe_test.go`: ISO BMFF major brand
  определяет MP4; QuickTime `qt` остаётся MOV. MP4 больше не отклоняется как MOV.
- `scripts/deploy/check-dev-env.sh`, `prepare-dev-env.sh`, `test-dev-env.sh`,
  `docs/runbooks/DEV.md`: включение/проверка media pipeline и ffprobe в DEV.
- `internal/adapter/inbound/websession/handler.go`; новые `video_jobs.go`,
  `video_jobs_test.go`: owned video history/result routes, только завершённый
  и прошедший moderation результат.
- `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx` и тест;
  новые `VideoFiles/VideoFiles.tsx`, тест, CSS, `video-data.ts` в той же feature:
  видео на вкладках «Видео»/«Все», ограниченная параллельная загрузка превью.
- `docs/ARCHITECTURE.md`, `web/platform/docs/ui-catalog.md`,
  `web/platform/docs/ui-index.md`: документация пакета.

По истории другого чата, runtime DEV уже исправлен: видеообработка была
отключена, MP4 определялся как MOV. Без повторных генераций, списаний и дублей
восстановлены четыре сохранённых видео: Kling V3, Veo 3.1 Lite,
Gemini Omni EXT, Seedance 2 Fast. Проверены выдача через API и перемотка.
После восстановления диалогов исправлена история, ранее запрашивавшая только
изображения. Там также отмечены отключённый Seedance 2.5 и отдельный отказ
APIMart для MiniMax H3. Это не отменяет ошибки конкретных попыток выше.

Для воспроизводимого переноса сохранить все 18 файлов, включая untracked,
создать рабочую ветку от `f329e8f2` без reset/clean, проверить и закоммитить
пакет, затем интегрировать обычным merge/PR. Runtime hotfix не считать
входящим в push этого документа. Проверки пакета:

```sh
go test ./internal/service/mediaprobe ./internal/adapter/inbound/websession
go vet ./internal/service/mediaprobe ./internal/adapter/inbound/websession
bash scripts/deploy/test-dev-env.sh
```

Из `web/platform`: FilesWorkspace/VideoFiles tests, lint, typecheck,
production build/packaging. Ранний video fix прошёл проверки в другом чате;
окончательные 18 файлов здесь не прогонялись.

Для DEV нужны доступный ffprobe и настройки:
`MEDIA_PIPELINE_ENABLED=true`, `MEDIA_VIDEO_PROBE_POLICY=probe_required`,
`MEDIA_VIDEO_TRANSCODE_POLICY=never`,
`MEDIA_DELIVER_RAW_PROVIDER_VIDEO=if_probe_passed`,
`MEDIA_ALLOWED_VIDEO_CONTAINERS=mp4,webm`, корректный `FFPROBE_PATH`.
Merge не изменяет env работающих контейнеров и не восстанавливает все старые
Jobs. После интеграции — штатный DEV deploy и smoke; recovery исторических
Jobs выполняется отдельно без provider resubmit и повторного capture.

## Следующие действия

1. Сохранить/интегрировать незакоммиченный video/history пакет и подтвердить
   сохранение media env/ffprobe после штатного DEV deploy.
2. Разобрать Nano Banana 2 preview на существующем Job без новой генерации.
3. Проверить ошибки Imagen/GPT/DeepSeek и отсутствие Job Vidu Pro по безопасным
   кодам и public model mapping. Новые платные попытки согласовать отдельно.
4. Для music проверить access session перед prepare; writes автоматически
   не переигрывать.

Точки входа: [каталог](../docs/runbooks/MODEL_CATALOG.md),
[onboarding](../docs/runbooks/MODEL_ONBOARDING.md), [DEV](../docs/runbooks/DEV.md),
[account/browser](../docs/runbooks/BROWSER_ACCOUNT.md), [архитектура](../docs/ARCHITECTURE.md).
Старая передача от 21 сентября архивирована; её infrastructure blocker
не является актуальным статусом этого пакета.
