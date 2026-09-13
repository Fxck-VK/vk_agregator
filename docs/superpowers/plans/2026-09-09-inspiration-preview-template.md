# Inspiration Preview Template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the current Inspiration preview into a reusable media-preview template without changing its appearance or behavior, while preserving the old dialog as a disconnected rollback path.

**Architecture:** A generic `MediaPreviewDialogTemplate<T>` owns the exact current modal structure, responsive CSS, selection wrapping, keyboard navigation, thumbnail scrolling, and close behavior. A new Inspiration adapter keeps media, prompt, copy/share, recreation, and panel content specific to Inspiration. The existing `InspirationExampleDialog` remains unchanged and `InspirationGallery` changes one import to activate the adapter.

**Tech Stack:** React 19, Next.js, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Copy the existing Inspiration dialog markup and CSS; do not reconstruct it from screenshots.
- Preserve all accessible names, test IDs, CSS values, and current interactions.
- Keep the existing dialog implementation available but disconnected from the active Inspiration gallery.
- Do not modify the Files preview in this phase.
- Do not create a commit; the user requested changes remain uncommitted.

---

### Task 1: Generic media preview template

**Files:**
- Create: `web/platform/src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx`
- Create: `web/platform/src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.module.css`
- Create: `web/platform/src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.test.tsx`

**Interfaces:**
- Consumes: generic `items`, controlled `selectedIndex`, `onSelect`, `onClose`, localized labels, item key/thumbnail label functions, aspect ratio, and render callbacks.
- Produces: `MediaPreviewDialogTemplate<T>` and `MediaPreviewDialogRenderClasses`.

- [x] **Step 1: Write failing template behavior tests**

Cover slot rendering, thumbnail selection, previous/next wrapping, document Left/Right navigation, editable-target keyboard guard, and close callback. Use a three-item fixture and accessible labels so assertions exercise the public interface rather than CSS-module internals.

- [x] **Step 2: Run the template test to verify RED**

Run: `npx vitest run src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.test.tsx --reporter=dot`

Expected: FAIL because the component does not exist.

- [x] **Step 3: Implement the generic component**

Use this public shape:

```tsx
export type MediaPreviewDialogRenderClasses = {
  previewMedia: string;
  thumbnailMedia: string;
};

export type MediaPreviewDialogTemplateProps<T> = {
  ariaLabel: string;
  closeLabel: string;
  getItemKey: (item: T) => string;
  getPreviewDimensions: (item: T) => { height: number; width: number };
  getThumbnailLabel: (item: T) => string;
  infoPanel: (item: T) => ReactNode;
  items: readonly T[];
  nextLabel: string;
  onClose: () => void;
  onSelect: (index: number) => void;
  previousLabel: string;
  renderPreview: (item: T, classes: MediaPreviewDialogRenderClasses) => ReactNode;
  renderThumbnail: (item: T, classes: MediaPreviewDialogRenderClasses) => ReactNode;
  selectedIndex: number;
  testIdPrefix: string;
  thumbnailRailLabel: string;
};
```

Copy the current dialog DOM hierarchy for `ModalBackdrop`, close control, thumbnail `ScrollArea`, preview stage/surface, navigation, and information-panel `ScrollArea`. Move the current editable-target guard, modulo selection, document key listener, close focus, and selected-thumbnail scrolling into this component.

- [x] **Step 4: Copy the current dialog CSS exactly**

Copy the dialog portion of `InspirationExampleCard.module.css` into the new module, including `.dialog`, close, thumbnail, preview, media, info-panel, focus/hover, animation, and all responsive rules. Keep the old stylesheet unchanged for the legacy implementation.

- [x] **Step 5: Run the template test to verify GREEN**

Run: `npx vitest run src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.test.tsx --reporter=dot`

Expected: PASS.

---

### Task 2: Inspiration adapter and active import switch

**Files:**
- Create: `web/platform/src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.styles.test.ts`
- Test: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx`

**Interfaces:**
- Consumes: `MediaPreviewDialogTemplate<InspirationExample>` from Task 1.
- Produces: active `InspirationExampleDialog` adapter with the same props as the legacy export.

- [x] **Step 1: Add a failing active-template contract assertion**

Update the gallery style test to load the new template stylesheet before the Inspiration content stylesheet. Add a source assertion that `InspirationGallery.tsx` imports `InspirationExampleDialog` from `InspirationExampleDialogTemplate` rather than the legacy card module.

- [x] **Step 2: Run Inspiration tests to verify RED**

Run: `npx vitest run src/features/inspiration/InspirationGallery/InspirationGallery.styles.test.ts src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx --reporter=dot`

Expected: FAIL because the adapter and active import do not exist.

- [x] **Step 3: Implement the adapter by moving current item-specific logic**

Copy the current `InspirationExampleDialog` state/effects and information-panel JSX into the adapter. Keep image/video rendering, autoplay/reset, recreation URL, prompt measurement/expansion, clipboard feedback, sharing, and action links unchanged. Pass thumbnails, preview media, aspect ratio, panel contents, labels, selection, and close callbacks into `MediaPreviewDialogTemplate`.

- [x] **Step 4: Switch the active gallery import**

Keep `InspirationExampleCard` imported from `InspirationExampleCard.tsx` and import `InspirationExampleDialog` from `InspirationExampleDialogTemplate.tsx`. Do not delete or edit the legacy dialog export.

- [x] **Step 5: Run focused tests and static checks**

Run:

```text
npx vitest run src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.test.tsx src/features/inspiration/InspirationGallery/InspirationGallery.styles.test.ts src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx --reporter=dot
npm run typecheck
npx eslint src/components/media/MediaPreviewDialogTemplate src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx src/features/inspiration/InspirationGallery/InspirationGallery.tsx src/features/inspiration/InspirationGallery/InspirationGallery.styles.test.ts
```

Expected: all commands exit successfully.

- [x] **Step 6: Verify visual parity**

Open the active Inspiration dialog at desktop and narrow widths. Confirm the dialog dimensions, thumbnail rail, selected state, preview containment, arrow and close positions, information-panel spacing, prompt controls, and action layout match the pre-refactor viewer.
