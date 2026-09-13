# File Card Hover Actions Design

## Goal

Show contextual actions over completed media in “Мои файлы” without changing the masonry layout or reducing the media area.

## Interaction

- A completed card remains full-bleed media in its resting state.
- Hovering the card, or focusing one of its actions from the keyboard, reveals a subtle dark overlay.
- The download affordance appears near the lower-left edge as a white download icon with the label “Скачать”. The existing full-card download link remains the actual download target.
- Hovering directly over the download affordance fades its icon and label together to `70%` opacity with a short transition. Hovering elsewhere on the card does not dim the affordance.
- A white trash icon appears in a compact control near the upper-right edge.
- The trash control is visibly present but disabled until the server exposes a persistent file-deletion endpoint. It is labelled “Удаление файлов пока недоступно” so it does not pretend to delete data only locally.
- On touch devices, where hover is unavailable, the actions remain visible.
- With reduced motion enabled, the overlay and download affordance change state without animation.

## Visual treatment

The overlay stays inside the rounded card boundary. It uses a transparent-to-dark gradient so the image remains visible, while the controls remain readable. Controls are inset from the card edges and do not affect card dimensions, column packing, or side margins.

## Accessibility

The card keeps one keyboard-focusable download link with an explicit accessible name containing the prompt. The hover state also appears with `:focus-within`. Decorative SVG paths are hidden from assistive technology. The unavailable delete control is a disabled button with an explicit accessible label and title.

## Verification

- Component tests verify the visible download label, working download link, and unavailable delete control.
- CSS contract tests verify hidden/resting and revealed hover/focus states, the direct download hover fade, edge insets, touch visibility, and reduced-motion behavior.
- The focused FileCard test suite, the complete Vitest suite with four workers, asset validation, type checking, linting, and a browser hover inspection must pass.

## Scope

This slice does not add a destructive backend endpoint. Persistent deletion will require a separately designed authenticated, CSRF-protected server mutation and confirmation flow.
