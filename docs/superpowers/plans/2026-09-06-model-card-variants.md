# Shared model card variants implementation plan

> Execution is in the current approved worktree. Do not commit or publish.

## Task 1: Specify the shared component contract

- Add failing tests for the selector variant, canonical navigation URL, shared description, and selected state.
- Move selector-card style expectations to the shared `ModelCard` stylesheet.
- Add a source contract proving the selector and shortcuts consume the shared presentation path.

## Task 2: Centralize model presentation

- Expand `model-card-content.ts` into the single resolver for description, optional artwork, and generator URL.
- Keep safe fallback content for unknown API models.

## Task 3: Add card variants

- Preserve the existing catalogue variant as the default.
- Add the compact selector button variant with the existing square selection indicator.
- Move compact-card CSS into `ModelCard.module.css`.

## Task 4: Adopt the shared component

- Replace the selector's duplicated model-row markup with `ModelCard variant="selector"`.
- Make `FeaturedModelShortcuts` read artwork and href from the shared resolver.
- Remove the shortcut-specific artwork override prop.

## Task 5: Verify

- Run targeted tests first.
- Run TypeScript, lint, asset checks, and the complete Vitest suite with `--maxWorkers=4`.
- Inspect the selector and catalogue in the running application when the browser is available.
