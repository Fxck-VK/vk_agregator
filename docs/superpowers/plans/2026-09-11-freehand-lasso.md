# Freehand Lasso Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a toggleable freehand lasso that visibly follows the held pointer and creates closed purple mask selections in the file image editor.

**Architecture:** Extend the editor's existing tool and stroke model with `lasso`, keep pointer collection and history in the current controller flow, render its in-progress points as an open SVG polyline, and render committed points as a closed SVG polygon in the shared luminance mask. Reuse the current tool-button styling and conditionally omit only the brush-size field while lasso is active.

**Tech Stack:** React 19, TypeScript, CSS Modules, SVG masks, Vitest, Testing Library.

## Global Constraints

- Preserve the existing visual style of the editor controls.
- Keep clear, undo, and redo state in `useFileEditorController` as the single source of truth.
- Do not add dependencies or a second drawing/history implementation.
- Do not modify unrelated dirty-worktree changes.

---

### Task 1: Add the freehand lasso to the shared editor flow

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.module.css`

**Interfaces:**
- Consumes: `FileEditorController.toggleTool(tool)`, `FileEditorController.addStroke(stroke)`, and the existing SVG luminance mask.
- Produces: the new `EditTool` value `lasso`, an accessible `Лассо` toggle, and filled `<polygon data-edit-tool="lasso">` mask geometry.

- [x] **Step 1: Write failing interaction and mask tests**

Add assertions that the editor exposes a non-active `Лассо` button, that selecting it hides the `Размер кисти` slider, that a repeated click deactivates it, and that drawing four points produces a filled SVG polygon which can be undone and redone.

```tsx
const lasso = within(infoPanel).getByRole("button", { name: "Лассо" });
fireEvent.click(lasso);
expect(lasso).toHaveAttribute("aria-pressed", "true");
expect(within(infoPanel).queryByRole("slider", { name: "Размер кисти" })).toBeNull();

fireEvent.pointerDown(editSurface, { clientX: 10, clientY: 10, pointerId: 1 });
fireEvent.pointerMove(editSurface, { clientX: 60, clientY: 10, pointerId: 1 });
fireEvent.pointerMove(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });
fireEvent.pointerUp(editSurface, { clientX: 10, clientY: 60, pointerId: 1 });
expect(editMask.querySelector('polygon[data-edit-tool="lasso"]')).toHaveAttribute(
  "points",
  "10,10 60,10 60,60 10,60",
);
```

- [x] **Step 2: Run the focused test and verify RED**

Run: `npm exec -- vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: FAIL because no accessible `Лассо` button exists.

- [x] **Step 3: Implement the minimum lasso behaviour**

Extend the union and use the existing pointer flow:

```tsx
type EditTool = "brush" | "eraser" | "lasso";

if (stroke.tool === "lasso") {
  return (
    <polygon
      data-edit-stroke="true"
      data-edit-tool="lasso"
      fill="#fff"
      key={key}
      points={stroke.points.map((point) => `${point.x},${point.y}`).join(" ")}
    />
  );
}
```

Add the `Лассо` toggle button, hide the existing brush-size label while `controller.tool === "lasso"`, and keep `.selectionModes` split into two equal columns.

- [x] **Step 4: Run the focused test and verify GREEN**

Run: `npm exec -- vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: the complete file passes with zero failures.

- [x] **Step 5: Verify the editor change**

Run: `npm exec -- vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

Run: `npm run typecheck`

Run: `npm run lint`

Expected: all commands exit successfully with no warnings or errors.

## Verification Results

- Focused RED: three assertions failed because the `Лассо` control did not exist.
- Focused GREEN: 38 of 38 component tests passed.
- Editor regression: 53 of 53 component and style tests passed.
- TypeScript and ESLint exited successfully.
- Full platform regression: 1023 of 1024 tests passed. The remaining pre-existing failure is `WorkspaceHeader.styles.test.ts`, which was already modified in the dirty worktree before this task and is outside the lasso change.

---

