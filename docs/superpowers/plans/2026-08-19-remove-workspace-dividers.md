# Remove Workspace Dividers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove only the three approved workspace divider lines without changing layout or other component borders.

**Architecture:** Keep the existing React component tree intact and make a narrowly scoped CSS Modules change. Protect the visual contract with a source-level Vitest regression test that targets the relevant class blocks rather than banning border properties across whole files.

**Tech Stack:** TypeScript, React, Next.js, CSS Modules, Vitest

## Global Constraints

- Preserve all scrolling, sticky positioning, dimensions, colors, and typography.
- Preserve borders belonging to cards, inputs, balance controls, and the account section.
- Do not modify backend or API behavior.

---

### Task 1: Remove the approved workspace dividers

**Files:**
- Create: `web/platform/src/components/layout/AppShell/WorkspaceDividers.test.ts`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.module.css`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.module.css`

- [x] Add a focused test asserting that the sidebar panel, workspace header, and composer dock do not declare their respective divider borders.
- [x] Run the focused test and confirm it fails against the current CSS.
- [x] Remove `border-inline-end` only from `.panel` in `Sidebar.module.css`.
- [x] Remove `border-block-end` only from `.header` in `WorkspaceHeader.module.css`.
- [x] Remove `border-block-start` only from `.dock` in `ConversationComposer.module.css`.
- [x] Run the focused test and confirm it passes.
- [x] Run the web platform tests and production build.
- [x] Review the final diff to confirm no unrelated styling changed.
