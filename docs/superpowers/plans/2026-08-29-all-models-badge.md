# All-models 90+ Badge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the supplied tilted `90+` image as a decorative sticker protruding from the transparent “Все нейросети” arrow tile.

**Architecture:** Register the PNG in the existing centralized asset map, render it with the existing Next.js `Image` component inside the arrow tile, and position it absolutely relative to that tile. Preserve the current link semantics and hover behavior.

**Tech Stack:** Next.js, React, TypeScript, CSS Modules, Vitest

## Global Constraints

- Do not rotate or redraw the supplied PNG.
- Keep the arrow tile at 56 × 56 px with a 16 px radius.
- Keep the tile transparent at rest and use the existing accent border on hover and `:focus-visible`.
- The badge is decorative and must not change the link’s accessible name.
- Do not change the tool rail grid or destination URL.

---

### Task 1: Add and position the 90+ badge

**Files:**
- Create: `web/platform/public/assets/images/workspace/all-models-90-plus.png`
- Modify: `web/platform/src/assets/asset-paths.ts`
- Modify: `web/platform/src/assets/asset-paths.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Test: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

**Interfaces:**
- Consumes: `assetPaths`, Next.js `Image`, and the existing `.arrowIcon` tile.
- Produces: `assetPaths.images.workspace.allModelsBadge` and the `.modelCountBadge` decorative image style.

- [ ] **Step 1: Write the failing contract tests**

Add assertions that the asset map contains `/assets/images/workspace/all-models-90-plus.png`, the component renders an `Image` with `alt=""` and `.modelCountBadge`, and the stylesheet positions the badge absolutely with negative block and inline offsets.

- [ ] **Step 2: Run the focused tests to verify RED**

Run: `npm exec -- vitest run src/assets/asset-paths.test.ts src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Expected: FAIL because `allModelsBadge` and `.modelCountBadge` do not exist.

- [ ] **Step 3: Add the asset and minimal implementation**

Copy the supplied PNG to `public/assets/images/workspace/all-models-90-plus.png`, register it as `assetPaths.images.workspace.allModelsBadge`, render it inside `.arrowIcon`, set `.arrowIcon` to `position: relative`, and absolutely position `.modelCountBadge` over the upper-left corner without changing the existing tile geometry.

- [ ] **Step 4: Run focused tests to verify GREEN**

Run: `npm exec -- vitest run src/assets/asset-paths.test.ts src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Expected: both files PASS.

- [ ] **Step 5: Verify locally and tune only badge size/offset**

Reload `http://localhost:7158/app`, inspect the badge at rest and on hover, and adjust only `.modelCountBadge` size and offsets until the sticker protrudes cleanly without covering the arrow.

- [ ] **Step 6: Run full verification**

Run `npm test`, `npm run lint`, and `npm run typecheck` from `web/platform`.

Expected: all commands exit successfully.

- [ ] **Step 7: Commit**

Stage the asset, source, stylesheet, tests, spec, and plan. Commit with `style(platform): add all-models count badge`.
