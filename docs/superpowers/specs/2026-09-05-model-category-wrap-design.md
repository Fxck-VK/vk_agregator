# Model Category Wrap Design

## Goal

Replace the horizontal category scroller in the models catalog with a wrapping layout that occupies two rows at the regular desktop content width.

## Behavior

- Category pills retain their content-sized width and existing order.
- The list wraps with equal horizontal and vertical gaps.
- The horizontal scrollbar and its custom scrollbar styling are removed.
- Narrow layouts may use more than two rows so labels are never clipped or forced outside the content frame.
- Existing tab semantics, selection, focus, and keyboard navigation are unchanged.

## Verification

- A CSS contract test requires wrapping and rejects horizontal scrolling.
- The focused styles test, full test suite, typecheck, lint, and diff check must pass.
- The running models page is visually checked at a desktop viewport to confirm two rows and no horizontal scrollbar.
