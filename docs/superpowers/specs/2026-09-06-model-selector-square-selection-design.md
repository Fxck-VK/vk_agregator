# Model Selector Square Selection Design

## Goal

Replace the circular model-selection indicator with the product's rounded-square selection style and make selected, hover, focus, and unselected states visually distinct.

## Interaction and appearance

- Keep every model row as the clickable control and retain its existing `aria-pressed` state.
- Keep the indicator at `1.5rem` square and use `var(--radius-sm)` instead of a circular radius.
- Unselected: transparent background and `var(--color-border)` border.
- Hover and keyboard focus: accent-colored indicator border.
- Selected: keep the neutral outer border and render a smaller solid accent square centered inside it without an icon or text mark.
- Keep the selected row's raised-surface background.
- Keep the selection indicator visible at the mobile breakpoint.
- Keep the floating vertical scrollbar outside the model-card content area by reserving `var(--space-4)` at the viewport's inline end.
- Do not show model prices or the decorative `✦` mark in selector rows.
- After removing the price, use three grid columns: model icon, flexible copy, and selection square.

## Panel motion

- Reveal the popover from its top edge toward the bottom when it opens.
- Hide the popover from its bottom edge toward the top when it closes.
- Animate a `clip-path` mask with a subtle opacity change so text, icons, and cards are not geometrically compressed.
- Use a short `220ms` duration and the shared product easing token.
- Keep the popover mounted during its exit animation, then remove it on `animationend`.
- Mark the exiting panel as hidden from assistive technology and disable pointer events while it closes.
- If the panel is reopened during its exit, cancel the closing state and play the opening animation again.
- Under `prefers-reduced-motion: reduce`, reduce both animations to effectively immediate motion while preserving the same mount/unmount lifecycle.

The rejected alternatives are animating `height`, which forces repeated layout work and complicates the scrollable body, and `scaleY`, which visibly distorts the panel contents.

## Inset selected indicator

- Keep the existing empty `selectionMark` element at `1.5rem` square.
- Keep its transparent background and neutral outer border in both selected and unselected states.
- Use `selectionMark::after` for the inner selected mark so no extra decorative markup is required.
- Size the inner mark at `0.75rem`, position it absolutely at `50%` on both axes, translate it by `-50% -50%`, and use a `0.25rem` radius plus `var(--color-accent)`.
- Hide the pseudo-element by default and show it only under `.optionSelected`.
- Preserve the accent outer border on hover and keyboard focus.

## Inset catalogue-link hover

- Remove the horizontal divider above the `Все нейросети и возможности` link.
- Keep the link's full footer row clickable instead of shrinking it with margins.
- Draw the hover and keyboard-focus surface with `catalogueLink::before` behind the content.
- Inset that surface by `var(--space-2)` at the top and `var(--space-3)` at the inline and bottom edges.
- Use `var(--radius-md)` for the internal rectangular surface.
- Keep the surface transparent at rest and use `var(--color-surface-raised)` only on hover or keyboard focus.
- Keep the label and arrow above the pseudo-element with a local stacking layer.

## Scope

Only the selector presentation, its open/close lifecycle, responsive columns, and local scrollbar clearance are changed. Price data remains available to the rest of the application; the shared `ScrollArea`, model loading, filtering, selection, navigation, and keyboard behavior remain unchanged.

## Verification

- A component test must confirm that neither selected nor unselected indicators contain a decorative icon or text mark.
- A style contract must require the rounded outer square, interaction colors, selected inner fill, and mobile visibility.
- A style contract must require the selector to pass `styles.scrollViewport` to `ScrollArea` and reserve `var(--space-4)` at the viewport's inline end.
- Component tests must confirm selector options contain no `✦` price label.
- A style contract must require three option columns at desktop and mobile sizes and no `.price` rule.
- The selector must be checked visually in the local browser in both selected and unselected states.
- Component tests must prove that closing keeps the dialog mounted until `animationend`, then removes it.
- A style contract must require opposite open/close reveal directions, the `220ms` duration, and reduced-motion handling.
- The opening and closing motion must be checked visually in the local browser.
- A style contract must require a geometrically centered `0.75rem` accent `::after` square through absolute 50% positioning and `translate: -50% -50%`, while prohibiting a selected-state fill on the outer indicator.
- A style contract must prohibit the footer divider and full-bleed hover background while requiring an inset `catalogueLink::before` hover/focus surface.
