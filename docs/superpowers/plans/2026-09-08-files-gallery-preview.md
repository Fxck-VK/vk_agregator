# Files Gallery Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the “Мои файлы” preview into an inspiration-style gallery with thumbnails, cyclic navigation, file metadata, prompt copying, and file-specific actions.

**Architecture:** `FilesWorkspace` continues to own fetched jobs/results and derives a stable ordered list of `{ job, artifact }` preview entries. `FilePreviewDialog` becomes a controlled gallery adapter that owns only transient UI feedback and prompt expansion, while existing shared `ModalBackdrop`, `ScrollArea`, icons, and model presentation remain reused.

**Tech Stack:** React 19, Next.js, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Preserve the card-level direct-download action.
- Arrow-key navigation is scoped to the open file dialog and wraps at collection boundaries.
- “Общая” remains active; “Оживить”, “Улучшить”, “Удалить фон”, and “Редактировать” remain disabled.
- Use existing white copy, download, and repost icons; add no dependency or API endpoint.
- Missing metadata rows are omitted.
- Do not commit user changes.

---

### Task 1: Controlled file-preview collection

**Files:**
- Modify: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.tsx`
- Modify: `web/platform/src/features/files/FilesGrid/FilesGrid.tsx`
- Modify: `web/platform/src/features/files/FileCard/FileCard.tsx`
- Test: `web/platform/src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

**Interfaces:**
- Produces `FilePreviewItem = { artifact: ImageJobResult["artifacts"][number]; job: ImageJob }`.
- Changes the dialog contract to `items`, `selectedIndex`, `onSelect`, and `onClose`.

- [ ] **Step 1: Write the failing workspace gallery test**

Add a test with two successful jobs/results. Open the second card and assert that the dialog exposes both thumbnail buttons, initially previews the second artifact, and changes back to the first artifact after selecting its thumbnail.

- [ ] **Step 2: Run the workspace test to verify RED**

Run: `npx vitest run src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

Expected: FAIL because the current dialog receives only one `job` and `artifact`.

- [ ] **Step 3: Implement the controlled collection**

Derive entries in job order and artifact order:

```ts
const previewItems = useMemo(() => readyJobs.flatMap((job) =>
  (resultsByJobID[job.id]?.artifacts ?? []).map((artifact) => ({ artifact, job })),
), [readyJobs, resultsByJobID]);
```

Store the selected artifact id, compute its current index, and pass the complete collection plus `onSelect` to `FilePreviewDialog`. Keep the selected id stable as more queued results arrive.

- [ ] **Step 4: Run the workspace test to verify GREEN**

Run: `npx vitest run src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

Expected: PASS.

### Task 2: Thumbnail rail and cyclic navigation

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes `items`, `selectedIndex`, `onSelect`, and `onClose`.
- Produces thumbnail selection plus `selectOffset(offset: number)` with modulo wrapping.

- [ ] **Step 1: Write failing dialog navigation tests**

Render three preview items and assert:

```ts
fireEvent.click(screen.getByRole("button", { name: "Следующий файл" }));
expect(onSelect).toHaveBeenCalledWith(1);
fireEvent.keyDown(document, { key: "ArrowLeft" });
expect(onSelect).toHaveBeenCalledWith(2);
```

Also assert every thumbnail exists, the selected one has `aria-current="true"`, editable targets do not trigger navigation, and key events after unmount do nothing.

- [ ] **Step 2: Run the dialog tests to verify RED**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: FAIL because thumbnails/arrows/document navigation do not exist.

- [ ] **Step 3: Implement navigation**

Use the existing `ScrollArea` for a vertical thumbnail viewport, add previous/next buttons, scroll the selected thumbnail into view, and register a document `keydown` listener only while mounted. Use:

```ts
const selectOffset = (offset: number) => {
  onSelect((selectedIndex + offset + items.length) % items.length);
};
```

Reuse the inspiration viewer’s editable-target guard and keep navigation controls outside the video/image surface.

- [ ] **Step 4: Run the dialog tests to verify GREEN**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: PASS.

### Task 3: Prompt, copy feedback, and file metadata

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Uses selected item fields: `job.prompt`, `job.created_at`, `job.image_quality`, `artifact.width`, `artifact.height`, `artifact.mime_type`, and `artifact.size_bytes`.
- Produces localized metadata labels and prompt-copy feedback.

