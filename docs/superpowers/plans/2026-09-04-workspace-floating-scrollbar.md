# Workspace Floating Scrollbar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the document-level scrollbar in `AppShell` and make the remaining native workspace scrollbar appear as a floating thumb without a visible track.

**Architecture:** Keep `workspaceScroller` as the only scroll owner. Mark `AppShell` with `data-app-shell`, scope the root overflow lock to pages containing that marker, and visually blend the native scrollbar gutter into the workspace background without adding JavaScript.

**Tech Stack:** Next.js 16, React 19, CSS Modules, Vitest

## Global Constraints

- Public pages outside `AppShell` must retain normal document scrolling.
- Native wheel, keyboard, touch, and scrollbar dragging must continue to work.
- Do not add a custom scrollbar library or JavaScript scroll synchronization.
- The unrelated existing change in `web/platform/next-env.d.ts` must remain untouched and unstaged.

---

### Task 1: Single floating workspace scrollbar

**Files:**
- Modify: `web/platform/src/components/layout/AppShell/AppShell.tsx`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.module.css`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.scrollbar.test.ts`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.floating-gap.test.ts`

**Interfaces:**
- Consumes: the existing `AppShell` wrapper and `workspaceScroller` native overflow container.
- Produces: a stable `data-app-shell` DOM marker and CSS that leaves `workspaceScroller` as the only vertical scroll owner.

- [ ] **Step 1: Write the failing scrollbar contract tests**

Assert that the component contains `data-app-shell`, the root-lock selectors use that marker with `overflow: hidden`, `.workspaceScroller` has no right margin, its background matches `var(--color-background)`, and both browser scrollbar tracks are transparent.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `npm test -- src/components/layout/AppShell/AppShell.scrollbar.test.ts src/components/layout/AppShell/AppShell.floating-gap.test.ts`

Expected: FAIL because the marker and root lock are absent and the old right margin remains.

- [ ] **Step 3: Implement the minimal CSS and markup change**

Add `data-app-shell` to the shell wrapper. Add scoped global `html`/`body` overflow locking in the CSS module, remove `margin-inline-end`, set the scroller background to `var(--color-background)`, and retain transparent native tracks with the rounded thumb.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run: `npm test -- src/components/layout/AppShell/AppShell.scrollbar.test.ts src/components/layout/AppShell/AppShell.floating-gap.test.ts`

Expected: both test files pass.

- [ ] **Step 5: Verify the platform and runtime**

Run: `npm test`, `npm run typecheck`, `npm run lint`, and `npm run build` from `web/platform`.

Expected: all commands exit with code 0. In the local browser, `html` and `body` report `overflow: hidden`, the document client height equals the viewport height, and the workspace scroller remains overflowed and scrollable. The Next.js development overlay may still make the root `scrollHeight` numerically larger than its client height.

- [ ] **Step 6: Commit only the scrollbar implementation**

```bash
git add web/platform/src/components/layout/AppShell/AppShell.tsx web/platform/src/components/layout/AppShell/AppShell.module.css web/platform/src/components/layout/AppShell/AppShell.scrollbar.test.ts web/platform/src/components/layout/AppShell/AppShell.floating-gap.test.ts
git commit -m "fix(platform): keep a single floating workspace scrollbar"
```
