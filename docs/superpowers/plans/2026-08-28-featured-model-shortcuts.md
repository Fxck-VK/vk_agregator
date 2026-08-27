# Featured Model Shortcuts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the four square workspace section shortcuts with four available flagship model shortcuts and keep the final full-catalogue link.

**Architecture:** Add a focused client component that consumes the existing cached image-model catalogue, selects its first four items, and renders model-specific generator links with the shared `ModelIcon`. Keep the server-rendered `WorkspaceLanding` responsible only for composition and keep loading/failure behavior inside the new component.

**Tech Stack:** React 19, Next.js 16, TypeScript, CSS Modules, Vitest, Testing Library

## Global Constraints

- Use the first four server-returned models; never synthesize unavailable models.
- Each model link must target `/app/image?model=<encoded id>` with `prefetch={false}`.
- Use the existing `ModelIcon` fallback unless optional artwork is provided by model ID.
- Keep `Все нейросети` last and linked to `/app/models`.
- Preserve the current rail geometry and responsive column rules.
- Do not change backend contracts, the lower featured-model cards, or previous uncommitted changes.
- Do not commit or push until explicitly requested.

---

### Task 1: Add the catalogue-backed shortcut component

**Files:**
- Create: `web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx`
- Create: `web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.tsx`
- Create: `web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.module.css`

**Interfaces:**
- Consumes: `loadImageModelCatalog(): Promise<ImageModelList>`, `ModelIcon`, and optional `artworkByModelId?: Readonly<Record<string, string>>`
- Produces: `FeaturedModelShortcuts` rendering zero to four direct generator links or four inert loading skeletons

- [ ] **Step 1: Write failing component tests**

Mock `loadImageModelCatalog` and assert:

```tsx
expect(await screen.findAllByTestId("featured-model-shortcut")).toHaveLength(4);
expect(screen.queryByText("Fifth Model")).toBeNull();
expect(screen.getByRole("link", { name: "Открыть генератор: Nano / Banana" })).toHaveAttribute(
  "href",
  "/app/image?model=nano%20%2F%20banana",
);
```

Assert that the default `ModelIcon` source contains `default-model-87465de8.png`, an optional artwork mapping replaces the first source, loading renders four `featured-model-shortcut-skeleton` elements, and rejected/empty catalogues render no model shortcut.

- [ ] **Step 2: Run the new test and verify RED**

Run:

```bash
npx vitest run src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx
```

Expected: FAIL because `FeaturedModelShortcuts` does not exist.

- [ ] **Step 3: Implement the client component**

Create a client component with `featuredModelShortcutLimit = 4`, local `loading | ready | failed` state, active-effect cleanup, `catalog.items.slice(0, featuredModelShortcutLimit)`, encoded links, `prefetch={false}`, and:

```tsx
<ModelIcon className={styles.icon} src={artworkByModelId[model.id]} />
```

Return four inert skeletons while loading and `null` for failure or an empty result.

- [ ] **Step 4: Add scoped shortcut styles**

Create CSS-module rules for `.shortcut`, `.icon`, and `.skeleton`. Match the existing 3.5rem square, 1rem radius, centered label, hover lift, border colors, and responsive behavior. Keep skeletons inert and `aria-hidden`.

- [ ] **Step 5: Run the component test and verify GREEN**

Run the same Vitest command. Expected: all new tests pass.

---

### Task 2: Replace the old workspace rail entries

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/workspace-home-content.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

**Interfaces:**
- Consumes: `FeaturedModelShortcuts`
- Produces: `Основные возможности` navigation containing model shortcuts followed by `Все нейросети`

- [ ] **Step 1: Add a failing integration assertion**

In the existing truthful catalogue test, scope to `navigation` named `Основные возможности` and assert four `featured-model-shortcut` links, the first-four model names in server order, the encoded generator href, absence of `NeiroHub Chat`, `Генератор изображений`, `Каталог нейросетей`, and `Вдохновение`, and that `Все нейросети` is the last link.

- [ ] **Step 2: Run the workspace test and verify RED**

Run:

```bash
npx vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx
```

Expected: FAIL because the rail still renders `primaryTools`.

- [ ] **Step 3: Integrate the component and remove obsolete data**

Import and render `<FeaturedModelShortcuts />` before the existing full-catalogue link. Remove the `primaryTools` import, `WorkspaceHomeTool` type, and `primaryTools` array. Do not change `capabilityLinks` or other page content.

- [ ] **Step 4: Remove only obsolete rail CSS**

Remove `.toolShortcut`, `.toolIcon`, and the `.blue`, `.cyan`, `.violet`, `.orange` rules. Retain `.toolRail`, `.allToolsShortcut`, `.arrowIcon`, and all responsive column definitions.

- [ ] **Step 5: Run focused workspace tests and verify GREEN**

Run:

```bash
npx vitest run src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx
```

Expected: both files pass.

- [ ] **Step 6: Run project verification**

Run:

```bash
npm run lint
npm run typecheck
npm test
npm run build
git diff --check
```

Expected: all commands exit successfully and all earlier uncommitted work remains in the final status.
