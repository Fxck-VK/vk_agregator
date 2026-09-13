# Task 6: Remove the nested preview surface

## Goal

Make the Files preview match the approved Inspiration composition: one shared translucent outer surface contains the thumbnail rail, central preview, information panel, and close control. The central preview must not look like a second card inside the outer surface.

## Required changes

- Update `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.module.css`.
- Update `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`.
- Update `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.tsx`.
- Update `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`.
- Keep the existing shared translucent `.dialog` surface.
- Remove the independent border, border radius, background, and box shadow from `.previewPane`; retain its layout, spacing, overflow, and media/tool behavior.
- Move the close button so it is a direct child of the dialog rather than a child of `.previewPane`.
- Position the close button at the shared surface's top-right on desktop without covering the information panel. Keep it usable on narrow layouts.
- Keep `.infoPanel` as the only separate inner panel.
- Preserve thumbnail navigation, focus containment, responsive layout, prompt, metadata, and actions.

## TDD steps

1. Add style assertions proving `.previewPane` has no border, radius, background, or shadow.
2. Add a component assertion proving the close control is a direct child of the dialog.
3. Run `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx --reporter=dot` and verify the new assertions fail for the expected nested-surface reason.
4. Apply the minimal TSX and CSS implementation.
5. Re-run the same command and verify all tests pass.
6. Run `npm run typecheck`.

## Constraints

- Do not commit.
- Do not modify unrelated files.
- Use `apply_patch` for edits.
- Write the full outcome, commands, results, and concerns to `.superpowers/sdd/task-6-report.md`.
