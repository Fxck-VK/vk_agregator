# Handoff: serega DEV Contour And VK Video Menu

Дата: 2026-06-18

Назначение файла: быстро ввести следующего агента в контекст последних изменений ветки `serega`.
Файл не содержит секретов. После завершения merge/передачи его можно оставить как историческую заметку или удалить по договоренности команды.

## Перед Работой

Прочитать:

- `AGENTS.md`
- `.agents/state.json`
- этот файл
- `docs/DEV_CONTOUR.md`, если задача касается локального DEV
- `RUNBOOK.md`, если задача касается запуска/deploy/env
- `docs/merge/FASTLIFE_VIDEO_ROUTER_MERGE_GUIDE.md`, если задача касается video providers/router

Не читать архивы и логи по умолчанию. Не коммитить `.env`, `.runtime`, токены, ключи, пароли, launch params, raw provider payloads или PII.

## Текущий Контекст

- Рабочая ветка: `serega`
- Последний handoff-коммит: `d3427d4 dev: add production-shaped local contour`
- Перед пушем в `serega` были пройдены:
  - `go test ./internal/adapter/inbound/vk ./internal/app/vkbot ./internal/platform/config ./internal/service/commandrouter`
  - `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/ci/validate-infra.ps1`
  - `git diff --check`
- Push в `origin/serega` выполнен.

## Что Было Сделано

### 1. DEV Контур Как Копия Production

Цель: локальная разработка должна отличаться от production только окружением, туннелем, доменами и внешними VK/YooKassa/provider секретами, а не отдельной логикой.

Добавлены/обновлены:

- `.env.dev.example`
- `docs/DEV_CONTOUR.md`
- `RUNBOOK.md`
- `deployments/cloudflare/README.md`
- `deployments/nginx/nginx.prod.conf`
- `scripts/dev/start-dev-stack.ps1`
- `scripts/dev/stop-dev-stack.ps1`
- `scripts/dev/status-dev-stack.ps1`
- `scripts/dev/smoke-dev.ps1`
- `scripts/dev/check-dev-reverse-proxy.ps1`
- `scripts/ci/validate-infra.ps1`

DEV public URLs:

```text
https://dev-vk.neiirohub.ru/webhooks/vk
https://dev-app.neiirohub.ru
https://dev-app.neiirohub.ru/miniapp/*
https://dev.neiirohub.ru/billing/webhooks/yookassa
```

Cloudflare DEV tunnel должен вести все три hostname на один локальный reverse proxy:

```text
http://127.0.0.1:8088
```

Reverse proxy уже разводит:

```text
dev-vk.neiirohub.ru/webhooks/vk       -> cmd/api
dev-vk.neiirohub.ru/health            -> cmd/api
dev-app.neiirohub.ru                  -> miniapp frontend
dev-app.neiirohub.ru/miniapp/*        -> cmd/api
dev.neiirohub.ru/billing/webhooks/... -> cmd/provider-webhook
```

Основные команды:

```powershell
.\scripts\dev\start-dev-stack.ps1 -WithCloudflare
.\scripts\dev\status-dev-stack.ps1
.\scripts\dev\smoke-dev.ps1
.\scripts\dev\stop-dev-stack.ps1
```

Важный принцип: стандартный DEV startup собирает runtime из текущего working tree. Это нужно, чтобы DEV реально тестировал текущий код, а не старые GHCR images.

### 2. DEV Safety

DEV отделен от production:

- отдельное VK-сообщество;
- отдельный Cloudflare tunnel/token;
- отдельные dev hostnames;
- production tunnel не запускать локально;
- production VK Callback API не менять для локальных тестов;
- production secrets не переносить в локальный `.env`;
- payment provider по умолчанию `mock`;
- YooKassa в DEV включать только на тестовом магазине;
- real AI providers в DEV разрешены только через DEV/test ключи.

В `.env.dev.example` сейчас заложен production-shaped режим для ручного тестирования реальных видео provider routes:

```text
DEV_ALLOW_REAL_AI_PROVIDERS=true
PROVIDER_CHAIN=deepinfra,apimart,poyo,runway,mock
APIMART_PROVIDER_ENABLED=true
POYO_PROVIDER_ENABLED=true
RUNWAY_PROVIDER_ENABLED=true
PAYMENT_PROVIDER=mock
```

Это не означает, что реальные секреты можно коммитить. Только локальный `.env`.

### 3. VK Bot Video Menu

Обновлено меню VK bot для видео.

Главный экран видео теперь должен показывать:

```text
hailuo v2.3
kling v3
seedance v2 fast
runway
Pruna
Назад
```

Подменю:

```text
hailuo v2.3:
- hailuo v2.3 обычный
- hailuo v2.3 fast

runway:
- runway 4.5
- runway 4 turbo
```

Тексты режимов теперь явно говорят, какой тип модели:

```text
Тип модели: text-only.
```

