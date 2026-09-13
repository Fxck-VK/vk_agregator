# Shared Model Card Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render the same model-card component and approved popular-card style on the landing page and in the full catalog.

**Architecture:** Move model presentation copy and the popular card visual structure into `ModelCard`. Replace the duplicate JSX and CSS in `FeaturedModels` with calls to the shared component while leaving section-level loading and reveal behavior local.

**Tech Stack:** React 19, Next.js 16, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Both surfaces consume `loadImageModelCatalog` and identify models by stable `id`.
- Keep the popular section's 4/6 reveal behavior and reduced-motion support.
- Keep URL encoding, accessible names, and disabled prefetch behavior.
- Do not change API contracts or invent ratings and usage counts.

---

### Task 1: Define the shared card presentation

**Files:**
- Create: `web/platform/src/features/models/ModelCard/model-card-content.ts`
- Modify: `web/platform/src/features/models/ModelCard/ModelCard.test.tsx`
- Modify: `web/platform/src/features/models/ModelCard/ModelCard.tsx`
- Modify: `web/platform/src/features/models/ModelCard/ModelCard.module.css`

**Interfaces:**
- Produces: `getModelDescription(model: ImageModel): string`.
- Produces: `ModelCard({ model, className?, revealed?, testId? })`.

- [ ] **Step 1: Write failing tests for approved descriptions, minimum price, and compact markup**
- [ ] **Step 2: Run the focused tests and confirm RED**
- [ ] **Step 3: Implement the id-keyed copy helper and compact shared card**
- [ ] **Step 4: Run the focused tests and confirm GREEN**

### Task 2: Migrate FeaturedModels to the shared component

**Files:**
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.test.tsx`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.styles.test.ts`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.tsx`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

**Interfaces:**
- Consumes: the shared `ModelCard` props from Task 1.
- Preserves: `featured-model-card`, `data-revealed`, and reveal animation classes.

- [ ] **Step 1: Write failing source/style tests rejecting duplicate card markup**
- [ ] **Step 2: Run focused tests and confirm RED**
- [ ] **Step 3: Replace duplicate card JSX/CSS and preserve skeleton/reveal behavior**
- [ ] **Step 4: Run focused tests and confirm GREEN**

### Task 3: Align catalog and global contracts

**Files:**
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.test.tsx`
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.styles.test.ts`
- Modify: `web/platform/src/app/typography.contract.test.ts`
- Modify: `web/platform/src/app/palette-surfaces.contract.test.ts`

**Interfaces:**
- Consumes: the shared card without catalog-specific visual metadata.

- [ ] **Step 1: Write failing assertions for identical description and price presentation**
- [ ] **Step 2: Run focused tests and confirm RED**
- [ ] **Step 3: Update shared style-contract ownership and remove obsolete expectations**
- [ ] **Step 4: Run focused tests and confirm GREEN**
- [ ] **Step 5: Compare the same model in both browser surfaces**
- [ ] **Step 6: Run `npm test`, `npm run typecheck`, `npm run lint`, and `git diff --check`**
