# Workspace Polish and Model Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make existing workspace controls visually legible and provide a real, frontend-only catalog of available image models with a safe handoff to the current generator.

**Architecture:** Keep conversation styling inside `ConversationRow`, which already owns the rename/archive popover. Add an isolated client `ModelsCatalog` feature that fetches and filters only the existing image-model DTO. The optional model id travels in the generator URL and is resolved only after the generator's normal model fetch.

**Tech Stack:** TypeScript, React, Next App Router, CSS Modules, Vitest, Testing Library, Zod contracts, existing same-origin `webBrowserFetch`.

## Global Constraints

- Frontend only: do not modify Go, database, API routes, API contracts, auth/session code, or deployment configuration.
- Reuse `GET /web/v1/image-models`, `webBrowserFetch`, and `parseImageModelList`; do not add a catalog API.
- Render only model id/name/quality/reference facts from the DTO; do not fabricate prices, providers, descriptions, or non-image categories.
- Preserve completed chat behavior, conversation mutations, focus, Escape, and mobile drawer lifecycle.
- Keep components in their own folders with TSX, CSS Module, and tests.
- Use TDD: add a failing behavioral test, watch it fail, implement minimally, then verify green.

---

### Task 1: Make conversation action controls visually readable

**Files:**

- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.module.css`
- Modify: `web/platform/src/features/conversations/ConversationRow/ConversationRow.styles.test.ts`

**Interfaces:**

- Consumes: existing `.renameForm input`, `.panel`, and `.formActions` markup from `ConversationRow.tsx`.
- Produces: a dark readable rename surface without changing the component API or interaction behavior.

- [ ] **Step 1: Write the failing style contract test**

Add a separate test:

    it("gives the rename control an explicit readable dark surface", () => {
      expect(stylesheet).toMatch(/\.renameForm input\s*\{[^}]*background:\s*var\(--color-surface-raised\);/s);
      expect(stylesheet).toMatch(/\.renameForm input\s*\{[^}]*color:\s*var\(--color-text\);/s);
      expect(stylesheet).toMatch(/\.renameForm input\s*\{[^}]*caret-color:\s*var\(--color-text\);/s);
      expect(stylesheet).toMatch(/\.formActions\s*\{[^}]*align-items:\s*stretch;/s);
    });

- [ ] **Step 2: Verify the test is red**

Run:

    npm.cmd --prefix web/platform test -- --run src/features/conversations/ConversationRow/ConversationRow.styles.test.ts

Expected: the background and caret assertions fail because the stylesheet relies on the browser's native input background.

- [ ] **Step 3: Add minimal explicit visual styling**

In `.renameForm input`, add `background: var(--color-surface-raised)`, `color: var(--color-text)`, `caret-color: var(--color-text)`, `appearance: none`, and a border-color/background transition. Extend `.formActions` with `align-items: stretch`. Do not change `ConversationRow.tsx`.

- [ ] **Step 4: Verify green and behavior preservation**

Run:

    npm.cmd --prefix web/platform test -- --run src/features/conversations/ConversationRow/ConversationRow.styles.test.ts src/features/conversations/ConversationRow/ConversationRow.test.tsx

Expected: all style and existing rename/archive behavior tests pass.

- [ ] **Step 5: Commit**

    git add web/platform/src/features/conversations/ConversationRow/ConversationRow.module.css web/platform/src/features/conversations/ConversationRow/ConversationRow.styles.test.ts
    git commit -m "fix: improve conversation action controls"

### Task 2: Build a truthful, filterable image-model catalog

**Files:**

- Create: `web/platform/src/features/models/ModelsCatalog/model-filters.ts`
- Create: `web/platform/src/features/models/ModelsCatalog/model-filters.test.ts`
- Create: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.tsx`
- Create: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.module.css`
- Create: `web/platform/src/features/models/ModelsCatalog/ModelsCatalog.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**

- Consumes: `ImageModel`, `parseImageModelList`, and `webBrowserFetch`.
- Produces: `<ModelsCatalog />`, a self-contained client screen that links each card to `/app/image?model=<encoded-id>`.

- [ ] **Step 1: Write failing pure filtering tests**

Define and test this interface before writing the helper:

    type ModelCatalogFilters = {
      query: string;
      referenceOnly: boolean;
      quality: string | null;
    };

    expect(filterImageModels(models, { query: "banana", referenceOnly: true, quality: "2K" }))
      .toEqual([models[0]]);
    expect(filterImageModels(models, { query: "MODEL-ID", referenceOnly: false, quality: null }))
      .toEqual([models[1]]);

Fixtures must cover name match, id match, reference capability, and distinct quality options.

- [ ] **Step 2: Verify the helper test is red**

Run:

    npm.cmd --prefix web/platform test -- --run src/features/models/ModelsCatalog/model-filters.test.ts

Expected: FAIL because `filterImageModels` does not exist.

- [ ] **Step 3: Implement the pure helper**

Implement `filterImageModels(models, filters)` with trimmed/lower-cased matching against only `model.name` and `model.id`; require `supports_reference_image` only when requested; require `quality_options.includes(quality)` only for non-null quality. Add `imageModelQualities(models)` returning de-duplicated values in source order.

- [ ] **Step 4: Verify the helper is green**

Run the same focused test. Expected: PASS.

- [ ] **Step 5: Write failing catalog component tests**

Mock only `webBrowserFetch`. Test:

    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/image-models");
    expect(await screen.findByRole("link", { name: `${ru.modelsCatalog.openGeneratorLabel}: Nano Banana` }))
      .toHaveAttribute("href", "/app/image?model=nano-banana-2");
    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelsCatalog.searchLabel }), {
      target: { value: "banana" },
    });
    expect(screen.queryByText("Other Model")).not.toBeInTheDocument();

