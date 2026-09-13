# Header Control Radius Design

## Goal

Use the shared `--radius-sm` design token (`0.5rem`) for the three outer interactive controls in the workspace header: the model selector trigger, the balance top-up button, and the subscription plans button.

## Scope

- Change only the outer border radius of the three header controls.
- Preserve existing dimensions, padding, borders, colors, gradients, shadows, icons, spacing, hover states, active states, and responsive behavior.
- Keep the model selector popover and all nested icon shapes unchanged.

## Implementation

The model selector trigger owns its shape in `WorkspaceModelSelector.module.css`. The balance and tariff buttons own theirs in `WorkspaceHeader.module.css`. Each selector will reference `var(--radius-sm)` so all three controls share the existing global `0.5rem` token rather than duplicating a literal value.

## Verification

A stylesheet contract test will assert that `.trigger`, `.balance`, and `.tariffButton` each declare `border-radius: var(--radius-sm)`. Existing focused header and model-selector tests, type checking, and linting will guard against unrelated regressions.
