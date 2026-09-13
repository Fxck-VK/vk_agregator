# Shared Model Card Design

## Goal

Use one model-card component and one presentation-data source in both the landing page's popular-model section and the full models catalog.

## Architecture

- `ModelCard` owns the shared visual markup, full-card link, hover/focus states, icon, minimum verified price, name, and description.
- A model-description helper keyed by stable model id supplies the approved Russian copy and a safe fallback for unknown catalog entries.
- `FeaturedModels` owns only loading, the 4-to-6 reveal state, the reveal animation class, and the catalogue action.
- `ModelsCatalog` owns filtering and category state and renders the same `ModelCard` without a separate card variant.
- Both consumers continue to load the same `loadImageModelCatalog` data source.

## Shared Appearance

The shared component uses the approved popular-card style: icon and price on the top row, model name and description below, compact `11.5rem` minimum height, NeiroHub surfaces, and the existing full-card hover/focus behavior. Catalog-only quality and reference rows are removed from the card surface; their source data remains available for filtering and generation flows.

## Compatibility

The popular section preserves its 4/6 reveal behavior, animation, loading skeleton, and “Все нейросети” action. Accessible link names and URL encoding remain unchanged.

## Verification

- Tests require `FeaturedModels` to render `ModelCard` instead of duplicating link markup.
- Component and catalog tests require the same description and unprefixed minimum price.
- Style contracts require the shared compact card and remove obsolete duplicate card selectors.
- Browser inspection compares the same model on the landing page and catalog.
- Full tests, typecheck, lint, and diff validation must pass.
