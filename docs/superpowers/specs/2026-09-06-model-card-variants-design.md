# Shared model card variants

## Goal

Represent a model with one reusable component and one presentation registry while preserving the different layouts required by the catalogue and the workspace selector.

This design extends and supersedes the earlier single-layout decision in `2026-09-05-shared-model-card-design.md`.

## Design

- `ModelCard` exposes a discriminated `catalog` or `selector` variant.
- The catalogue variant remains a full-card link and keeps the current price and reveal hooks.
- The selector variant remains a button, exposes `aria-pressed`, and calls its activation callback with the model plus the canonical generator URL. The selector continues to own state updates, focus restoration, popover closing, and navigation.
- Model descriptions, optional model artwork, and the canonical generator URL are resolved by `model-card-content.ts`. Adding or replacing a model logo therefore requires changing one model registry entry; every consumer receives the same artwork.
- `FeaturedModelShortcuts` is not visually converted into a card, but it uses the same presentation resolver so its logo and target cannot diverge from the card variants.
- Selector-only card styles move from `WorkspaceModelSelector.module.css` into `ModelCard.module.css`. The selector layout file retains only panel, section, list, and scrolling styles.

## Compatibility

- Existing `ModelCard` calls default to the catalogue variant.
- Existing model URLs remain `/app/image?model=<encoded id>`.
- Search, grouped sections, animation, scrollbar spacing, selection state, and catalogue footer behavior remain unchanged.

## Verification

- Component tests cover both variants and their shared URL/content.
- Contract tests verify that the selector and shortcuts consume the shared model presentation path.
- Selector behavior and styling tests protect current interaction and layout.
- Run targeted tests, type checking, linting, asset verification, and the complete Vitest suite with four workers.
