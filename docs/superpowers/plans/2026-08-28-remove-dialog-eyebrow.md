# Remove Dialog Eyebrow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the blue uppercase `Диалог` marker from loaded and pending chat pages without changing conversation behavior.

**Architecture:** Delete the label at both render sites and remove the shared presentation styles and translation that become unused. Existing conversation section labels remain available to assistive technology, so no replacement element is required.

**Tech Stack:** React 19, Next.js 16, TypeScript, CSS Modules, Vitest, Testing Library

## Global Constraints

- Preserve messages, the model selector, chat controls, loading and error states, and all conversation data flow.
- Preserve `aria-label={ru.conversations.historyTitle}` on the conversation sections.
- Remove the empty header wrappers so they do not reserve spacing.
- Preserve all previously uncommitted brand-eyebrow changes.
- Do not commit or push until the user explicitly requests it.

---

### Task 1: Remove the dialog eyebrow from every chat state

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`
- Modify: `web/platform/src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.module.css`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `ru.conversations.historyTitle`, existing conversation history and pending bootstrap props
- Produces: loaded and pending chat layouts without a visible `Диалог` marker; no new public interface

- [ ] **Step 1: Add failing assertions for both states**

In the first render test of each test file, add:

```tsx
expect(screen.queryByText("Диалог", { exact: true })).toBeNull();
```

Keep the existing assertions that messages, typing state, and accessibility labels still render.

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
npx vitest run src/features/conversations/ConversationHistory/ConversationHistory.test.tsx src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.test.tsx
```

Expected: one new assertion in each file fails because the current header renders `Диалог`.

- [ ] **Step 3: Implement the minimal removal**

Delete this complete block from both production components:

```tsx
<header className={styles.header}>
  <p className={styles.eyebrow}>{ru.conversations.historyEyebrow}</p>
</header>
```

Delete only these now-unused CSS rules from `ConversationHistory.module.css`:

```css
.header {
  margin-block-end: var(--space-6);
}

.eyebrow,
```

and:

```css
.eyebrow {
  margin-block-end: var(--space-2);
  color: var(--color-accent);
  font-size: 0.875rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
```

Delete `historyEyebrow: "Диалог",` from `ru.conversations`.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the same Vitest command. Expected: both files and all their tests pass.

- [ ] **Step 5: Run project verification**

Run:

```bash
npm run lint
npm run typecheck
npm test
npm run build
git diff --check
```

Expected: every command exits successfully, the existing uncommitted changes remain, and no unrelated files are modified.
