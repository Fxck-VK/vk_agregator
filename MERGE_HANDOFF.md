# MERGE_HANDOFF — `serega` Branch

Дата подготовки: 2026-06-10.

Цель файла: дать агенту коллеги короткий актуальный контекст перед слиянием веток. Этот файл не заменяет `AGENTS.md` и `.agents/state.json`; сначала нужно соблюдать их, затем использовать этот handoff как merge-карту по изменениям ветки `serega`.

## Текущий Контекст

- Текущая ветка: `serega`.
- Последний HEAD на момент подготовки файла: `4244cbe docs: compact active context`.
- В рабочем дереве есть незакоммиченные изменения. Не считать их мусором: это текущая работа по VK bot, видео-доставке, YooKassa billing smoke/cancel и конфигам.
- Секреты, `.env`, реальные токены и локальные runtime credentials не должны попадать в merge/commit.
- Последняя проверка после правки video delivery filename: `go test ./...` — green.

## Что Мы Меняли В `serega`

### VK Bot UX

- Главное меню VK bot сейчас включает управляемые feature flags для кнопок.
- Активные пользовательские сценарии:
  - `Создать фото`;
  - `Спросить у НейроХаб`;
  - `Мой аккаунт`;
  - `Пополнить баланс`;
  - `Создать видео`.
- Раздел `Создать видео` сейчас показывает только `PrunaAI` и `Назад`. Остальные видео-модели оставлены в command/domain/router как задел, но не должны внезапно появиться в UX без отдельного включения.
- После выбора `PrunaAI` бот ставит dialog mode `video:prunaai`; следующий обычный текст пользователя становится `video.generate` job.
- Пример для видео в VK bot переведен на русский: `кот в очках едет на жирафе`.
- Placeholder для видео: `НейроХаб готовит видео...`.
- После генерации видео файл больше не должен называться UUID-only. Worker берет первые 25 символов prompt, очищает небезопасные символы и использует это как имя `.mp4`. Если prompt пустой, остается fallback UUID.

### VK Bot Payments

- Кнопка `Пополнить баланс` ведет в выбор пакетов:
  - `99 кристаллов — 99Р`;
  - `150 кристаллов — 150Р`;
  - `250 кристаллов — 250Р`;
  - `400 кристаллов — 400Р`;
  - `700 кристаллов — 700Р`.
- VK bot quick top-up больше не спрашивает email/phone в чате.
- Для YooKassa receipt в VK bot используются серверные env:
  - `VK_TOP_UP_RECEIPT_EMAIL`;
  - `VK_TOP_UP_RECEIPT_PHONE`.
- Если у пользователя уже есть активный intent, бот должен корректно показывать/переиспользовать активный payment link или создавать новый только по явному сценарию.
- Нельзя начислять баланс по redirect/клику пользователя. Начисление только через webhook/reconciliation -> payment service -> ledger topup.

### YooKassa / Billing

- Добавлен/расширен protected operator path для payment lifecycle smoke:
  - `POST /billing/payment-intents/{id}/sync`;
  - `POST /billing/payment-intents/{id}/cancel`;
  - `POST /billing/payment-intents/{id}/refund`;
  - `GET /billing/payment-intents/pending`;
  - `GET /billing/payment-events/unprocessed`.
- Для теста `payment.canceled` добавлен путь создания intent с `capture=false` через protected billing/admin API. Обычные пользовательские платежи остаются `capture=true`.
- `CancelIntent` вызывает provider cancel, затем проверяет состояние через тот же sync/reconciliation path.
- Поздний `payment.canceled` не должен откатывать `succeeded`.
- Refund остается operator-only. Автоматически возвращать потраченные кредиты нельзя, пока нет lot/FIFO attribution.
- YooKassa adapter должен держать Basic Auth и provider details внутри adapter. Наружу только safe DTO.

### VK Delivery / Video

- Default для видео-доставки: `VK_VIDEO_DELIVERY_MODE=doc`.
- В `doc` mode mp4 отправляется как VK document attachment через community token. Это стабильный путь для бота.
- Опциональный native video path оставлен через:
  - `VK_VIDEO_DELIVERY_MODE=video`;
  - `VK_VIDEO_ACCESS_TOKEN`;
  - `VK_VIDEO_UPLOAD_GROUP_ID`.
- Native `video.save` требует user token с video rights. Не включать этот путь без явного решения: это хуже по операционным рискам.
- Worker не должен загружать media attachment слишком рано: attachment готовится при send, чтобы использовать prompt-derived filename и корректно ретраить upload.

## Затронутые Зоны

Ожидаемые файлы/зоны конфликтов:

- `internal/adapter/inbound/vk/handler.go`
  - dialog modes GPT/photo/video;
  - top-up flow без запроса контакта в чате;
  - создание `video.generate` jobs через orchestrator.
