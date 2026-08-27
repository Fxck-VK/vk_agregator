# Workspace Scrollbar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the workspace scrollbar start visually lower and keep its dark reference color in every interaction state without moving page content.

**Architecture:** Retain the native `.workspaceScroller` and its `scrollTop = 0`. Apply a Chromium/WebKit-only top margin to the transparent scrollbar track and remove hover/focus color overrides; retain a constant Firefox `scrollbar-color`.

**Tech Stack:** CSS Modules, Vitest, Next.js 16, React 19.

## Global Constraints

- Do not change content padding, panel dimensions, radii, or application scroll state.
- Keep the scrollbar track transparent, the thumb narrow and rounded, and the thumb color `var(--color-border)` in every state.
- Use `calc(var(--space-8) + var(--space-3))` (`2.75rem`) as the Chromium/WebKit track-start inset.
- Do not commit or push without a separate user request.

---

### Task 1: Lock and implement the workspace scrollbar appearance

**Files:**
- Modify: `web/platform/src/components/layout/AppShell/AppShell.scrollbar.test.ts`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.module.css`

**Interfaces:**
- Consumes: `.workspaceScroller` as the existing native vertical scroll container.
- Produces: A constant dark native scrollbar whose visible thumb begins below the rounded workspace top in Chromium/WebKit.

- [ ] **Step 1: Write the failing style test**

Add a `trackRule` extraction and assertions that the track is inset and no muted hover/focus color remains:

```ts
const trackRule = stylesheet.match(/\.workspaceScroller::-webkit-scrollbar-track \{([\s\S]*?)\n\}/)?.[1];

expect(trackRule).toContain("margin-block-start: calc(var(--space-8) + var(--space-3))");
expect(stylesheet).not.toContain("var(--color-text-muted)");
```

- [ ] **Step 2: Run the focused test and verify RED**

Run from `web/platform`:

```powershell
npx vitest run src/components/layout/AppShell/AppShell.scrollbar.test.ts
```

Expected: FAIL because the track has no `margin-block-start` and the stylesheet still contains `var(--color-text-muted)`.

- [ ] **Step 3: Implement the minimal CSS change**

Change the track rule and remove the Firefox and WebKit hover/focus overrides:

```css
.workspaceScroller::-webkit-scrollbar-track {
  margin-block-start: calc(var(--space-8) + var(--space-3));
  background: transparent;
}
```

Keep both default color declarations unchanged:

```css
scrollbar-color: var(--color-border) transparent;
background-color: var(--color-border);
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run from `web/platform`:

```powershell
npx vitest run src/components/layout/AppShell/AppShell.scrollbar.test.ts
```

Expected: the AppShell scrollbar tests pass.

- [ ] **Step 5: Run full frontend verification**

Run from `web/platform`:

```powershell
npm run lint
npm run typecheck
npm test
npm run build
```

Expected: every command exits with code `0` and all Vitest/asset tests pass.

- [ ] **Step 6: Verify visually and inspect Git state**

Open the model catalog at the desktop reference viewport, confirm the thumb is dark and begins below the rounded top while content remains at its top position, then run:

```powershell
git diff --check
git status --short --branch
```

Expected: only the design/plan documents, AppShell CSS, and AppShell scrollbar test are modified; no commit or push is created.
