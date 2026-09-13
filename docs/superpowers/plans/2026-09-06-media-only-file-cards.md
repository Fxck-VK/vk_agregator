# Media-only File Cards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render completed file cards as edge-to-edge media with no visible inscriptions while preserving download and non-completed states.

**Architecture:** `FileCard` branches its presentation only when a loaded artifact exists. Loaded results render a full-card download link; all jobs without a loaded artifact continue through the existing status and retry presentation.

**Tech Stack:** React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Preserve masonry order and column spacing.
- Preserve lazy result loading, filtering, retry controls, and accessible download behavior.
- Do not commit or publish changes.

---

### Task 1: Specify the completed-card presentation

**Files:**
- Create: `web/platform/src/features/files/FileCard/FileCard.test.tsx`
- Create: `web/platform/src/features/files/FileCard/FileCard.styles.test.ts`

**Interfaces:**
- Consumes: `FileCardProps` through the public `FileCard` React component.
- Produces: regression coverage for loaded artifacts and non-completed jobs.

- [x] **Step 1: Write the failing behavior test**

Render a succeeded job with one loaded artifact and assert one download link named `Скачать: <prompt>`, the artifact URL, no figcaption or visible metadata panel, and a visually hidden metadata block.

- [x] **Step 2: Write the failing style test**

Assert that `.mediaLink`, `.mediaFigure`, and `.media` span the card and that `.media` is block-level with `height: auto` and `object-fit: cover`.

- [x] **Step 3: Run the new tests and confirm they fail for the existing caption and metadata panel**

Run `npx vitest run src/features/files/FileCard/FileCard.test.tsx src/features/files/FileCard/FileCard.styles.test.ts --maxWorkers=4`.

### Task 2: Implement the media-only card

**Files:**
- Modify: `web/platform/src/features/files/FileCard/FileCard.tsx`
- Modify: `web/platform/src/features/files/FileCard/FileCard.module.css`

**Interfaces:**
- Consumes: `result.artifacts`, `job.prompt`, and the platform artifact route.
- Produces: a full-card download link for loaded results and the unchanged state-card presentation otherwise.

- [x] **Step 1: Render loaded artifacts as accessible full-card links**

Use `aria-label={`${ru.files.download}: ${job.prompt}`}`, keep the `download` attribute, and place the image directly inside a media figure without a caption.

- [x] **Step 2: Remove the visible metadata panel for loaded results**

Render `.content` only when `result === null`. For a loaded result, render status, prompt, and model in `.accessibleMetadata`, positioned outside layout for screen readers and existing semantic queries.

- [x] **Step 3: Make media edge-to-edge**

Remove preview padding for loaded artifacts through the loaded-card branch and let the image define the tile height from its intrinsic aspect ratio.

- [x] **Step 4: Run targeted tests until green**

Run `npx vitest run src/features/files/FileCard/FileCard.test.tsx src/features/files/FileCard/FileCard.styles.test.ts src/features/files/FilesWorkspace/FilesWorkspace.test.tsx src/app/typography.contract.test.ts --maxWorkers=4`.

### Task 3: Verify the complete change

**Files:**
- Verify only; no additional production files.

**Interfaces:**
- Consumes: the completed implementation.
- Produces: evidence that the project remains valid.

- [x] **Step 1: Run all tests**

Run `npx vitest run --maxWorkers=4` and require zero failures.

- [x] **Step 2: Run static checks**

Run `npm run test:assets`, `npm run typecheck`, and `npm run lint` and require exit code 0.

- [x] **Step 3: Inspect the local files page**

Open `/app/files?layout=wide` and confirm completed media fills each card, text is absent from completed cards, masonry gutters are unchanged, and non-completed cards still expose status and retry actions.
