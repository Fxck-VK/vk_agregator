# Chat Submit Button States Design

## Goal

Bring the shared send button into the current hollow control style without changing its size, icon asset, accessibility, or submit behavior.

## Visual states

- Empty input: the button stays disabled, transparent, and without an accent border; the arrow is muted.
- Text present: the enabled button remains transparent and gains the accent border; the arrow stays muted.
- Text present and pointer hover: the accent border remains and the arrow becomes fully white.
- Keyboard focus: the existing visible focus outline remains available.

## Component boundary

`ChatSubmitButton` continues to derive all visual states from its existing `disabled` prop. Its consumers already set that prop from whether the normalized input contains text, so no new state or callbacks are required.

## Testing

Stylesheet tests cover transparent backgrounds, border visibility, and arrow opacity for disabled, enabled, and hover states. Existing component and composer tests continue to cover the shared asset, accessible label, disabled state, and submit behavior.
