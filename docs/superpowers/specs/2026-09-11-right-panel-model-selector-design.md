# Right Panel Model Selector Design

## Goal

Give the shared model selector a right-panel presentation for the Animate, Enhance, Remove background, and Edit modes without changing the compact workspace-header selector.

## Interaction and Appearance

- The right-panel trigger fills the available panel width.
- Its border uses the neutral panel-control color and `var(--radius-sm)` corners.
- Its interior is transparent, so it exactly inherits the right panel's background.
- The model icon and chevron keep fixed sizes while the model name receives all remaining width.
- The selected model name never uses an ellipsis. If the available width is insufficient, it wraps onto additional lines and the trigger grows vertically.
- Hover does not change the trigger's border, background, shadow, or position. Keyboard focus remains visible for accessibility, while open/close, placement, filtering, selection, and pricing behavior remains unchanged.

## Architecture

Add a typed `ModelSelectorVariant` with `compact` and `panel` values to the existing shared `ModelSelector`. The root exposes the selected value through `data-variant`; shared CSS applies the full-width outlined styling only below `[data-variant="panel"]`. `FileTaskModelSelector` selects the panel variant for all four file tasks, while `WorkspaceModelSelector` continues to use the default compact variant.

## Testing

Component tests verify that file-task selectors request the panel variant while the default selector remains compact. A stylesheet contract test verifies the full width, accent border, transparent background, small-radius token, and wrapping name rules. Existing model-selector and file-preview tests, TypeScript, and ESLint must remain green.
