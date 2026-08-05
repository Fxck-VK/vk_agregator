# Copy Message Feedback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить общую для пользовательских и ассистентских сообщений галочку после копирования и кастомную подсказку NeiroHub.

**Architecture:** Состояние успешного копирования и двухсекундный таймер живут внутри существующего `ConversationMessageActions`. Кнопка публикует текст подсказки через `data-tooltip`, а CSS-модуль визуализирует его псевдоэлементом при наведении или фокусе.

**Tech Stack:** TypeScript, React, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Меняется только общая кнопка копирования сообщений.
- Состояние «Скопировано» длится ровно 2 секунды после успешной записи в буфер.
- При ошибке Clipboard API успешное состояние не показывается.
- Нативный атрибут `title` у кнопки копирования не используется.
- Сервер и API не меняются.

---

### Task 1: Copy feedback and tooltip

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx`
- Modify: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.module.css`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `navigator.clipboard.writeText(messageText)` and `ru.conversations.copyMessage`.
- Produces: transient `copied` UI state, `ru.conversations.copiedMessage`, `data-tooltip`, and `CheckIcon`.

- [ ] **Step 1: Write the failing test**

Add a test that clicks the copy button, resolves Clipboard API, asserts `data-tooltip="Скопировано"`, `aria-label="Скопировано"`, and the check icon, advances fake timers by 2000 ms, then asserts the original label and copy icon. Assert the button never has a `title` attribute.

- [ ] **Step 2: Run test to verify it fails**

Run: `npm.cmd test -- ConversationHistory.test.tsx`

Expected: FAIL because the current button retains `title`, has no success state, and never renders a check icon.

- [ ] **Step 3: Write minimal implementation**

Add `copied` state, a reset timer cleaned up on unmount, and a `CheckIcon`. Update the state only after `writeText` succeeds. Bind the current localized label to `aria-label` and `data-tooltip`, remove `title`, and use a positioned CSS pseudo-element for the NeiroHub tooltip.

- [ ] **Step 4: Run focused test to verify it passes**

Run: `npm.cmd test -- ConversationHistory.test.tsx`

Expected: PASS with successful copy feedback for both message roles.

- [ ] **Step 5: Run frontend verification**

Run: `npm.cmd run lint`, `npm.cmd run typecheck`, `npm.cmd test`, and `.\node_modules\.bin\next.cmd build --webpack`.

Expected: all commands exit with code 0 and the production build completes.

- [ ] **Step 6: Commit the implementation**

Stage only the four implementation/test files and these two documents, then commit with `feat(web): add copied message feedback`.
