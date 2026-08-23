# Image Generation Guide Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить под генератором изображений адаптивный обучающий блок с двумя вкладками и тремя шагами.

**Architecture:** Новый клиентский `ImageGenerationGuide` хранит только состояние активной вкладки и рендерит статический локализованный контент. `ImageWorkspace` композиционно размещает его между рабочим генератором и историей, а изображения до появления реальных материалов представлены CSS-заглушками.

**Tech Stack:** TypeScript, React 19, Next.js 16, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Бэкенд и API не изменяются.
- Существующий генератор и история остаются работоспособными.
- Все строки пользовательского интерфейса находятся в `ru.ts`.
- Декоративные элементы не должны выглядеть интерактивными для скринридеров.

---

### Task 1: ImageGenerationGuide behavior

**Files:**
- Create: `web/platform/src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.test.tsx`
- Create: `web/platform/src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx`
- Create: `web/platform/src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.module.css`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `ru.imageGeneration.guide`
- Produces: `export function ImageGenerationGuide(): JSX.Element`

- [ ] **Step 1: Write the failing test**

Проверить две доступные вкладки, три шага по умолчанию и появление панели примеров после клика.

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.test.tsx`
Expected: FAIL, потому что компонент ещё не существует.

- [ ] **Step 3: Write minimal implementation**

Создать компонент с `useState<"guide" | "examples">`, семантикой tabs, тремя карточками шагов, галереей заглушек и адаптивным CSS.

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.test.tsx`
Expected: PASS.

### Task 2: Workspace integration

**Files:**
- Modify: `web/platform/src/features/image-generation/ImageWorkspace/ImageWorkspace.tsx`
- Modify: `web/platform/src/features/image-generation/ImageWorkspace/ImageWorkspace.test.tsx`
- Modify: `web/platform/src/features/image-generation/ImageWorkspace/ImageWorkspace.module.css`

**Interfaces:**
- Consumes: `<ImageGenerationGuide />`
- Produces: порядок `ImageGenerationPanel → ImageGenerationGuide → ImageJobHistory`

- [ ] **Step 1: Write the failing integration test**

Замокать `ImageGenerationGuide` и проверить, что он находится между панелью генератора и историей.

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/features/image-generation/ImageWorkspace/ImageWorkspace.test.tsx`
Expected: FAIL, потому что guide отсутствует в workspace.

- [ ] **Step 3: Add the guide to ImageWorkspace**

Импортировать компонент, разместить после `ImageGenerationPanel`, добавить вертикальный интервал между секциями.

- [ ] **Step 4: Run targeted tests**

Run: `npx vitest run src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.test.tsx src/features/image-generation/ImageWorkspace/ImageWorkspace.test.tsx`
Expected: PASS.

### Task 3: Verification

**Files:**
- Verify only.

- [ ] **Step 1: Run typecheck**

Run: `npm run typecheck`
Expected: exit 0.

- [ ] **Step 2: Run lint**

Run: `npm run lint`
Expected: exit 0.

- [ ] **Step 3: Inspect diff**

Run: `git diff --check` and `git status --short`
Expected: no whitespace errors; `next-env.d.ts` remains unstaged and untouched by this feature.
