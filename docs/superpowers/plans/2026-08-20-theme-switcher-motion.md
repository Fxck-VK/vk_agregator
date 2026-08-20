# Theme Switcher Motion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a sliding selected-theme indicator and synchronized palette transitions without changing theme semantics.

**Architecture:** `AccountMenu` publishes its selected theme as a data attribute. CSS positions a single pseudo-element indicator with transforms, while global token-driven colors receive short transitions. Existing reduced-motion rules neutralize the transitions.

**Tech Stack:** TypeScript, React, CSS Modules, Vitest, Testing Library, Next.js.

## Global Constraints

- Keep all three theme buttons and their current accessible labels.
- Use no animation dependency.
- Use `220ms ease` through the existing `--motion-normal` token.
- Respect `prefers-reduced-motion: reduce`.

---

### Task 1: Sliding theme indicator

**Files:**
- Modify: `web/platform/src/features/account/AccountControl/AccountControl.test.tsx`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.styles.test.ts`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.tsx`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.module.css`

**Interfaces:**
- Consumes: `ThemePreference = "system" | "light" | "dark"`.
- Produces: `data-theme-preference` on the theme switcher and a transform-driven decorative indicator.

- [ ] **Step 1: Write failing behavior and style tests**

Assert that clicking dark changes `data-theme-preference` to `dark`, and that the stylesheet contains one `::before` indicator whose transform changes for light and dark.

- [ ] **Step 2: Run tests and verify RED**

Run: `npx vitest run src/features/account/AccountControl/AccountControl.test.tsx src/features/account/AccountMenu/AccountMenu.styles.test.ts --reporter=dot`

Expected: FAIL because the data attribute and indicator rules do not exist.

- [ ] **Step 3: Implement the minimal component and CSS changes**

Add `data-theme-preference={themePreference}`, create the shared pseudo-element, keep buttons above it, and move the indicator with `translateX(calc(100% + var(--space-1)))` and twice that distance.

- [ ] **Step 4: Re-run targeted tests and verify GREEN**

Run the same Vitest command and expect all targeted tests to pass.

### Task 2: Smooth palette transition

**Files:**
- Modify: `web/platform/src/app/globals.theme.test.ts`
- Modify: `web/platform/src/app/globals.css`

**Interfaces:**
- Consumes: current CSS theme tokens.
- Produces: short color/background/border transitions neutralized by the existing reduced-motion media query.

- [ ] **Step 1: Write a failing global-style test**

Assert that theme-driven page surfaces receive `var(--motion-normal)` transitions and that reduced-motion remains defined.

- [ ] **Step 2: Run the test and verify RED**

Run: `npx vitest run src/app/globals.theme.test.ts --reporter=dot`

Expected: FAIL because global color transitions are absent.

- [ ] **Step 3: Implement minimal global transitions**

Add token-driven transitions to `html`, `body`, and common form controls without animating layout properties.

- [ ] **Step 4: Run full verification**

Run: `npm run validate:assets; npm run lint; npm run typecheck; npx vitest run --reporter=dot; npm run build; git diff --check`

Expected: exit code `0` for every command.
