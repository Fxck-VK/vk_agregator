# Inspiration thumbnail rail design

## Goal

The inspiration preview must show every inspiration image and video in a scrollable thumbnail rail. Opening any gallery card selects that item, and selecting another thumbnail updates the preview and its metadata without closing the dialog.

## Structure

`InspirationGallery` owns the selected index because it owns the complete `inspirationExamples` collection. `InspirationExampleCard` only renders and opens a card. `InspirationExampleDialog` renders the shared modal, receives the complete collection, and reports thumbnail selection and close events to the gallery.

## Interaction

- The thumbnail order matches the masonry gallery order.
- The selected thumbnail has `aria-current="true"` and an accent border.
- The rail scrolls vertically on desktop and horizontally on narrow screens.
- Changing the selected thumbnail updates the preview, model, prompt, download, share, and recreate actions.
- Closing restores focus to the card that opened the dialog.
- The selected thumbnail is scrolled into view.

## Verification

Component tests cover the complete thumbnail collection, initial selection, switching, and focus restoration. Browser verification checks the rendered rail and selected preview.
