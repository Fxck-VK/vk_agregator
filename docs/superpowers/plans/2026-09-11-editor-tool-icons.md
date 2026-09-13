# Editor Tool Icons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use the user-provided brush and lasso SVG files in the editor without changing button layout or state colors.

**Architecture:** Store both SVGs in the existing public UI icon library and consume them as CSS masks. Replace inline SVG functions with decorative spans whose background follows `currentColor`.

**Tech Stack:** React 19, TypeScript, CSS Modules, SVG, Vitest.

## Global Constraints

- Preserve the supplied SVG path data and `viewBox="0 0 24 24"`.
- Keep both icons at `1.2rem` and preserve all existing button states and interactions.
- Do not modify unrelated dirty-worktree changes.

---

### Task 1: Replace inline editor tool artwork with project SVG assets

**Files:**
- Create: `web/platform/public/assets/icons/ui/brush-white.svg`
- Create: `web/platform/public/assets/icons/ui/lasso-white.svg`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.module.css`
- Test: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

**Interfaces:**
- Consumes: the existing selection buttons' `currentColor` states.
- Produces: `.selectionToolIcon`, `.brushIcon`, and `.lassoIcon`, backed by `/assets/icons/ui/brush-white.svg` and `/assets/icons/ui/lasso-white.svg`.

- [x] **Step 1: Add a failing style contract test**

```ts
it("uses the supplied brush and lasso assets without losing button state colors", () => {
  const selectionToolIconRule = editorStylesheet.match(
    /\.selectionToolIcon\s*\{([^}]*)\}/s,
  )?.[1] ?? "";

  expect(editorSource).not.toContain("function BrushIcon");
  expect(editorSource).not.toContain("function LassoIcon");
  expect(editorSource).toContain("styles.brushIcon");
  expect(editorSource).toContain("styles.lassoIcon");
  expect(selectionToolIconRule).toContain("background: currentColor");
  expect(editorStylesheet).toContain('url("/assets/icons/ui/brush-white.svg")');
  expect(editorStylesheet).toContain('url("/assets/icons/ui/lasso-white.svg")');
});
```

- [x] **Step 2: Run the style test and verify RED**

Run: `npm exec -- vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

Expected: FAIL because the inline icon functions still exist and the new CSS mask classes do not.

- [x] **Step 3: Add the assets and mask implementation**

Copy the supplied SVG contents unchanged. Replace each inline component call with:

```tsx
<span
  aria-hidden="true"
  className={`${styles.selectionToolIcon} ${styles.brushIcon}`}
/>
```

Use the corresponding `styles.lassoIcon` class for lasso. Add:

```css
.selectionToolIcon {
  inline-size: 1.2rem;
  block-size: 1.2rem;
  flex: 0 0 auto;
  background: currentColor;
  -webkit-mask: var(--selection-tool-icon) center / contain no-repeat;
  mask: var(--selection-tool-icon) center / contain no-repeat;
}

.brushIcon {
  --selection-tool-icon: url("/assets/icons/ui/brush-white.svg");
}

.lassoIcon {
  --selection-tool-icon: url("/assets/icons/ui/lasso-white.svg");
}
```

- [x] **Step 4: Run focused tests and verify GREEN**

Run: `npm exec -- vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

Expected: all editor component and style tests pass.

- [x] **Step 5: Verify assets and static checks**

Run: `npm run test:assets`

Run: `npm run typecheck`

Run: `npm run lint`

Expected: every command exits successfully with no warnings or errors.

## Verification Results

- RED: the style contract failed while `BrushIcon` and `LassoIcon` were still inline.
- GREEN: 54 of 54 editor component and style tests passed.
- Asset validation: 6 of 6 checks passed.
- The copied brush and lasso SVG contents match the supplied files exactly.
- TypeScript and ESLint exited successfully.
