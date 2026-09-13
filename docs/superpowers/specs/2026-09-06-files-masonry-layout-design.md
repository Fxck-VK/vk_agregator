# Files Masonry Layout Design

## Goal

Display file cards as a compact masonry gallery so cards of different natural heights share the same viewport region instead of forming height-aligned rows.

## Layout

- Keep the existing full-width files content frame and its shared side gutters.
- Replace the row-based CSS Grid in `FilesGrid` with CSS multi-column layout.
- Use a preferred column width of `17rem`; the browser derives a responsive 3/2/1-column layout from the available width.
- Keep the existing `var(--space-4)` horizontal and vertical spacing.
- Prevent a card from splitting between columns with `break-inside: avoid`.
- Keep each card at its natural height and full column width.

## Ordering

The component receives jobs in the same order as today and does not sort or group them by dimensions. CSS flows that sequence from top to bottom through the columns while balancing their occupied height.

## Scope

Only `FilesGrid` layout styling changes. Card content, loading, retry behavior, filters, API order, and workspace gutters remain unchanged.

## Verification

- A style contract test must reject the old row grid and require multi-column masonry rules.
- Existing component tests, type checking, linting, and the asset test suite must remain green.
- A browser check must confirm that tall and short cards occupy the same gallery region without row-sized gaps.
