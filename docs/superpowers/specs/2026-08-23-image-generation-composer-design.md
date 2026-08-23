# Image generation composer design

## Goal

Replace the separate image-generation form with the platform's shared composer while preserving the existing image preparation, confirmation, job tracking, result, and history flows.

## Scope

- Remove the visible `ImageGenerationEditor` card, including its duplicated model selector, labels, textarea, price row, and rectangular submit button.
- Keep the floating workspace model selector as the single model selector.
- Keep `ImageJobHistory` below the generation flow.
- Do not change backend contracts, billing, confirmation, queueing, or history behavior.

## Component architecture

`ChatComposer` remains the single text-and-media input surface. It receives an optional, domain-neutral controls slot instead of image-specific boolean flags.

`ImageGenerationComposer` is a thin image-domain wrapper around `ChatComposer`. It owns the image settings toolbar and maps controlled panel state to the shared composer:

- prompt;
- media picker;
- image quality;
- submit state;
- price note;
- preparation error.

`ImageGenerationPanel` continues to own catalog loading, selected model, selected quality, prompt, price calculation, idempotency, confirmation, activation, tracking, and result state.

## Controls and data boundaries

The initial working controls are:

- `Загрузить медиа`, using the existing shared media menu and file picker;
- quality, populated from `quality_options` of the selected server model;
- circular submit button;
- server-derived price below the composer.

The visual toolbar is extensible for templates, aspect ratio, and image count, but those controls are not presented as working until the web API exposes and validates the corresponding fields. The frontend must not silently display settings that are ignored by the server.

The floating model selector and the generation panel share `WorkspaceModelSelection`. A model change in the floating selector updates the generation panel and resets quality to the selected model's default.

## Interaction states

- Loading: keep the page stable and show a compact neutral loading state where the composer will appear.
- Ready: show the shared composer with image controls and current price.
- Preparing: retain the entered prompt, disable settings and submit, and indicate progress on the submit control.
- Preparation error: return to the same composer with the prompt preserved and show a compact error below it.
- Confirmation, tracking, and result: retain their existing functional components.
- Catalog failure: retain the current retry action.

## Responsive behavior

On desktop, settings form one toolbar along the bottom of the composer. On narrow screens, controls wrap without horizontal overflow; the submit button remains on the trailing edge. The history section remains below the active generation state.

## Testing

- `ChatComposer` renders optional custom controls without affecting chat variants.
- `ImageGenerationComposer` renders media, quality, price, and circular submit behavior.
- model selection in `WorkspaceModelSelection` updates image model and default quality;
- prepare requests still contain only supported server fields;
- errors preserve the prompt;
- confirmation, tracking, result, and history flows remain available;
- existing conversation, new-chat, hero, and workspace composer tests continue to pass.
