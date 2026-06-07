# Mini App — журнал исправлений (FIXLOG)

Отдельный лог **конкретных багов и UX-фиксов** Mini App. После каждого исправления
ошибки, регрессии или заметного UX-изменения агент **добавляет запись сюда**
(новая секция сверху в «Журнале»).

Связанные документы:
- `AUDIT.md` — production readiness, инварианты, крупные архитектурные заметки
- `docs/VIDEO_GENERATION.md` — env, БД, безопасность, layout `deepinfra` adapter
- `PROGRESS.md` / `TASKS.md` — релизный прогресс и backlog
- `docs/MINIAPP_REDESIGN_CONTEXT.md` — контекст редизайна

## Формат новой записи

```markdown
### FIX-YYYY-MM-DD-NN — краткий заголовок
- **Симптом:** что видел пользователь
- **Причина:** root cause
- **Исправление:** файлы / что изменили
- **Проверка:** build / smoke
- **Осталось:** если есть follow-up
```

---

## Ускорение чата — что можно и что нельзя

| Уровень | Мера | Эффект | Статус |
|--------|------|--------|--------|
| Провайдер | Быстрее/ближе модель (DeepInfra, кэш) | −секунды на генерации | Операционно |
| Backend | Пропуск VK delivery для `source=miniapp` (ответ уже в UI) | −1 этап worker, нет retry доставки | **Backlog** |
| Backend | SSE/WebSocket статуса job вместо polling | −0.5–2 с воспринимаемой задержки | **Backlog** |
| Frontend | Poll 2s → 1.2s | −~0.4 с в среднем | ✅ FIX-2026-06-07-09 |
| Frontend | Показ текста из `conversation_messages` до `succeeded` | −1–3 с (ответ в БД после провайдера) | ✅ FIX-2026-06-07-10 |
| Frontend | «Отвечает…» вместо «Отправка»/«Доставка» | UX, не latency | ✅ FIX-2026-06-07-09 |

**Нельзя** убрать без смены архитектуры: очередь job, модерацию output, вызов
провайдера, сохранение в Postgres/MinIO — это ядро платформы (см. `AGENTS.md`).

### Почему «пропуск delivery» для Mini App — не «пропуск шага пайплайна»

Речь **только** о вызове VK `messages.send` в личку. Это **канал доставки** для
VK-бота, а не обязательная бизнес-логика job.

| Этап | VK-бот | Mini App | Можно пропустить для miniapp? |
|------|--------|----------|-------------------------------|
| Job + резерв кредитов | да | да | **Нет** |
| Провайдер + артефакт | да | да | **Нет** |
| Модерация output | да | да | **Нет** |
| Ответ в `conversation_messages` / BFF | опционально | да (основной UI) | — |
| VK `messages.send` | да (это и есть UI) | дубль в личку | **Да** — UI уже в Mini App |
| Billing capture + `succeeded` | после VK send | сейчас тоже после VK send | **Нет** — capture остаётся |

Безопасная реализация (backlog): в `DeliveryWorker` для job с
`conversation_source=miniapp` или correlation `miniapp:*` — **не** вызывать VK API,
но **сохранить** delivery row (audit), **выполнить** `CaptureForJob`, перевести в
`succeeded`. Feature flag + тесты.

Риски: (1) пользователь не получит копию в VK-чате — продуктовое решение;
(2) нельзя путать с отключением модерации/capture; (3) ранний показ текста в чате
сейчас читает `conversation_messages` до `succeeded` — ответ пишется worker'ом
**до** `moderateOutput` (отдельный hardening, не часть skip-delivery).

---

## Журнал

### FIX-2026-06-07-13 — video: env, .env.example, ops docs
- **Симптом:** `VIDEO_*` только в dev-скрипте; `.env.example` без video; дубль
  `PRICES` в локальном `.env` (`video_generate=50`).
- **Исправление:** `.env.example` + локальный `.env` (video block, `PRICES`
  `video_generate=10`, `WORKER_PROVIDER_CALL_TIMEOUT=180s`); `docs/VIDEO_GENERATION.md`;
  `AUDIT.md` § video; ссылка в FIXLOG/REDESIGN_CONTEXT.
- **Безопасность:** секреты не в репо; production note `DEEPINFRA_VIDEO_DRAFT=false`.
- **Проверка:** grep env keys present/missing (без вывода значений).

