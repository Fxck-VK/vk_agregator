# Model Selector Square Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the model selector's circular dot with a solid rounded-square selected state.

**Architecture:** Preserve the existing button and `aria-pressed` model-selection behavior. Keep the indicator empty in the markup and express selected, hover, focus, and unselected states entirely through the existing CSS module.

**Tech Stack:** React 19, Next.js 16, CSS Modules, Testing Library, Vitest

## Global Constraints

- Keep the entire model row clickable.
- Keep `aria-pressed` as the accessible source of truth.
- Do not render an icon or text mark inside the indicator.
- Keep the indicator visible on mobile.
- Reserve `var(--space-4)` between the model cards and the floating vertical scrollbar without changing the shared `ScrollArea`.
- Do not display model prices or `✦` marks in selector rows.
- Keep exactly three option columns at desktop and mobile widths: icon, copy, selection square.
- Reveal the popover top-to-bottom and close it bottom-to-top in `220ms` without scaling its contents.
- Keep the exiting popover mounted until its own closing animation ends.
- Keep the selected indicator's outer square transparent and render a centered `0.75rem` accent square inside it.
- Center the inner selected square independently of grid layout with absolute 50% positioning and `translate: -50% -50%`.
- Remove the catalogue footer divider and inset its visual hover surface without shrinking the link hit area.
- Do not commit or publish the shared worktree as part of this task.

---

### Task 1: Square model-selection indicator

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: `CheckIcon(props: Readonly<IconProps>)` and the existing `isSelected` boolean.
- Produces: a decorative check inside the existing `selectionMark` while `aria-pressed` remains on the option button.

- [x] **Step 1: Write failing behavior and style tests**

Require the selected option to contain `[data-icon="check"]`, require unselected options not to contain it, and require square indicator styles with hover/focus and selected color states. Require the mobile rule to retain four grid columns and not hide `.selectionMark`.

- [x] **Step 2: Verify RED**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: FAIL because the component renders `●`, the indicator is circular, selected fill is absent, and mobile hides it.

- [x] **Step 3: Implement the minimal markup and styles**

Import `CheckIcon`, replace the text dot with `{isSelected ? <CheckIcon className={styles.selectionIcon} /> : null}`, add rounded-square state rules, and keep four option columns below `48rem`.

- [x] **Step 4: Verify GREEN**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: both test files pass.

- [x] **Step 5: Verify visually**

Open the model selector locally and confirm the selected row shows a filled purple rounded square with a check, unselected rows show empty neutral squares, and hover/focus use the accent border.

- [x] **Step 6: Run full checks**

Run: `npm test`

Run: `npm run typecheck`

Run: `npm run lint`

Expected: all commands exit with code `0` and no warnings.

### Task 2: Remove the selected-state check

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: the existing `isSelected` class and `aria-pressed` boolean.
- Produces: an empty decorative `selectionMark` whose selected state is represented by solid accent fill only.

- [x] **Step 1: Write the failing test**

Require both selected and unselected option indicators to contain no `[data-icon]` element and no text content. Remove the obsolete style assertion for `.selectionIcon`.

- [x] **Step 2: Verify RED**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: FAIL because the selected option still renders `CheckIcon`.

- [x] **Step 3: Remove the check icon**

Delete the `CheckIcon` import, leave `<span aria-hidden="true" className={styles.selectionMark} />`, remove `.selectionIcon`, and keep the accent fill rule unchanged.

- [x] **Step 4: Verify GREEN and visually inspect**

Run the focused test command again, then open the model selector and confirm that the selected model uses a plain filled purple square while unselected models use empty outlined squares.

- [x] **Step 5: Run full checks**

Run `npm test`, `npm run typecheck`, and `npm run lint`. All commands must exit with code `0`.

### Task 4: Remove prices from selector rows

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: the existing `ImageModel` catalogue items and selected-model state.
- Produces: selector rows containing only `ModelIcon`, `optionCopy`, and `selectionMark`.

- [x] **Step 1: Write failing behavior and style tests**

