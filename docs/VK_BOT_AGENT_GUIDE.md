# VK Bot Agent Guide

Этот файл для агента, который работает рядом с VK Mini App или shared backend и
может случайно затронуть VK bot. Перед изменениями в общих backend-модулях
прочитай этот guide, корневой `AGENTS.md` и релевантные части `RUNBOOK.md`.

## Главная идея

VK bot - это только surface над общей AI Job Processing Platform.

Он принимает VK events, показывает меню, включает локальные режимы диалога и
создает Jobs. Он не должен напрямую вызывать AI providers, менять баланс или
самостоятельно доставлять provider artifacts в обход worker/delivery pipeline.

## Архитектурные запреты

- Не вызывай AI providers из `internal/adapter/inbound/vk` или
  `internal/app/vkbot`.
- Не вызывай AI providers из Mini App BFF.
- Provider calls живут в `cmd/worker` и `internal/adapter/provider`.
- VK API calls живут через `internal/adapter/delivery/vk`.
- Billing только через ledger/reservation/capture/release. Не мутируй баланс
  напрямую.
- VK handlers создают Jobs только после пользовательского prompt, а не при
  клике по меню.
- Все external events должны оставаться idempotent.
- Не логируй tokens, raw launch params, provider keys, prompts целиком, PII или
  private media URLs.
- Не коммить `.env`, `.env.ps1`, real tokens, cloudflared credentials JSON,
  runtime logs.

## Текущее поведение VK bot

Главное меню:

- `Показать меню` всегда открывает свежую главную панель внизу.
- Welcome banner подключается через локальный `VK_WELCOME_ATTACHMENT`.
- Banner применяется только к `Старт`, `Показать меню` и menu repair.
- Submenu screens не должны наследовать banner.

Текущие основные кнопки:

- `🖼️ Создать фото`
- `💬 Спросить у НейроХаб`
- `👤 Мой аккаунт`

Видео, студенческий раздел и пополнение баланса реализованы частично/скрыты
feature flags. Не включай их без отдельного product decision.

## Фото-режим

`Создать фото` сразу включает Redis-backed `photo_text` mode для VK peer.

Пользователь после этого просто пишет обычное сообщение, и только тогда бот
создает `image.generate` Job через `joborchestrator`.

Текущий UX:

- На фото-экране остается только `⬅️ Назад`.
- Кнопка `Фото по тексту` выключена: `VK_MENU_IMAGE_TEXT_ENABLED=false`.
- `Фото с референсом` выключено: `VK_MENU_IMAGE_REFERENCE_ENABLED=false`.
- Старые stale payloads выключенных кнопок не должны создавать Jobs.
- Placeholder: `НейроХаб рисует...`.
- Лимит: `100` фото-генераций на пользователя за `24h`.
- Цена текущей VK photo quota: `PRICES=image_generate=0`.

Если меняешь image flow, сохраняй путь:

```text
VK event -> VK handler -> joborchestrator -> Redis stream -> cmd/worker
-> provider -> Artifact -> delivery worker -> VK photo delivery
```

## Текстовый режим НейроХаб

`Спросить у НейроХаб` включает Redis-backed GPT/text mode для peer.

Следующий обычный текст или стикер становится `text.ask` Job. Бот сначала
отправляет `НейроХаб думает...`, worker получает provider answer, delivery
worker редактирует этот же VK message.

Обычный текст вне active GPT/photo mode не должен создавать billable Jobs.
Default behavior: ответить `Выберите режим в меню выше или нажмите на кнопку
показать меню`.

Provider output перед отправкой в VK нормализуется: простой Markdown очищается,
списки становятся `•`, длинные ответы режутся на chunks.

## Rate Limits

Текущие VK bot limits:

- Обычные пользователи: `40` любых events за `60s`.
- Новые пользователи первые `4h`: `30` любых events за `60s`.
- GPT/text Jobs: `3` за `30s`.
- Новые пользователи GPT/text Jobs: `1` за `15s`.
- Active GPT Jobs: максимум `2` одновременно.
- Фото: `100` text-to-image Jobs за `24h`.
- Repeated violations: `5/10m -> 15m` temporary block.

