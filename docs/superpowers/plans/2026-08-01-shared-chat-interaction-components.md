# Shared Chat Interaction Components Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use reusable UI components to enforce fixed chat inputs, keyboard submission, and left-aligned assistant typing indicators across web chat entry points.

**Architecture:** `ChatTextInput` is a controlled presentation component used by both start-chat and conversation forms; each caller owns its API/retry lifecycle. `AssistantTypingIndicator` is a presentation component rendered by `ConversationHistory`, which remains the sole owner of polling and adds a temporary user bubble so the indicator always follows the sent question.

**Tech Stack:** TypeScript, React, Next.js, CSS Modules, Vitest, Testing Library.

## Global Constraints

- User-facing copy remains in `web/platform/src/i18n/ru.ts`.
- Every reusable React component has its own directory, TSX file, CSS module, and colocated test.
- Chat browser calls remain only through `/web/v1`; preserve existing idempotency, safe-response validation, retry, and polling behaviour.
- `Enter` submits only non-empty enabled chat drafts; `Shift+Enter` and IME composition do not submit.
- Do not alter the image-generation configuration textarea or any backend/API contract.
- The assistant typing state is visually in the stream, below the pending user message on the left-aligned assistant side, never in the composer footer.

---

### Task 1: Shared fixed chat input

**Files:**
- Create: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.tsx`
- Create: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.module.css`
- Create: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.module.css`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.module.css`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx`

**Interfaces:**
- Produces `ChatTextInput({ value, onChange, onSend, disabled, placeholder, rows, size })` where `size` is `"compact" | "expanded"`.
- `ConversationComposer` passes its existing guarded `submit` as `onSend` and uses `size="compact"`.
- `WorkspacePrompt` extracts its guarded submit function from the form event, passes it as `onSend`, and uses `size="expanded"`.

- [ ] **Step 1: Write the failing test**

```tsx
it("submits once on Enter and preserves Shift+Enter and IME composition", () => {
  const onSend = vi.fn();
  render(<ChatTextInput onChange={vi.fn()} onSend={onSend} value="draft" />);
  const input = screen.getByRole("textbox");
  fireEvent.keyDown(input, { key: "Enter" });
  fireEvent.keyDown(input, { key: "Enter", shiftKey: true });
  fireEvent.keyDown(input, { key: "Enter", isComposing: true });
  expect(onSend).toHaveBeenCalledTimes(1);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm.cmd --prefix web/platform test -- --run src/components/chat/ChatTextInput/ChatTextInput.test.tsx src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx`

Expected: the new component import and keyboard assertion fail because the shared input does not exist and the workspace textarea has no Enter handler.

- [ ] **Step 3: Write minimal implementation**

```tsx
const submitOnEnter = (event: KeyboardEvent<HTMLTextAreaElement>) => {
  if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return;
  event.preventDefault();
  onSend();
};
```

Create a CSS-module fixed block size per variant and `resize: none`. Move no API request or error/retry state into the new component.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm.cmd --prefix web/platform test -- --run src/components/chat/ChatTextInput/ChatTextInput.test.tsx src/features/conversations/ConversationComposer/ConversationComposer.test.tsx src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx`

Expected: all listed tests pass and existing conversation idempotency tests keep their prior assertions.

Run: `npm.cmd --prefix web/platform run typecheck; npm.cmd --prefix web/platform run lint`

Expected: exit code 0 for both commands.

### Task 2: Typing indicator in the conversation stream

**Files:**
- Create: `web/platform/src/components/chat/AssistantTypingIndicator/AssistantTypingIndicator.tsx`
- Create: `web/platform/src/components/chat/AssistantTypingIndicator/AssistantTypingIndicator.module.css`
- Create: `web/platform/src/components/chat/AssistantTypingIndicator/AssistantTypingIndicator.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.module.css`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.module.css`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`

**Interfaces:**
- Produces `AssistantTypingIndicator({ label })`, which renders a live status with exactly three `aria-hidden` dot elements.
- `ConversationComposer.onAccepted` changes to `(prompt: string) => void` and supplies the normalized accepted prompt.
- `ConversationHistory.beginRefresh(prompt?: string)` records the prompt, starts the existing poll, and clears the optimistic prompt once a matching persisted user record arrives.

- [ ] **Step 1: Write the failing test**

```tsx
it("renders the sent question followed by the left-aligned typing indicator", async () => {
  fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));
  const items = await screen.findAllByRole("listitem");
  expect(items.at(-2)).toHaveTextContent("new question");
  expect(items.at(-1)).toHaveAttribute("data-chat-pending", "assistant");
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm.cmd --prefix web/platform test -- --run src/components/chat/AssistantTypingIndicator/AssistantTypingIndicator.test.tsx src/features/conversations/ConversationComposer/ConversationComposer.test.tsx src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`

Expected: the component import fails and the current footer-level status fails the stream-placement assertion.

- [ ] **Step 3: Write minimal implementation**

```tsx
{optimisticPrompt !== null ? <li className={styles.userMessage}>{optimisticPrompt}</li> : null}
{activeRefreshID !== null ? (
  <li className={styles.assistantMessage} data-chat-pending="assistant">
    <AssistantTypingIndicator label={ru.conversations.composerAwaitingResponse} />
  </li>
) : null}
```

Remove all indicator rendering and animation CSS from `ConversationComposer`. Keep its disabled state while `activeRefreshID` is set. On a fetched user message whose text equals the optimistic prompt, clear the optimistic prompt after it is appended to `messages`.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm.cmd --prefix web/platform test -- --run src/components/chat/AssistantTypingIndicator/AssistantTypingIndicator.test.tsx src/features/conversations/ConversationComposer/ConversationComposer.test.tsx src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`

Expected: all listed tests pass, exactly one question is visible after reconciliation, and the status is absent from the composer.

Run: `npm.cmd --prefix web/platform test -- --run`

Expected: all frontend tests pass.

- [ ] **Step 5: Commit and review**

```powershell
git add web/platform/src/components/chat web/platform/src/features/conversations web/platform/src/features/workspace docs/superpowers/specs/2026-08-01-shared-chat-interaction-components-design.md docs/superpowers/plans/2026-08-01-shared-chat-interaction-components.md
git commit -m "feat: share chat interaction components"
```

Run a task review against the base commit before publishing the DEV branch.
