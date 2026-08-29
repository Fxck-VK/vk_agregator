# All-models Button PNG Background Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the popular-models “Все нейросети” button’s solid fill with the supplied transparent PNG while retaining code-controlled geometry and accessible text.

**Architecture:** Register the PNG in the shared asset map and render it with Next.js `Image` as an absolute decorative layer inside the existing link. Keep a separate foreground label and use CSS Modules for stacking, stretching, and hover brightness.

**Tech Stack:** Next.js, React, TypeScript, CSS Modules, Vitest

## Global Constraints

- Change only the `.primaryButton` in the popular-models section.
- Preserve the label “Все нейросети” and `/app/models` destination.
- Preserve the existing minimum height, padding, pill radius, font weight, and upward hover translation.
- Keep all button dimensions controlled by CSS rather than the source PNG dimensions.
- The PNG must remain decorative and must not change the link’s accessible name.
- Do not modify or stage the unrelated generated `web/platform/next-env.d.ts` change.

---

### Task 1: Add the optimized button artwork layer

**Files:**
- Create: `web/platform/public/assets/images/workspace/all-models-button-background.png`
- Modify: `web/platform/src/assets/asset-paths.ts`
- Modify: `web/platform/src/assets/asset-paths.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Test: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

**Interfaces:**
- Consumes: Next.js `Image`, `assetPaths`, `.primaryButton`, and the supplied 2172 × 724 transparent PNG.
- Produces: `assetPaths.images.workspace.allModelsButtonBackground`, `.primaryButtonBackground`, and `.primaryButtonLabel`.

- [ ] **Step 1: Write failing contract tests**

Add assertions for `/assets/images/workspace/all-models-button-background.png`, a decorative `Image` using `.primaryButtonBackground`, a foreground `.primaryButtonLabel`, a transparent `.primaryButton` background, and a hover rule that applies `filter: brightness(...)` only to the artwork.

- [ ] **Step 2: Verify RED**

Run `npm exec -- vitest run src/assets/asset-paths.test.ts src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts` from `web/platform`.

Expected: FAIL because the asset path and the two button layers do not exist.

- [ ] **Step 3: Implement the minimal artwork layer**

Copy the supplied PNG to the public workspace-assets directory, register the path, render an `Image` with `alt=""`, `fill`, `.primaryButtonBackground`, and `sizes="12rem"`, wrap the label in `.primaryButtonLabel`, and update the button styles to stack both layers with a transparent base.

- [ ] **Step 4: Verify GREEN**

Run `npm exec -- vitest run src/assets/asset-paths.test.ts src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`.

Expected: both test files PASS.

- [ ] **Step 5: Verify the visual result locally**

Reload `http://localhost:7158/app`, inspect the button at rest and on hover, and adjust only `.primaryButtonBackground` scaling or object fit if the artwork does not fill the existing code-sized button cleanly.

- [ ] **Step 6: Run full verification**

Run `npm test`, `npm run lint`, and `npm run typecheck` from `web/platform`.

Expected: all commands exit successfully.

- [ ] **Step 7: Commit**

Stage only the new PNG, asset map, component, CSS, tests, spec, and plan. Commit with `style(platform): use artwork for all-models button`.