Also cover loading status, failed/invalid response alert, valid empty list, combined reference/quality filtering, and DTO-only card facts.

- [ ] **Step 6: Verify the component test is red**

Run:

    npm.cmd --prefix web/platform test -- --run src/features/models/ModelsCatalog/ModelsCatalog.test.tsx

Expected: FAIL because `<ModelsCatalog />` does not exist.

- [ ] **Step 7: Implement ModelsCatalog and isolated styles**

Use a client component with `useEffect` for the one-time fetch. State is `loading | ready | failure`; a ready empty list is distinct from failure. Use native accessible controls:

    <input
      aria-label={ru.modelsCatalog.searchLabel}
      type="search"
      value={query}
      onChange={(event) => setQuery(event.target.value)}
    />
    <label>
      <input
        checked={referenceOnly}
        type="checkbox"
        onChange={(event) => setReferenceOnly(event.target.checked)}
      />
      {ru.modelsCatalog.referenceFilterLabel}
    </label>
    <select
      aria-label={ru.modelsCatalog.qualityFilterLabel}
      value={quality ?? ""}
      onChange={(event) => setQuality(event.target.value || null)}
    >
      <option value="">{ru.modelsCatalog.allQualitiesLabel}</option>
      {imageModelQualities(models).map((value) => <option key={value} value={value}>{value}</option>)}
    </select>

Each card shows `model.name`, the truthful type `Генерация изображений`, quality chips, and a reference-image label. Build the link using `encodeURIComponent(model.id)`. Add exactly these Russian strings under `modelsCatalog` in `ru`: `eyebrow: "NeiroHub"`, `title: "Все нейросети"`, `description: "Выберите модель для генерации изображений."`, `searchLabel: "Поиск модели"`, `referenceFilterLabel: "С поддержкой референсов"`, `qualityFilterLabel: "Качество"`, `allQualitiesLabel: "Любое качество"`, `imageTypeLabel: "Генерация изображений"`, `referenceSupportedLabel: "Референсы поддерживаются"`, `referenceUnsupportedLabel: "Без референсов"`, `loading: "Загружаем доступные модели…"`, `loadFailure: "Не удалось загрузить каталог моделей. Попробуйте ещё раз."`, `empty: "Подходящих моделей пока нет."`, `openGeneratorLabel: "Открыть генератор"`, and `clearFiltersLabel: "Сбросить фильтры"`.

- [ ] **Step 8: Verify green and commit**

Run:

    npm.cmd --prefix web/platform test -- --run src/features/models/ModelsCatalog/model-filters.test.ts src/features/models/ModelsCatalog/ModelsCatalog.test.tsx
    git add web/platform/src/features/models/ModelsCatalog web/platform/src/i18n/ru.ts
    git commit -m "feat: add image model catalog"

### Task 3: Route the catalog and honor a selected model in the generator

**Files:**

- Modify: `web/platform/src/app/app/models/page.tsx`
- Create: `web/platform/src/app/app/models/page.test.tsx`
- Modify: `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx`
- Modify: `web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx`

**Interfaces:**

- Consumes: `<ModelsCatalog />` and the `model` query parameter.
- Produces: a real `/app/models` route and a generator that prefers a known requested model only after its normal image-model fetch.

- [ ] **Step 1: Write failing route and selection tests**

Mock `ModelsCatalog` in the page test and expect it to render. In the panel test, mock the client router query as `model=nano-banana-2`, supply two valid models, click the existing explicit open button, and expect the model combobox to select `nano-banana-2`. Add a separate unknown-id test expecting the first model/default quality.

- [ ] **Step 2: Verify the tests are red**

Run:

    npm.cmd --prefix web/platform test -- --run src/app/app/models/page.test.tsx src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx

Expected: the page renders `WorkspaceHome`; the generator always selects the first model.

- [ ] **Step 3: Implement the narrow route and selection handoff**

Replace the placeholder page with:

    import { ModelsCatalog } from "@/features/models/ModelsCatalog/ModelsCatalog";

    export default function ModelsPage() {
      return <ModelsCatalog />;
    }

In `ImageGenerationPanel`, read only the `model` client-router query. In `openGenerator`, resolve that id from `catalog.items` before using the first-model fallback:

    const requestedModel = requestedModelID === null
      ? undefined
      : catalog.items.find((model) => model.id === requestedModelID);
    const initialModel = requestedModel ?? catalog.items[0];

Keep the generator closed on first render; catalog navigation does not automatically fetch or activate a job.

- [ ] **Step 4: Verify focused behavior**

Run the Step 2 command. Expected: route and known/unknown selection tests pass while existing explicit-open tests remain green.

- [ ] **Step 5: Commit**

    git add web/platform/src/app/app/models/page.tsx web/platform/src/app/app/models/page.test.tsx web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx
    git commit -m "feat: connect model catalog to generator"

## Final verification

- [ ] Run `npm.cmd --prefix web/platform test -- --run`.
- [ ] Run `npm.cmd --prefix web/platform run typecheck`, `lint`, `build`, and `test:packaging`.
- [ ] Run `git diff --check` from the pre-slice base through HEAD.
- [ ] Run independent task reviews and a whole-delivery review; resolve every Critical or Important finding.
- [ ] Push `dev-deploy`, wait for CI and signed image build, run the existing DEV deployment, and smoke-check the protected DEV address.