Require every selector option to omit the `✦` price label. Require `.option` and its mobile override to use `grid-template-columns: auto minmax(0, 1fr) auto`, and require the stylesheet to contain no `.price` rule.

- [x] **Step 2: Verify RED**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: FAIL because the component still renders minimum prices and the CSS still defines a four-column layout plus `.price`.

- [x] **Step 3: Remove price presentation**

Delete `getMinimumPrice`, the mapped `minimumPrice` value, and the `.price` span. Change both option grid declarations to `auto minmax(0, 1fr) auto`, then remove the unused `.price` rule.

- [x] **Step 4: Verify GREEN and inspect visually**

Run the focused command again. Open the selector locally and confirm rows show only the icon, model text, and selection square with no price or unused gap.

- [x] **Step 5: Run full checks**

Run `npx vitest run --maxWorkers=4`, `npm run test:assets`, `npm run typecheck`, and `npm run lint`. All commands must exit with code `0`.

### Task 6: Inset selected-state square

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: the existing empty `.selectionMark` and `.optionSelected` class.
- Produces: `.selectionMark::after` as a centered decorative inner square; no JSX or accessibility contract changes.

- [x] **Step 1: Write the failing style contract**

Require `.selectionMark::after` to use `inline-size: 0.75rem`, `block-size: 0.75rem`, `border-radius: 0.25rem`, `background: var(--color-accent)`, and `opacity: 0`. Require `.optionSelected .selectionMark::after` to set `opacity: 1`, while `.optionSelected .selectionMark` keeps `background: transparent` and `border-color: var(--color-border)`.

- [x] **Step 2: Run the focused style test and verify RED**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: FAIL because the selected outer mark is still fully filled and no inner pseudo-element exists.

- [x] **Step 3: Implement the inset mark**

Add the following pseudo-element and replace the selected outer fill:

```css
.selectionMark::after {
  inline-size: 0.75rem;
  block-size: 0.75rem;
  border-radius: 0.25rem;
  background: var(--color-accent);
  opacity: 0;
  content: "";
}

.optionSelected .selectionMark {
  border-color: var(--color-border);
  background: transparent;
}

.optionSelected .selectionMark::after {
  opacity: 1;
}
```

- [x] **Step 4: Run the focused tests and verify GREEN**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: both files pass with no warnings.

- [x] **Step 5: Verify visually and run full checks**

Open the selector locally and confirm the selected model shows a small centered purple square inside the neutral outline. Then run `npx vitest run --maxWorkers=4`, `npm run test:assets`, `npm run typecheck`, and `npm run lint`; every command must exit with code `0`.

### Task 7: Inset catalogue footer hover

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: the existing `.catalogueLink` anchor and shared spacing/surface tokens.
- Produces: a decorative `.catalogueLink::before` layer inset inside the full-size link.

- [x] **Step 1: Write the failing style contract**

Require `.catalogueLink` to use `position: relative`, `isolation: isolate`, no `border-block-start`, and no full-bleed hover background. Require `.catalogueLink::before` to use `position: absolute`, `inset: var(--space-2) var(--space-3) var(--space-3)`, `border-radius: var(--radius-md)`, transparent background, and empty content. Require hover/focus to apply `var(--color-surface-raised)` only to the pseudo-element, with direct children positioned above it.

- [x] **Step 2: Run the focused style test and verify RED**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: FAIL because the footer still has a divider and its hover background fills the entire link.

- [x] **Step 3: Implement the inset hover surface**

Remove `border-block-start` and direct hover background. Add a locally isolated pseudo-element with the specified inset and radius, then give direct children `position: relative` and `z-index: 1`. Apply the raised background to `:hover::before` and `:focus-visible::before`.

- [x] **Step 4: Run focused tests and verify GREEN**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: both files pass with no warnings.

- [x] **Step 5: Verify visually and run full checks**

Open the selector locally, confirm the divider is absent, and hover the catalogue link to confirm visible spacing on every outer edge. Then run `npx vitest run --maxWorkers=4`, `npm run test:assets`, `npm run typecheck`, and `npm run lint`; every command must exit with code `0`.