### FIX-2026-06-07-12 — Mini App video: DeepInfra PrunaAI/p-video (draft $0.005/с)
- **Симптом:** `video_generate` в Create не доходил до реального провайдера (только mock).
- **Исправление:** `deepinfra/video.go`, `VIDEO_PROVIDER`, worker video defaults,
  `miniapp/models.go` (`kling` → `PrunaAI/p-video` в job params only), dev env draft mode.
- **Scope:** Mini App BFF + worker provider only; **VK bot не трогали**.
- **Безопасность:** draft/duration из worker env (не из клиента); ref-фото для video rejected;
  idempotency + SSRF downloader для `video_url`.
- **Проверка:** `go test` deepinfra + miniapp handler; `npm run build`.
- **Осталось:** live smoke с `DEEPINFRA_API_KEY`; VK bot intake — коллега.

### FIX-2026-06-07-11 — процесс: verify → commit → push `fastlife_dev`
- **Симптом:** накопились незакоммиченные Mini App UX-фиксы; нужен откатываемый push.
- **Проверка:** `npm run build`; `go test ./internal/adapter/inbound/miniapp/...`;
  security: нет provider/billing на фронте, BFF-auth для early text, dev-only
  `VK_DELIVERY_MODE=mock`, без `innerHTML`/секретов.
- **Коммит:** один логический пакет (чат + create/result + dev scripts + FIXLOG).
- **Осталось:** backend skip VK delivery для miniapp (backlog).

### FIX-2026-06-07-10 — ранний показ ответа чата из истории диалога
- **Симптом:** ответ в чате появлялся только после `succeeded`, хотя worker уже
  сохранил текст в `conversation_messages` на этапе провайдера.
- **Причина:** UI ждал terminal job + `resolveBotText` (артефакт доступен BFF
  только на `succeeded`); этап delivery добавлял задержку.
- **Исправление:** `ChatScreen.tsx` — при polling, если в
  `GET /miniapp/chat/conversations/{id}/messages` уже есть bot message с `job_id`,
  показываем текст сразу (`pending: false`), polling до terminal продолжается.
- **Проверка:** `npm run build`
- **Осталось:** backend skip-delivery для miniapp (см. таблицу выше)

### FIX-2026-06-07-09 — «отвечает…» и быстрее polling чата
- **Симптом:** под пузырём бота технические статусы «Отправка», «Доставка».
- **Причина:** `MessageBubble` выводил `statusLabel(job.status)` для pending bot.
- **Исправление:** `RespondingLabel` в `ui.tsx`, `MessageBubble.tsx`;
  `POLL_MS` 2000 → 1200 в `ChatScreen.tsx`.
- **Проверка:** `npm run build`

### FIX-2026-06-07-08 — навигация Create: назад из истории
- **Симптом:** из истории генераций «Назад» сбрасывало на главную Create, а не
  на экран результата/статуса.
- **Исправление:** `flowReturnScreenRef` в `WorkflowMode.tsx`.
- **Проверка:** `npm run build`

### FIX-2026-06-07-18 — чат: бот не прыгает над сообщением пользователя
- **Симптом:** в новом чате пузырь бота сначала над «Вы», через секунду опускается вниз.
- **Причина:** `patchInChat` ставил боту `createdAt=job.created_at`, у user не было timestamp;
  `mergeHistoryMessages` сортировал по времени → бот оказывался выше.
- **Исправление:** общий `sentAt` для пары user/bot; `createdAt` job не трогаем; сортировка по
  `seq` + tie-break user-before-bot; user получает `jobId` для merge с history.
- **Проверка:** `npm run build`, ручной smoke новый чат → send.

### FIX-2026-06-07-17 — история диалогов, overlay-скачивание, ephemeral media note
- **Симптом:** в «Диалоги» каждое сообщение чата = отдельная строка; скачивание только
  текстовой кнопкой; запрос не хранить фото/видео в БД.
- **Исправление:**
  - `dedupeHistoryJobs` — одна строка на `conversation_id` (первый промпт диалога);
  - `JobDTO.conversation_id` + открытие чата по thread id;
  - `MediaResultPreview` — круглая кнопка на превью + неон-предупреждение о скачивании;
  - `Neirohub_{context|id}.ext` через overlay.
- **Хранение медиа:** бинарники в **MinIO**, в Postgres только метаданные artifact/job
  (нужны для превью/биллинга). **Backlog:** TTL-очистка output artifacts для miniapp.
- **Проверка:** `go test ./internal/adapter/inbound/miniapp/...`, `npm run build`