В любые events входят кнопки, текст, стикеры, `Старт`, `Показать меню`.

Если меняешь limits, обнови:

- `.env.example`
- `internal/platform/config/config.go`
- `internal/platform/config/config_test.go`
- `RUNBOOK.md`
- `TASKS.md`
- `.agents/logs/errors.jsonl` only for non-obvious repeatable errors

## Referral

Referral-система общая для VK bot и VK Mini App:

- Один internal user имеет один стабильный public referral code.
- Не создавай отдельную referral identity для Mini App.
- Rewards только через billing ledger entries с idempotency keys.
- `/start <code>` и VK `ref` - control path, не billable Job.
- Mini App referral endpoint использует тот же `referralservice` и должен
  проверять launch params перед применением кода.

## Cloudflare Tunnel

Bot named tunnel локально управляется файлом:

```text
.runtime/vk-bot/cloudflared/config.yml
```

Этот файл можно хранить в git, если в нем нет секретов. Он содержит tunnel id,
путь к credentials file и ingress routes.

Нельзя коммитить:

```text
C:/Users/<user>/.cloudflared/<tunnel-id>.json
cert.pem
tokens
```

Если Mini App нужен отдельный hostname, лучше отдельный tunnel друга, например
`neiirohub-miniapp`, или отдельный route в его локальном config. Dashboard не
редактирует routes у locally managed tunnel.

## Где можно работать Mini App агенту

Обычно безопасные зоны Mini App:

- `web/miniapp/**`
- `internal/app/miniapp/**`
- `internal/adapter/inbound/miniapp/**`
- Mini App-specific docs/tests

Но даже там нельзя:

- вызывать providers напрямую;
- считать баланс на фронте;
- доверять frontend user identity без VK launch params verification;
- отдавать artifacts без ownership check;
- раскрывать real provider model names пользователю.

## Общие файлы повышенного риска

Перед изменениями согласуй или внимательно проверь:

- `internal/adapter/inbound/vk/**`
- `internal/app/vkbot/**`
- `internal/service/joborchestrator/**`
- `internal/service/billingservice/**`
- `internal/worker/**`
- `internal/adapter/provider/**`
- `internal/adapter/delivery/vk/**`
- `internal/platform/config/**`
- `migrations/**`
- `.env.example`
- `.runtime/vk-bot/cloudflared/config.yml`

## Feature Flags

Любая новая VK menu кнопка должна иметь свой `VK_MENU_*_ENABLED` flag.

Disabled button requirements:

- не показывается в новых keyboards;
- stale payload из старого сообщения не открывает скрытую секцию;
- stale payload не создает Job;
- docs/tests обновлены.

## Проверки перед merge

Минимум для backend/VK bot/shared changes:

```powershell
gofmt -l .
go test ./...
git diff --check
```

Если менялся Mini App:

```powershell
npm --prefix web/miniapp run build
```

Если менялись tunnel/routes:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev\status-bot.ps1
```

Перед коммитом проверь:

```powershell
git status --short
git diff --cached --name-only
```

В staged не должно быть `.env`, real tokens, credentials JSON, logs, raw launch
params или PII.

## Merge Checklist

Перед слиянием ветки Mini App с веткой VK bot:

- `/webhooks/vk` все еще mounted.
- `/miniapp/*` все еще mounted.
- `cmd/api` не вызывает providers.
- `cmd/worker` все еще единственное место provider submit/poll.
- VK menu behavior сохранено:
  - welcome/menu banner только на main panel;
  - `Создать фото` сразу включает `photo_text`;
  - фото-экран показывает только `Назад`;
  - `Спросить у НейроХаб` включает text mode;
  - обычный текст вне режима не создает Job;
  - callback buttons получают `sendMessageEventAnswer`.
- Billing ledger/reservations не ослаблены.
- Referral остается shared.
- Secrets не попали в git.

Если появляется конфликт в worker, billing, job orchestrator, provider routing
или migrations, не разруливай "по вкусу". Сначала сравни обе стороны и сохрани
инварианты выше.
