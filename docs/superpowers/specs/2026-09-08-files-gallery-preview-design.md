# Files Gallery Preview Design

## Goal

Upgrade the existing “Мои файлы” preview so it uses the same gallery structure and interaction model as the approved “Вдохновение” viewer while keeping file-specific metadata and tools. This design supersedes the layout and metadata decisions in `2026-09-06-file-preview-dialog-design.md` without changing the card-level direct-download action.

## Architecture

- Use the extracted `MediaPreviewDialogTemplate<T>` already active in “Вдохновение” as the sole viewer shell for “Мои файлы”: thumbnail rail, bounded media stage, previous/next controls, close behavior, focus containment, and keyboard-navigation rules.
- Keep `InspirationExampleDialog` and `FilePreviewDialog` as separate feature adapters so prompts, actions, loading, and metadata remain feature-specific.
- Delete the former file-specific modal markup and structural stylesheet rules after the adapter is connected. Do not keep a second legacy file viewer.
- `FilesWorkspace` owns the ordered preview collection and selected index because it owns the current file category, ready jobs, and loaded artifacts.
- Opening a file passes the complete current-category collection to `FilePreviewDialog`; selecting a thumbnail or using previous/next updates the index without closing the dialog.
- Existing result loading remains bounded. Missing preview data is requested through the existing file-result queue instead of adding a new API endpoint or eagerly issuing unbounded requests.

## Layout and content

- Desktop uses the same overall composition as “Вдохновение”: a scrollable vertical thumbnail rail, a large contained preview with previous/next controls, and a separate information panel.
- The thumbnail rail, preview, and information panel sit inside one shared translucent surface with a subtle border, rounded corners, and visible spacing between the internal regions, matching the approved “Вдохновение” viewer. The central preview has no independent card background, border, radius, or shadow; only the information panel remains a separate inner surface. The close control belongs to the shared outer surface.
- Narrow layouts move the thumbnail rail above the preview and stack the information panel below it, following the existing “Вдохновение” breakpoints and spacing.
- The information panel contains the shared model icon and model name; a prompt limited to five lines with inline “Показать ещё”/“Свернуть”; a copy button with temporary checkmark and “Скопировано”; creation date, resolution, format, file size, and generation quality; and “Скачать”/“Поделиться” actions using the approved white icons.
- Metadata rows are omitted when their source value is absent. File size and format come from the artifact; date and quality come from the generation job.
- The bottom tool rail remains file-specific. “Общая” stays selected, while “Оживить”, “Улучшить”, “Удалить фон”, and “Редактировать” remain visible and disabled.
- The shared template exposes a file-agnostic footer slot for that tool rail and a selectable-item predicate so loading or unavailable thumbnails stay visible but disabled and are skipped by cyclic navigation.

## Interaction

- Thumbnail order matches the current filtered file order. Multiple artifacts from one generation remain adjacent.
- The selected thumbnail has the shared accent treatment and scrolls into view.
- On-screen arrows and keyboard Left/Right select the previous or next file and wrap continuously at the ends, matching “Вдохновение”.
- Keyboard navigation is active only while the file preview is open and does not intercept keystrokes from editable controls.
- Clicking non-control areas does not disable gallery keyboard navigation.
- Closing with the close button, Escape, or backdrop restores focus to the card that opened the preview.
- Download remains a native download link. Share uses the platform share sheet when available and copies the artifact URL as fallback.

## Loading and failure behavior

- The initially selected artifact renders immediately.
- Missing adjacent result data is loaded through the existing queue. Its thumbnail shows a neutral loading state until the artifact becomes available.
- A failed adjacent preview shows an unavailable state and remains skippable; it does not close or break the viewer.
- Copy and share failures use the existing localized feedback region.

## Accessibility and motion

- The viewer remains a labelled modal dialog with focus containment, Escape handling, background scroll lock, and restored trigger focus.
- Thumbnails expose selection state, navigation buttons have accessible names, and disabled tools remain semantically disabled.
- Reduced-motion preferences disable entrance and navigation transitions.

## Testing

- Component tests cover opening at the clicked file, ordered thumbnails, arrows, keyboard navigation, boundary behavior, editable-control exclusion, selection updates, and focus restoration.
- File-specific tests cover prompt expansion, prompt-copy success feedback, metadata formatting and omission, download, share fallback, disabled future tools, and adjacent loading failure.
- Style contracts cover desktop and narrow compositions plus contained media.
- Final verification includes focused Vitest tests, TypeScript checking, diff validation, and browser checks for desktop and narrow layouts.
