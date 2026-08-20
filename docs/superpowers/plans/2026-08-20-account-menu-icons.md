# Account Menu Icons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Profile, Support, and What's New account-menu glyphs with the three approved SVG artworks.

**Architecture:** Keep the approved artwork in the shared icon library as theme-aware React icons. `AccountMenu` imports those icons while retaining its local wrapper only for the unrelated theme, chevron, and logout glyphs.

**Tech Stack:** TypeScript, React 19, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Preserve all current account-menu behavior, layout, copy, and accessibility labels.
- Replace only Profile, Support, and What's New artwork.
- Icons must inherit `currentColor` and remain decorative.
- Do not commit or push unless the user asks separately.

---

### Task 1: Lock the approved icon contract

**Files:**
- Modify: `web/platform/src/features/account/AccountControl/AccountControl.test.tsx`

**Interfaces:**
- Consumes: accessible labels from `ru.account`.
- Produces: regression assertions for `data-icon="profile"`, `data-icon="support"`, and `data-icon="megaphone"`.

- [x] **Step 1: Write the failing test**

Add assertions that each account-menu action contains its approved shared icon identified by `data-icon`.

- [x] **Step 2: Run test to verify it fails**

Run: `npm test -- src/features/account/AccountControl/AccountControl.test.tsx`

Expected: FAIL because the three approved `data-icon` values are absent.

### Task 2: Add shared icon components and connect the menu

**Files:**
- Create: `web/platform/src/components/icons/ProfileIcon/ProfileIcon.tsx`
- Create: `web/platform/src/components/icons/ProfileIcon/index.ts`
- Create: `web/platform/src/components/icons/SupportIcon/SupportIcon.tsx`
- Create: `web/platform/src/components/icons/SupportIcon/index.ts`
- Create: `web/platform/src/components/icons/MegaphoneIcon/MegaphoneIcon.tsx`
- Create: `web/platform/src/components/icons/MegaphoneIcon/index.ts`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.tsx`

**Interfaces:**
- Consumes: `IconProps` from `src/components/icons/IconProps.ts`.
- Produces: `ProfileIcon`, `SupportIcon`, and `MegaphoneIcon` theme-aware decorative SVG components.

- [x] **Step 1: Implement the three approved artworks**

Each component forwards SVG props, sets `aria-hidden="true"`, `focusable="false"`, a stable `data-icon`, the approved viewBox, and renders approved paths with `currentColor`.

- [x] **Step 2: Replace only the three inline menu glyphs**

Import the shared components in `AccountMenu.tsx`; leave theme, chevron, and logout glyphs unchanged.

- [x] **Step 3: Run the focused test**

Run: `npm test -- src/features/account/AccountControl/AccountControl.test.tsx`

Expected: PASS.

### Task 3: Verify the shared icon library and platform checks

**Files:**
- Modify: `web/platform/src/components/icons/icons.test.tsx`

**Interfaces:**
- Consumes: the three new shared icon exports.
- Produces: direct accessibility, theme-color, and artwork regression coverage.

- [x] **Step 1: Add direct shared-icon assertions**

Verify decorative semantics, stable `data-icon`, approved viewBox, and `fill="currentColor"` paths.

- [x] **Step 2: Run the icon and account tests**

Run: `npx vitest run src/components/icons/icons.test.tsx src/features/account/AccountControl/AccountControl.test.tsx`

Expected: PASS.

- [x] **Step 3: Run static checks**

Run: `npm run lint` and `npm run typecheck`.

Expected: both commands exit successfully with no warnings or type errors.
