# User Message Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить копирование и перенос текста пользовательского сообщения в черновик без фокуса, прокрутки и автоматической отправки.

**Architecture:** Отдельный `ConversationMessageActions` рендерит иконки действий. `ConversationHistory` передаёт одноразовый типизированный запрос на замену черновика в контролируемый внутренним состоянием `ConversationComposer`.

**Tech Stack:** TypeScript, React 19, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Кнопки показываются только у сообщений пользователя.
- «Пересоздать» не фокусирует поле, не прокручивает страницу и не отправляет запрос.
- Существующий сценарий отправки сообщений и polling не изменяется.

---

### Task 1: Проверить поведение действий сообщением

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.test.tsx`

**Interfaces:**
- Consumes: существующий `ConversationHistory` и `ConversationComposer`.
- Produces: тестовый контракт для `draftRequest: { id: number; text: string } | null` и кнопок с локализованными accessible names.

- [ ] **Step 1: Write the failing tests**

Добавить проверки точного текста в Clipboard API, замены существующего черновика и сохранения фокуса на кнопке «Пересоздать».

- [ ] **Step 2: Run tests to verify they fail**

Run: `npm test -- src/features/conversations/ConversationHistory/ConversationHistory.test.tsx src/features/conversations/ConversationComposer/ConversationComposer.test.tsx`

Expected: FAIL, потому что кнопок и `draftRequest` ещё нет.

### Task 2: Реализовать действия и связь с композером

**Files:**
- Create: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx`
- Create: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.module.css`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.module.css`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `messageText: string`, `onRecreate: (text: string) => void`.
- Produces: `ConversationMessageActions` и `ComposerDraftRequest`.

- [ ] **Step 1: Write minimal implementation**

Добавить иконки, Clipboard API и запрос на замену черновика; не использовать `focus()`, `scrollIntoView()` или отправку формы.

- [ ] **Step 2: Run focused tests**

Run: `npm test -- src/features/conversations/ConversationHistory/ConversationHistory.test.tsx src/features/conversations/ConversationComposer/ConversationComposer.test.tsx`

Expected: PASS.

- [ ] **Step 3: Run platform validation**

Run: `npm run lint && npm run typecheck && npm test`

Expected: все команды завершаются без ошибок и предупреждений.

