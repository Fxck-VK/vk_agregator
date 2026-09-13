# Toggle Edit Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow brush, rectangle, and eraser controls to be turned off by pressing the active control again.

**Architecture:** Widen the editor controller's selected tool state to `EditTool | null`. Centralize toggle behavior in the controller and make the drawing surface ignore pointer drawing while no tool is selected.

**Tech Stack:** React, TypeScript, CSS Modules, Vitest, Testing Library

## Global Constraints

- Keep brush selected when the editor first opens.
- Keep the three tools mutually exclusive.
- Preserve existing strokes and unrelated editor behavior when a tool is disabled.

---

### Task 1: Add the disabled-tool state

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.module.css`
- Test: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

**Interfaces:**
- Consumes: existing `FileEditorController.tool` and the brush, rectangle, and eraser buttons.
- Produces: `FileEditorController.tool: EditTool | null` and `toggleTool(tool: EditTool): void`.

- [x] **Step 1: Write failing interaction tests**

Add tests that press each active tool twice and expect every button to report `aria-pressed="false"`. Add a test that disables the brush, sends pointer events to the edit surface, and expects no edit stroke or custom cursor.

- [x] **Step 2: Verify the tests fail**

Run `npm exec vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`. Expect failures because active tools cannot yet be deselected and drawing still starts.

- [x] **Step 3: Implement the minimal state and pointer guards**

Store `EditTool | null`, add `toggleTool`, connect all three buttons to it, represent the disabled surface as `data-tool="none"`, and return from pointer handlers when no tool is active.

- [x] **Step 4: Verify behavior and regressions**

Run the focused test file, TypeScript checking, targeted ESLint, and inspect the interaction in the local browser. Expect all checks to pass and the active style/cursor to disappear after a second press.
