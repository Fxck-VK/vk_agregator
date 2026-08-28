# Unified Model Artwork Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Показывать существующую базовую заставку модели во всех карточках и в селекторе, не оставляя браузерный значок сломанного изображения.

**Architecture:** Общий `ModelIcon` становится единственной реализацией заставки. Он загружает публичный PNG напрямую и при событии `error` заменяет его встроенным SVG-fallback; селектор переиспользует этот компонент вместо отдельных символов.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Использовать `public/assets/images/models/default-model-87465de8.png` как основной базовый ресурс.
- Не менять размеры карточек, их внутренние отступы, скругления или данные моделей.
- Не оставлять пользователю браузерный индикатор сломанного изображения.

---

### Task 1: Надёжный общий `ModelIcon`

**Files:**
- Create: `web/platform/src/features/models/ModelIcon/ModelIcon.test.tsx`
- Modify: `web/platform/src/features/models/ModelIcon/ModelIcon.tsx`
- Modify: `web/platform/src/features/models/ModelIcon/ModelIcon.module.css`

**Interfaces:**
- Consumes: `assetPaths.images.models.fallback: string`, optional `src?: string | null`.
- Produces: `ModelIcon({ className, src })`, `data-testid="model-icon"` for the image and `data-testid="model-icon-fallback"` after a load error.

- [ ] **Step 1: Write the failing tests**

```tsx
it("loads the public fallback directly", () => {
  render(<ModelIcon />);
  expect(screen.getByTestId("model-icon")).toHaveAttribute(
    "src",
    "/assets/images/models/default-model-87465de8.png",
  );
});

it("replaces a failed image with the embedded fallback", () => {
  render(<ModelIcon />);
  fireEvent.error(screen.getByTestId("model-icon"));
  expect(screen.getByTestId("model-icon-fallback")).toBeInTheDocument();
  expect(screen.queryByTestId("model-icon")).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run the test and verify RED**

Run: `npm test -- --run src/features/models/ModelIcon/ModelIcon.test.tsx`

Expected: FAIL because the current image URL is optimized and there is no error fallback.

- [ ] **Step 3: Implement the minimal component behavior**

Add `"use client"`, track the failed source, pass `unoptimized` to `Image`, and return an accessible-hidden span containing the white smiling-chip SVG when that source fails. Apply the same `styles.icon` and caller `className` to both states.

- [ ] **Step 4: Run the test and verify GREEN**

Run: `npm test -- --run src/features/models/ModelIcon/ModelIcon.test.tsx`

Expected: both tests PASS.

### Task 2: Переиспользование заставки в селекторе

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`

**Interfaces:**
- Consumes: shared `ModelIcon` from `../ModelIcon/ModelIcon`.
- Produces: identical base artwork in the selector trigger and every visible option.

- [ ] **Step 1: Write the failing selector assertion**

After opening the loaded selector, assert that the trigger and both option buttons contain `data-testid="model-icon"`, for a total of three shared model icons.

- [ ] **Step 2: Run the selector test and verify RED**

Run: `npm test -- --run src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx`

Expected: FAIL because the selector currently renders `✦` spans instead of `ModelIcon`.

- [ ] **Step 3: Replace separate symbols with `ModelIcon`**

Import `ModelIcon`, use it in the trigger and each option, and remove only the obsolete gradient/color declarations while retaining the existing 1.75rem and 2rem icon sizes.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run: `npm test -- --run src/features/models/ModelIcon/ModelIcon.test.tsx src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx src/features/models/ModelCard/ModelCard.test.tsx src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx`

Expected: all focused tests PASS.

### Task 3: Полная проверка

**Files:**
- Verify only.

**Interfaces:**
- Consumes: completed component integration.
- Produces: evidence that behavior and production packaging remain valid.

- [ ] **Step 1: Run the platform test suite**

Run: `npm test`

Expected: all Vitest and asset-validation tests PASS.

- [ ] **Step 2: Run static checks**

Run: `npm run lint` and `npm run typecheck`

Expected: both commands exit 0 with no warnings or errors.

- [ ] **Step 3: Run the production build and packaging check**

Run: `npm run build` and `npm run test:packaging`

Expected: production build and standalone packaging assertions PASS.

- [ ] **Step 4: Inspect the local page**

Open the workspace landing page and model selector locally; confirm the same smiling base artwork appears in the popular cards, catalogue cards, selector trigger, and selector options, with no layout changes.

- [ ] **Step 5: Review the final diff**

Run: `git diff --check` and `git status --short`

Expected: no whitespace errors; only the planned source, test, CSS, and plan files are modified.
