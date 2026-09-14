Status: archived
Do not use for current implementation.
See: docs/HANDOFF_CURRENT.md

# Передача коллеге: мердж интеграций моделей

Status: active
Дата: 2026-09-14
Версия кода: `f2dfe9554f2f3174b1305ebe392fd7f59401bf3d`.

## Что передаём

Ветки `origin/fastlife_dev` и `origin/dev-deploy` указывают на одну версию `f2dfe95`; удалённые ссылки проверены 14.09.2026. Эта версия успешно развёрнута в DEV 13.09.2026. В `main` в рамках этой работы изменения не переносились.

Документ охватывает весь блок интеграций за 8–13 сентября: **16 коммитов, 218 изменённых файлов**, диапазон `31129c3..f2dfe95`. `31129c30e07ae0fcf136ad111a04d0776cfdc16c` — граница описываемой работы, а не установленная общая база с веткой коллеги. Ветка коллеги не указана.

Если у коллеги уже есть `4e9294b`, последний пакет состоит только из `9fa44c3` и `f2dfe95`. Он добавляет четыре варианта изображений и два видео-маршрута. Этот файл подготовлен после релиза и сам пока не закоммичен.

## Дополнение 14 сентября: каталог возможностей, пока без коммита

После указанного релиза в рабочем дереве подготовлена отдельная доработка. Она **не входит в `f2dfe95` и ещё не опубликована**; при передаче только перечисленных ниже коммитов этих изменений у коллеги не будет.

- `internal/service/providermodels/registry_{text,image,video,audio}.go`: реестр разделён по назначению, смешанный `Limits` заменён отдельными типами. Аудио-раздел пока пуст.
- `internal/service/providermodels/capabilities*.go`: для всех 41 модели отдельно описаны возможности API и реализация приложения. Входные форматы, лимиты фото, пропорции, разрешение, детализация, скорость, длительность, звук и начальный/конечный кадры получают явные значения либо статус «не подтверждено».
- Mini App и Web получают безопасное поле `capabilities`. Оно учитывает ограничения конкретного интерфейса и доступные тарифные параметры. Старые поля сохранены; Web больше не обещает несуществующую передачу фото-референсов.
- Qwen Image 3.0 и Seedream 4.5 отклоняют неподдерживаемые пропорции до расчёта тарифа. В клиенте снят общий предел четырёх референсов; интерфейс использует лимит модели, backend повторно проверяет количество и владельца файлов.
- В обоих интерфейсах добавлены сведения о возможностях модели. Текстовый чат не предлагает прикрепление файлов, которые не передаются в запрос.
- [Единый каталог возможностей](MODEL_CAPABILITIES.md) генерируется командой `go run ./cmd/model-catalog -format markdown`; JSON доступен с `-format json`. Отдельные планы каждой модели не создаются.

Проверены Go-тесты каталога, запросов, inbound-обработчиков, оркестратора, worker, провайдеров и app wiring; `go vet`; frontend lint/typecheck, профильные тесты и сборки обеих приложений. Новых миграций, env-переменных или тарифов эта доработка не добавляет. Платные запросы к моделям не выполнялись.

## Коммиты по порядку

