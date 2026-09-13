# Full-width files grid

## Goal

Fill the complete content area allocated to `/app/files` with file cards while preserving the shared workspace shell width and its existing side gutters.

## Design

The shared `WorkspacePageFrame` keeps its `66rem` shell, responsive inline padding, and default `46rem` content column for other pages. The files page passes a local frame class that overrides only `--workspace-content-frame-width` to `100%`. Its existing grid remains `repeat(auto-fill, minmax(min(100%, 17rem), 1fr))`, so available space is divided into three, two, or one responsive columns without fixed viewport-specific card widths.

No card dimensions, image treatment, header spacing, or page gutters change.

## Verification

- A style contract proves the override belongs only to the files page.
- The contract preserves the shared frame gutter and responsive grid definition.
- Browser verification confirms the wider grid at desktop width.

