# Workspace Polish and Model Catalog Design

## Goal

Finish the visual quality of the existing workspace chat controls and replace the `/app/models` placeholder with a truthful, frontend-only catalog of the image models currently available to the signed-in user.

## Scope and decisions

This slice changes only `web/platform`. It must not change Go services, database schemas, API routes, account/session behavior, or the already completed ordinary-chat mechanics.

The workspace part is intentionally narrow:

- Fix the rename input whose native white background makes the existing light text unreadable.
- Keep the rename/archive behavior, focus handling, Escape behavior, mobile drawer behavior, and mutation lifecycle unchanged.
- Make the action popover and its Save/Cancel controls visually consistent with the dark platform surface.
- Retain the existing semantic loading, error, and empty states rather than adding duplicate mechanisms.

The catalog uses `GET /web/v1/image-models`, the existing authenticated same-origin browser API. Its DTO is the source of truth and exposes only a model id, display name, quality options/default, and reference-image capability. Therefore the catalog must not invent prices, provider names, descriptions, video/text categories, or model detail pages.

The selected approach is a real data-driven catalog rather than static mock cards or a new backend catalog API. It keeps the user-facing screen useful now, does not introduce false information, and leaves room for a broader cross-modal catalog once the server deliberately exposes that data.

## Component boundaries

### ConversationRow visual surface

`features/conversations/ConversationRow` remains the sole owner of the action popover. Its CSS explicitly gives rename inputs a dark background, readable text and caret, stable borders, and aligned form actions. No interaction logic moves into Sidebar components.

### ModelsCatalog

Create `features/models/ModelsCatalog` as a client feature with its own component, CSS module, pure filtering helper, and tests.

- Fetches models only through `webBrowserFetch("/web/v1/image-models")` after the component mounts.
- Parses data through the existing `parseImageModelList` contract.
- Keeps query and filters in client state; no URL mutation or server-side data fetch is required.
- Filters are only derived from real DTO fields: case-insensitive name/id search, reference-image capability, and available quality values.
- Every card identifies the truthful type, `Генерация изображений`, and shows only quality/reference facts contained in the DTO.
- A card links to `/app/image?model=<encoded-public-id>`.

### Image generator handoff

`ImageGenerationPanel` reads the optional `model` URL query through the client router. On explicit opening of the generator, it selects that model if it exists in the fetched catalog; an absent, unknown, or unavailable id safely falls back to the first available model. The generator remains closed until the user explicitly opens it, preserving its existing loading and consent behavior.

## States and accessibility

- Catalog loading uses a status region; failed fetch or invalid data exposes one neutral alert; an empty valid list renders a dedicated empty state.
- Search has an explicit label. Capability and quality controls use labelled native form controls so keyboard and screen-reader behavior is native and predictable.
- Every model card uses a normal link to the generator; no click-only card behavior.
- Existing rename field keeps its accessible label and its typed draft after mutation errors.
- Small screens collapse the catalog grid into one column without horizontal scrolling.

## Acceptance criteria

1. A rename field has an explicit dark surface, visible light text/caret, and readable, aligned Save/Cancel controls in Chrome and the existing test environment.
2. Existing rename/archive controls preserve their current API calls, draft retention, error alerts, focus, Escape, and drawer behavior.
3. `/app/models` fetches and renders only valid models from `/web/v1/image-models`; it has loading, failure, and empty states.
4. Search, reference capability, and quality filters combine using only model DTO fields and work case-insensitively for model names and public ids.
5. Catalog cards contain no fabricated price/provider/description fields and link to the matching generator query.
6. The generator honors a known `model` query after its normal model fetch and falls back safely for unknown ids.
7. Typecheck, lint, focused tests, full platform tests, production build, packaging check, and `git diff --check` pass before DEV rollout.
