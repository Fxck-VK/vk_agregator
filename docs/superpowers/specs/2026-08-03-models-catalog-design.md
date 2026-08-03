# NeiroHub Models Catalog Visual Refresh

## Goal

Replace the current dense image-model filter panel with the visual structure approved by the user: a catalogue heading, task-oriented search, horizontally scrollable category chips, and prominent two-column model cards.

## Scope

- Use only verified data returned by the existing image-model catalogue API.
- Keep the existing search behaviour and safe model-specific generator links.
- Show the smallest current price for each model when the API exposes `price_by_quality`; otherwise say that the price is shown after a quality is selected.
- Provide category chips for the future full catalogue. `Популярные` and `Изображения` show the current image catalogue. Other chips show a clear planned-category empty state instead of invented models.
- Do not add fabricated ratings, usage counts, provider names, or model descriptions.

## Components

- `ModelsCatalog`: owns loading state, search state, selected catalogue category, filtering, section title, and empty/loading/error states.
- `ModelCatalogToolbar`: renders the search field and accessible horizontal category tab list.
- `ModelCard`: renders a single real model with a deterministic visual icon, minimum verified price, truthful description, capabilities, and generator link.

## Interaction and accessibility

- Search updates the visible cards without another API request.
- Category chips support click, ArrowLeft/ArrowRight, Home, and End navigation.
- The horizontal chip row remains scrollable on narrow screens.
- The model grid is two columns on desktop and one column on small screens.
- All card actions retain explicit accessible names.

## Data and error handling

- The page continues to use `loadImageModelCatalog`; no new client endpoint or backend work is introduced.
- Loading and failure states keep the page shell stable.
- A category not yet connected to a backend catalogue gets an explanatory empty state, not a false zero-result search state.

## Verification

- Component tests cover new labels, verified prices, search, category selection, placeholder categories, and generator links.
- Styling tests assert horizontal chip overflow and mobile single-column layout.
- Run focused tests, the full frontend test suite, typecheck, lint, and production build before deploy.
