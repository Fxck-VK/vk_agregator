# Remove Rectangle Tool Design

## Goal

Remove the `Квадрат` selection tool completely from the file image editor rather than merely hiding its button.

## Scope

- Remove the `Квадрат` button and its inline icon from the editor panel.
- Remove `rectangle` from the editor tool type.
- Remove rectangle-specific SVG mask rendering and pointer-processing branches.
- Keep brush, lasso, eraser, their toggle behaviour, shared history, and mask rendering unchanged.
- Return the selection button grid to two equal columns for `Кисть` and `Лассо`.
- Update tests so the absence of `Квадрат` is an explicit interface requirement.

## Architecture

The controller remains the single source of truth. After removal, freehand tools share one point-collection path: brush and eraser additionally update the circular cursor, while lasso records points without that cursor. No replacement component, feature flag, or dormant rectangle implementation remains.

## Verification

The component test must first fail while the `Квадрат` button still exists, then pass after removal. The focused editor tests, editor style tests, TypeScript check, and ESLint must pass. A final text search excluding tests must find no rectangle tool identifiers or `Квадрат` UI copy in `web/platform/src`; the test keeps the copy only to assert that the button is absent.
