# File Card Hover Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add accessible hover actions over completed cards in “Мои файлы” while keeping media full-bleed and the masonry layout unchanged.

**Architecture:** Keep the existing completed-result branch in `FileCard` and layer presentation-only controls inside the same positioned card. The existing anchor remains the one download action; a disabled delete button communicates the currently unavailable server capability without introducing temporary client-only deletion.

**Tech Stack:** React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Do not change masonry sizing, card side margins, or media aspect ratios.
- Do not add a fake client-only deletion flow.
- Do not commit or publish the changes.
- Run Vitest with no more than four workers.

---

### Task 1: Specify completed-card actions

**Files:**
- Modify: `web/platform/src/features/files/FileCard/FileCard.test.tsx`
- Modify: `web/platform/src/features/files/FileCard/FileCard.styles.test.ts`

**Interfaces:**
- Consumes: `FileCard` completed-result rendering and `ru.files` copy.
- Produces: tests requiring a visible “Скачать” label, an unavailable delete button, and hover/focus/touch CSS behavior.

- [x] **Step 1: Write the failing component assertions**

Update the completed-result test to require the download link to contain visible text matching `ru.files.download`, and require a disabled button named `ru.files.deleteUnavailable`.

- [x] **Step 2: Write the failing style assertions**

Require `.mediaOverlay` to be absolutely inset, initially transparent, and non-interactive; require card hover and focus-within selectors to reveal it; require `.deleteControl` and `.downloadLabel` to be inset controls; require touch and reduced-motion media queries.

- [x] **Step 3: Verify RED**

Run:

```powershell
npm exec vitest run -- src/features/files/FileCard/FileCard.test.tsx src/features/files/FileCard/FileCard.styles.test.ts --maxWorkers=4
```

Expected: failures because the hover overlay, delete label, and CSS rules do not exist.

### Task 2: Implement the hover action layer

**Files:**
- Modify: `web/platform/src/features/files/FileCard/FileCard.tsx`
- Modify: `web/platform/src/features/files/FileCard/FileCard.module.css`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `ru.files.download`, the artifact URL, and the existing full-card download link.
- Produces: `ru.files.deleteUnavailable` and the `mediaOverlay`, `downloadLabel`, and `deleteControl` presentation hooks.

- [x] **Step 1: Add exact Russian copy**

Add `deleteUnavailable: "Удаление файлов пока недоступно"` under `ru.files`.

- [x] **Step 2: Add semantic actions**

Inside the completed-card branch, add an overlay inside the download anchor with a decorative download SVG and the `ru.files.download` label. Add a sibling disabled button with a decorative trash SVG, `aria-label`, and `title` set to `ru.files.deleteUnavailable`.

- [x] **Step 3: Add isolated overlay styles**

Position the overlay and delete button absolutely inside `.mediaCard`. Reveal them on `.mediaCard:hover` and `.mediaCard:focus-within`, keep the controls inset from all edges, and preserve the current media sizing rules. Add touch visibility and reduced-motion overrides.

- [x] **Step 4: Verify GREEN**

Run:

```powershell
npm exec vitest run -- src/features/files/FileCard/FileCard.test.tsx src/features/files/FileCard/FileCard.styles.test.ts --maxWorkers=4
```

Expected: both files pass with no warnings.

### Task 3: Verify the integrated result

**Files:**
- Verify only: `web/platform/src/features/files/FileCard/*`

**Interfaces:**
- Consumes: the implemented card action layer.
- Produces: automated and visual evidence that the feature works without regressions.

- [x] **Step 1: Run all automated checks**

Run the full Vitest suite with `--maxWorkers=4`, then `npm run test:assets`, `npm run typecheck`, and `npm run lint`.

- [x] **Step 2: Inspect in the browser**

Open `/app/files?layout=wide`, hover a completed image, and verify the overlay, the two inset controls, unaltered card dimensions, and correct download href.

- [x] **Step 3: Review the diff**

Confirm that only the FileCard action presentation, copy, tests, and these design documents changed for this slice.

### Task 4: Add direct hover feedback to “Скачать”

**Files:**
- Modify: `web/platform/src/features/files/FileCard/FileCard.styles.test.ts`
- Modify: `web/platform/src/features/files/FileCard/FileCard.module.css`

**Interfaces:**
- Consumes: the existing `.downloadLabel` element inside the full-card download link.
- Produces: a direct hover target that fades the icon and text together without changing layout or the general card-hover state.

- [x] **Step 1: Write the failing CSS contract**

Require `.downloadLabel` to accept pointer events and transition opacity, require `.downloadLabel:hover` to use `opacity: 0.7`, and require the reduced-motion block to remove the label transition.

- [x] **Step 2: Verify RED**

Run:

```powershell
npm exec vitest run -- src/features/files/FileCard/FileCard.styles.test.ts --maxWorkers=4
```

Expected: failure because the download label has no direct hover feedback yet.

- [x] **Step 3: Implement the focused hover state**

Add `pointer-events: auto` and an opacity transition to `.downloadLabel`, then add `.downloadLabel:hover { opacity: 0.7; }`. Include `.downloadLabel` in the existing reduced-motion transition override.

- [x] **Step 4: Verify the focused and complete suites**

Run the focused FileCard tests, the full Vitest suite with at most four workers, asset validation, type checking, and linting. Inspect the hover in the local browser without changing the card dimensions.
