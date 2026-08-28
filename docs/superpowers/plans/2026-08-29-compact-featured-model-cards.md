# Compact Featured Model Cards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the featured model cards narrower and simpler, with model-specific descriptions and prices without the `от` prefix.

**Architecture:** Keep the runtime catalogue as the source of names and prices. Add a small presentation-only description map inside `FeaturedModels`, remove the facts row from its markup, and constrain only this component's grid in its CSS module.

**Tech Stack:** React, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Only the featured model cards change.
- Desktop remains a two-column grid; mobile remains one column.
- Icon size, colors, borders, radii, links, catalogue order, API prices, and other platform cards remain unchanged.
- Push is not performed without a separate user request.

---

### Task 1: Simplify featured card content and geometry

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`
- Create: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.styles.test.ts`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.tsx`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.module.css`

**Interfaces:**
- Consumes: the first four `ImageModel` records returned by `loadImageModelCatalog()`.
- Produces: the existing `/app/image?model=<id>` links with simplified visual content.

- [ ] **Step 1: Write failing content assertions**

Update the existing `WorkspaceHome` test to expect `55 звёзд`, all four approved descriptions, and no `1K`, `2K`, `4K`, `Поддерживает референсы`, or `По текстовому запросу` text inside featured cards.

- [ ] **Step 2: Write the failing CSS contract**

Create a test that reads `FeaturedModels.module.css` and requires `.grid` to contain `inline-size: min(100%, 58rem)` plus `margin-inline: auto`, and `.card` to contain `min-block-size: 11.5rem`.

- [ ] **Step 3: Verify RED**

Run: `npm exec -- vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx src/features/workspace/FeaturedModels/FeaturedModels.styles.test.ts`

Expected: FAIL because the prefix, generic descriptions, facts row, and old geometry are still present.

- [ ] **Step 4: Implement the approved descriptions and markup**

Add a description map for Nano Banana 2, Nano Banana Pro, GPT Image 2, and Seedream 4.5; use a safe fallback based on `model.name`; remove `prefix="от"` and the entire facts row.

- [ ] **Step 5: Implement the compact geometry**

Set the grid to `inline-size: min(100%, 58rem)` with `margin-inline: auto`; change the card to two grid rows, `gap: var(--space-3)`, and `min-block-size: 11.5rem`; delete unused `.facts` rules and the mobile height override.

- [ ] **Step 6: Verify GREEN and the full project**

Run the targeted Vitest files, then `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`, `npm run test:packaging`, and `git diff --check`.

- [ ] **Step 7: Verify locally and commit**

Inspect desktop and mobile rendering in the local browser, then commit the focused files locally with message `fix(platform): compact featured model cards` without pushing.
