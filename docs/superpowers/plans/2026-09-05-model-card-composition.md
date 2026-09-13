# Model Card Composition Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert model catalog cards to a compact vertical layout matching the approved reference.

**Architecture:** Keep `ModelCard` as the single linked article and retain all current data. Introduce one semantic metadata wrapper so CSS Grid can position the icon and price at the top, text in the middle, and model capabilities at the bottom.

**Tech Stack:** React 19, Next.js 16, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Preserve the full-card link, accessible name, price calculation, quality labels, and reference-support copy.
- Use existing NeiroHub design tokens only.
- Do not add placeholder ratings, user counts, descriptions, or API fields.

---

### Task 1: Restructure and restyle ModelCard

**Files:**
- Modify: `web/platform/src/features/models/ModelCard/ModelCard.test.tsx`
- Modify: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.styles.test.ts`
- Modify: `web/platform/src/features/models/ModelCard/ModelCard.tsx`
- Modify: `web/platform/src/features/models/ModelCard/ModelCard.module.css`

**Interfaces:**
- Consumes: `ImageModel`, `ModelIcon`, and `CreditAmount` without signature changes.
- Produces: the existing linked card with a new internal `.details` metadata group.

- [ ] **Step 1: Write failing component and CSS contract tests**

Require the name to precede the type label, require the compact `spark/price`, `heading`, and `details` grid, and require the details group to align to the card bottom.

- [ ] **Step 2: Verify RED**

Run: `npx vitest run src/features/models/ModelCard/ModelCard.test.tsx src/features/models/ModelsCatalog/ModelsCatalog.styles.test.ts`

Expected: FAIL because the existing heading order and horizontal card grid do not match the new composition.

- [ ] **Step 3: Implement the minimal component structure and CSS**

Place the heading before its supporting type label, wrap qualities and reference copy in `.details`, set the three-row grid, and reduce the minimum card height to `16rem`.

- [ ] **Step 4: Verify GREEN and inspect the browser**

Run the focused tests and inspect `/app/models` at desktop width to confirm two compact columns and the intended content order.

- [ ] **Step 5: Run project verification**

Run `npm test`, `npm run typecheck`, `npm run lint`, and `git diff --check`.
