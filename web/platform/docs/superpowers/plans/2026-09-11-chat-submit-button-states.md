# Chat Submit Button States Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the shared send button hollow, show its accent border only when text enables submission, and turn its arrow fully white only on hover.

**Architecture:** Keep the public `ChatSubmitButton` API unchanged and express the three states entirely through native `:disabled`, `:not(:disabled)`, and `:hover` selectors. Existing composer consumers remain the source of truth for whether trimmed input text permits submission.

**Tech Stack:** React, Next.js, CSS Modules, Vitest

## Global Constraints

- Preserve the existing button dimensions and `0.5rem` radius token.
- Preserve the shared `send-message-white.svg` asset and Tooltip behavior.
- Do not add runtime state or JavaScript hover handlers.
- Do not commit changes unless the user explicitly requests it.

---

### Task 1: Implement the three visual states

**Files:**
- Modify: `src/components/chat/ChatSubmitButton/ChatSubmitButton.styles.test.ts`
- Modify: `src/components/chat/ChatSubmitButton/ChatSubmitButton.module.css`

**Interfaces:**
- Consumes: `ChatSubmitButtonProps.disabled: boolean`
- Produces: disabled, enabled, and enabled-hover visual states through CSS selectors

- [x] **Step 1: Write the failing stylesheet assertions**

```ts
expect(baseRule).toContain("background: transparent");
expect(disabledRule).toContain("border-color: transparent");
expect(enabledRule).toContain("border-color: var(--color-accent)");
expect(iconRule).toContain("opacity: 0.55");
expect(hoverIconRule).toContain("opacity: 1");
```

- [x] **Step 2: Run the focused test and verify RED**

Run: `npx vitest run src/components/chat/ChatSubmitButton/ChatSubmitButton.styles.test.ts`

Expected: FAIL because the old button uses filled backgrounds and lacks the enabled and hover icon rules.

- [x] **Step 3: Implement the minimal CSS state rules**

```css
.button {
  border-color: transparent;
  background: transparent;
}

.button:disabled {
  border-color: transparent;
  background: transparent;
}

.button:not(:disabled) {
  border-color: var(--color-accent);
}

.button img {
  opacity: 0.55;
}

.button:not(:disabled):hover img {
  opacity: 1;
}
```

- [x] **Step 4: Verify focused and dependent tests**

Run: `npx vitest run src/components/chat/ChatSubmitButton src/components/chat/ChatComposer`

Expected: all selected test files pass.

- [x] **Step 5: Verify static checks and the local interface**

Run: `npm run typecheck` and `npm run lint`.

Expected: both commands exit with code 0. Confirm the disabled and enabled-hover states in the local browser without submitting a message.