| Коммит | Дата 2026 | Изменение |
| --- | --- | --- |
| [fad075d](https://github.com/Fxck-VK/vk_agregator/commit/fad075da46a8644747f552382dd4a4bc4d91a458) | 08.09 | Qwen Image 3.0, каталог Study24 и проверка APIMart без генераций. |
| [7a0186e](https://github.com/Fxck-VK/vk_agregator/commit/7a0186eda82a790037ecfa4b759b36dbc39967dc) | 08.09 | Исправлены ссылки документации APIMart для CI. |
| [cdc3b17](https://github.com/Fxck-VK/vk_agregator/commit/cdc3b17b9162d375fb5d4d8fe651c7ff61163788) | 08.09 | Обновлены Go-зависимости и базовые образы с исправлениями уязвимостей. |
| [5a50cd4](https://github.com/Fxck-VK/vk_agregator/commit/5a50cd444387607492a49464de93c7b7fe86daef) | 08.09 | Grok Imagine 1.5 и 2.0: адаптеры, каталог, тарифы, флаги, интерфейсы. |
| [0eaf237](https://github.com/Fxck-VK/vk_agregator/commit/0eaf237810c60ecf3f13b9770047f2deda2da130) | 08.09 | Обновлена база Browserslist для проверок релиза. |
| [ac8b843](https://github.com/Fxck-VK/vk_agregator/commit/ac8b843095b63731c8e393bcf186bdef2a497f81) | 08.09 | Документированы существующие исключения проверок CSRF и CLI. |
| [6e1da6f](https://github.com/Fxck-VK/vk_agregator/commit/6e1da6f324ffde2856bf99f0ccd8e722b21d492a) | 08.09 | Восстановлены меню фото VK и параметры заданий Grok. |
| [20ca4ac](https://github.com/Fxck-VK/vk_agregator/commit/20ca4ac16f4e418cf8b74dbf2531596c71c8ee7d) | 10.09 | Seedance 2.5, Midjourney V7, FLUX.2 Pro, 11 платных текстовых моделей; миграция 000052; удалены отдельные планы моделей. |
| [c9e0660](https://github.com/Fxck-VK/vk_agregator/commit/c9e0660b09fd8b63195b0e35cf0270ab58b25c7e) | 10.09 | Обновлены зависимости Mini App, Admin и Web Platform для релиза. |
| [5d46262](https://github.com/Fxck-VK/vk_agregator/commit/5d462625f20a3198e129b55a7292c62a22b69662) | 10.09 | Исправлены уязвимости gRPC и среды выполнения Mini App. |
| [a18ade5](https://github.com/Fxck-VK/vk_agregator/commit/a18ade53afeb99b7bbfe8364f2717feef02bc67e) | 11.09 | Проверка миграций принимает точный проверенный SHA-256 миграции 000052; добавлены регрессии. |
| [74e23b6](https://github.com/Fxck-VK/vk_agregator/commit/74e23b6b06455a973762d2cdec33b538a338b4f6) | 12.09 | Omni Flash/EXT, Kling V3, Motion Control 2.6, Veo 3.1 Lite/Fast/Quality; защищённая передача видеореференсов. |
| [10c5c97](https://github.com/Fxck-VK/vk_agregator/commit/10c5c97b85c680866d82f141058155817a95ba7a) | 12.09 | Исправлены проверки документации видео и ложное срабатывание Secret Scan на тестовом маркере Kling. |
| [4e9294b](https://github.com/Fxck-VK/vk_agregator/commit/4e9294b6abda0f927c561eed2bfcebe25971435e) | 12.09 | DEV-деплой повторно использует установленные образы Postgres/Redis/MinIO; данные и volumes сохраняются. |
| [9fa44c3](https://github.com/Fxck-VK/vk_agregator/commit/9fa44c363738d8c1d822a7837aee262b587e1a69) | 13.09 | GPT Image 2.5 Flare/Sunburst, Seedream 5.0 Lite/Pro, Kling 3.0 Turbo, MiniMax H3; тарифы и проверки биллинга. |
| [f2dfe95](https://github.com/Fxck-VK/vk_agregator/commit/f2dfe9554f2f3174b1305ebe392fd7f59401bf3d) | 13.09 | Точное исключение Gitleaks для несекретного тестового IdempotencyKey h3-priced-job; после исправления CI и деплой успешны. |

## Добавленные модели

В этом диапазоне добавлены 11 платных текстовых маршрутов, 9 маршрутов изображений и 10 видео-маршрутов. Наличие реализации не означает доступность в любом окружении: нужны флаг, конфигурация провайдера и тариф.

### Текст

| Модель | Провайдер | Public ID |
| --- | --- | --- |
| GPT-5.5 | KIE | `gpt_5_5` |
| Claude Opus 4.7 | KIE | `claude_opus_4_7` |
| Gemini 3.1 Pro | KIE | `gemini_3_1_pro` |
| Claude Opus 4.8 | KIE | `claude_opus_4_8` |
| GPT 5.6 Terra | KIE | `gpt_5_6_terra` |
| GPT 6 Astra | KIE | `gpt_6_astra` |
| Claude Opus 5 | KIE | `claude_opus_5` |
| Gemini 3.7 Flash | KIE | `gemini_3_7_flash` |
| Claude Fable 5.1 | APIMart | `claude_fable_5_1` |
| Claude Fable 5 | KIE | `claude_fable_5` |
| Gemini 3.6 Flash | KIE | `gemini_3_6_flash` |

Добавлены общий текстовый HTTP-адаптер, точные маршруты провайдеров, серверное ограничение входа/выхода, проверка usage и выбор модели в Web/Mini App. Ответ оплачивается по заранее рассчитанной фиксированной цене. Прежний маршрут чата и генерации заголовков сохранён.

### Изображения — APIMart

| Модель | Public ID | Основные настройки |
| --- | --- | --- |
| Qwen Image 3.0 | `qwen_image_3` | 1K/2K |
| Grok Imagine 1.5 | `grok_image_1_5` | Standard |
| Grok Imagine 2.0 | `grok_image_2_0` | Standard |
| Midjourney V7 | `midjourney_v7` | Imagine |
| FLUX.2 Pro | `flux_2_pro` | 1/2/3/4 MP |
| GPT Image 2.5 Flare | `gpt_image_2_5_flare` | 1K/2K/4K × low/medium/high/xhigh/max |
| GPT Image 2.5 Sunburst | `gpt_image_2_5_sunburst` | 1K/2K/4K × low/medium/high/xhigh/max |
| Seedream 5.0 Lite | `seedream_5_0_lite` | 2K/3K/4K |
| Seedream 5.0 Pro | `seedream_5_0_pro` | 1K/1.5K/2K |

Для новых GPT Image 2.5 загрузка референсов публично закрыта; цена учитывает размер, качество, соотношение сторон и число результатов. Seedream поддерживает собственные изображения-референсы в Mini App/VK; отдельный Web-интерфейс использует генерацию по тексту. В Web добавлены корректные подписи качества и подтверждение серверной стоимости.

### Видео — APIMart

| Модель | Public alias | Особенности |
| --- | --- | --- |
| Seedance 2.5 | `video_seedance_2_5` | Отдельный адаптер, тариф и выбор параметров |
| Gemini Omni 1.1 Flash | `video_gemini_omni_1_1_flash` | Автоматическая длительность, изображения-референсы |
| Gemini Omni 1.1 Flash EXT | `video_gemini_omni_1_1_flash_ext` | 4/6/8/10 секунд |
| Kling V3 | `video_kling_v3` | 3–15 секунд, 720p/1080p/4K, вариант со звуком |
| Kling 2.6 Motion Control | `video_kling_2_6_motion_control` | Изображение + собственное видео, std/pro; загрузка через Mini App |
| Veo 3.1 Lite | `video_veo_3_1_lite` | 8 секунд, генерация по тексту |
| Veo 3.1 Fast | `video_veo_3_1_fast` | 8 секунд, до трёх изображений |
| Veo 3.1 Quality | `video_veo_3_1_quality` | 8 секунд, до двух изображений |
| Kling 3.0 Turbo | `video_kling_3_0_turbo` | 3–15 секунд, 720p/1080p; текст или первый кадр |
| MiniMax H3 | `video_minimax_h3` | 4–15 секунд, 768P/2K; текст или первый кадр |

Для Kling Turbo промпт необязателен, если приложен один собственный первый кадр. Для H3 текст обязателен; native ID строго `MiniMax-H3`. Расширенные video/audio/reference-режимы H3 не выведены в публичный интерфейс.

Видео настроено в Mini App и каталоге VK; Motion Control не предлагается в VK-чате, где нет нужной загрузки видео. Отдельный Web Platform пока не имеет интерфейса генерации видео. Клавиатура видео VK размещает кнопки по три в строке и укладывается в лимит шести строк.

## Общие изменения и что сохранить при конфликте

| Область | Файлы/каталоги | Что нельзя потерять |
| --- | --- | --- |
| Провайдеры и обработка Jobs | `internal/adapter/provider/{apimart,kie,textapi}/`, `internal/worker/` | Нативные параметры, submit/poll, сохранённые идентификаторы задач, запрет повторной платной отправки при неизвестном исходе |
| Каталог и конфигурация | `internal/service/{providermodels,modelcatalog,productcatalog,videorouter,textgeneration}/`, `internal/platform/config/` | Public ID, ограничения моделей, флаги и проверка готовности провайдера |
| Биллинг | `internal/service/pricingcatalog/`, `internal/service/joborchestrator/` | Размерности тарифа, версия каталога 15, серверный snapshot, резервирование и защита от изменения запроса при replay |
| Изображения | `internal/service/imagegeneration/`, `internal/worker/paid_images.go` | Проверка числа результатов, количества референсов и точной стоимости до платного вызова |
| Референсы Motion | `internal/service/providerreference/`, `internal/service/videoreference/`, `internal/adapter/inbound/miniapp/motion_video.go` | Проверка владельца, ffprobe, привязка к Job, HMAC и срок действия ссылок |
| Mini App/VK | `web/miniapp/src/workflow/WorkflowMode.tsx`, `web/miniapp/src/api/client.ts`, `internal/adapter/inbound/{miniapp,vk}/` | Параметры, оценка цены, загрузки, выбор модели, разрешённый пустой промпт только для Kling Turbo с кадром |
| Web | `web/platform/src/features/{conversations,image-generation,models}/`, `internal/adapter/inbound/websession/` | Выбор текстовой модели, качества изображения и подтверждение серверной цены |
| Деплой и зависимости | `scripts/deploy/`, `Dockerfile.*`, `deployments/nginx/nginx.prod.conf`, `go.mod/go.sum`, `web/*/package*.json`, `.gitleaks.toml` | Исправления сборки, миграции, закреплённые версии зависимостей, точные исключения тестовых маркеров |

Провайдеры вызываются только из worker/адаптеров. Каждый результат проходит хранение как Artifact и модерацию. Стоимость и маршрутизация определяются сервером; резерв/capture/release остаются в ledger. При мердже с изменениями учётных записей необходимо сохранить проверки владельца Job и входных/выходных артефактов.

Тарифы новых интеграций: себестоимость ×3, округление вверх до 5 внутренних кредитов. Один внутренний кредит = $0.005; кредит APIMart = $0.10. Не заменять точные дробные расчёты округлёнными бюджетами провайдера. Последний каталог — `StaticCatalogVersion = 15`; при DB-каталоге соответствующие тарифы активируются отдельно через операторский процесс.

## Миграция и окружение

- Добавлена `migrations/000052_text_model_pricing.up.sql`: поле `text_model_id`, обновление UNIQUE/CHECK для ключей тарифов. Существующие версии цен и snapshots Jobs не переписываются, тарифы не активируются миграцией.
- Вместе с миграцией сохранить `.gitattributes`, `scripts/deploy/migration-safety.sha256` и обе проверки `check-migrations-safe.*`. Разрешение связано с точным содержимым файла и LF; нельзя принимать произвольный DROP CONSTRAINT.
- Если у коллеги уже есть другая миграция `000052`, сначала сверить содержимое и историю применённых миграций каждого контура. Нельзя перезаписывать или переименовывать уже применённую миграцию; согласовать отдельное совместимое продолжение.
- Флаги новых моделей по умолчанию выключены. DEV-подготовка включает предусмотренные APIMart-модели при наличии конфигурации и ключа, сохраняя явно заданное `false`. Список флагов находится в registry, config и `scripts/deploy/prepare-dev-env.sh`.
- Последний пакет: `FEATURE_APIMART_GPT_IMAGE_2_5_FLARE_ENABLED`, `FEATURE_APIMART_GPT_IMAGE_2_5_SUNBURST_ENABLED`, `FEATURE_APIMART_SEEDREAM_5_0_LITE_ENABLED`, `FEATURE_APIMART_SEEDREAM_5_0_PRO_ENABLED`, `FEATURE_APIMART_KLING_3_0_TURBO_ENABLED`, `FEATURE_APIMART_MINIMAX_H3_ENABLED`.
- Текст требует отдельных `FEATURE_TEXT_*`, конфигурации KIE/APIMart и проверенных ограничений через `KIE_TEXT_LIMITS_VERIFIED` / `APIMART_TEXT_LIMITS_VERIFIED`. Сам мердж не подтверждает лимиты и стоимость реального аккаунта.
- Motion требует `PROVIDER_REFERENCE_BASE_URL`, отдельного `PROVIDER_REFERENCE_SIGNING_KEY`, ffprobe в API-образе и соответствующих nginx-маршрутов/лимитов загрузки. Ключ не менять, пока активные задания используют подписанные ссылки.
- Секреты передаются через штатные GitHub Repository Secrets и серверное окружение. Значения `.env`/`.env.d`, ключи и токены в файл мерджа не включены; DEV-секреты нельзя переносить в PROD.

## Как забрать изменения

Рекомендуется обычный merge ветки `origin/fastlife_dev` в рабочую ветку коллеги: коммиты взаимосвязаны. Перед началом сохранить свои незакоммиченные изменения. Команды ниже выполнять уже в нужной ветке коллеги, не в `main`:

```bash
git fetch origin
git status --short --branch
git log --oneline HEAD..origin/fastlife_dev
git diff --stat HEAD...origin/fastlife_dev
git merge --no-ff --no-commit origin/fastlife_dev
```

Не выбирать целиком ours/theirs для общих каталогов, `worker.go`, `orchestrator.go`, API DTO и `WorkflowMode.tsx`: сохранить обе стороны поведения и сверить перечисленные контракты. При конфликтах зависимостей согласовать версии в manifest, затем обновить lock-файл штатным менеджером пакетов.

Если нужен только последний пакет и `4e9294b` уже входит в ветку коллеги, допустим перенос в порядке `9fa44c3`, затем `f2dfe95`. Один первый коммит без второго вызывает ложное срабатывание Secret Scan. Проверять историю до переноса, чтобы не дублировать уже влитые изменения.

После разрешения конфликтов выполнить проверки, затем завершить merge-коммит. Продвижение в `main` — через принятый PR/review-процесс; production deploy только через GitHub Actions. Не выполнять сброс данных, `docker compose down -v` или автоматический откат схемы.

## Проверки и подтверждённый деплой

Для `f2dfe95` подтверждено 13.09.2026:

- `go test ./...`, `go vet ./...`; проверки Go выполнялись также на закреплённом Go 1.25.13.
- Mini App: 48 тестов; Admin: 7; Web Platform: 755 и 4 проверки assets. Lint/typecheck и сборки изменённых интерфейсов прошли.
- Полный DEV preflight: tests, npm audit, govulncheck, infrastructure, Trivy. Проверки Secret Scan и контрольных искусственных утечек прошли.
- [CI](https://github.com/Fxck-VK/vk_agregator/actions/runs/34765424103), [восемь подписанных Docker-образов](https://github.com/Fxck-VK/vk_agregator/actions/runs/34765424108), [успешный DEV-деплой и smoke](https://github.com/Fxck-VK/vk_agregator/actions/runs/34765682440).
- Серверный тег: `sha-f2dfe9554f2f3174b1305ebe392fd7f59401bf3d`. После деплоя health API и Mini App отвечали 200; Web без авторизации — ожидаемый 401.

Эти результаты относятся к опубликованной версии, а не к будущему результату мерджа. Платные генерации последнего пакета GPT/Seedream/Kling Turbo/H3 не запускались; smoke подтверждает работу инфраструктуры, но не качество или фактическую тарификацию генераций.

После мерджа повторить профильные Go-тесты, lint/typecheck/test/build затронутых frontend-пакетов, `git diff --check`, проверки миграции и Secret Scan. Перед push в `dev-deploy` штатный полный preflight выполняется на чистом закоммиченном дереве:

```powershell
$env:GOTOOLCHAIN = 'go1.25.13'
$env:PATH = 'C:\Program Files\Git\bin;' + $env:PATH
pwsh -NoProfile -File scripts/ci/dev-deploy-preflight.ps1
```

Известная локальная особенность Windows: Trivy долго обходит Git-метаданные и служебные `.cache`/`.runtime`. При последнем запуске только эти не входящие в релиз каталоги были временно скрыты пустыми монтированиями внутри сканера; исходники, lock-файлы, правила и пороги проверки сохранены. В репозиторный preflight этот обход не внесён. Локальный Go 1.26.4 давал находки стандартной библиотеки; для проверки использовался закреплённый в проекте Go 1.25.13.

## Документация

- [Архитектура](ARCHITECTURE.md), [видео](VIDEO_GENERATION.md), [DEV и новые изображения](runbooks/DEV.md), [текстовые модели](runbooks/KIE_TEXT_MODELS.md). В старых вводных runbook могут оставаться статусы до деплоя; факт релиза в этом документе привязан к SHA и Actions.
- [Общий каталог Study24](superpowers/plans/2026-09-08-study24-apimart/README.md). Отдельные Markdown-планы каждой модели удалены по решению пользователя; восстанавливать их при merge не нужно.
- [Предыдущая передача по account identity, архив](archive/handoffs/ACCOUNT_IDENTITY_2026-07-05.md). Это исторический снимок, не описание текущего релиза.

Документ содержит опубликованные коммиты и отдельно отмеченные локальные изменения каталога. Merge с веткой коллеги, новый коммит и push в рамках этой доработки не выполнялись.
