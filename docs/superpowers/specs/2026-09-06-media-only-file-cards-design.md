# Media-only file cards

## Goal

Make every completed file card a clean, edge-to-edge media tile with no visible status, title, model metadata, or download caption.

## Chosen approach

Completed cards use the whole card as the download link. The image fills the complete card surface and keeps the artifact aspect ratio, so masonry columns continue to combine portrait, landscape, and square results naturally. The link receives an accessible label containing the job prompt. Status, prompt, and model remain available as visually hidden metadata, while the visible interface contains no text or overlay.

Alternative approaches were rejected: removing download access would regress existing behavior, and a hover overlay would conflict with the requirement that the card contain no inscriptions.

## States without media

Queued, expired, payment-required, failed, and preview-loading cards keep their current explanatory text and retry controls. They do not have an artifact capable of filling the card. Once a result becomes available, the same card switches to the media-only presentation.

## Layout

- A completed card has no padding between its border and media.
- The artifact link, figure, and image all span the card width.
- The image has no inner border radius; clipping is owned by the card.
- Completed cards replace the visible metadata panel with an out-of-layout, visually hidden metadata block.
- Non-completed cards retain the current preview and metadata panel.

## Verification

Component tests assert that a completed card has one full-card download link, no visible metadata, and a working artifact path. Styling tests assert edge-to-edge sizing. Existing workspace tests protect lazy loading, filtering, retries, and error states.
