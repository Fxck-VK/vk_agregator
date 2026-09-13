# Freehand Lasso Design

## Goal

Add a Photoshop-style freehand lasso to the file image editor. The user holds the primary pointer button, traces an irregular area, and releases the button to close the contour and add it to the purple edit mask.

## Interaction

- `Лассо` is a selection button beside `Кисть` and uses the existing active, hover, focus, and toggled-off states.
- Selecting it deactivates brush and eraser. Pressing the active lasso button again turns all drawing tools off.
- Pointer movement records the freehand contour and immediately shows an open line through every recorded point, so the visible trace follows the held pointer.
- Releasing the pointer removes the open draft line, completes the action, and lets the SVG polygon close the last point back to the first automatically.
- Lasso actions participate in the existing clear, undo, and redo history.
- The brush-size control is absent while lasso is selected and returns with its previous value after lasso is deactivated or another tool is selected.
- The lasso uses the edit surface's crosshair cursor and never shows the circular brush cursor or brush-size preview.

## Architecture

Extend the existing `EditTool` union with `lasso`. Keep lasso points in the existing `EditStroke` structure and route them through `FileEditorController.addStroke`, so history remains the single source of truth. While the pointer is held, render the draft lasso as an open white SVG `polyline` inside the existing luminance mask; this makes the existing purple overlay show a continuous trace. Render completed lasso strokes as filled SVG `polygon` elements so the released contour becomes a closed selected area.

No new component state, history stack, canvas, or dependency is introduced. The inline icon is local to `FileEditorPanel`, matching the current brush and rectangle icon pattern.

## Edge Behaviour

- A press without movement may show no visible draft and then produce a zero-area polygon with no visible mask; it remains a normal history action, consistent with the existing brush completion flow.
- Pointer cancellation discards only the draft contour, using the current cancellation path.
- Existing brush and eraser rendering is unchanged.

## Accessibility and Tests

The button exposes the accessible name `Лассо` and `aria-pressed`. Component tests cover presence, mutual exclusion, repeated-click deactivation, brush-size visibility, the open draft line during movement, removal of that draft after release, polygon rendering, automatic SVG closure semantics, and undo/redo integration. Existing editor tests, type checking, and linting must remain green.
