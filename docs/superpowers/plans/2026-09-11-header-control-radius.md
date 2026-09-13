# Header Control Radius Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the shared `0.5rem` corner-radius token to all three workspace-header buttons without changing their other styling or behavior.

**Architecture:** Keep ownership where it already exists: the model trigger remains styled by the model-selector CSS module, while balance and tariff actions remain styled by the header CSS module. Extend the existing header stylesheet contract test so one focused regression check covers all three outer controls.

**Tech Stack:** React, TypeScript, CSS Modules, Vitest

## Global Constraints

- Use `var(--radius-sm)`, whose global value is `0.5rem`.
- Change only the outer border radius of the model trigger, balance button, and tariff button.
- Preserve nested icon radii and all other visual and responsive rules.

---

### Task 1: Unify the workspace-header control radius

**Files:**
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.styles.test.ts`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: the global CSS custom property `--radius-sm: 0.5rem`
- Produces: a consistent `border-radius: var(--radius-sm)` contract for `.trigger`, `.balance`, and `.tariffButton`

- [x] **Step 1: Write the failing style-contract test**

```ts
it("uses the small radius token for all three header controls", () => {
  expect(modelSelectorStylesheet).toMatch(
    /\.trigger\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
  );
  expect(headerStylesheet).toMatch(
    /\.balance\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
  );
  expect(headerStylesheet).toMatch(
    /\.tariffButton\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
  );
});
```

- [x] **Step 2: Run the focused test and verify RED**

Run: `npm exec vitest run -- src/components/layout/WorkspaceHeader/WorkspaceHeader.styles.test.ts -t "uses the small radius token for all three header controls"`

Expected: FAIL because all three controls currently use the pill value `999px`.

- [x] **Step 3: Apply the shared token**

```css
.trigger {
  border-radius: var(--radius-sm);
}

.balance {
  border-radius: var(--radius-sm);
}

.tariffButton {
  border-radius: var(--radius-sm);
}
```

- [x] **Step 4: Run focused verification and verify GREEN**

Run: `npm exec vitest run -- src/components/layout/WorkspaceHeader/WorkspaceHeader.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`

Expected: PASS with `32` passed tests.

Run: `npm exec vitest run -- src/components/layout/WorkspaceHeader/WorkspaceHeader.styles.test.ts -t "styles the tariff action|keeps every floating header control|uses the small radius token|keeps subscription plans"`

Expected: PASS with `4` passed tests; the unrelated legacy `sidebarZIndex` contract remains excluded because its `.sidebar` selector no longer exists.

- [x] **Step 5: Run static verification**

Run: `npm run typecheck`

Expected: exit code `0`.

Run: `npm run lint`

Expected: exit code `0`.

- [ ] **Step 6: Commit the focused change when requested**

```bash
git add web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.styles.test.ts web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css docs/superpowers/specs/2026-09-11-header-control-radius-design.md docs/superpowers/plans/2026-09-11-header-control-radius.md
git commit -m "style: unify workspace header control radius"
```
