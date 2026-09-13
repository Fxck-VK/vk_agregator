# File Preview Dialog Design

## Goal

Open a full-screen preview when a ready card in “Мои файлы” is pressed, while keeping download as a separate explicit action.

## Approved interaction

- Pressing the media surface opens the selected artifact in a modal preview.
- The existing “Скачать” action downloads immediately and does not open the preview.
- The disabled delete control stays separate from both interactions.
- The shared `ModalBackdrop` supplies the backdrop, portal, scroll lock, Escape handling, backdrop-click handling, and exit animation.
- The dialog closes from its close button, Escape, or the shared backdrop.
- Images use their loaded natural aspect ratio inside an absolutely bounded `object-fit: contain` viewport, so portrait, landscape, and square media remain fully visible even when stored artifact metadata is inaccurate.

## Layout

- Desktop: a large preview surface on the left and a separate dark information panel on the right, with a visible gap between them.
- The preview surface uses two rows: a bounded, centered media viewport followed by an action rail in normal document flow. The rail never overlays the image.
- The close button stays in the upper-right of the preview surface. The image remains centered with free background around its natural aspect ratio.
- The information panel shows only the shared model icon treatment and model name at the top, with “Скачать” and “Поделиться” at the bottom; prompt and quality metadata are omitted.
- Below `72rem`, the surfaces stack and the preview receives an explicit viewport height so intrinsic media dimensions cannot expand or crop the layout.

## Scope

- Implement the preview for the image artifacts currently returned by the files API.
- Future editing controls are visible but disabled until their product flows exist.
- “Поделиться” uses the platform share sheet when available and copies the artifact URL as a fallback.
- No new API endpoint, dependency, or duplicate backdrop implementation is introduced.

## Accessibility and motion

- The surface is a labelled modal dialog with an autofocus close control.
- Card media is a semantic button; download remains a semantic link.
- Icon-only controls have accessible names.
- Reduced-motion users receive no dialog entrance transition.

## Testing

- Component tests cover opening a selected artifact, preserving the direct download action, closing, model metadata, and sharing fallback.
- Style contracts cover the responsive two-panel layout, contained media, and reduced-motion behavior.
- The focused feature tests, complete Vitest suite, assets, typecheck, lint, and local browser rendering are verified.