или

```text
Тип модели: требует стартовую картинку.
```

Текущий UX пример для видео:

```text
Пример: кот в очках едет на жирафе
```

### 4. VK Command Router

Добавлены text/callback aliases для новых кнопок, чтобы VK callback и обычный текст кнопки одинаково роутился:

```text
Pruna
hailuo v2.3
hailuo v2.3 обычный
hailuo v2.3 fast
kling v3
seedance v2 fast
runway
runway 4.5
runway 4 turbo
```

Файлы:

- `internal/service/commandrouter/router.go`
- `internal/service/commandrouter/router_test.go`

### 5. VK Video Job Params

Для video route режимов VK bot сохраняет route snapshot в job params, но provider calls по-прежнему не делает.

Файл:

- `internal/adapter/inbound/vk/handler.go`

Важно: VK handler создает job и кладет параметры. Реальный provider call должен оставаться только в worker.

### 6. Menu Feature Flags

Файл:

- `internal/app/vkbot/module.go`

Добавлен DEV preview flag:

```text
VK_MENU_VIDEO_ROUTES_PREVIEW_ENABLED
```

Назначение: в DEV показывать production-shaped video menu, даже если отдельные live route flags еще закрываются/настраиваются. В production это не должно включаться автоматически через bypass.

Pruna показывается, если задан `DEEPINFRA_VIDEO_MODEL`.

Runway group показывается, если включен хотя бы один из:

```text
FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED
FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED
```

## Что Было Проверено В DEV

DEV stack поднимался через:

```powershell
.\scripts\dev\start-dev-stack.ps1 -WithCloudflare
```

Публичный DEV callback начал отвечать после синхронизации `VK_CONFIRMATION_TOKEN`.

Кнопки основного меню появились после того, как DEV перестал быть отдельной урезанной конфигурацией и стал production-shaped.

## Текущая Проблема С Real Video Providers

Пользователь проверял `kling v3`.

Job был создан и обработан worker, но завершился:

```text
status=failed_terminal
error_code=invalid_request
error_message=request is not supported; credits were not charged
route=video_kling_o3_standard
provider=poyo
model_code=kling-o3/standard
```

Вывод:

- VK callback работает;
- меню работает;
- job создается;
- worker работает;
- billing safety работает, кредиты не списались;
- проблема не в туннеле и не в кнопках;
- проблема в provider adapter / route mapping / payload для PoYo `kling v3`.

Также ранее похожие ошибки были на части APIMart/Hailuo routes. Pruna через DeepInfra уже проходила, значит общий video pipeline живой.

Следующий технический шаг по video providers: сверить реальные API APIMart/PoYo/Runway с текущими adapter/router payloads и поправить model codes/request bodies. Не включать paid smoke без явного согласования.

## Hot Zones

Если следующий агент мерджит или правит эти области, смотреть особенно внимательно:

- `.env.dev.example`
- `docs/DEV_CONTOUR.md`
- `RUNBOOK.md`
- `deployments/cloudflare/README.md`
- `deployments/nginx/nginx.prod.conf`
- `scripts/dev/*.ps1`
- `scripts/ci/validate-infra.ps1`
- `internal/adapter/inbound/vk/handler.go`
- `internal/adapter/inbound/vk/menu.go`
- `internal/app/vkbot/module.go`
- `internal/platform/config/config.go`
- `internal/service/commandrouter/router.go`
- `internal/adapter/provider/apimart`
- `internal/adapter/provider/poyo`
- `internal/adapter/provider/runway`
- `internal/worker`

## Инварианты, Которые Нельзя Ломать

- VK Bot, Mini App и `cmd/api` не вызывают AI providers напрямую.
- Provider calls только через worker/job flow.
- Billing остается append-only ledger.
- Capture после безопасного storage/delivery success path.
- Technical provider/media failure должен release credits, не capture.
- Frontend/VK не должны получать raw provider secrets, private URLs или internal provider payloads.
- Artifact access должен оставаться owner-checked.
- Production Cloudflare/VK/YooKassa настройки не трогать при DEV тестах.
- `.env` и реальные ключи не коммитить.

## Рекомендованные Проверки После Дальнейших Правок

Минимум для VK/dev изменений:

```powershell
go test ./internal/adapter/inbound/vk ./internal/app/vkbot ./internal/platform/config ./internal/service/commandrouter
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/ci/validate-infra.ps1
git diff --check
```

Для video provider/router изменений:

```powershell
go test ./internal/domain ./internal/platform/config ./internal/service/joborchestrator ./internal/worker
go test ./internal/adapter/provider/apimart ./internal/adapter/provider/poyo ./internal/adapter/provider/runway
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke/video-routes.ps1 -Mode DryRun
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke/video-routes.ps1 -Mode Mock
```

Live provider smoke делать только с явным подтверждением пользователя, потому что он может тратить provider balance.
