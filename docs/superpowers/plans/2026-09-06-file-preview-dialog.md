# File Preview Dialog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a reusable full-screen artifact preview to completed cards in “Мои файлы”.

**Architecture:** `FileCard` owns only its card interactions and reports the selected artifact through `onOpenPreview`. `FilesWorkspace` owns the selected preview state and renders a focused `FilePreviewDialog`, which reuses `ModalBackdrop` and `ModelIcon`. Download and share remain ordinary explicit actions inside the dialog.

**Tech Stack:** React, TypeScript, CSS Modules, Testing Library, Vitest.

## Global Constraints

- Reuse `src/components/ui/ModalBackdrop/ModalBackdrop.tsx`.
- Do not add a dependency or backend endpoint.
- Do not commit or publish the changes.
- Run Vitest with no more than four workers.

---

### Task 1: Separate card preview and download interactions

**Files:**
- Modify: `web/platform/src/features/files/FileCard/FileCard.tsx`
- Modify: `web/platform/src/features/files/FileCard/FileCard.module.css`
- Modify: `web/platform/src/features/files/FileCard/FileCard.test.tsx`
- Modify: `web/platform/src/features/files/FileCard/FileCard.styles.test.ts`
- Modify: `web/platform/src/features/files/FilesGrid/FilesGrid.tsx`

**Interfaces:**
- Consumes: `ImageJob`, `ImageJobResult`, and each result artifact.
- Produces: `onOpenPreview(job: ImageJob, artifact: ImageJobResult["artifacts"][number]): void`.

- [x] **Step 1: Write the failing card behavior test**

Render a completed card with `onOpenPreview={vi.fn()}`, press its labelled preview button, and require the callback to receive the job and artifact. Require the nested “Скачать” link to retain `download` and the artifact URL.

- [x] **Step 2: Verify RED**

Run `npm exec vitest run -- src/features/files/FileCard/FileCard.test.tsx --maxWorkers=4`. Expected: the preview button and `onOpenPreview` prop do not exist.

- [x] **Step 3: Implement the minimal card interaction**

Replace the full-card download anchor with a media preview button and position a separate download link over the lower-left media edge. Thread `onOpenPreview` through `FilesGrid`.

- [x] **Step 4: Verify GREEN**

Run the focused FileCard tests and require zero failures.

### Task 2: Add the modal preview surface

**Files:**
- Create: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.tsx`
- Create: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.module.css`
- Create: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`
- Create: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `{ artifact, job, onClose }` and the existing modal/model-icon components.
- Produces: a labelled dialog rendered from `FilesWorkspace` while a selected artifact exists.

- [x] **Step 1: Write failing dialog and workspace tests**

Require the dialog to show the selected image, model name, close control, download link, share button, and disabled future actions. Require a card press in `FilesWorkspace` to mount that dialog and an animated close to remove it.

- [x] **Step 2: Verify RED**

Run the focused FilePreviewDialog and FilesWorkspace tests. Expected: the new component and modal behavior are absent.

- [x] **Step 3: Implement dialog state and surface**

Store `{ job, artifact } | null` in `FilesWorkspace`, render `FilePreviewDialog` for that selection, and clear it from the shared backdrop close callback. Use `ModelIcon`, `object-fit: contain`, a responsive two-column surface, and disabled future edit controls.

- [x] **Step 4: Implement download and share actions**

Keep the artifact download as an anchor. On “Поделиться”, call `navigator.share({ title: job.prompt, url: absoluteArtifactURL })` when available; otherwise call `navigator.clipboard.writeText(absoluteArtifactURL)`. Expose a polite status message after copying.

- [x] **Step 5: Verify GREEN and browser behavior**

Run all focused tests with four workers, then inspect open, close, download separation, portrait/landscape fitting, and responsive layout on the local files page.

### Task 3: Complete verification

**Files:**
- Verify all files listed above.

**Interfaces:**
- Consumes: the complete preview feature.
- Produces: fresh verification evidence.

- [x] **Step 1: Run the complete suite**

Run `npm exec vitest run -- --maxWorkers=4` and require zero failures.

- [x] **Step 2: Run repository checks**

Run `npm run test:assets`, `npm run typecheck`, and `npm run lint` sequentially and require zero failures.

- [x] **Step 3: Inspect the final diff**

Run `git diff --check` on the touched source, tests, and documentation, and confirm unrelated dirty files were preserved.

### Task 4: Match the approved media composition

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.module.css`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

- [x] **Step 1: Reproduce the image overflow and overlay**

Open a generated file locally and confirm that the media fills the left stage while the absolutely positioned action rail overlays it.

- [x] **Step 2: Add failing composition contracts**

Require separated surfaces, an in-flow action row below the media, a contained natural image ratio, and an information panel without prompt or quality text.

- [x] **Step 3: Implement the approved composition**

Split the preview surface into bounded media and toolbar rows, remove visible metadata from the information panel, preserve the loaded image ratio, and stack the surfaces before the toolbar becomes cramped.

- [x] **Step 4: Verify the full project and browser rendering**

Run focused and complete tests, assets, typecheck, lint, diff checks, and inspect the local modal at runtime.
