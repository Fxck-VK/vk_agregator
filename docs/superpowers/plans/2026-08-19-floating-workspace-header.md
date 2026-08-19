# Floating Workspace Header Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the workspace model selector and balance/login action float above content without a full-width header strip.

**Architecture:** Keep `WorkspaceHeader` as the persistent route-aware component. Convert its visual wrapper into a zero-height sticky overlay and position two interactive child regions at the top edges of the existing workspace scroll container.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Vitest.

## Global Constraints

- Do not change authentication, balance, model-selection, billing, or API behaviour.
- Keep the header below the sidebar overlay z-index.
- Preserve the mobile sidebar-toggle clearance.
- Do not add dependencies.

---

### Task 1: Floating workspace header

**Files:**
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.styles.test.ts`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.tsx`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css`

**Interfaces:**
- Consumes: existing `WorkspaceHeaderProps`, `WorkspaceModelSelector`, balance, and `trailingAction`.
- Produces: unchanged `WorkspaceHeader` public component with floating layout only.

- [ ] **Step 1: Write the failing stylesheet contract test**

Assert that `.header` is a sticky, zero-height, transparent, pointer-transparent overlay and that `.leading` and `.trailing` are absolutely positioned and restore pointer events.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npm test -- src/components/layout/WorkspaceHeader/WorkspaceHeader.styles.test.ts`

Expected: FAIL because the existing header still has a minimum height, padding, background, and normal-flow controls.

- [ ] **Step 3: Implement the minimal floating layout**

Wrap the trailing content in `styles.trailing`. Update the CSS module so the wrapper has zero height and the two children float at the top edges. Add the existing mobile offset to the floating leading region.

- [ ] **Step 4: Run focused and component tests**

Run: `npm test -- src/components/layout/WorkspaceHeader/WorkspaceHeader.styles.test.ts src/components/layout/WorkspaceHeader/WorkspaceHeader.test.tsx src/components/layout/WorkspaceFrame/WorkspaceFrame.test.tsx`

Expected: PASS.

- [ ] **Step 5: Run frontend verification**

Run: `npm test`, `npm run typecheck`, and `npm run build` from `web/platform`.

Expected: all commands complete successfully.