- `internal/adapter/inbound/vk/menu.go`
  - главное меню;
  - video model menu только с `PrunaAI`;
  - top-up package menu;
  - account/referral copy.
- `internal/service/commandrouter/router.go`
  - текстовые payloads/commands для меню и video flow.
- `internal/domain/command.go`
  - command types для video menu.
- `internal/adapter/delivery/vk/real.go`
  - raw photo upload;
  - mp4-as-doc upload;
  - optional native video upload.
- `internal/worker/delivery.go`
  - lazy media attachment;
  - prompt-based media filename;
  - delivery retry semantics.
- `internal/service/paymentservice/**`
  - active intent handling;
  - `capture=false` metadata;
  - provider sync/cancel/refund.
- `internal/adapter/inbound/billing/handler.go`
  - protected operator endpoints.
- `internal/adapter/payment/yookassa/**`
  - capture flag;
  - idempotency;
  - receipt handling.
- `internal/platform/config/config.go`, `.env.example`, `cmd/worker/main.go`
  - `VK_VIDEO_*`;
  - `VK_TOP_UP_RECEIPT_*`;
  - payment/video config validation.
- `migrations/000012_neirohub_crystal_catalog.*`
  - product catalog for current crystal packages.

## Что Нельзя Сломать При Слиянии

- VK bot handler не должен напрямую вызывать AI provider.
- Mini App не должен напрямую вызывать AI provider.
- Video/photo/text prompts должны становиться Jobs через orchestrator.
- Worker остается единственным местом provider calls и media artifact delivery.
- Balance/top-up/referral rewards только через billing ledger. Никаких прямых `balance +=`.
- YooKassa webhook/reconciliation должны верифицировать provider state через adapter before ledger mutation.
- `/billing/*` operator endpoints должны оставаться protected admin-only.
- Public user-facing DTO не должен отдавать raw YooKassa payload.
- `.env`, токены, cloudflared token, YooKassa secret, VK access token не коммитить.
- `VK_VIDEO_DELIVERY_MODE=doc` должен оставаться безопасным default.
- Mini App routes и BFF auth не ослаблять при разрешении конфликтов.

## Рекомендованный Merge-Порядок

1. Перед merge прочитать `AGENTS.md` и `.agents/state.json`.
2. Не читать `docs/archive/**` как active context, если не расследуется старая регрессия.
3. Сделать `git fetch --all --prune`.
4. Посмотреть diff между ветками:
   - `git diff --stat main...serega`;
   - `git diff --stat <miniapp-branch>...serega`.
5. Сначала руками разобрать конфликты в shared backend:
   - config;
   - paymentservice;
   - billing handler;
   - command/domain router.
6. Потом разбирать VK-specific файлы:
   - `internal/adapter/inbound/vk/**`;
   - `internal/adapter/delivery/vk/**`;
   - `internal/worker/delivery.go`.
7. Mini App изменения коллеги сохранять, но не позволять им обходить shared payment/job/provider boundaries.

## Проверки После Merge

Минимум:

```powershell
gofmt -l .
go test ./...
git diff --check
git grep -nE "^(<<<<<<<|=======|>>>>>>>)"
```

Если Mini App затронут merge:

```powershell
npm --prefix web/miniapp run build
```

Ручной smoke после запуска:

- VK bot: `Показать меню`;
- `Спросить у НейроХаб` -> текстовый ответ;
- `Создать фото` -> prompt -> image job;
- `Создать видео` -> `PrunaAI` -> prompt -> mp4 document с понятным filename;
- `Пополнить баланс` -> выбор разных пакетов должен создавать payment link нужной суммы, не переиспользовать старый пакет молча;
- YooKassa test: `payment.succeeded`, duplicate webhook, `capture=false -> cancel`, refund.

## Известные Операционные Детали

- Бот локально запускается через `scripts/dev/start-bot.ps1`.
- Статус бота проверяется через `scripts/dev/status-bot.ps1`.
- Для VK callback используется `https://vk.neiirohub.ru/webhooks/vk`.
- Для Mini App у коллеги может быть отдельный tunnel/hostname. Не менять `vk.neiirohub.ru` без согласования.
- YooKassa webhook должен идти на HTTPS endpoint provider-webhook runtime, а не в обычный VK callback.

## Финальный Вердикт Для Агента Коллеги

Сохранять обе стороны. Ветка `serega` добавляет рабочий VK bot слой поверх shared backend: меню, GPT/photo/video modes, top-up UX, YooKassa smoke/cancel support и видео-доставку. Это не должно быть заменено Mini App-логикой. Если конфликт между Mini App и VK Bot возникает в shared сервисах, правильное решение — общий сервис/порт + surface-specific adapter, а не копирование бизнес-логики в surface.
