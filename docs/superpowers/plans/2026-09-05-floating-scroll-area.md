# Floating Scroll Area Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace duplicated vertical scrollbar visuals with one accessible overlay `ScrollArea` that does not reserve content width.

**Architecture:** Add a dependency-free shared component with a native scroll viewport and an absolutely positioned overlay track/thumb. Preserve consumer layout and semantics through configurable root and viewport element types, forwarded refs, and viewport props; migrate only vertical scroll areas.

**Tech Stack:** React 19, TypeScript 5.9, CSS Modules, Vitest, Testing Library

## Global Constraints

- Preserve wheel, touchpad, touch, keyboard, and programmatic scrolling.
- Keep the overlay scrollbar decorative and hidden from assistive technology.
- Do not migrate horizontal tab or carousel scrolling.
- Do not add a third-party dependency.
- Preserve all unrelated uncommitted user changes in the shared worktree.

---

### Task 1: Shared ScrollArea component

**Files:**
- Create: `web/platform/src/components/ui/ScrollArea/ScrollArea.tsx`
- Create: `web/platform/src/components/ui/ScrollArea/ScrollArea.module.css`
- Create: `web/platform/src/components/ui/ScrollArea/ScrollArea.test.tsx`
- Create: `web/platform/src/components/ui/ScrollArea/ScrollArea.styles.test.ts`

**Interfaces:**
- Produces: `ScrollArea`, accepting `viewportAs`, `className`, `viewportClassName`, `viewportProps`, `viewportRef`, and children.
- Preserves: the native viewport as the only scroll owner; a forwarded ref targets the outer root.

- [ ] **Step 1: Write failing behavior tests**

Render overflow metrics with `clientHeight: 100`, `scrollHeight: 400`, and assert a 25px thumb; dispatch scroll and assert the thumb offset and active state; click the track and assert `scrollTop`; drag the thumb and assert proportional scrolling.

- [ ] **Step 2: Write the failing stylesheet test**

Assert the viewport hides native scrollbar chrome, the track is absolutely overlaid at the inline end, the thumb uses a brand color and fades in for hover/focus/scroll/drag, and reduced motion disables transitions.

- [ ] **Step 3: Verify RED**

Run:

```powershell
npm exec vitest run src/components/ui/ScrollArea/ScrollArea.test.tsx src/components/ui/ScrollArea/ScrollArea.styles.test.ts
```

Expected: FAIL because `ScrollArea` does not exist.

- [ ] **Step 4: Implement the minimal component**

Use a native overflow viewport, `ResizeObserver`, `MutationObserver`, captured `load`, and `requestAnimationFrame` to write thumb size/offset to CSS custom properties. Use pointer capture for drag and map track clicks to `scrollTop`. Keep the overlay `aria-hidden`.

- [ ] **Step 5: Verify GREEN**

Run the focused command from Step 3 and expect both files to pass.

### Task 2: Main workspace and subscription modal

**Files:**
- Modify: `web/platform/src/components/layout/AppShell/AppShell.tsx`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.module.css`
- Modify: `web/platform/src/components/layout/AppShell/AppShell.scrollbar.test.ts`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.module.css`
- Modify: `web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.test.tsx`

**Interfaces:**
- Consumes: `ScrollArea` from Task 1.
- Preserves: `main` landmark, `workspace-scroll-region` test ID, modal scroll lock, modal layout, and focus behavior.

- [ ] **Step 1: Update tests to require shared ScrollArea usage and verify RED**
- [ ] **Step 2: Replace native overflow elements with ScrollArea while moving only layout-specific CSS to consumer classes**
- [ ] **Step 3: Run AppShell, ModalBackdrop, and WorkspaceHeader tests and verify GREEN**

### Task 3: Remaining vertical consumers

**Files:**
- Modify: `web/platform/src/components/ui/ModalBackdrop/ModalBackdrop.tsx`
- Modify: `web/platform/src/components/ui/ModalBackdrop/ModalBackdrop.module.css`
- Modify: `web/platform/src/components/chat/ChatFilePicker/ChatFilePicker.tsx`
- Modify: `web/platform/src/components/chat/ChatFilePicker/ChatFilePicker.module.css`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.tsx`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.module.css`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.tsx`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.module.css`
- Modify: `web/platform/src/features/account/AccountUpdatesPanel/AccountUpdatesPanel.tsx`
- Modify: `web/platform/src/features/account/AccountUpdatesPanel/AccountUpdatesPanel.module.css`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`
- Modify: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx`
- Modify: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css`
- Modify: `web/platform/src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx`
- Modify: `web/platform/src/features/image-generation/ImageQualitySelector/ImageQualitySelector.module.css`
- Modify: `web/platform/src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx`
- Modify: `web/platform/src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.module.css`
- Modify: `web/platform/src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationExampleCard/InspirationExampleCard.module.css`
- Create: `web/platform/src/components/ui/ScrollArea/ScrollArea.consumers.test.ts`

**Interfaces:**
- Consumes: the same `ScrollArea` API from Task 1.
- Preserves: semantic lists, dialogs, forwarded panel refs, keyboard handlers, padding, dimensions, and existing responsive rules.

- [ ] **Step 1: Add a failing consumer contract test listing all scoped TSX consumers**
- [ ] **Step 2: Migrate each listed element and remove its duplicated vertical scrollbar visuals**
- [ ] **Step 3: Run each consumer's existing tests plus the contract test and verify GREEN**

### Task 4: Complete verification

- [ ] **Step 1: Run `npm test` and require zero failures**
- [ ] **Step 2: Run `npm run typecheck` and require exit code 0**
- [ ] **Step 3: Run ESLint for every changed TS/TSX file and require no output**
- [ ] **Step 4: Run `git diff --check` and require no whitespace errors**
- [ ] **Step 5: Inspect the localhost workspace, subscription dialog, sidebar, and selectors at desktop and narrow widths**

Because this is a dirty shared worktree, do not create mixed commits that would absorb unrelated user changes. Commit only after the user requests integration or the work is isolated.
