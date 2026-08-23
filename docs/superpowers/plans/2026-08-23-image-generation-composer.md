# Image Generation Composer Implementation Plan

**Status:** Implemented and verified on 2026-08-23.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the legacy image-generation form with the shared compact chat composer, image-quality control, server-derived price, and the existing generation workflow.

**Architecture:** `ChatComposer` gains one domain-neutral controls slot. A focused `ImageGenerationComposer` adapts image-generation state into that shared UI, while `ImageGenerationPanel` remains the workflow owner for catalog loading, quote preparation, confirmation, activation, tracking, results, and model-selection synchronization.

**Tech Stack:** Next.js 16, React 19, TypeScript 5.9, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Do not change backend endpoints, billing, job activation, tracking, result retrieval, or history behavior.
- The prepare request remains exactly `prompt`, `model_id`, and `image_quality` plus the existing idempotency header.
- The floating workspace model selector is the only visible model selector.
- The initial working composer controls are media, image quality, submit, and the server-catalog price.
- Do not display non-functional template, aspect-ratio, or image-count controls until the web API supports them.
- Preserve confirmation before spending stars and all current error/retry behavior.
- Do not modify or stage `web/platform/next-env.d.ts`.

---

## File Structure

- Modify `web/platform/src/components/chat/ChatComposer/ChatComposer.tsx`: add a neutral `additionalControls` React node and render it in the shared control row.
- Modify `web/platform/src/components/chat/ChatComposer/ChatComposer.module.css`: keep shared controls aligned without changing existing variants.
- Modify `web/platform/src/components/chat/ChatComposer/ChatComposer.test.tsx`: verify the extension slot renders and existing send/media behavior remains intact.
- Create `web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx`: image-specific adapter around `ChatComposer`.
- Create `web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.module.css`: compact quality pill, price note, and error presentation.
- Create `web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.test.tsx`: verify prompt, quality, submit, price, disabled, and error states.
- Modify `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx`: replace the legacy editor, remove duplicated heading/model UI, and synchronize valid workspace model changes.
- Modify `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.module.css`: make the editor stage visually unboxed while retaining failure/confirmation/tracking spacing.
- Modify `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx`: adapt selectors and cover top-selector synchronization and unchanged API payload.
- Delete `web/platform/src/features/image-generation/ImageGenerationEditor/ImageGenerationEditor.tsx`.
- Delete `web/platform/src/features/image-generation/ImageGenerationEditor/ImageGenerationEditor.module.css`.
- Delete `web/platform/src/features/image-generation/ImageGenerationEditor/ImageGenerationEditor.test.tsx`.

### Task 1: Add a domain-neutral controls slot to ChatComposer

**Files:**
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.tsx`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.module.css`
- Test: `web/platform/src/components/chat/ChatComposer/ChatComposer.test.tsx`

**Interfaces:**
- Consumes: existing `ChatComposerProps` and `ReactNode`.
- Produces: optional `additionalControls?: ReactNode` prop rendered between `ChatMediaMenu` and the submit button.

- [ ] **Step 1: Write the failing extension-slot test**

Add a test that renders `additionalControls={<button type="button">1K</button>}` and asserts both the `1K` control and existing submit button are present and enabled according to their own state.

- [ ] **Step 2: Run the focused test and verify failure**

Run: `npm test -- --run src/components/chat/ChatComposer/ChatComposer.test.tsx`

Expected: TypeScript/Vitest failure because `additionalControls` is not a valid prop.

- [ ] **Step 3: Implement the minimal extension slot**

Import `ReactNode`, add `additionalControls?: ReactNode`, destructure it, and render it in a neutral wrapper immediately after `ChatMediaMenu`. Add only layout CSS needed for wrapping and alignment; keep all existing composer variants unchanged.

- [ ] **Step 4: Run the focused test and verify pass**

Run: `npm test -- --run src/components/chat/ChatComposer/ChatComposer.test.tsx`

Expected: all `ChatComposer` tests pass.

- [ ] **Step 5: Commit the shared-component change**

```bash
git add web/platform/src/components/chat/ChatComposer/ChatComposer.tsx web/platform/src/components/chat/ChatComposer/ChatComposer.module.css web/platform/src/components/chat/ChatComposer/ChatComposer.test.tsx
git commit -m "feat(web): extend shared chat composer controls"
```

### Task 2: Build the compact ImageGenerationComposer adapter

**Files:**
- Create: `web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx`
- Create: `web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.module.css`
- Create: `web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.test.tsx`

**Interfaces:**
- Consumes: `ChatComposer`, the current prompt, quality options, selected quality, public price, and workflow callbacks.
- Produces: `ImageGenerationComposer` with props `canSubmit`, `errorMessage`, `imageQuality`, `isSubmitting`, `onImageQualityChange`, `onPromptChange`, `onSubmit`, `price`, `prompt`, and `qualityOptions`.

- [ ] **Step 1: Write failing behavior tests**

Cover these exact behaviors: the shared textarea exposes `ru.imageGeneration.promptLabel`; changing the quality calls `onImageQualityChange`; submit calls `onSubmit`; the note displays `Стоимость: N ★`; missing price shows `priceUnavailable`; preparing disables textarea, quality, and submit; an error renders with `role="alert"`.

- [ ] **Step 2: Run the focused test and verify failure**

Run: `npm test -- --run src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.test.tsx`

