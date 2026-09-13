# Editor Tool Icons Design

## Goal

Replace the hand-written brush and lasso icons in the file editor with the exact SVG artwork supplied by the user.

## Assets

- Copy `D:/Downloads/brush-white.svg` to `web/platform/public/assets/icons/ui/brush-white.svg` without changing its paths or view box.
- Copy `D:/Downloads/lasso-white.svg` to `web/platform/public/assets/icons/ui/lasso-white.svg` without changing its paths or view box.

## Rendering

Render each asset through a CSS mask on a decorative span. The mask preserves the supplied silhouette while `background: currentColor` keeps the existing button-state behaviour: muted when inactive and white on hover or while selected. Keep the current `1.2rem` icon size, button labels, button dimensions, spacing, and interactions unchanged.

Remove the local `BrushIcon` and `LassoIcon` JSX functions so the project assets are the only source of their artwork.

## Verification

A style contract test must fail before implementation because the mask classes and asset paths do not exist, then pass after replacement. Run the editor component/style tests, asset validation, TypeScript, and ESLint.
