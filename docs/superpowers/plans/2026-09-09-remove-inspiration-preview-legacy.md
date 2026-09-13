# Remove Inspiration Preview Legacy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the shared template the only Inspiration preview implementation and delete the inactive legacy shell.

**Architecture:** `InspirationExampleCard` retains only card rendering and local open state. Its standalone fallback imports `InspirationExampleDialog` from `InspirationExampleDialogTemplate.tsx`; the duplicate modal implementation and structural CSS are removed.

**Tech Stack:** React 19, Next.js, TypeScript, CSS Modules, Vitest.

## Global Constraints

- Preserve standalone card preview behavior on the workspace landing and image-generation guide.
- Preserve focus restoration to the opening card.
- Keep the active Inspiration gallery adapter unchanged.
- Do not create a commit.

---

### Task 1: Remove the legacy Inspiration dialog

**Files:**
- Modify: `web/platform/src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationExampleCard/InspirationExampleCard.module.css`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.styles.test.ts`

**Interfaces:**
- Consumes: `InspirationExampleDialog` from `InspirationExampleDialogTemplate.tsx`.
- Produces: standalone cards that open the shared dialog without a duplicate modal shell.

- [x] **Step 1: Add a failing source/style contract** asserting that the card imports the shared dialog adapter and contains no `ModalBackdrop`, `ScrollArea`, exported legacy dialog, or structural dialog CSS.
- [x] **Step 2: Run the style contract and verify RED** because the legacy implementation is still present.
- [x] **Step 3: Replace the card fallback with the shared dialog import** and remove legacy-only hooks, helpers, types, and markup.
- [x] **Step 4: Remove unused structural dialog CSS** while retaining card and active adapter content styles.
- [x] **Step 5: Run focused Inspiration tests, TypeScript, ESLint, and the full test suite.**
