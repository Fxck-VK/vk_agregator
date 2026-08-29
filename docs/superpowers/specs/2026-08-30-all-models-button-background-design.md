# All-models button PNG background design

## Goal

Replace the solid violet fill of the “Все нейросети” button in the popular-models section with the supplied transparent PNG artwork while keeping button geometry and text controlled by code.

## Approved composition

- Apply the artwork only to the `.primaryButton` link beside the “Популярные нейросети” heading.
- Keep the existing label, destination, minimum height, padding, pill radius, font weight, and layout behavior.
- Render the PNG as a decorative `next/image` layer so Next.js can serve a size-appropriate image instead of downloading the full 2172 × 724 source to the small button.
- Keep the link text as a separate foreground layer and preserve the accessible name “Все нейросети”.
- Remove the solid accent and accent-strong fills from this button.
- On hover, keep the existing upward translation and brighten only the artwork slightly; do not resize the button.
- Do not apply this background to other primary, secondary, light, or outline buttons.

## Asset handling

Store the unchanged supplied PNG at `public/assets/images/workspace/all-models-button-background.png` and expose it as `assetPaths.images.workspace.allModelsButtonBackground`. The source has a real alpha channel.

## Verification

- A failing contract test must cover the asset path, decorative image markup, foreground label, transparent button background, and artwork-only hover brightness.
- Verify the focused test fails before implementation and passes afterward.
- Inspect the normal and hover states locally at `http://localhost:7158/app`.
- Run the platform tests, asset validation, lint, and typecheck before committing.
