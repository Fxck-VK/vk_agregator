# Model Selector Sections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add five ordered content sections to the model selector without inventing unsupported models.

**Architecture:** Derive stable popular model IDs from the first two unfiltered catalogue entries, filter real models with the existing query, and then assign each match to exactly one section. Render the five semantic sections inside the existing scroll area while leaving the search, footer, selection behavior, and routing unchanged.

**Tech Stack:** React 19, Next.js 16, TypeScript, CSS Modules, Testing Library, Vitest

## Global Constraints

- Render sections in this exact order: `Популярные`, `Изображения`, `Текст`, `Видео`, `Аудио`.
- Keep the first two catalogue models in `Популярные` and the remaining real models in `Изображения`.
- Render every real model exactly once.
- Do not invent text, video, or audio models or routes.
- Keep search and the catalogue link fixed outside the scrolling section stack.
- Preserve existing selection, navigation, motion, and accessibility behavior.
- Do not commit or publish the shared worktree as part of this task.

---

### Task 1: Group the selector into five sections

**Files:**
- Modify: `web/platform/src/i18n/ru.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: `ImageModel[]`, the existing `filteredModels`, and the catalogue's stable order.
- Produces: five semantic selector sections whose populated lists retain the existing model option buttons.

- [x] **Step 1: Write failing component and style tests**

Extend the test catalogue to three models. Require the five section headings in order, require the first two model buttons inside the `Популярные` section, require the third inside `Изображения`, require no duplicate model buttons, and require `Скоро появятся` in each unsupported section. Require `.modelSections` to use a grid with `gap: var(--space-4)` and `.sectionEmpty` to use muted compact text.

- [x] **Step 2: Run focused tests and verify RED**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: FAIL because the selector currently renders only one `Изображения` heading and has no section-stack styles.

- [x] **Step 3: Implement grouping, copy, and presentation**

Add localized category labels and `Скоро появятся`. Derive the first two catalogue IDs as popular before applying the search filter, build five ordered section records, and render each record as a semantic `<section>` with a heading, an accessible list when populated, or compact empty copy otherwise. Add the section-stack CSS without changing the existing scroll container.

- [x] **Step 4: Run focused tests and verify GREEN**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: both files pass with no warnings.

- [x] **Step 5: Verify locally and run full checks**

Open the local selector and confirm all five headings appear in order, the six real models are distributed between the first two sections, and the final three sections use compact empty states. Then run `npx vitest run --maxWorkers=4`, `npm run test:assets`, `npm run typecheck`, and `npm run lint`; every command must exit with code `0`.
