# Shared Masonry Grid Design

## Goal

Provide one reusable Pinterest-style photo layout for the Inspiration gallery, My Files, and the image-template picker, without duplicating column or spacing rules.

## Existing State

- Inspiration and My Files already use the same CSS multi-column layout, but each feature owns a duplicate stylesheet.
- The image-template picker uses a row-aligned CSS grid and forces every card to a `4 / 5` aspect ratio.
- All three surfaces render ordered lists whose direct children are list items.

## Considered Approaches

1. Extract the existing CSS multi-column layout into a shared component. This preserves current behavior, requires no runtime measurement, and works in every supported browser. This is the selected approach.
2. Balance items with JavaScript by measuring card heights and assigning each item to the shortest column. This gives stricter visual balancing but adds resize observers, hydration work, and more complex focus and reading order.
3. Use native CSS Grid masonry. This is the cleanest future implementation but is not yet suitable as the only cross-browser production layout.

## Component Design

Create `src/components/ui/MasonryGrid/MasonryGrid.tsx` and `MasonryGrid.module.css`.

`MasonryGrid` renders an ordered list and accepts standard ordered-list attributes plus `children`. It owns only layout concerns:

- minimum column width of `17rem`;
- horizontal and vertical gaps from `var(--space-4)`;
- list margin, padding, and marker reset;
- full-width direct list items;
- `break-inside: avoid` so a card never splits between columns.

The component does not know about files, inspiration examples, templates, selection, loading, or dialogs. Each feature continues to own its card markup and interactions.

## Consumer Changes

### Inspiration

Replace the local `<ol className={styles.grid}>` with `<MasonryGrid>`. Remove only the duplicated masonry rules from `InspirationGallery.module.css`.

### My Files

Replace the local `<ol className={styles.grid}>` with `<MasonryGrid>`. Remove the now-unused `FilesGrid.module.css` import and stylesheet.

### Image-template picker

Replace the local grid list with `<MasonryGrid>`. Remove the three-, two-, and one-column grid rules and the forced card aspect ratio. Render each template image with its stored `mediaWidth` and `mediaHeight`, then size it to the card width with automatic height. Overlays, metadata, hover treatment, selection, search, and scrollbar placement remain unchanged.

## Accessibility and Ordering

All surfaces remain semantic ordered lists with list items. DOM order and keyboard order remain the source-data order. The shared component adds no interactive elements.

## Testing

- Add component tests proving ordered-list semantics, attribute forwarding, and children rendering.
- Add a style test for the shared column width, gap, reset, and non-breaking list items.
- Update the three consumer tests to require `MasonryGrid` and reject local masonry/grid ownership.
- Run focused tests, type checking, linting, and visual checks of all three surfaces at desktop and narrow widths.

## Non-goals

- No changes to card visuals or actions outside the template image's removal of forced cropping.
- No JavaScript height measurement or reordering.
- No changes to data fetching, filtering, preview dialogs, or file behavior.
