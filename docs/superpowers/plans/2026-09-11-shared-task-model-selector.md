# Shared Task Model Selector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reuse the existing workspace model-selector UI in the Animate, Enhance, Remove background, and Edit file-preview tasks while keeping task compatibility, selection, and pricing typed and centralized.

**Architecture:** Extract the existing selector markup and interaction into a controlled `ModelSelector` component that continues to use the current CSS module and `ModelCard` selector variant. Keep `WorkspaceModelSelector` as the catalogue-loading/router adapter, and add a file-task model registry plus a small task adapter for the four preview modes.

**Tech Stack:** React 19, Next.js 16, TypeScript 5.9, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Do not change the existing model selector's visual styles.
- The four file-preview modes are `animate`, `enhance`, `remove-background`, and `edit`.
- Model metadata for those modes must come from one typed registry.
- Preserve per-mode selected model and credit cost behavior.

---

### Task 1: Controlled shared model selector

**Files:**
- Create: `web/platform/src/features/models/WorkspaceModelSelector/ModelSelector.tsx`
- Create: `web/platform/src/features/models/WorkspaceModelSelector/ModelSelector.test.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx`
- Modify: `web/platform/src/features/models/ModelCard/ModelCard.tsx`

**Interfaces:**
- Consumes: `ModelCard`, `ModelIcon`, `WorkspaceModelSelector.module.css`, and `Pick<ImageModel, "id" | "name">`.
- Produces: `ModelSelector`, `ModelSelectorModel`, `ModelSelectorCategory`, and controlled props `models`, `selectedModelId`, `onSelect`, `status`, `triggerAriaLabel`, and `renderInPortal`.

- [x] **Step 1: Write the failing shared-component test**

```tsx
render(<ModelSelector models={models} selectedModelId="video-generator" onSelect={onSelect} />);
fireEvent.click(screen.getByRole("button", { name: /Генератор видео/ }));
expect(screen.getByRole("searchbox", { name: "Поиск нейросети" })).toBeInTheDocument();
fireEvent.click(screen.getByRole("button", { name: /Google Veo 3.1/ }));
expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({ id: "google-veo-3-1" }));
```

- [x] **Step 2: Run the test to verify it fails**

Run: `npm exec vitest run src/features/models/WorkspaceModelSelector/ModelSelector.test.tsx`

Expected: FAIL because `ModelSelector.tsx` does not exist.

- [x] **Step 3: Implement the controlled selector and header adapter**

```ts
export type ModelSelectorModel = Pick<ImageModel, "id" | "name"> & {
  category: Exclude<ModelSelectorCategory, "popular">;
};
```

`ModelSelector` reuses the current trigger, search, sections, `ModelCard`, footer, and CSS module. `WorkspaceModelSelector` remains responsible for loading the image catalogue, resolving URL/workspace selection, updating the workspace selection, and routing after a selection.

- [x] **Step 4: Run selector tests**

Run: `npm exec vitest run src/features/models/WorkspaceModelSelector/ModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`

Expected: PASS.

### Task 2: Typed file-task model registry and panel adoption

**Files:**
- Create: `web/platform/src/features/files/FilePreviewDialog/file-action-models.ts`
- Create: `web/platform/src/features/files/FilePreviewDialog/FileTaskModelSelector.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileAnimationPanel.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`
- Modify: `web/platform/src/features/models/ModelCard/model-card-content.ts`
- Test: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

**Interfaces:**
- Consumes: `ModelSelector` from Task 1.
- Produces: `FileModelTask`, `FileActionModel`, `fileActionModelsByTask`, and `FileTaskModelSelector` with controlled `selectedModelId` and `onSelect`.

- [x] **Step 1: Update preview behavior tests first**

```tsx
fireEvent.click(within(toolbar).getByRole("button", { name: "Оживить" }));
fireEvent.click(within(infoPanel).getByRole("button", { name: /Генератор видео/ }));
expect(screen.getByRole("dialog", { name: "Выбор нейросети для «Оживить»" })).toBeInTheDocument();
fireEvent.click(screen.getByRole("button", { name: /Google Veo 3.1/ }));
expect(within(infoPanel).getByRole("button", { name: "Оживить за 264 звезды" })).toBeInTheDocument();
```

- [x] **Step 2: Run the preview tests to verify the new contract fails**

Run: `npm exec vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: FAIL because the panels still render their local radio menus.

- [x] **Step 3: Implement the task registry and adapters**

```ts
export type FileModelTask = "animate" | "enhance" | "remove-background" | "edit";

export const fileActionModelsByTask = {
  animate: videoModels,
  enhance: enhancementModels,
  "remove-background": backgroundRemovalModels,
  edit: editModels,
} satisfies Record<FileModelTask, readonly FileActionModel[]>;
```

Replace the local selector JSX in both panel files with `FileTaskModelSelector`, keeping action-button costs derived from the selected registry item.

- [x] **Step 4: Run targeted tests and type checking**

Run: `npm exec vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx src/features/models/WorkspaceModelSelector/ModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`

Run: `npm run typecheck`

Expected: all targeted tests and TypeScript pass.

### Task 3: Browser verification

**Files:**
- Verify only; no planned source changes.

**Interfaces:**
- Consumes: the running app at `http://localhost:7158/app/files`.
- Produces: visual confirmation for all four modes at the current viewport and a narrow viewport.

- [x] **Step 1: Open each task mode and its selector**

Confirm that Animate, Enhance, Remove background, and Edit all render the same trigger, search, grouped list, selected marker, internal scroll area, and footer as the workspace selector.

- [x] **Step 2: Verify task filtering and state**

Confirm that each mode shows only its compatible models, selecting a model updates the trigger and action cost, and the dropdown remains inside the viewport by opening above when required.

- [x] **Step 3: Run final static verification**

Run: `npm exec eslint src/features/models/WorkspaceModelSelector/ModelSelector.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx src/features/models/ModelCard/ModelCard.tsx src/features/models/ModelCard/model-card-content.ts src/features/files/FilePreviewDialog/file-action-models.ts src/features/files/FilePreviewDialog/FileTaskModelSelector.tsx src/features/files/FilePreviewDialog/FileAnimationPanel.tsx src/features/files/FilePreviewDialog/FileEditorPanel.tsx`

Run: `git diff --check -- web/platform/src/features/models web/platform/src/features/files/FilePreviewDialog`

Expected: no lint errors and no whitespace errors.
