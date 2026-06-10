# Merge Handoff: branch `serega`

Этот файл нужен для агента, который будет сливать нашу VK bot ветку с веткой Mini App.
Он описывает изменения, которые нельзя потерять при merge.

Дата снимка: 2026-06-06.

## Ветки

- База: `main` -> `e1d5c45 integration: merge miniapp and vk bot backend into main`
- Наша ветка: `serega` -> `fd98355 feat: add shared VK referral system`
- Ветка друга, видимая локально: `origin/fastlife_dev` -> `3b26d43 miniapp: disable create post flow`
- Общий merge-base для `serega` и `origin/fastlife_dev`: `e1d5c45`

Важно: в рабочем дереве `serega` есть незакоммиченные изменения после `fd98355`.
Перед реальным merge их надо закоммитить и запушить отдельным коммитом в `serega`.

## Что делала наша ветка

### VK bot UX

Нужно сохранить текущую bot-логику:

- Главное inline-меню сейчас показывает только включенные env-флагами кнопки.
- Текущий bot-facing профиль в `.env.example`:
  - `VK_MENU_GPT_ENABLED=true`
  - `VK_MENU_ACCOUNT_ENABLED=true`
  - `VK_MENU_VIDEO_ENABLED=false`
  - `VK_MENU_IMAGE_ENABLED=false`
  - `VK_MENU_STUDENTS_ENABLED=false`
  - `VK_MENU_TOP_UP_ENABLED=false`
- Все скрытые кнопки должны оставаться в коде и быстро возвращаться через `VK_MENU_*_ENABLED`.
- Disabled stale payload не должен открывать скрытую секцию и не должен создавать billable Job.
- `callback`-кнопки должны подтверждаться через `messages.sendMessageEventAnswer`, чтобы в VK не крутилась бесконечная загрузка.
- Нижняя persistent-кнопка `Показать меню` всегда отправляет свежее меню вниз.
- Inline-переходы внутри активного меню редактируют текущее menu-message через `messages.edit`, если оно еще активно.
- Обычный текст вне активного режима НейроХаб не должен уходить в GPT.
- Текст ответа вне режима:
  `Выберите режим в меню выше или нажмите на кнопку показать меню`
- Repair-фразы должны открывать меню и восстанавливать нижнюю кнопку:
  `меню`, `нет меню`, `нет кнопки`, `где меню`, `старт`, `/start`, `начать`.
- Первый обычный non-payload контакт нового пользователя должен запускать onboarding, чтобы пользователь не застрял без меню.

### Text mode НейроХаб

Нужно сохранить текущую text-mode логику:

- Кнопка называется `💬 Спросить у НейроХаб`.
- При входе в режим сообщение:
  `🤖 НейроХаб активен!`
- Pending placeholder:
  `НейроХаб думает...`
- Следующее обычное сообщение пользователя в этом режиме создает `text.ask` Job.
- Provider answer редактирует placeholder через VK `messages.edit`.
- После ответа не надо автоматически присылать главное меню.
- Dialog mode хранится в Redis и переживает restart API в рамках `VK_DIALOG_MODE_TTL`.

### VK text formatting

Ответы provider перед отправкой в VK проходят легкую нормализацию:

- убираются `**bold**`, backticks и markdown heading hashes;
- markdown bullets `* ` и `- ` превращаются в `•`;
- это делается в worker delivery, а не в VK handler.

Цель: VK не показывает пользователю сырые `**` и `*`.

### Shared referral system

Реферальная система должна остаться общей для VK Bot и VK Mini App:

- Canonical user identity остается общей по `users.vk_user_id`.
- У одного VK пользователя один стабильный referral code.
- Таблицы:
  - `referral_codes`
  - `referrals`
- Источники referral:
  - `vk_bot`
  - `vk_miniapp`
- VK Bot применяет ref code из `/start <code>` и из VK `ref`.
- Referral relation idempotent.
- Self-referral запрещен.
- Один referred user может иметь только одного referrer.
- Signup reward идет через `billingservice.Grant`, то есть через append-only ledger, без прямой мутации баланса.

Текущий экран `Мой аккаунт` в VK Bot:

```text
👤 Мой аккаунт

• безлимитное общение с НейроХаб!

👥 Реферальная программа

• Приглашённых: <count>

• Ссылка: <vk referral link>

Поддержка: @neirohub_help
```

Кнопки на экране аккаунта:

- только `⬅️ Назад`;
- кнопки `Поделиться` сейчас нет;
- `VK_REFERRAL_SHARE_BASE` остается reserved для будущих share/open-link сценариев.

Текущая ссылка по умолчанию:

```text
https://vk.com/write-<group_id>?ref=<code>
```

Не возвращать старый вариант `vk.com/im?...`, потому что он открывал не тот UX.

### Dialog context

Нужно сохранить backend context для текстовой модели:

- История хранится в Postgres, не целиком в prompt.
- Таблицы:
  - `conversations`
  - `conversation_messages`
  - `conversation_summaries`
- В prompt отправляется compact packet:
  - system prompt;
  - bot/profile context;
  - summary;
  - последние сообщения;
  - текущий user prompt.