### Task 2: Show the freehand trace while the pointer is held

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`

**Interfaces:**
- Consumes: the existing `draftStroke` state populated by `pointerdown` and every `pointermove`
- Produces: an open `<polyline data-edit-draft="true" data-edit-tool="lasso">` while drawing and the existing closed `<polygon data-edit-tool="lasso">` after release

- [x] **Step 1: Write the failing live-trace test**

After two pointer movements and before `pointerup`, assert that the mask contains an open lasso polyline with all recorded points, a white stroke, and no fill. After `pointerup`, assert that the draft polyline is gone and the completed polygon remains.

```tsx
fireEvent.pointerDown(editSurface, { clientX: 10, clientY: 10, pointerId: 1 });
fireEvent.pointerMove(editSurface, { clientX: 40, clientY: 20, pointerId: 1 });
fireEvent.pointerMove(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });

const lassoDraft = editMask.querySelector(
  'polyline[data-edit-draft="true"][data-edit-tool="lasso"]',
);
expect(lassoDraft).toHaveAttribute("points", "10,10 40,20 60,60");
expect(lassoDraft).toHaveAttribute("fill", "none");
expect(lassoDraft).toHaveAttribute("stroke", "#fff");

fireEvent.pointerUp(editSurface, { clientX: 10, clientY: 60, pointerId: 1 });
expect(editMask.querySelector('[data-edit-draft="true"]')).not.toBeInTheDocument();
expect(editMask.querySelector('polygon[data-edit-tool="lasso"]')).toBeInTheDocument();
```

- [x] **Step 2: Run the focused test and verify RED**

Run: `npm exec vitest run -- src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx -t "shows the lasso trace while the pointer is held"`

Expected: FAIL because the draft currently uses a fill-only polygon and has no `data-edit-draft` polyline.

- [x] **Step 3: Render draft and completed lasso geometry separately**

Add an `isDraft` argument to `renderMaskStroke`. For a draft lasso return an open polyline with `fill="none"`, `stroke="#fff"`, rounded joins/caps, a constant visible stroke width, and `data-edit-draft="true"`. Keep completed lasso history strokes as the existing filled polygons. Pass `true` only when rendering `draftStroke`.

```tsx
function renderMaskStroke(
  stroke: EditStroke | EditStrokeInput,
  key: number | string,
  isDraft = false,
) {
  const firstPoint = stroke.points[0];
  const lastPoint = stroke.points.at(-1);
  if (!firstPoint || !lastPoint) return null;
  const points = stroke.points.map((point) => `${point.x},${point.y}`).join(" ");

  if (stroke.tool === "lasso" && isDraft) {
    return (
      <polyline
        data-edit-draft="true"
        data-edit-stroke="true"
        data-edit-tool="lasso"
        fill="none"
        key={key}
        points={points}
        stroke="#fff"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.5"
        vectorEffect="non-scaling-stroke"
      />
    );
  }

  if (stroke.tool === "lasso") {
    return <polygon data-edit-stroke="true" data-edit-tool="lasso" fill="#fff" key={key} points={points} />;
  }

  return (
    <polyline
      data-edit-stroke="true"
      data-edit-tool={stroke.tool}
      fill="none"
      key={key}
      points={points}
      stroke={stroke.tool === "eraser" ? "#000" : "#fff"}
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={stroke.brushSize}
      vectorEffect="non-scaling-stroke"
    />
  );
}

{draftStroke ? renderMaskStroke(draftStroke, "draft", true) : null}
```

- [x] **Step 4: Run the focused test and verify GREEN**

Run: `npm exec vitest run -- src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx -t "shows the lasso trace while the pointer is held"`

Expected: PASS with the live trace present during movement and absent after release.

- [x] **Step 5: Verify editor regressions and static checks**

Run: `npm exec vitest run -- src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

Run: `npm run typecheck`

Run: `npm run lint`

Expected: all three commands exit successfully with no test failures, type errors, lint errors, or warnings.

## Live Trace Verification Results

- Focused RED: the draft polyline query returned `null` because the in-progress lasso used a fill-only polygon.
- Focused GREEN: the live-trace test passed after separating draft and completed lasso rendering.
- Editor regression: 55 of 55 component and style tests passed.
- TypeScript and ESLint exited successfully.
