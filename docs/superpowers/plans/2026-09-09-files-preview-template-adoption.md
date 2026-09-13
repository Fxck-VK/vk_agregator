# Files Preview Template Adoption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the file-specific preview shell with the reusable media preview template already used by “Вдохновение”, while preserving all file-only content and behavior.

**Architecture:** Extend `MediaPreviewDialogTemplate<T>` with optional disabled-item handling, a preview footer slot, backdrop/info-panel test hooks, narrow-rail orientation, and focus containment. Refactor `FilePreviewDialog` into a feature adapter that supplies file media, metadata, feedback, actions, and the disabled tool rail. Remove the old file-specific structural CSS instead of retaining a legacy implementation.

**Tech Stack:** React 19, Next.js, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Keep the active Inspiration viewer visually and behaviorally unchanged.
- Preserve loading and unavailable file thumbnails; they remain disabled and are skipped by cyclic navigation.
- Preserve prompt copy, download, share fallback, metadata omission, focus restoration, and disabled future tools.
- Remove old file-only modal layout and navigation code after the adapter is connected.
- Do not modify `FilesWorkspace` data fetching or create a commit.

---

### Task 1: Extend the shared template contract

**Files:**
- Modify: `web/platform/src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx`
- Modify: `web/platform/src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.module.css`
- Test: `web/platform/src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.test.tsx`

**Interfaces:**
- Consumes optional `isItemSelectable`, `renderPreviewFooter`, `backdropTestId`, and `infoPanelTestId` props.
- Produces disabled thumbnails, selectable-only cyclic navigation, a shared preview footer slot, responsive scrollbar orientation, and focus containment.

- [x] **Step 1: Write failing template tests** for disabled thumbnail navigation, preview footer rendering, narrow horizontal orientation, test hooks, and Tab focus containment.
- [x] **Step 2: Run the template test and verify RED** with failures caused by the missing optional props and behavior.
- [x] **Step 3: Implement the minimal generic behavior** without importing file feature types or styles.
- [x] **Step 4: Run the template and Inspiration tests and verify GREEN.**

### Task 2: Convert FilePreviewDialog into the adapter

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.tsx`
- Replace: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.module.css`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`
- Test: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

**Interfaces:**
- Consumes `MediaPreviewDialogTemplate<FilePreviewItem>` and the existing controlled `items`, `selectedIndex`, `onSelect`, `onClose`, and `returnFocusTo` props.
- Produces the same accessible labels and file-specific UI while delegating all shell behavior to the template.

- [x] **Step 1: Add a failing source/style contract** asserting the file dialog imports the template and no longer contains `ModalBackdrop`, `ScrollArea`, structural `.dialog`, `.thumbnailRail`, `.previewPane`, or navigation rules.
- [x] **Step 2: Run file preview tests and verify RED.**
- [x] **Step 3: Refactor the component into an adapter** by retaining formatter, feedback, prompt, metadata, share, download, and tool-rail code and supplying render callbacks to the shared template.
- [x] **Step 4: Delete old structural CSS** and keep only adapter content/tool styles plus footer positioning classes.
- [x] **Step 5: Run file, workspace, Inspiration, typecheck, and ESLint verification.**
- [x] **Step 6: Verify desktop and narrow layouts in the local browser.**
