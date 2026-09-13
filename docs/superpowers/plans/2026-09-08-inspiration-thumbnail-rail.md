# Inspiration Thumbnail Rail Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show every inspiration item in the preview dialog's thumbnail rail and switch the active item without closing the dialog.

**Architecture:** Lift modal selection state into `InspirationGallery`, keep card hover/video behavior in `InspirationExampleCard`, and export a shared `InspirationExampleDialog` from the same feature module. The dialog receives the full immutable examples list and a controlled selected index.

**Tech Stack:** React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Preserve the existing inspiration masonry layout and natural media aspect ratios.
- Keep desktop thumbnails vertical and narrow-screen thumbnails horizontal.
- Run only inspiration-related tests during the implementation cycle.

---

### Task 1: Controlled inspiration preview

**Files:**
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationExampleCard/InspirationExampleCard.module.css`

**Interfaces:**
- Consumes: `readonly InspirationExample[]` and the selected example index.
- Produces: `InspirationExampleDialog` with controlled `onSelect(index)` and `onClose()` callbacks.

- [ ] **Step 1: Write the failing test**

Add a test that opens the first card, finds one thumbnail per `inspirationExamples` item, clicks the second thumbnail, and expects its download URL and `aria-current` state to become active.

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx`

Expected: FAIL because the current dialog renders one thumbnail only.

- [ ] **Step 3: Write minimal implementation**

Move the selected index into `InspirationGallery`, render one shared dialog beside the gallery list, map every example into the thumbnail rail, and call `onSelect(index)` from each thumbnail.

- [ ] **Step 4: Run related tests**

Run: `npx vitest run src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx src/features/inspiration/InspirationGallery/InspirationGallery.styles.test.ts`

Expected: both files pass with zero failed tests.

- [ ] **Step 5: Verify the browser behavior**

Open `/app/inspiration`, select a card, inspect the thumbnail count, switch to another thumbnail, and confirm the selected preview and active marker change.
