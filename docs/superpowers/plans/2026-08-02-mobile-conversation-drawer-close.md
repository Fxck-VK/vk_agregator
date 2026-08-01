# Mobile Conversation Drawer Close Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the open mobile navigation drawer immediately when a user selects a recent chat.

**Architecture:** Keep drawer state inside `Sidebar`. The existing conversations slot receives one capture-phase handler that recognizes only normal `/app/chat/...` navigations while the narrow drawer is open, calls the existing close function, and lets the `Link` complete its route transition. No server layout, API contract, or conversation component needs to change.

**Tech Stack:** Next.js App Router, React 19, TypeScript, Vitest, Testing Library.

## Global Constraints

- Narrow is strictly below `48rem`; desktop behavior must remain unchanged.
- The handler applies only to recent-chat links matching `/app/chat/...` inside the existing conversations slot.
- Closing a chat selection must not restore focus to the hamburger button.
- Modified/non-primary pointer activation remains untouched.
- Preserve the existing mobile focus trap, static navigation behavior, component-folder convention, and all visible copy.

---

### Task 1: Close drawer on recent-chat selection

**Files:**
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.test.tsx`

**Interfaces:**
- Consumes: existing `isNarrowViewport`, `isOpen`, `closeNavigation`, and `conversations` slot in `Sidebar`.
- Produces: a capture handler on `conversationsSlot` that closes only an open narrow drawer before the selected `Link` completes normal navigation.

- [ ] **Step 1: Write the failing behavioral test**

Add this test to the existing `Sidebar` suite after the mobile-navigation tests:

```tsx
it("closes the narrow drawer when a recent chat is selected", () => {
  const { panel, trigger } = renderNarrowSidebar({
    conversations: <Link href="/app/chat/recent">Недавний чат</Link>,
  });
  openNavigation(trigger);
  const recentChat = screen.getByRole("link", { name: "Недавний чат" });

  recentChat.addEventListener("click", (event) => event.preventDefault(), { once: true });
  fireEvent.click(recentChat);

  expect(panel).toHaveAttribute("data-open", "false");
  expect(panel).toHaveAttribute("aria-hidden", "true");
  expect(panel).toHaveAttribute("inert");
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
npm.cmd --prefix web/platform test -- --run src/components/layout/Sidebar/Sidebar.test.tsx
```

Expected: the new test fails because clicks inside `conversationsSlot` do not call `closeNavigation`.

- [ ] **Step 3: Add the minimal slot handler**

Extend the React import with `type MouseEvent`. Add this handler inside `Sidebar`, beside `toggleNavigation`:

```tsx
const closeSelectedConversation = (event: MouseEvent<HTMLDivElement>) => {
  if (
    !isNarrowViewport ||
    !isOpen ||
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey ||
    !(event.target instanceof Element) ||
    event.target.closest('a[href^="/app/chat/"]') === null
  ) {
    return;
  }

  closeNavigation();
};
```

Attach it only to the existing slot:

```tsx
{conversations ? (
  <div className={styles.conversationsSlot} onClickCapture={closeSelectedConversation}>
    {conversations}
  </div>
) : null}
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run:

```bash
npm.cmd --prefix web/platform test -- --run src/components/layout/Sidebar/Sidebar.test.tsx
```

Expected: all `Sidebar` tests pass, including the new recent-chat selection scenario.

- [ ] **Step 5: Verify typing and commit**

Run:

```bash
npm.cmd --prefix web/platform run typecheck
npm.cmd --prefix web/platform run lint
```

Then commit only the changed source and test files:

```bash
git add web/platform/src/components/layout/Sidebar/Sidebar.tsx web/platform/src/components/layout/Sidebar/Sidebar.test.tsx
git commit -m "fix: close mobile drawer after chat selection"
```