Expected: module-not-found failure because the component does not exist.

- [ ] **Step 3: Implement the adapter with the shared composer**

Render a `<form>` containing `ChatComposer`. Pass the image prompt strings, media control, circular submit, public price through `note`, and the quality `<select>` through `additionalControls`. The submit label remains `generate` or `preparing`; the UI must not contain a model select.

- [ ] **Step 4: Style the compact image controls**

Use a pill-shaped quality control that matches the existing dark tokens, remains keyboard accessible, wraps on narrow screens, and does not introduce hard-coded blue backgrounds.

- [ ] **Step 5: Run the focused test and verify pass**

Run: `npm test -- --run src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.test.tsx`

Expected: all new component tests pass.

- [ ] **Step 6: Commit the image composer**

```bash
git add web/platform/src/features/image-generation/ImageGenerationComposer
git commit -m "feat(web): add compact image generation composer"
```

### Task 3: Replace the legacy editor and synchronize the floating model selector

**Files:**
- Modify: `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx`
- Modify: `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.module.css`
- Modify: `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx`
- Delete: `web/platform/src/features/image-generation/ImageGenerationEditor/ImageGenerationEditor.tsx`
- Delete: `web/platform/src/features/image-generation/ImageGenerationEditor/ImageGenerationEditor.module.css`
- Delete: `web/platform/src/features/image-generation/ImageGenerationEditor/ImageGenerationEditor.test.tsx`

**Interfaces:**
- Consumes: `ImageGenerationComposer` and `WorkspaceModelSelection.selectedModelId`.
- Produces: the existing panel workflow with a compact editor stage and no duplicated model selector.

- [ ] **Step 1: Rewrite editor-stage tests to describe the new UI**

Assert that the editor exposes the prompt and quality controls, displays the immediate catalog price, has no combobox named `modelLabel`, and retains the circular generate action. Add a provider harness that sets the shared selection to a second valid image model and assert quality/price update to that model's defaults.

- [ ] **Step 2: Run panel tests and verify failure**

Run: `npm test -- --run src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx`

Expected: failures because the old model select/header/editor are still rendered and external selection is not consumed.

- [ ] **Step 3: Replace ImageGenerationEditor with ImageGenerationComposer**

Remove the visible title/description and model-select props. Pass `selectedModel.quality_options`, prompt state, quality state, price, submit state, and current prepare error to the new adapter. Keep loading failure, confirmation, activation, tracking, and result branches unchanged.

- [ ] **Step 4: Synchronize valid external model changes**

Read `workspaceModelSelection?.selectedModelId`. After the catalog is loaded, when that ID exists in `models` and differs from `modelID`, update `modelID`, reset quality to that model's `default_quality`, clear stale preparation/error state, and do not write a second routing state. Ignore external IDs absent from the image catalog.

- [ ] **Step 5: Preserve query-param initialization precedence**

On first load choose a valid explicit `?model=` first, then a valid workspace selection, then the first catalog model. Preserve a requested quality only when the selected model supports it; otherwise use `default_quality`.

- [ ] **Step 6: Remove obsolete editor files and unboxed editor styling**

Delete `ImageGenerationEditor` and its tests/styles. Update panel CSS so the editor stage has no large bordered card or duplicated title while error/confirmation/tracker/result spacing remains stable.

- [ ] **Step 7: Run panel and workspace tests**

Run: `npm test -- --run src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx src/features/image-generation/ImageWorkspace/ImageWorkspace.test.tsx`

Expected: all tests pass, including the unchanged prepare payload and history rendering.

- [ ] **Step 8: Commit the workflow integration**

```bash
git add web/platform/src/features/image-generation/ImageGenerationComposer web/platform/src/features/image-generation/ImageGenerationPanel web/platform/src/features/image-generation/ImageGenerationEditor
git commit -m "refactor(web): use shared composer for image generation"
```

### Task 4: Verify the complete frontend change

**Files:**
- Verify: `web/platform/src/components/chat/ChatComposer/**`
- Verify: `web/platform/src/features/image-generation/**`

**Interfaces:**
- Consumes: completed Tasks 1-3.
- Produces: a verified frontend change ready for an explicitly requested deployment push.

- [ ] **Step 1: Run targeted regression tests**

Run: `npm test -- --run src/components/chat/ChatComposer/ChatComposer.test.tsx src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.test.tsx src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx src/features/image-generation/ImageWorkspace/ImageWorkspace.test.tsx`

Expected: all targeted tests pass.

- [ ] **Step 2: Run typecheck and lint**

Run: `npm run typecheck`

Expected: exit code 0.

Run: `npm run lint`

Expected: exit code 0 with zero warnings.

- [ ] **Step 3: Run the complete frontend test suite**

Run: `npm test`

Expected: Vitest and asset validation pass.

- [ ] **Step 4: Run a production build**

Run: `npm run build`

Expected: Next.js production build completes successfully.

- [ ] **Step 5: Inspect the final diff and protected local file**

Run: `git status --short`

Expected: only intentional feature changes plus the pre-existing unstaged `web/platform/next-env.d.ts`; that file must not be staged.

Run: `git diff --check`

Expected: no whitespace errors.

- [ ] **Step 6: Commit any final test-only corrections**

Stage exact corrected paths only and commit with `test(web): cover image composer integration`. Do not push until the user explicitly requests a dev deployment.
