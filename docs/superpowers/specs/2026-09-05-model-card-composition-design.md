# Model Card Composition Design

## Goal

Restyle catalog cards to match the compact vertical composition in the approved reference while preserving NeiroHub colors, existing model data, and full-card link behavior.

## Composition

- The model icon sits at the top left.
- The verified minimum launch price sits at the top right.
- The model name appears below the top row, followed by the existing image-generation type label as supporting copy.
- Quality options and reference support are grouped at the bottom of the card.
- The card uses the existing surface, border, focus, and hover tokens.
- The desktop and narrow layouts share the same composition; narrow cards may wrap their metadata naturally.

## Scope

No API fields, catalog filtering, navigation, pricing calculation, or model data change. The generic type label is used as supporting copy because the current `ImageModel` contract does not provide per-model descriptions.

## Verification

- Component tests verify content order and retained data.
- CSS contract tests verify the new grid areas, compact height, and bottom metadata group.
- The running catalog is inspected at desktop width.
- Full tests, typecheck, lint, and diff validation must pass.
