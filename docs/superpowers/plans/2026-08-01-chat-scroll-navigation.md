# Chat scroll navigation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add shared scroll-to-latest navigation to ordinary NeiroHub text chats without interrupting users reading older messages.

**Architecture:** `ChatScrollToBottom` receives the existing workspace scrollport, a content version and a force-scroll counter. It owns bottom detection and the accessible round arrow button. `ConversationHistory` owns scroll requests and supplies the state to `ConversationComposer`, which anchors the visual component above its fixed input.

**Tech Stack:** TypeScript, React 19, Next.js, CSS Modules, Vitest, React Testing Library.

## Global Constraints

- Use one component folder containing TSX, CSS Module and colocated test.
- All visible strings come from `web/platform/src/i18n/ru.ts`.
- Use only the existing `main[data-testid="workspace-scroll-region"]` scrollport.
- The button appears whenever the user is away from bottom, independent of message arrival.
- Do not modify backend/API contracts, polling limits, image generation or the existing text-input keyboard contract.

---

### Task 1: Shared `ChatScrollToBottom` component

**Files:**
- Create: `web/platform/src/components/chat/ChatScrollToBottom/ChatScrollToBottom.tsx`
- Create: `web/platform/src/components/chat/ChatScrollToBottom/ChatScrollToBottom.module.css`
- Create: `web/platform/src/components/chat/ChatScrollToBottom/ChatScrollToBottom.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `scrollContainer: HTMLElement | null`, `contentVersion: string`, `forceScrollRequest: number`.
- Produces: a button named `ru.conversations.scrollToLatest` only while the container is more than one CSS pixel above its bottom.

- [ ] **Step 1: Write failing component tests**

```tsx
render(<ChatScrollToBottom contentVersion="1" forceScrollRequest={0} scrollContainer={region} />);
region.scrollTop = 200;
fireEvent.scroll(region);
expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();
fireEvent.click(screen.getByRole("button", { name: ru.conversations.scrollToLatest }));
expect(region.scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: 1200 });
```

- [ ] **Step 2: Run component test and verify it fails because the component does not exist**

Run: `npm.cmd --prefix web/platform test -- --run src/components/chat/ChatScrollToBottom/ChatScrollToBottom.test.tsx`

Expected: FAIL with a module-not-found error for `ChatScrollToBottom`.

- [ ] **Step 3: Implement the minimum bottom detection and button**

```tsx
const isAtBottom = scrollContainer.scrollHeight - scrollContainer.scrollTop - scrollContainer.clientHeight <= 1;

if (isAtBottom) return null;

return <button aria-label={ru.conversations.scrollToLatest} onClick={scrollToLatest} type="button">…</button>;
```

Use a `scroll` listener with cleanup, native button semantics, an `aria-hidden` SVG arrow and CSS that centers the control above its fixed composer host.

- [ ] **Step 4: Run component test and verify it passes**

Run: `npm.cmd --prefix web/platform test -- --run src/components/chat/ChatScrollToBottom/ChatScrollToBottom.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/platform/src/components/chat/ChatScrollToBottom web/platform/src/i18n/ru.ts
git commit -m "feat: add chat scroll navigation"
```

### Task 2: Integrate the shared component with normal chats

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx`
- Modify: `web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.module.css`

**Interfaces:**
- Consumes: `ChatScrollToBottom` from Task 1.
- Produces: `ConversationHistory` increments `forceScrollRequest` after a safe accepted send and provides a `contentVersion` that changes only for pending/new latest records.

- [ ] **Step 1: Write failing integration tests**

```tsx
fireEvent.keyDown(textarea, { key: "Enter" });
await screen.findByText("Pending stream prompt");
expect(region.scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: region.scrollHeight });

region.scrollTop = 100;
fireEvent.scroll(region);
resolveRefresh(Response.json({ items: [assistantReply] }));
await screen.findByText(assistantReply.text);
expect(region.scrollTo).not.toHaveBeenCalled();
expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();
```

- [ ] **Step 2: Run integration tests and verify they fail before the integration**

Run: `npm.cmd --prefix web/platform test -- --run src/features/conversations/ConversationHistory/ConversationHistory.test.tsx`

Expected: FAIL because the composer has no scroll control and an accepted send does not scroll the workspace.

- [ ] **Step 3: Implement the smallest integration**

```tsx
const [forceScrollRequest, setForceScrollRequest] = useState(0);

const beginRefresh = (prompt: string) => {
  setForceScrollRequest((request) => request + 1);
  // retain the existing polling setup
};

<ConversationComposer
  contentVersion={`${messages.at(-1)?.id ?? ""}:${pendingTurn?.id ?? ""}:${activeRefreshID ?? ""}`}
  forceScrollRequest={forceScrollRequest}
  scrollContainer={workspaceScrollRegion}
  {...existingProps}
/>
```

Look up the workspace `main` only on the client, retain its element in state, and do not alter API requests or poll timing.

- [ ] **Step 4: Run focused tests and verify they pass**

Run: `npm.cmd --prefix web/platform test -- --run src/features/conversations/ConversationHistory/ConversationHistory.test.tsx src/features/conversations/ConversationComposer/ConversationComposer.test.tsx src/components/chat/ChatScrollToBottom/ChatScrollToBottom.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/platform/src/features/conversations/ConversationHistory web/platform/src/features/conversations/ConversationComposer
git commit -m "feat: scroll chat to latest messages"
```

## Verification

- [ ] Run the full frontend test suite: `npm.cmd --prefix web/platform test`.
- [ ] Run typecheck: `npm.cmd --prefix web/platform run typecheck`.
- [ ] Run lint: `npm.cmd --prefix web/platform run lint`.
- [ ] Run build: `npm.cmd --prefix web/platform run build`.
- [ ] Run packaging check: `npm.cmd --prefix web/platform run test:packaging`.
- [ ] Request a code review before DEV deployment.
