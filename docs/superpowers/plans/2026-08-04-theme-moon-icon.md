# Filled Moon Theme Icon Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the invisible dark-theme SVG geometry with a reliably rendered filled moon icon.

**Architecture:** Keep the existing `AccountMenu` theme state, accessibility labels, and styles unchanged. Make one visual SVG-path replacement and protect it with a focused component regression test.

**Tech Stack:** TypeScript, React 19, Next.js 16, Vitest, Testing Library, CSS Modules.

## Global Constraints

- Use an inline SVG with no new dependency.
- Keep the existing `1.25rem` icon size and `currentColor` state styling.
- Do not change theme persistence, `aria-pressed`, palettes, backend, or API behavior.

---

### Task 1: Render a filled moon in the dark-theme control

**Files:**
- Modify: `web/platform/src/features/account/AccountControl/AccountControl.test.tsx`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.tsx`

**Interfaces:**
- Consumes: the existing button labelled by `ru.account.darkThemeLabel`.
- Produces: an inline path whose `d` is `M9.528 1.718a.75.75 0 0 1 1.162.81 8.25 8.25 0 0 0 10.78 10.78.75.75 0 0 1 .81 1.163A9.75 9.75 0 1 1 9.528 1.718Z` and whose fill is `currentColor`.

- [ ] **Step 1: Write the failing regression test**

Add an assertion that opens the account menu, locates the dark-theme button, and expects its path to have the exact render-safe moon geometry and `fill="currentColor"`.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npm test -- AccountControl.test.tsx` from `web/platform`.

Expected: FAIL because the existing dark-theme path has different geometry.

- [ ] **Step 3: Replace only the moon path**

Replace the existing dark-theme `<path>` with:

```tsx
<path
  d="M9.528 1.718a.75.75 0 0 1 1.162.81 8.25 8.25 0 0 0 10.78 10.78.75.75 0 0 1 .81 1.163A9.75 9.75 0 1 1 9.528 1.718Z"
  fill="currentColor"
/>
```

- [ ] **Step 4: Verify GREEN and regressions**

Run from `web/platform`:

```text
npm test -- AccountControl.test.tsx
npm test
npm run typecheck
npm run lint
npm run build
```

Expected: every command exits with code 0.

- [ ] **Step 5: Commit and push**

Commit the test and component change, push the resulting `dev-deploy` history, and monitor the DEV workflow through smoke success.
