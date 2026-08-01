# Chat Composer Interaction Refinement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refine the web chat composer so it is fixed-size, keyboard-first, and uses a three-dot reply-waiting indicator.

**Architecture:** `ConversationHistory` owns the bounded reply refresh and passes its state explicitly to `ConversationComposer`. The composer handles user input and submission, while its CSS module owns the fixed textarea and accessible typing-dot animation. All new visible and accessible copy stays in the Russian dictionary.

**Tech Stack:** Next.js, React, TypeScript, Vitest, Testing Library, CSS Modules.

## Global Constraints

- Keep the existing account-safe `/web/v1` mutation, idempotency, retry, CSRF, and bounded polling behavior unchanged.
- Use the exact Russian placeholder `Задайте вопрос NeiroHub` from `web/platform/src/i18n/ru.ts`.
- Enter submits; Shift+Enter inserts a newline; IME composition must not submit.
- The former accepted-message sentence is not rendered; errors remain visible through the existing safe error message.
- The three-dot indicator is visible only while `ConversationHistory` is waiting for the bounded refresh and has a non-visible Russian accessibility label.
- Preserve unrelated dirty files and stage only files modified by this task.

---

### Task 1: Keyboard-first fixed composer and reply indicator

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.module.css`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- `ConversationComposer` accepts `isAwaitingResponse?: boolean` in addition to its existing props.
- `ConversationHistory` passes `activeRefreshID !== null` as `isAwaitingResponse` and keeps `disabled` for controls.

- [ ] **Step 1: Add failing component tests.**

```tsx
it("submits a non-empty draft when Enter is pressed", async () => {
  // keydown Enter posts the current draft to the exact conversation endpoint.
});

it("leaves Shift+Enter as a newline action without posting", () => {
  // shift keydown does not call webBrowserMutation and does not prevent default.
});

it("shows an accessible three-dot indicator only while awaiting a reply", () => {
  // `isAwaitingResponse` renders a status with three visual dots.
});
```

Update the history test to assert the indicator is present after an accepted send and absent after the assistant reply arrives. Assert the new placeholder literally.

- [ ] **Step 2: Run the focused tests and verify the new tests fail.**

Run: `npm.cmd --prefix web/platform test -- --run src/features/conversations/ConversationComposer/ConversationComposer.test.tsx src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`

Expected: FAIL because Enter has no handler, the old placeholder remains, and no awaiting indicator exists.

- [ ] **Step 3: Add the minimal interaction implementation.**

```tsx
const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
  if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return;
  event.preventDefault();
  void submit();
};
```

Guard the call with the existing pending/disabled/empty validation. Replace accepted feedback with the explicit awaiting prop. Render three `aria-hidden` dots inside a polite status whose `aria-label` comes from `ru.ts`. Set a fixed textarea block size and `resize: none`; animate the dots with staggered CSS delays and disable animation for reduced motion.

- [ ] **Step 4: Run focused tests and static checks.**

Run: `npm.cmd --prefix web/platform test -- --run src/features/conversations/ConversationComposer/ConversationComposer.test.tsx src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`

Run: `npm.cmd --prefix web/platform run lint`

Run: `npm.cmd --prefix web/platform run typecheck`

Expected: PASS.

- [ ] **Step 5: Commit only task files.**

```bash
git add web/platform/src/features/conversations/ConversationComposer web/platform/src/features/conversations/ConversationHistory web/platform/src/i18n/ru.ts docs/superpowers/specs/2026-08-01-chat-composer-interaction-refinement-design.md docs/superpowers/plans/2026-08-01-chat-composer-interaction-refinement.md
git commit -m "feat: refine web chat composer interactions"
```

### Task 2: Verification and protected DEV delivery

**Files:**
- Modify only if Task 1 checks expose a defect.

- [ ] **Step 1: Run the full frontend verification.**

Run: `npm.cmd --prefix web/platform test -- --run`

Run: `npm.cmd --prefix web/platform run build`

Expected: PASS.

- [ ] **Step 2: Check scope before delivery.**

Run: `git -c safe.directory=D:/агрегатор diff --check`

Run: `git -c safe.directory=D:/агрегатор status --short`

Expected: no whitespace errors; unrelated user changes remain unstaged.

- [ ] **Step 3: Push only the task commit to `dev-deploy` and run the existing CI, signed-image, and DEV deployment gates.**

Confirm `https://dev-web.neiirohub.ru` remains behind DEV Basic Auth after deployment. Do not create another tunnel or touch production.
