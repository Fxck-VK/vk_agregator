# Remove Rectangle Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Completely remove the `Квадрат` selection tool from the file image editor.

**Architecture:** Delete the rectangle UI, type value, icon, SVG branch, and pointer branches from the existing editor component. Preserve the shared controller and simplify the remaining pointer flow for brush, eraser, and lasso.

**Tech Stack:** React 19, TypeScript, CSS Modules, SVG masks, Vitest, Testing Library.

## Global Constraints

- Preserve brush, lasso, eraser, shared history, and mask behaviour.
- Leave no dormant rectangle implementation in `web/platform/src`.
- Do not add dependencies or modify unrelated dirty-worktree files.

---

### Task 1: Remove the rectangle selection tool

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.module.css`

**Interfaces:**
- Consumes: the existing `FileEditorController`, selection tool group, pointer handlers, and SVG mask.
- Produces: an editor whose available tools are exactly `brush`, `eraser`, and `lasso`, with only `Кисть` and `Лассо` in the selection button group.

- [x] **Step 1: Make the existing component test require the button's absence**

```tsx
expect(within(infoPanel).queryByRole("button", { name: "Квадрат" })).toBeNull();
expect(within(infoPanel).getByRole("button", { name: "Лассо" })).toHaveAttribute(
  "aria-pressed",
  "false",
);
```

Remove rectangle interactions from the general tool-toggle test while retaining brush, eraser, and lasso mutual-exclusion assertions.

- [x] **Step 2: Run the focused test and verify RED**

Run: `npm exec -- vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: FAIL because the accessible `Квадрат` button still exists.

- [x] **Step 3: Remove rectangle production code**

Use the reduced tool union:

```tsx
type EditTool = "brush" | "eraser" | "lasso";
```

Delete `RectangleIcon`, its button, and the `<rect data-edit-tool="rectangle">` rendering branch. Replace rectangle-specific pointer conditions with one cursor condition:

```tsx
const usesBrushCursor = controller.tool === "brush" || controller.tool === "eraser";
```

All remaining draft strokes append pointer points. Set `.selectionModes` to `repeat(2, minmax(0, 1fr))`.

- [x] **Step 4: Run the focused test and verify GREEN**

Run: `npm exec -- vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: all component tests pass.

- [x] **Step 5: Verify removal and regressions**

Run: `npm exec -- vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

Run: `npm run typecheck`

Run: `npm run lint`

Run: `rg -n "rectangle|RectangleIcon|Квадрат" web/platform/src --glob '!*.test.*'`

Expected: tests, type checking, and linting exit successfully; the final production-source search returns no matches. The component test retains `Квадрат` only in `queryByRole(...).toBeNull()`.
