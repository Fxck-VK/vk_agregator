# Model Category Wrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show model category pills in a wrapping two-row desktop layout without horizontal scrolling.

**Architecture:** Keep the existing `ModelCatalogToolbar` markup and interaction behavior. Change only its CSS layout contract from a horizontal scroller to a wrapping flex container.

**Tech Stack:** CSS Modules, Vitest, React 19, Next.js 16.

## Global Constraints

- Preserve content-sized category pills and their source order.
- Remove horizontal scrolling and custom scrollbar styling.
- Allow additional wrapping on narrow screens rather than clipping labels.
- Do not change tab semantics or keyboard behavior.

---

### Task 1: Wrap the model categories

**Files:**
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.styles.test.ts`
- Modify: `web/platform/src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.module.css`

**Interfaces:**
- Consumes: the existing `.categoryList` and `.category` CSS classes.
- Produces: a wrapping flex layout with no horizontal scrollbar.

- [ ] **Step 1: Write the failing test**

Replace the horizontal-scroller assertion with assertions for `flex-wrap: wrap`, equal `row-gap` and `column-gap`, and the absence of `overflow-x: auto` and scrollbar selectors.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `npx vitest run src/features/models/ModelsCatalog/ModelsCatalog.styles.test.ts`

Expected: FAIL because `.categoryList` still scrolls horizontally and does not wrap.

- [ ] **Step 3: Write the minimal implementation**

Add `flex-wrap: wrap`, define equal row and column gaps with the shared spacing token, and remove horizontal overflow, bottom scrollbar padding, and WebKit scrollbar rules.

- [ ] **Step 4: Run focused and project verification**

Run the focused Vitest file, then `npm test`, `npm run typecheck`, `npm run lint`, and `git diff --check`.

- [ ] **Step 5: Check the running page**

Open `/app/models` at desktop width and confirm the six category pills occupy two rows without a scrollbar.
