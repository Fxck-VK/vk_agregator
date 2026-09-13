# Inspiration preview template extraction

## Goal

Turn the existing, approved Inspiration preview dialog into the reusable source of truth for media preview layout and navigation without recreating its appearance from screenshots. The active Inspiration gallery must look and behave exactly as it does before the refactor. The existing dialog implementation remains in place as an unreferenced legacy fallback for quick rollback.

## Scope

This phase changes only the Inspiration viewer and introduces the shared template. It does not connect the Files viewer to the template yet and does not redesign any visual detail.

## Architecture

Create a generic `MediaPreviewDialogTemplate` under `src/components/media`. Its markup and CSS are copied from the current `InspirationExampleDialog` structure, preserving the existing dialog grid, thumbnail rail, close button, preview stage, navigation buttons, information panel, breakpoints, spacing, focus treatment, and motion.

The template owns only behavior common to any gallery viewer:

- modal backdrop and close control;
- selected-thumbnail focus/scroll handling;
- previous/next selection with wrapping;
- document-level Left/Right navigation that ignores editable controls;
- thumbnail rail, preview stage, and scrollable information-panel structure;
- the exact current responsive layout and visual styles.

The template receives item-specific content through typed render callbacks:

- stable item key and accessible thumbnail label;
- thumbnail media;
- preview media and aspect ratio;
- information-panel contents;
- localized dialog, close, rail, previous, and next labels.

An Inspiration adapter owns Inspiration-specific behavior and data: image/video rendering, video autoplay/reset, prompt expansion, copy feedback, sharing, recreation URL, model title, prompt and action buttons. It renders those pieces through the template callbacks.

## Legacy fallback

The current `InspirationExampleDialog` code remains unchanged in `InspirationExampleCard.tsx`. `InspirationGallery` switches its import to the new adapter, so the site uses the shared template while the old implementation is no longer on the active gallery path. Reverting requires changing a single import back to the legacy export. The fallback is temporary and must not be edited in parallel with the active template.

## Compatibility requirements

- Preserve all current accessible names and test IDs.
- Preserve modal closing, focus restoration, thumbnail selection, arrow wrapping, keyboard scoping, video behavior, prompt controls, copy/share feedback, and download/recreate links.
- Preserve the current desktop and narrow-screen DOM order and CSS values.
- Do not add a runtime feature flag or ship both implementations in the active bundle.
- Do not modify the Files preview in this phase.

## Verification

- Add focused tests for the generic template's selection wrapping, keyboard guard, close behavior, and slot rendering.
- Keep the existing Inspiration gallery behavioral and style tests unchanged where possible; they must pass against the new adapter.
- Run focused template and Inspiration tests, TypeScript checking, and ESLint.
- Compare the active Inspiration dialog before and after at desktop and narrow widths to confirm no visual or interaction regression.

