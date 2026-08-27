# Featured model shortcuts

## Goal

Replace the four square workspace navigation shortcuts with four flagship image models that are currently available in the server catalogue. Keep the final `Все нейросети` shortcut to the full catalogue.

## Selection

- Use the first four items from `loadImageModelCatalog()`.
- This matches the existing `Популярные нейросети` selection and treats server catalogue order as the current popularity order.
- Never render a hard-coded model that is absent from the live catalogue.
- If fewer than four models are returned, render only the available models.

## Navigation

- Each model shortcut opens `/app/image?model=<encoded model id>`.
- Disable Next.js prefetch for the model-specific generator links, matching existing model cards.
- Keep `Все нейросети` as the final link to `/app/models`.
- Remove the former links to chat, the generic image generator, catalogue, and inspiration from this rail only.

## Artwork

- Render each shortcut with the existing shared `ModelIcon` component.
- `ModelIcon` uses `assetPaths.images.models.fallback`, the same base artwork already used by the model catalogue, whenever no model-specific artwork is supplied.
- The component boundary must allow a model-specific `src` to be added later when the user provides artwork, without changing navigation or selection behavior.

## States

- Loading: show four inert square skeletons in the existing rail positions.
- Failure or an empty catalogue: show no fake models; retain only the `Все нейросети` link.
- Success: show up to four model shortcuts followed by `Все нейросети`.

## Scope

- Preserve the existing rail dimensions, responsive column rules, hero content, prompt, lower `Популярные нейросети` cards, and all other workspace sections.
- Do not change the model API contract or backend.
- Preserve all existing uncommitted changes for the NeiroHub and dialog eyebrow removals.

## Verification

- Add focused tests for catalogue selection, encoded model links, fallback artwork, old-link removal, order, loading, and failure behavior.
- Prove the new behavior fails before implementation and passes afterward.
- Run lint, typecheck, the full test suite, and a production build.
- Review the final diff for unrelated or lost changes.
