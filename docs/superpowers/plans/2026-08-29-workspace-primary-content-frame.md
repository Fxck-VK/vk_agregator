# Workspace Primary Content Frame Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the hero and featured-model sections to one centered `50rem` frame with equal left and right margins.

**Architecture:** Add one reusable CSS Module class in `WorkspaceLanding.module.css` and apply it to exactly two existing sections in `WorkspaceLanding.tsx`. No child component or global layout geometry changes.

**Tech Stack:** React, TypeScript, CSS Modules, Vitest.

## Global Constraints

- The frame is exactly `inline-size: min(100%, 50rem)` with `margin-inline: auto`.
- Only the hero and popular-model sections receive it.
- Existing internal padding, component sizes, sidebar geometry, `.main`, and later sections remain unchanged.
- Push is not performed without a separate user request.

---

### Task 1: Add the shared primary content frame

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`

**Interfaces:**
- Consumes: the existing `section`, `hero`, prompt, shortcut rail, section heading, and featured-model components.
- Produces: one `contentFrame` CSS class shared by the two primary sections.

- [ ] **Step 1: Write the failing contract**

Read both the stylesheet and component source in `WorkspaceLanding.styles.test.ts`. Assert that `.contentFrame` contains `inline-size: min(100%, 50rem)` and `margin-inline: auto`, and that `styles.contentFrame` appears exactly twice in the component.

- [ ] **Step 2: Verify RED**

Run: `npm exec -- vitest run src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Expected: FAIL because `contentFrame` does not exist.

- [ ] **Step 3: Implement the shared frame**

Add:

```css
.contentFrame {
  inline-size: min(100%, 50rem);
  margin-inline: auto;
}
```

Apply `styles.contentFrame` to the hero section and the section labelled by `workspace-models-title`.

- [ ] **Step 4: Verify GREEN and full project**

Run the targeted test, then `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`, `npm run test:packaging`, and `git diff --check`.

- [ ] **Step 5: Commit locally**

Commit the focused change with message `fix(platform): align workspace primary content` and do not push.
