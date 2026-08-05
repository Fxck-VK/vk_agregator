# Workspace model selector design

## Scope

Add one reusable model selector to the persistent authenticated workspace header. It is visible on every `/app` route except the whole `/app/inspiration` section.

## Behaviour

- Closed state shows the selected model icon, name, and disclosure chevron.
- Opening shows a searchable popover with its own scroll area, image-model category, server-provided models, capability-derived descriptions, minimum server-provided star price, selection markers, and a fixed footer link to `/app/models`.
- Selecting a model closes the popover and navigates client-side to `/app/image?model=<id>`.
- Search is local over the already cached safe catalogue; no request is made per keystroke.
- Escape and an outside click close the popover. Opening focuses search. Escape and model selection restore trigger focus; an outside click preserves focus on the clicked target.
- On `/app/inspiration` and nested inspiration routes the selector is not rendered; the section title remains.

## Data and safety

The selector consumes only `/web/v1/image-models` through the existing in-memory catalogue cache. It does not invent models, prices, providers, or private metadata. Descriptions are derived only from public capability fields. Later text/video categories require a server-owned safe catalogue extension.

## Responsive layout

The popover is anchored under the header pill on desktop and constrained to the viewport on mobile. Only the model list scrolls; search and footer stay visible.
