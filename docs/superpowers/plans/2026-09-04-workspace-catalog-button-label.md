# Workspace Catalog Button Label Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the catalogue button's black label with a light, readable label that fits the dark NeiroHub interface.

**Architecture:** Keep the existing shared `catalogAction` component and PNG background. Change only its CSS color and add a subtle text shadow, protected by the existing static style contract test.

**Tech Stack:** Next.js 16, React 19, CSS Modules, Vitest

## Global Constraints

- Apply the same label treatment to `Показать ещё` and `Все нейросети`.
- Use `#f5f5f7` and `0 0.0625rem 0.2rem rgb(0 0 0 / 45%)` exactly.
- Do not change the PNG background, button geometry, click behavior, or hover movement.
- Do not stage unrelated existing workspace changes.

---

### Task 1: Light catalogue action label

**Files:**
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.module.css`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.styles.test.ts`

**Interfaces:**
- Consumes: `.catalogAction` used by the shared `CatalogActionContent` component.
- Produces: a light catalogue action label with a dark readability shadow.

- [ ] **Step 1: Write the failing CSS contract test**

Add expectations that `.catalogAction` contains `color: #f5f5f7` and `text-shadow: 0 0.0625rem 0.2rem rgb(0 0 0 / 45%)`, and no longer uses `var(--color-text-on-accent)`.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npx vitest run src/features/workspace/FeaturedModels/FeaturedModels.styles.test.ts`

Expected: FAIL because the action still uses `var(--color-text-on-accent)` and has no text shadow.

- [ ] **Step 3: Implement the minimal style change**

Set the `.catalogAction` color to `#f5f5f7` and add `text-shadow: 0 0.0625rem 0.2rem rgb(0 0 0 / 45%)` without modifying any other action styles.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run: `npx vitest run src/features/workspace/FeaturedModels/FeaturedModels.styles.test.ts`

Expected: the test file passes.

- [ ] **Step 5: Verify and commit**

Run `npm test`, `npm run typecheck`, `npm run lint`, and `npm run build`. Stage only the two implementation files and commit with `style(platform): lighten catalogue button label`.
