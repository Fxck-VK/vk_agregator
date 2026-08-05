# Message Action Tooltips Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Показать под каждой кнопкой действия сообщения единую фирменную подсказку NeiroHub.

**Architecture:** Все кнопки публикуют локализованную подпись через `data-tooltip`, а один CSS-псевдоэлемент на `.action[data-tooltip]` отвечает за внешний вид и позицию. Существующие обработчики копирования, оценки и пересоздания остаются без изменений.

**Tech Stack:** TypeScript, React 19, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Подсказка появляется под кнопкой, а не над ней.
- Кнопки «Копировать», «Лайк», «Дизлайк» и «Пересоздать» используют одинаковый стиль.
- Нативный атрибут `title` не используется.
- Поведение кнопок, API и backend не меняются.

---

### Task 1: Общий контракт подсказок

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`
- Create: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.styles.test.ts`
- Modify: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx`
- Modify: `web/platform/src/features/conversations/ConversationMessageActions/ConversationMessageActions.module.css`

**Interfaces:**
- Consumes: `ru.conversations.copyMessage`, `ru.conversations.recreateMessage`, `ru.conversations.likeMessage`, `ru.conversations.dislikeMessage`.
- Produces: одинаковый атрибут `data-tooltip` на каждой кнопке и общий CSS-селектор `.action[data-tooltip]::after`.

- [ ] **Step 1: Write the failing DOM test**

Добавить тест, который рендерит пользовательское и ассистентское сообщения и для четырёх кнопок проверяет:

```ts
expect(button).toHaveAttribute("data-tooltip", expectedLabel);
expect(button).not.toHaveAttribute("title");
```

- [ ] **Step 2: Write the failing CSS contract test**

Создать тест, читающий CSS-модуль и проверяющий общий селектор и нижнюю позицию:

```ts
expect(stylesheet).toMatch(/\.action\[data-tooltip\]::after\s*\{/);
expect(stylesheet).toMatch(/inset-block-start:\s*calc\(100% \+ var\(--space-2\)\);/);
expect(stylesheet).not.toMatch(/inset-block-end:\s*calc\(100% \+ var\(--space-2\)\);/);
```

- [ ] **Step 3: Run focused tests and confirm RED**

Run: `npm.cmd test -- ConversationHistory.test.tsx ConversationMessageActions.styles.test.ts`

Expected: FAIL because recreate/like/dislike still use `title`, and CSS targets only the copy button above the icon.

- [ ] **Step 4: Implement the shared tooltip contract**

Set `data-tooltip` on recreate/like/dislike, remove their `title` attributes, remove the copy-only CSS class, and replace its selectors with:

```css
.action[data-tooltip]::after {
  inset-block-start: calc(100% + var(--space-2));
  content: attr(data-tooltip);
}

.action[data-tooltip]:hover::after,
.action[data-tooltip]:focus-visible::after {
  opacity: 1;
  visibility: visible;
}
```

- [ ] **Step 5: Run focused tests and confirm GREEN**

Run: `npm.cmd test -- ConversationHistory.test.tsx ConversationMessageActions.styles.test.ts`

Expected: both test files pass.

- [ ] **Step 6: Run complete frontend verification**

Run from `web/platform`: `npm.cmd run lint`, `npm.cmd run typecheck`, `npm.cmd test`, and `npm.cmd run build`.

Expected: every command exits with code 0.

- [ ] **Step 7: Commit and deploy to DEV**

Stage only the two documents and four implementation/test files, commit with `feat(web): unify message action tooltips`, push `HEAD:dev-deploy`, then monitor CI, image build and DEV deployment until terminal status.

