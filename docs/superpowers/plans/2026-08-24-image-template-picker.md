# Image Template Picker Implementation Plan

> **For Codex:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a reusable image-template selector to the universal image-generation composer, backed by the existing Inspiration catalog.

**Architecture:** A dedicated client component owns the trigger, modal, search, keyboard behavior, and selection UI. It reads the shared `inspirationExamples` catalog and reports the selected example to `ImageGenerationComposer`, which places the template prompt into the existing controlled input. No duplicate template data or separate route is introduced.

**Tech Stack:** TypeScript, React, Next.js, CSS Modules, Vitest, React Testing Library.

---

### Task 1: Specify the selection behavior

**Files:**
- Modify: `web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.test.tsx`
- Create: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.test.tsx`

- [ ] Verify the template trigger opens an accessible dialog.
- [ ] Verify search filters the shared Inspiration examples.
- [ ] Verify selecting a card closes the dialog and returns its prompt.
- [ ] Verify the composer immediately displays the selected prompt.

### Task 2: Build the reusable picker

**Files:**
- Create: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx`
- Create: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css`
- Modify: `web/platform/src/i18n/ru.ts`

- [ ] Use the shared input-control chip for the trigger.
- [ ] Render a responsive, scrollable modal with search and template cards.
- [ ] Support close button, Escape, and backdrop click.
- [ ] Restore focus and lock background scrolling while open.
- [ ] Read template cards exclusively from `inspirationExamples`.

### Task 3: Integrate with image generation

**Files:**
- Modify: `web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx`

- [ ] Place the template trigger beside “Загрузить медиа”.
- [ ] On selection, call the existing controlled prompt callback.
- [ ] Preserve all existing aspect-ratio, quality, output-count, price, and submit behavior.

### Task 4: Verify

**Files:**
- Test: `web/platform/src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.test.tsx`
- Test: `web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.test.tsx`

- [ ] Run focused tests.
- [ ] Run TypeScript validation.
- [ ] Review the final diff for unrelated changes.
