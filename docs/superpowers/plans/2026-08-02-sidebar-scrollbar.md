# Sidebar scrollbar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Style the chat-list scrollbar in NeiroHub colors without changing sidebar behavior.

**Architecture:** Keep `Sidebar` markup unchanged. Add cross-browser scrollbar declarations only to the existing `conversationsSlot` CSS selector and test the CSS contract as a colocated layout regression.

**Tech Stack:** CSS Modules, TypeScript, Vitest.

## Global Constraints

- Change only the sidebar conversation-list scrollbar; do not style the workspace scrollport.
- Use existing NeiroHub color tokens: `--color-border` at rest and `--color-accent` on hover/focus. Keep the Firefox standard fallback inside `@supports (-moz-appearance: none)` so Chromium/WebKit can use its exact pseudo-element styling.
- No visible WebKit scrollbar buttons or opaque track.
- Do not change sidebar DOM, scroll behavior, navigation, or account layout.

---

### Task 1: Style the sidebar chat-list scrollbar

**Files:**
- Create: `web/platform/src/components/layout/Sidebar/Sidebar.scrollbar.test.ts`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.module.css`

- [ ] **Step 1: Write the failing CSS contract test**

```ts
expect(stylesheet).toContain("scrollbar-width: thin");
expect(stylesheet).toContain("scrollbar-color: var(--color-border) transparent");
expect(stylesheet).toContain(".conversationsSlot::-webkit-scrollbar-button");
expect(stylesheet).toContain("background: var(--color-accent)");
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `npm.cmd --prefix web/platform test -- --run src/components/layout/Sidebar/Sidebar.scrollbar.test.ts`

Expected: FAIL because the custom scrollbar selectors are absent.

- [ ] **Step 3: Add the smallest scoped CSS**

```css
@supports (-moz-appearance: none) {
  .conversationsSlot {
    scrollbar-width: thin;
    scrollbar-color: var(--color-border) transparent;
  }
}

.conversationsSlot::-webkit-scrollbar { inline-size: 0.5rem; }
.conversationsSlot::-webkit-scrollbar-track { background: transparent; }
.conversationsSlot::-webkit-scrollbar-button { display: none; }
.conversationsSlot::-webkit-scrollbar-thumb { background-color: var(--color-border); }
.conversationsSlot:hover::-webkit-scrollbar-thumb { background-color: var(--color-accent); }
```

- [ ] **Step 4: Run focused checks**

Run: `npm.cmd --prefix web/platform test -- --run src/components/layout/Sidebar/Sidebar.scrollbar.test.ts src/components/layout/Sidebar/Sidebar.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/platform/src/components/layout/Sidebar/Sidebar.module.css web/platform/src/components/layout/Sidebar/Sidebar.scrollbar.test.ts
git commit -m "style: refine sidebar scrollbar"
```
