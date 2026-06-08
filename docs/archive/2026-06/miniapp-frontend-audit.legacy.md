# Mini App Frontend Audit

Дата: 2026-06-04.

Status: historical audit. Mini App chat local persistence was superseded by
PR-18.4/18.5: backend conversation list/history endpoints are now the source
of truth, and frontend `localStorage` keeps only active thread/tab/theme UI
state plus legacy cache cleanup.

Область: `web/miniapp/src` после UI-итерации с историей чатов, выбором
модальности/модели, графитовой темой `#1A1A1D` и фиксом scrollbar в composer.

## Безопасность

- **OK — секреты и токены.** В Mini App фронте нет захардкоженных токенов или
  секретов. VK user info используется только для отображения имени/аватара.
- **OK — launch params.** `X-Launch-Params` берётся только из
  `window.location.search` в `src/api/client.ts`; авторитетная проверка остаётся
  на backend.
- **OK — безопасный рендер текста.** `dangerouslySetInnerHTML`, `eval` и
  `new Function` не используются.
- **OK — media src.** `img`/`video` для результатов строятся только через
  `artifactUrl(id)`, который валидирует UUID artifact id. Произвольные URL из
  ответа не вставляются.
- **OK — логирование.** Нет `console.log` с launch params или пользовательскими
  данными.

## Утечки И Жизненный Цикл

- **OK — VK Bridge.** `useBridge` подписывается на `bridge.subscribe` и снимает
  подписку через `bridge.unsubscribe(handleConfigUpdate)` в cleanup.
- **OK — polling.** `ChatScreen` хранит `mountedRef`, не создаёт повторный poll
  для одного `jobId`, останавливается при терминальном статусе и прекращает
  обновления после unmount.
- **OK — timers.** Polling использует ограниченный цикл (`POLL_MAX`) и паузу
  `POLL_MS`; бесконечных таймеров нет.
- **OK — localStorage.** `loadChats`/`saveChats` обёрнуты в `try/catch`, сбои
  storage/quota/private mode не ломают UI.

## Оптимизация И UX

- **OK — immutable updates.** Сообщения обновляются иммутабельно по id через
  `patchInChat`/`setMessages`.
- **OK — лишние ререндеры.** Ключевые callbacks (`patchInChat`,
  `refreshBalance`, `poll`, `startPoll`) мемоизированы; polling dedupe хранится
  в `pollingRef`.
- **OK — автоскролл.** Автоскролл срабатывает только при изменении
  `activeChat?.messages`.
- **OK — лимит истории.** `saveChats` ограничивает историю `MAX_CHATS = 50`.
- **OK — composer scrollbar.** Textarea сохраняет внутреннюю прокрутку, но
  нативный scrollbar скрыт через `scrollbar-width`, `-ms-overflow-style` и
  `::-webkit-scrollbar`.

## TODO / Известные Ограничения

- **TODO — выбранная модель.** Выбор модели пока UI-only: в текущий
  `POST /miniapp/jobs` уходит только `operation` и `prompt`. Нужна отдельная
  backend/API договорённость для передачи model id.
- **TODO — artifacts route.** Фронт готов загружать text/media через
  `/miniapp/artifacts/{id}`, но backend route остаётся follow-up.