- [ ] **Step 1: Write failing information-panel tests**

Assert that the selected prompt appears, the copy control contains `/assets/icons/ui/copy-white.svg`, clicking it calls `navigator.clipboard.writeText(job.prompt)`, and the label becomes “Скопировано”. Assert rendered values for creation date, `1024 × 1536`, `PNG`, formatted bytes, and `2K`. Assert future tools are disabled and download/share use the supplied white icons.

- [ ] **Step 2: Run the dialog tests to verify RED**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: FAIL because the current information panel omits prompt and metadata.

- [ ] **Step 3: Implement information formatting and feedback**

Add pure local formatters for Russian date, MIME subtype, and bytes. Add prompt expansion state that resets when the selected artifact changes. Copy the prompt with a two-second “Скопировано” state and use the existing white copy/download/repost assets.

- [ ] **Step 4: Run the dialog tests to verify GREEN**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: PASS.

### Task 4: Inspiration-style responsive composition

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.module.css`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.tsx`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

**Interfaces:**
- Consumes the dialog regions `thumbnailRail`, `previewPane`, `infoPanel`, `previewNavigation`, `promptBlock`, `metadata`, and `toolRail`.

- [ ] **Step 1: Write failing style contracts**

Assert a desktop four-region grid with a vertical thumbnail rail, bounded contained preview, and info panel. Assert that below `60rem` the rail becomes horizontal, preview and information panel stack, and the action/tool layout remains usable. Assert five-line prompt clamping and reduced-motion behavior.

- [ ] **Step 2: Run style tests to verify RED**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

Expected: FAIL because the current two-column dialog has no gallery rail or navigation layout.

- [ ] **Step 3: Implement the responsive CSS**

Mirror the approved inspiration viewer proportions and tokens. Keep the file-specific tool rail below the media, use `object-fit: contain`, apply accent borders to the selected thumbnail, and use border-only hover/focus treatments for navigation and close controls.

- [ ] **Step 4: Run style tests to verify GREEN**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

Expected: PASS.

### Task 5: Integrated verification

**Files:**
- Test: all files changed by Tasks 1–4.

- [ ] **Step 1: Run focused feature tests**

Run: `npx vitest run src/features/files/FileCard/FileCard.test.tsx src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts src/features/files/FilesWorkspace/FilesWorkspace.test.tsx`

Expected: all tests PASS.

- [ ] **Step 2: Run static verification**

Run: `npm run typecheck`

Expected: exit code 0.

Run: `git diff --check`

Expected: exit code 0, allowing existing line-ending warnings.

- [ ] **Step 3: Verify browser behavior**

At desktop and narrow viewports, open a middle file and verify the selected thumbnail, arrows, keyboard wrapping, prompt copy feedback, metadata, download/share icons, contained image, disabled future tools, close behavior, and trigger-focus restoration.

### Task 6: Shared translucent viewer surface

**Files:**
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.module.css`
- Modify: `web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

**Interfaces:**
- Consumes the existing `.dialog` grid container.
- Produces one visual surface enclosing the thumbnail rail, preview pane, and information panel. The preview pane is structurally flat within this surface, while the information panel stays separate.

- [ ] **Step 1: Write a failing style contract**

Assert that `.dialog` has `overflow: hidden`, a subtle border, `border-radius: 1.5rem`, responsive padding, and a translucent `rgb(...)` background instead of `transparent`. Assert that `.previewPane` has no border, background, radius, or shadow. Assert that the close control is a direct child of the dialog.

- [ ] **Step 2: Run the style test to verify RED**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts`

Expected: FAIL because `.previewPane` still renders its own card surface and contains the close control.

- [ ] **Step 3: Implement the minimal shared surface**

Keep the shared border, radius, responsive padding, translucent background, and overlay shadow on `.dialog`. Remove the border, radius, background, and shadow from `.previewPane`; keep only its layout and spacing. Move the close control to be a direct child of `.dialog` and position it at the shared surface’s top-right on desktop, while keeping it usable in the narrow layout. Preserve the separate `.infoPanel`. At widths below `40rem`, retain the existing edge-to-edge square container and reduced outer padding.

- [ ] **Step 4: Verify GREEN and regressions**

Run: `npx vitest run src/features/files/FilePreviewDialog/FilePreviewDialog.styles.test.ts src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx`

Expected: PASS with the same desktop and narrow grid behavior.
