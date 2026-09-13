# File Preview Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the approved interactive “Редактировать” mode to the file preview dialog.

**Architecture:** A focused `FileEditorPanel` module owns the editor controller, the right-side controls, and the SVG mask overlay rendered above the preview image. `FilePreviewDialog` selects this mode while the existing preview template continues to provide layout, navigation, and the shared toolbar.

**Tech Stack:** React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Keep all existing preview modes unchanged.
- Use NeiroHub design tokens and the supplied `edit-white.svg` icon.
- Keep editing local; the bottom action does not call a generation API.
- Support brush and rectangle selections, prompt entry, brush size, clear, undo, and redo.

---

### Task 1: Editor controller and right panel

**Files:**
- Create: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`
- Create: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.module.css`
- Test: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

**Interfaces:**
- Produces: `useFileEditorController(itemKey)`, `FileEditorPanel`, and `FileEditPreview`.
- Consumes: the shared SVG assets and the preview image properties supplied by `FilePreviewDialog`.

- [x] **Step 1: Write the failing interaction test**

```tsx
fireEvent.click(within(toolbar).getByRole("button", { name: "Редактировать" }));
expect(within(infoPanel).getByRole("heading", { name: "Редактировать" })).toBeInTheDocument();
expect(within(infoPanel).getByRole("textbox", { name: "Промпт редактирования" })).toBeInTheDocument();
expect(within(infoPanel).getByRole("slider", { name: "Размер кисти" })).toHaveValue("32");
expect(within(infoPanel).getByRole("button", { name: "Редактировать за 55 звёзд" })).toBeInTheDocument();
```

- [x] **Step 2: Verify the test is red**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: the test fails because the edit tool still has an unavailable title and the editor panel is absent.

- [x] **Step 3: Implement the controller and panel**

```tsx
const editor = useFileEditorController(artifact.id);
<FileEditorPanel controller={editor} />;
```

The controller stores the selected tool, brush size, prompt, committed strokes, and redo history. The panel renders brush/rectangle toggles, clear/undo/redo, the prompt textarea, brush slider, Nano Banana Pro selector, 9:16 and 2K chips, the “Как работает” card, and the 55-credit action.

- [x] **Step 4: Verify the focused test is green**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: all tests in the file pass.

### Task 2: Editable preview overlay and dialog integration

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.module.css`

**Interfaces:**
- Consumes: `FileEditorController` from Task 1.
- Produces: an image overlay that emits normalized brush or rectangle strokes and renders the selection mask as SVG.

- [x] **Step 1: Extend the failing test with preview behavior**

```tsx
const editSurface = screen.getByLabelText("Область редактирования изображения");
fireEvent.pointerDown(editSurface, { clientX: 20, clientY: 20, pointerId: 1 });
fireEvent.pointerMove(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });
fireEvent.pointerUp(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });
expect(screen.getByTestId("file-edit-mask").childElementCount).toBeGreaterThan(0);
expect(within(infoPanel).getByRole("button", { name: "Отменить выделение" })).toBeEnabled();
```

- [x] **Step 2: Verify the overlay assertion is red**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: the test fails because the editable preview surface does not exist.

- [x] **Step 3: Integrate editor mode**

```tsx
const editor = useFileEditorController(artifact.id);
const preview = activeToolID === "edit"
  ? <FileEditPreview controller={editor} imageAlt={job.prompt} imageSrc={artifactPath} />
  : <img alt={job.prompt} className={classes.previewMedia} src={artifactPath} />;
```

Enable the edit tool, hide the shared download/recreate/share actions, render the editor panel, and use a dashed brand outline around the editable image.

- [x] **Step 4: Verify project quality gates**

Run: `npm test`

Expected: all Vitest and asset tests pass.

Run: `npm run typecheck`

Expected: TypeScript exits with code 0.

Run: `npm run lint`

Expected: ESLint exits with code 0 and no warnings.

### Task 3: Brush and eraser size cursor

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.module.css`
- Test: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

**Interfaces:**
- Consumes: `FileEditorController.tool` and `FileEditorController.brushSize`.
- Produces: a shared translucent circular cursor for the brush and eraser tools.

- [x] **Step 1: Write a failing interaction test**

Move the pointer over the editing surface and assert that the cursor becomes visible at the pointer coordinates with a diameter equal to the configured brush size. Switch to the eraser and assert that the same cursor remains available, then leave the surface and assert that it hides.

- [x] **Step 2: Verify the focused test is red**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx -t "shows the brush size cursor"`

Expected: FAIL because `file-edit-tool-cursor` is not rendered.

- [x] **Step 3: Implement the shared tool cursor**

Track normalized pointer coordinates in `FileEditPreview`, render one non-interactive circular element above the image for `brush` and `eraser`, and hide the native cursor for those two tools. Do not render the circle for `rectangle`.

- [x] **Step 4: Verify the focused test and quality gates**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx -t "shows the brush size cursor"`

Expected: PASS.

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: all tests in the file pass.

### Task 4: Centered brush-size preview

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.module.css`
- Test: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

**Interfaces:**
- Consumes: `FileEditorController.brushSize` updates from the existing range input.
- Produces: a temporary, non-interactive branded circle centered at `50% 50%` over the image.

- [x] **Step 1: Write the failing interaction test**

Change the “Размер кисти” slider to `64` and assert that the size-preview circle is visible, centered at `50% 50%`, and sized to `64px`.

- [x] **Step 2: Verify the test is red**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx -t "previews the brush size"`

Expected: FAIL because the centered size-preview element does not exist.

- [x] **Step 3: Implement the temporary preview**

Observe `controller.brushSize` in `FileEditPreview`, skip the initial render, show a separate circle at the image center after each change, and hide it 700 ms after the last change. Reuse the existing transparent branded cursor styling.

- [x] **Step 4: Verify behavior and regression coverage**

Run the focused test, all `FilePreviewDialog` tests, TypeScript type checking, and focused ESLint. Confirm the size preview visually in the local browser.

Run: `npm run typecheck`

Expected: TypeScript exits with code 0.