- Дефолтные лимиты:
  - input: `1600` estimated tokens;
  - output: `800`;
  - summary: `400`;
  - recent messages: `6`;
  - summarize after: `10` messages or `1500` estimated tokens.

### Anti-spam

Нужно сохранить Redis-backed anti-spam:

- лимит любых user events;
- отдельный лимит GPT-запросов;
- cooldown response;
- temporary block при повторных превышениях;
- stricter limits для новых пользователей;
- active jobs per user guard.

Эта логика защищает provider budget и worker queue.

### Dev scripts

Нужно сохранить локальные bot scripts:

- `scripts/dev/start-bot.ps1`
- `scripts/dev/stop-bot.ps1`
- `scripts/dev/status-bot.ps1`
- `scripts/dev/_bot-common.ps1`
- `scripts/dev/setup-cloudflare-tunnel.ps1`

Они запускают только bot runtime, не являются production deploy.

## Самые важные файлы нашей стороны

Backend:

- `cmd/api/main.go`
- `cmd/worker/main.go`
- `internal/adapter/inbound/vk/handler.go`
- `internal/adapter/inbound/vk/menu.go`
- `internal/adapter/delivery/vk/*`
- `internal/worker/generation.go`
- `internal/worker/delivery.go`
- `internal/worker/worker.go`
- `internal/service/referralservice/*`
- `internal/service/dialogcontext/*`
- `internal/service/dialogstate/*`
- `internal/service/antispam/*`
- `internal/platform/config/config.go`
- `internal/domain/user.go`
- `internal/domain/referral.go`
- `internal/domain/conversation.go`
- `internal/domain/repositories.go`

Storage and migrations:

- `migrations/000005_user_vk_profile.*.sql`
- `migrations/000006_conversation_context.*.sql`
- `migrations/000007_referrals.*.sql`
- `internal/adapter/storage/postgres/referral.go`
- `internal/adapter/storage/postgres/conversation.go`
- `internal/adapter/storage/memory/referral.go`
- `internal/adapter/storage/memory/conversation.go`

Docs and env:

- `.env.example`
- `AGENTS.md`
- `docs/AGENTS_FULL.md`
- `README.md`
- `RUNBOOK.md`
- `PROGRESS.md`
- `TASKS.md`
- `TESTING.md`

## Возможные зоны конфликтов с Mini App

### `cmd/api/main.go`

Нельзя выбрать только одну сторону.

Итоговый файл должен одновременно сохранить:

- VK webhook `/webhooks/vk`;
- VK `ControlClient`, `ProfileClient`, dialog state, anti-spam, referral service;
- Mini App BFF routes `/miniapp/*`;
- Mini App launch/session verification;
- admin/health/metrics.

### `internal/platform/config/config.go`

Конфликт решать через объединение полей, а не выбор стороны.

Сохранить:

- DeepInfra config;
- VK menu flags;
- VK dialog mode TTL;
- referral config;
- text context config;
- anti-spam config;
- Mini App config;
- provider timeout/model config друга.

### `internal/worker/generation.go` и `internal/worker/worker.go`

Сохранить обе логики:

- наша context сборка и сохранение turns для VK text mode;
- provider router/DeepInfra;
- Mini App async job flow и model selection друга;
- delivery/capture/moderation state machine.

### `internal/service/billingservice/service.go`

Сохранить:

- ledger-based grants/referral rewards;
- изменения друга по billing/cost estimate, если они есть.

Баланс напрямую не мутировать.

### `web/miniapp/**`

Наша ветка не должна ломать Mini App UI.
Если конфликт касается `web/miniapp/**`, приоритетно сохранять работу друга, но сверить backend contract:

- Mini App должен продолжать отправлять нужные поля для job creation;
- referral future hooks можно добавлять только через backend service, не через frontend-side trust.

## Прогноз конфликтов

Команда:

```powershell
git merge-tree --write-tree --name-only --messages serega origin/fastlife_dev
```

На зафиксированных HEAD веток показала content conflicts:

```text
TASKS.md
internal/worker/worker_test.go
```

Остальные пересечения auto-merge, но их надо ревьюить вручную:

- `AUDIT.md`
- `PROGRESS.md`
- `TASKS.md`
- `cmd/api/main.go`
- `internal/service/billingservice/service.go`
- `internal/worker/generation.go`
- `internal/worker/worker.go`
- `internal/worker/worker_test.go`

Внимание: это прогноз по committed HEAD. Текущие незакоммиченные изменения в `serega` могут добавить новые пересечения.

## Что нельзя делать при merge

- Не удалять `web/miniapp/**` изменения друга.
- Не удалять VK bot routing/menu/referral/context/anti-spam изменения нашей ветки.
- Не менять `.env` и не коммитить реальные secret-файлы.
- Не использовать `git add .`.
- Не делать force push.
- Не отключать billing/idempotency/moderation/signature checks ради merge.
- Не возвращать прямой вызов AI provider из VK handler или Mini App handler.
- Не делать frontend источником истины для balance/job status/referral identity.
