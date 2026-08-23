# Featured Model Cards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the generic workspace scenario cards with compact, truthful model cards backed by the existing image-model catalogue.

**Architecture:** A focused client component owns catalogue loading and card rendering. `WorkspaceLanding` only composes the section, while the existing cached API loader remains the single source of model names, capabilities, and prices.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Do not change the quick-action rail or any section outside the featured-model block.
- Do not hardcode live prices, ratings, or usage counts.
- Render four neutral icon slots; real model icons will be supplied later.
- Keep `/app/models` and the catalogue API unchanged.

---

### Task 1: Define and test the featured-model presentation

**Files:**
- Create: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.tsx`
- Create: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.module.css`
- Create: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.test.tsx`

**Interfaces:**
- Consumes: `loadImageModelCatalog(): Promise<ImageModelList>` and `ImageModel`.
- Produces: `FeaturedModels(): JSX.Element`.

- [ ] **Step 1: Write the failing component test**

Mock `loadImageModelCatalog` with five DTO models. Assert that only four cards render, their links include the encoded model ID, minimum server price is displayed, quality/reference facts are present, and no rating or launch count is invented.

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm exec vitest run src/features/workspace/FeaturedModels/FeaturedModels.test.tsx`

Expected: FAIL because `FeaturedModels` does not exist.

- [ ] **Step 3: Implement the minimal component**

Load the existing cached catalogue in `useEffect`, render four skeletons while pending, render the first four models after success, and render a quiet unavailable message only on failure or an empty response. Use `/app/image?model=${encodeURIComponent(model.id)}` for each full-card link.

- [ ] **Step 4: Add compact card styles**

Use a two-column desktop grid, one column on narrow screens, neutral icon slots, minimum price in the top-right, compact typography, and subtle hover/focus states. Do not use colored monograms or per-card action text.

- [ ] **Step 5: Run the component test**

Run: `npm exec vitest run src/features/workspace/FeaturedModels/FeaturedModels.test.tsx`

Expected: PASS.

### Task 2: Integrate the component into the workspace landing page

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`
- Modify: `web/platform/src/app/app/layout.test.tsx`

**Interfaces:**
- Consumes: `FeaturedModels` from Task 1.
- Produces: the revised `/app` model section.

- [ ] **Step 1: Change the integration assertions first**

Require «Популярные нейросети» and reject «Один аккаунт — разные сценарии» and «Нейросети для разных задач».

- [ ] **Step 2: Run the integration tests to verify they fail**

Run: `npm exec vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx src/app/app/layout.test.tsx`

Expected: FAIL on the old section heading.

- [ ] **Step 3: Replace the old mapped cards**

Import `FeaturedModels`, simplify the heading copy, keep the `/app/models` CTA, and remove the `primaryTools.map` card grid from this section. Keep `primaryTools` for the unchanged quick-action rail.

- [ ] **Step 4: Remove only obsolete landing-card CSS**

Delete `.modelGrid`, `.modelCard`, `.modelIcon`, and `.cardLink` rules while preserving `.toolIcon`, color accents, and all other landing styles.

- [ ] **Step 5: Run integration tests**

Run: `npm exec vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx src/app/app/layout.test.tsx`

Expected: PASS.

### Task 3: Verify the complete change

**Files:**
- Verify all files from Tasks 1 and 2.

- [ ] **Step 1: Run all targeted tests**

Run: `npm exec vitest run src/features/workspace/FeaturedModels/FeaturedModels.test.tsx src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx src/app/app/layout.test.tsx`

Expected: PASS with no warnings.

- [ ] **Step 2: Run type checking**

Run: `npm run typecheck`

Expected: PASS.

- [ ] **Step 3: Run linting**

Run: `npm run lint`

Expected: PASS with zero warnings.
