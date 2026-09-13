# Inspiration Preview Legacy Removal Design

## Goal

Remove the duplicate legacy Inspiration preview implementation after the shared `MediaPreviewDialogTemplate` became the active viewer.

## Design

- Keep `InspirationExampleDialogTemplate.tsx` as the only Inspiration dialog implementation.
- Make standalone `InspirationExampleCard` consumers open that dialog instead of the embedded legacy dialog.
- Preserve card-trigger focus restoration and the existing one-item preview behavior.
- Remove legacy dialog imports, hooks, markup, and structural CSS from `InspirationExampleCard.tsx` and its stylesheet.
- Keep card styles and Inspiration-specific information-panel/action styles used by the active adapter.

## Verification

- A source/style contract proves the card imports the new dialog and contains no legacy modal shell.
- Inspiration component and style tests remain green.
- TypeScript, ESLint, and the full test suite remain green.