### Task 3: Keep the scrollbar clear of model cards

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: `ScrollArea`'s existing `viewportClassName?: string` prop.
- Produces: `styles.scrollViewport`, a selector-local viewport class with `padding-inline-end: var(--space-4)`.

- [x] **Step 1: Write the failing style contract**

Require the component to render `<ScrollArea className={styles.scrollArea} viewportClassName={styles.scrollViewport}>` and require `.scrollViewport` to contain `padding-inline-end: var(--space-4)`.

- [x] **Step 2: Verify RED**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: FAIL because the viewport class and its right-side reserve do not exist.

- [x] **Step 3: Add local viewport clearance**

Pass `viewportClassName={styles.scrollViewport}` to `ScrollArea` and add:

```css
.scrollViewport {
  padding-inline-end: var(--space-4);
}
```

- [x] **Step 4: Verify GREEN and inspect visually**

Run the focused style test again. Open the selector locally and confirm that card backgrounds and selection squares end before the scrollbar track at desktop and narrow widths.

- [x] **Step 5: Run full checks**

Run `npm test`, `npm run typecheck`, and `npm run lint`. All commands must exit with code `0`.

### Task 5: Animate the selector panel lifecycle

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: trigger, Escape, outside pointer, option selection, catalogue navigation, and the existing `--motion-normal` token (`220ms ease`).
- Produces: `PopoverState = "closed" | "open" | "closing"`, `data-state`, and an `animationend`-driven exit lifecycle.

- [x] **Step 1: Write failing lifecycle and style tests**

Require the dialog to use `data-state="open"` after trigger activation. On Escape or an outside pointer, require `data-state="closing"`, `aria-hidden="true"`, and continued DOM presence until `fireEvent.animationEnd(..., { animationName: "workspaceModelSelectorClose" })`. Require the stylesheet to animate open and closing states with `clip-path: inset(0 0 100% 0)`, `var(--motion-normal)`, and a `1ms` reduced-motion duration.

- [x] **Step 2: Run the focused tests and verify RED**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: FAIL because the dialog has no `data-state`, unmounts immediately, and defines no reveal keyframes.

- [x] **Step 3: Implement the animated lifecycle**

Replace the boolean mount source with `PopoverState`. Set `closing` for every close path, keep rendering until the popover's own `workspaceModelSelectorClose` animation ends, and restore the existing trigger focus behavior. Add open/closing classes through `[data-state]`, mask keyframes, pointer blocking during exit, and reduced-motion duration.

- [x] **Step 4: Run the focused tests and verify GREEN**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: both files pass with no warnings.

- [x] **Step 5: Verify motion visually**

Open and close the selector locally. Confirm the top edge remains fixed while the lower edge reveals downward on open and retracts upward on close; confirm text and cards do not squash.

- [x] **Step 6: Run full checks**

Run `npx vitest run --maxWorkers=4`, `npm run test:assets`, `npm run typecheck`, and `npm run lint`. All commands must exit with code `0`.

### Task 8: Geometrically center the selected-state square

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: the existing `selectionMark::after` decorative square.
- Produces: layout-independent geometric centering inside the outer selection outline.

- [x] **Step 1: Write the failing style contract**

Require `.selectionMark` to establish a positioned containing block. Require `.selectionMark::after` to use `position: absolute`, `inset-block-start: 50%`, `inset-inline-start: 50%`, and `translate: -50% -50%`.

- [x] **Step 2: Run the focused style test and verify RED**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: FAIL because the inner square still relies only on grid alignment.

- [x] **Step 3: Implement absolute geometric centering**

Set `.selectionMark` to `position: relative`, then position its `::after` pseudo-element absolutely at 50% on both logical axes and translate it back by half its own dimensions.

- [x] **Step 4: Run focused tests and verify GREEN**

Run: `npx vitest run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`

Expected: both files pass with no warnings.

- [x] **Step 5: Verify visually and run full checks**

Open the selector locally and confirm the inner purple square is geometrically centered in the outline. Then run `npx vitest run --maxWorkers=4`, `npm run test:assets`, `npm run typecheck`, and `npm run lint`; every command must exit with code `0`.
