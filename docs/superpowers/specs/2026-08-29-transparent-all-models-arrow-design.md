# Transparent all-models arrow

## Goal

Reduce the visual weight of the «Все нейросети» shortcut while keeping its arrow immediately recognizable.

## States

- Default: the `3.5rem × 3.5rem` arrow container keeps its current rounded-square geometry, but both background and border are transparent. The arrow remains visible in the normal text color.
- Hover and keyboard focus: the container remains transparent and receives a `var(--color-accent)` border, matching the supplied reference.
- Preserve the current vertical hover movement, label, link target, dimensions, spacing, and arrow size.

## Verification

- Add a stylesheet contract before changing production CSS.
- Verify default, hover, and focus-visible computed styles locally.
- Run the relevant style test, full tests, lint, and typecheck.
