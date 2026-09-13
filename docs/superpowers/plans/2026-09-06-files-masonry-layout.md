# Files Masonry Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the row-aligned files grid with a responsive masonry layout that mixes naturally sized cards and fills vertical gaps.

**Architecture:** Keep `FilesGrid` markup and job ordering unchanged. Implement masonry entirely in its CSS module with multi-column flow, a `17rem` preferred column width, shared spacing tokens, and non-breaking list items.

**Tech Stack:** React 19, Next.js 16, CSS Modules, Vitest

## Global Constraints

- Preserve the shared workspace side gutters and the files page `100%` content-frame override.
- Do not add a dependency or JavaScript measurement logic.
- Do not sort or group jobs by card dimensions.
- Keep `var(--space-4)` between cards.

---

### Task 1: Files masonry style contract

**Files:**
- Modify: `web/platform/src/components/layout/WorkspacePageFrame/WorkspacePageFrame.styles.test.ts`
- Modify: `web/platform/src/features/files/FilesGrid/FilesGrid.module.css`

**Interfaces:**
- Consumes: the existing `FilesGrid` ordered `<ol>` and `<li>` markup.
- Produces: responsive CSS multi-column masonry without changing component props or data flow.

- [x] **Step 1: Write the failing style contract**

Replace the row-grid assertion with assertions requiring `column-width: 17rem`, `column-gap: var(--space-4)`, `break-inside: avoid`, full-width list items, and vertical spacing. Also assert that the old `grid-template-columns` rule is absent.

- [x] **Step 2: Run the focused test and verify RED**

Run: `npm test -- src/components/layout/WorkspacePageFrame/WorkspacePageFrame.styles.test.ts`

Expected: FAIL because `FilesGrid.module.css` still uses CSS Grid and lacks the masonry declarations.

- [x] **Step 3: Implement the minimal masonry styles**

Use this shape in `FilesGrid.module.css`:

```css
.grid {
  column-width: 17rem;
  column-gap: var(--space-4);
  margin: 0;
  padding: 0;
  list-style: none;
}

.grid > li {
  inline-size: 100%;
  margin-block-end: var(--space-4);
  break-inside: avoid;
}
```

- [x] **Step 4: Run the focused test and verify GREEN**

Run: `npm test -- src/components/layout/WorkspacePageFrame/WorkspacePageFrame.styles.test.ts`

Expected: the focused Vitest file and asset validation pass.

- [x] **Step 5: Verify the rendered gallery**

Open `/app/files` with local preview mode enabled and confirm tall and short cards are balanced into columns, cards are not split, and shared side gutters are unchanged.

- [x] **Step 6: Run the complete verification suite**

Run: `npm test`

Run: `npm run typecheck`

Run: `npm run lint`

Expected: all commands exit with code `0` and no warnings.
