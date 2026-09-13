# Shared Masonry Grid Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use one reusable Pinterest-style ordered-list layout in Inspiration, My Files, and the image-template picker.

**Architecture:** A presentation-only `MasonryGrid` component renders an `<ol>` and owns the existing CSS multi-column rules. Feature components continue to render their own `<li>` cards; the template picker switches its image from a forced fill box to intrinsic dimensions so the shared layout can preserve natural aspect ratios.

**Tech Stack:** React 19, TypeScript, CSS Modules, Next.js Image, Vitest, Testing Library.

## Global Constraints

- Preserve ordered-list semantics and source-data keyboard order.
- Use `17rem` minimum column width and `var(--space-4)` for both column and row gaps.
- Do not add JavaScript measurement, balancing, or item reordering.
- Do not change card actions, filtering, dialogs, file loading, or scrollbar placement.

---

### Task 1: Add the shared MasonryGrid component

**Files:**
- Create: `web/platform/src/components/ui/MasonryGrid/MasonryGrid.tsx`
- Create: `web/platform/src/components/ui/MasonryGrid/MasonryGrid.module.css`
- Create: `web/platform/src/components/ui/MasonryGrid/MasonryGrid.test.tsx`
- Create: `web/platform/src/components/ui/MasonryGrid/MasonryGrid.styles.test.ts`

**Interfaces:**
- Consumes: standard React `ComponentPropsWithoutRef<"ol">`.
- Produces: `MasonryGrid(props)` rendering an ordered list with merged shared and consumer class names.

- [ ] **Step 1: Write the failing component test**

```tsx
render(<MasonryGrid aria-label="Галерея" className="consumer"><li>Фото</li></MasonryGrid>);
expect(screen.getByRole("list", { name: "Галерея" })).toHaveClass("consumer");
expect(screen.getByRole("listitem")).toHaveTextContent("Фото");
```

- [ ] **Step 2: Write the failing style test**

Assert `.grid` contains `column-width: 17rem`, `column-gap: var(--space-4)`, list resets, and `.grid > li` contains full width, `margin-block-end: var(--space-4)`, and `break-inside: avoid`.

- [ ] **Step 3: Run the tests and verify RED**

Run: `npx vitest run src/components/ui/MasonryGrid`
Expected: FAIL because the component and stylesheet do not exist.

- [ ] **Step 4: Implement the component**

```tsx
import type { ComponentPropsWithoutRef } from "react";
import styles from "./MasonryGrid.module.css";

type MasonryGridProps = ComponentPropsWithoutRef<"ol">;

export function MasonryGrid({ className, ...props }: Readonly<MasonryGridProps>) {
  return <ol {...props} className={[styles.grid, className].filter(Boolean).join(" ")} />;
}
```

Implement the exact existing multi-column rules in the CSS module.

- [ ] **Step 5: Run the tests and verify GREEN**

Run: `npx vitest run src/components/ui/MasonryGrid`
Expected: both test files pass.

### Task 2: Migrate Inspiration and My Files

**Files:**
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.tsx`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.module.css`
- Modify: `web/platform/src/features/files/FilesGrid/FilesGrid.tsx`
- Delete: `web/platform/src/features/files/FilesGrid/FilesGrid.module.css`
- Modify: `web/platform/src/components/layout/WorkspacePageFrame/WorkspacePageFrame.styles.test.ts`
- Modify: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.styles.test.ts`
- Create: `web/platform/src/components/ui/MasonryGrid/MasonryGrid.consumers.test.ts`

**Interfaces:**
- Consumes: `MasonryGrid` from Task 1.
- Produces: both existing feeds use the shared layout without local masonry CSS.

- [ ] **Step 1: Write the failing consumer contract**

Read the Inspiration and Files source files and assert both import `MasonryGrid`, render `<MasonryGrid`, and contain no local `styles.grid`. Assert their feature styles do not contain `column-width` or `break-inside`.

- [ ] **Step 2: Run the consumer test and verify RED**

Run: `npx vitest run src/components/ui/MasonryGrid/MasonryGrid.consumers.test.ts`
Expected: FAIL because both features still own their lists and layout styles.

- [ ] **Step 3: Replace both ordered-list wrappers**

Import `MasonryGrid`, replace each `<ol className={styles.grid}>...</ol>` with `<MasonryGrid>...</MasonryGrid>`, remove the Inspiration masonry selectors, and delete the unused FilesGrid stylesheet and import.

- [ ] **Step 4: Point existing layout tests at the shared stylesheet**

Read `MasonryGrid.module.css` in the two existing style contracts and keep their current masonry assertions against that shared source.

- [ ] **Step 5: Run related tests and verify GREEN**

Run: `npx vitest run src/components/ui/MasonryGrid src/features/inspiration/InspirationGallery src/features/files/FilesWorkspace src/components/layout/WorkspacePageFrame/WorkspacePageFrame.styles.test.ts`
Expected: all selected tests pass.

### Task 3: Migrate the image-template picker and preserve natural proportions

**Files:**
- Modify: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx`
- Modify: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css`
- Modify: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.test.tsx`
- Modify: `web/platform/src/components/ui/MasonryGrid/MasonryGrid.consumers.test.ts`

**Interfaces:**
- Consumes: `MasonryGrid` and each template's `mediaWidth`/`mediaHeight`.
- Produces: a Pinterest-style template list with uncropped intrinsic-ratio images.

- [ ] **Step 1: Extend the consumer test and verify RED**

Assert the picker imports and renders `MasonryGrid`, does not render `styles.grid`, supplies `height={template.mediaHeight}` and `width={template.mediaWidth}`, and no longer uses `fill`. Assert the picker stylesheet contains no `grid-template-columns` and no forced `.card` aspect ratio.

- [ ] **Step 2: Replace the template list wrapper**

Use `<MasonryGrid>` around the unchanged template list items.

- [ ] **Step 3: Switch the template image to intrinsic dimensions**

```tsx
<Image
  alt={template.mediaAlt}
  className={styles.cardImage}
  height={template.mediaHeight}
  sizes="(max-width: 32rem) 100vw, (max-width: 64rem) 50vw, 33vw"
  src={template.mediaPath}
  width={template.mediaWidth}
/>
```

Remove the card aspect ratio and local responsive grid rules. Give `.cardImage` block display, full inline width, automatic block height, and `object-fit: contain`.

- [ ] **Step 4: Run related tests and verify GREEN**

Run: `npx vitest run src/components/ui/MasonryGrid src/features/image-generation/ImageTemplatePicker`
Expected: all selected tests pass.

### Task 4: Verify the integrated result

**Files:**
- Verify all files modified in Tasks 1-3.

**Interfaces:**
- Consumes: completed shared component and three migrated consumers.
- Produces: verified repository state and visual evidence.

- [ ] **Step 1: Run static checks**

Run: `npm run typecheck` and `npm run lint`.
Expected: both exit with code 0.

- [ ] **Step 2: Run focused regression tests**

Run the focused Vitest command from Tasks 2-3 plus `npm run test:assets`.
Expected: all selected tests pass.

- [ ] **Step 3: Run the full test suite**

Run: `npm test`.
Expected: no new failures; if unrelated existing failures remain, record their exact names separately.

- [ ] **Step 4: Perform visual verification**

Open Inspiration, My Files, and the image-template picker at desktop and narrow widths. Confirm mixed natural heights, stable gaps, unchanged interactions, and the template scrollbar remaining in its right gutter.
