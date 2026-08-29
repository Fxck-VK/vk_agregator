# Unified Home Width and Square Avatar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align every home-page section and footer inner content to the existing `50rem` featured-card frame and make the account avatar a rounded square.

**Architecture:** Reuse the existing `contentFrame` CSS Module class on all eight home sections instead of introducing a second width token. Constrain only `footerInner`, leaving the footer background full width. Change only the avatar radius token in `AccountMenu`.

**Tech Stack:** Next.js, React, CSS Modules, Vitest

## Global Constraints

- Use the existing centered `50rem` content frame as the only home-page content width.
- Preserve section spacing, internal padding, responsive stacking, typography, card styles, and sidebar dimensions.
- Preserve the avatar's `2.5rem` size, initials, and color.
- Do not change production data or routing.

---

### Task 1: Align all home sections and footer content

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Test: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

**Interfaces:**
- Consumes: `styles.contentFrame`, currently used by the hero and featured-model section.
- Produces: eight centered home sections plus a `50rem` footer inner frame.

- [ ] **Step 1: Write the failing contract**

Update the existing alignment test to require eight `styles.contentFrame` uses and add:

```ts
const footerInnerRule = stylesheet.match(/\.footerInner\s*\{[^}]*\}/s)?.[0] ?? "";
expect(componentSource.match(/styles\.contentFrame/g)).toHaveLength(8);
expect(footerInnerRule).toContain("inline-size: min(100%, 50rem)");
```

- [ ] **Step 2: Verify RED**

Run `npm exec -- vitest run src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`.

Expected: FAIL because only two sections use `contentFrame` and the footer still uses `66rem`.

- [ ] **Step 3: Apply the minimal layout change**

Add `styles.contentFrame` to the six remaining section `className` values and change:

```css
.footerInner {
  inline-size: min(100%, 50rem);
}
```

Keep every other declaration unchanged.

- [ ] **Step 4: Verify GREEN**

Run `npm exec -- vitest run src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`.

Expected: all tests pass.

### Task 2: Make the account avatar a rounded square

**Files:**
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.module.css`
- Test: `web/platform/src/features/account/AccountMenu/AccountMenu.styles.test.ts`

**Interfaces:**
- Consumes: the existing `.avatar` rule.
- Produces: the same `2.5rem × 2.5rem` avatar with `border-radius: var(--radius-sm)`.

- [ ] **Step 1: Write the failing contract**

Rename the circular-avatar test and require:

```ts
expect(stylesheet).toMatch(/\.avatar\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s);
```

- [ ] **Step 2: Verify RED**

Run `npm exec -- vitest run src/features/account/AccountMenu/AccountMenu.styles.test.ts`.

Expected: FAIL because `.avatar` still uses `50%`.

- [ ] **Step 3: Apply the minimal avatar change**

Replace only `border-radius: 50%` in `.avatar` with `border-radius: var(--radius-sm)`.

- [ ] **Step 4: Verify GREEN and related checks**

Run:

```powershell
npm exec -- vitest run src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts src/features/account/AccountMenu/AccountMenu.styles.test.ts
npm run lint
npm run typecheck
```

Expected: all commands exit with code `0`.

### Task 3: Local visual and full regression verification

**Files:** No production changes.

**Interfaces:**
- Consumes: the running local preview at `http://localhost:7158/app`.
- Produces: measured evidence for aligned widths and avatar geometry.

- [ ] **Step 1: Reload the local page**

Confirm all eight home sections have the same computed width and centered position.

- [ ] **Step 2: Measure the account avatar**

Confirm the avatar computes to `40×40 px` with an `8 px` radius.

- [ ] **Step 3: Run the full suite**

Run `npm test`, `npm run lint`, and `npm run typecheck`.

- [ ] **Step 4: Commit implementation**

```powershell
git add web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts web/platform/src/features/account/AccountMenu/AccountMenu.module.css web/platform/src/features/account/AccountMenu/AccountMenu.styles.test.ts
git commit -m "style(platform): unify home content width"
```