### FIX-2026-06-07-16 — история: контекстные названия, скачивание, чат layout
- **Симптом:** в «Диалоги»/истории заголовок «Текст» вместо промпта; у картинок нет
  контекстного имени и скачивания; при pending (чат/очередь) сообщения «съезжали».
- **Причина:** BFF `newJobDTO` не отдавал `prompt` для `text_generate` (только image/video);
  workflow history показывала тип операции; poll в `ChatScreen` трогал все pending jobs
  включая image; `.msg { align-items: flex-end }` ломал выравнивание при typing bubble.
- **Исправление:**
  - `dto.go` — `prompt` из `jobs.params` для всех операций;
  - `utils/jobDisplay.ts` — `jobDisplayTitle`, `historyCountLabel`;
  - `SettingsScreen`, `WorkflowMode` JobList — контекстные заголовки;
  - `ResultCard` + `utils/artifactDownload.ts` — кнопка «Скачать» `Neirohub_{context|id}.ext`;
  - `ChatScreen` — poll только `text_generate` с существующим bot-msg;
  - `theme.css` / `MessageBubble` — `align-items: flex-start`, `bubble--pending`.
- **Проверка:** `go test ./internal/adapter/inbound/miniapp/...`, `npm run build`
- **Безопасность:** prompt — только свой job пользователя в BFF; скачивание через
  существующий authenticated artifact blob URL, без новых секретов.

### FIX-2026-06-07-15 — dev Postgres: полный сброс + миграции
- **Симптом:** `checksum mismatch for 000008_conversation_sources` при `migrate up`.
- **Исправление:** удалён volume `vk-ai-aggregator_postgres_data`, Postgres пересоздан,
  `go run ./cmd/migrate up` — 8/8 applied; Redis `FLUSHALL` (очереди/кэш).
- **Проверка:** `migrate status`, повторный `migrate up` → `0 migration(s) applied`.

### FIX-2026-06-07-14 — видео: длительность 3/5/10 сек
- **Симптом:** выбранные 3 сек → ролик 5 сек.
- **Причина:** Vite подхватил UI, но **api/worker .exe не пересобраны** — `duration_sec` не попадал в `jobs.params` (проверено в Postgres).
- **Исправление:** `WorkflowMode` сегмент 3/5/10; BFF validate+persist; worker читает из params; после правок — `go build` api/worker + restart dev stack.
- **Чат typing:** убран только `RespondingLabel` (`отвечает..` под пузырём); точки внутри bubble остаются; статус в шапке «думает...» без изменений.
- **Проверка:** `go test ./internal/adapter/inbound/miniapp/...`, `npm run build`, SQL `params->duration_sec`

### FIX-2026-06-07-07 — названия чатов в drawer
- **Симптом:** в списке диалогов «Текст» / дефолтные заголовки вместо первого сообщения.
- **Исправление:** `chat/display.ts` (`displayChatTitle`), merge title с бэкенда,
  профиль: `historyRowTitle` / «Чат» вместо «Текст».
- **Проверка:** `npm run build`

### FIX-2026-06-07-06 — фото: preview + dev worker env
- **Симптом:** генерация фото — вечная «Обработка», нет картинки; 503 при мёртвом туннеле.
- **Причина:** `VK_DELIVERY_MODE=real` из `.env` в dev worker; UI ждал только
  `succeeded`; `null` blob URL → вечная загрузка; tunnel `no tunnel here`.
- **Исправление:** `Start-MiniAppExecutable`, `hasPreviewableMediaResult`,
  `useArtifactMediaUrl`, чат drawer без `job-*` ghost threads.
- **Проверка:** `npm run build`, worker log без `using real vk delivery client`
- **Детали:** также `AUDIT.md` § Mini App image preview + dev launcher fix note

### FIX-2026-06-07-05 — dev launcher `scripts/dev/start-miniapp.ps1`
- **Симптом:** разрозненный `start-miniapp-ngrok.ps1`, логи в `%TEMP%`.
- **Исправление:** `start/stop/status-miniapp.ps1`, `.runtime/vk-miniapp/`.
- **Проверка:** ручной smoke dev stack

### FIX-2026-06-07-04 — 503 на polling после сообщения в чат
- **Симптом:** `GET /miniapp/jobs/{id}` → 503.
- **Причина:** упавший localhost.run туннель, не API.
- **Исправление:** перезапуск туннеля; напоминание обновлять URL в dev.vk.com.
- **Осталось:** health-check туннеля в `status-miniapp.ps1` (опционально)
