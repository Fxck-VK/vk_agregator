# Popular Models Reveal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Показывать четыре популярные модели сразу, раскрывать ещё две по кнопке и после раскрытия предлагать переход в полный каталог.

**Architecture:** Состояние раскрытия и срез каталога находятся в существующем клиентском `FeaturedModels`. `WorkspaceLanding` больше не содержит прежнюю верхнюю ссылку. Общий визуальный компонент действия переиспользует текущий PNG-фон кнопки.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Не придумывать модели: показывать только элементы, возвращённые каталогом.
- Сразу показывать 4 карточки и раскрывать не более 2 дополнительных.
- Ссылка «Все нейросети» ведёт на `/app/models`.
- Не изменять `web/platform/next-env.d.ts`.

---

### Task 1: Интерактивное раскрытие популярных моделей

**Files:**
- Create: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.test.tsx`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.tsx`
- Modify: `web/platform/src/features/workspace/FeaturedModels/FeaturedModels.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

**Interfaces:**
- Consumes: `loadImageModelCatalog(): Promise<ImageModelList>` и `assetPaths.images.workspace.allModelsButtonBackground`.
- Produces: `FeaturedModels`, показывающий 4 или 6 карточек и переключающий действия без навигации.

- [ ] **Step 1: Write the failing component test**

Create `FeaturedModels.test.tsx` with a mocked six-item catalogue and this behavior assertion:

```tsx
render(<FeaturedModels />);

expect(await screen.findAllByTestId("featured-model-card")).toHaveLength(4);
expect(screen.getByRole("button", { name: "Показать ещё" })).toHaveAttribute("aria-expanded", "false");
expect(screen.queryByRole("link", { name: "Все нейросети" })).toBeNull();

fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));

expect(screen.getAllByTestId("featured-model-card")).toHaveLength(6);
expect(screen.queryByRole("button", { name: "Показать ещё" })).toBeNull();
expect(screen.getByRole("link", { name: "Все нейросети" })).toHaveAttribute("href", "/app/models");
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npm test -- src/features/workspace/FeaturedModels/FeaturedModels.test.tsx`

Expected: FAIL, потому что текущий компонент обрезает каталог до четырёх карточек и не рендерит действия.

- [ ] **Step 3: Implement the minimum behavior**

Use explicit limits and derive the visible slice from `expanded`:

```tsx
const collapsedModelLimit = 4;
const expandedModelLimit = 6;

const [expanded, setExpanded] = useState(false);
const visibleModels = models.slice(0, expanded ? expandedModelLimit : collapsedModelLimit);
const canExpand = models.length > collapsedModelLimit;
```

Render an accessible grid followed by exactly one action:

```tsx
<div className={styles.grid} id="featured-models-grid">
  {visibleModels.map(renderModelCard)}
</div>
<div className={styles.actions}>
  {canExpand && !expanded ? (
    <button
      aria-controls="featured-models-grid"
      aria-expanded="false"
      className={styles.catalogAction}
      onClick={() => setExpanded(true)}
      type="button"
    >
      <ActionArtwork label="Показать ещё" />
    </button>
  ) : (
    <Link className={styles.catalogAction} href="/app/models">
      <ActionArtwork label="Все нейросети" />
    </Link>
  )}
</div>
```

- [ ] **Step 4: Update layout contracts**

Delete the `primaryButton` link from `WorkspaceLanding.tsx` and its dedicated CSS. Add centered action styling to `FeaturedModels.module.css`:

```css
.actions {
  display: flex;
  justify-content: center;
  margin-block-start: var(--space-6);
}

.catalogAction {
  position: relative;
  isolation: isolate;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-block-size: 2.75rem;
  border: 0;
  border-radius: 999px;
  padding: var(--space-2) var(--space-5);
  background: transparent;
  color: var(--color-text-on-accent);
  font: inherit;
  font-weight: 700;
  text-decoration: none;
  cursor: pointer;
}
```

- [ ] **Step 5: Run focused and full verification**

Run: `npm test -- src/features/workspace/FeaturedModels/FeaturedModels.test.tsx src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Run: `npm test`

Run: `npm run typecheck`

Run: `npm run lint`

Expected: все команды завершаются успешно.

- [ ] **Step 6: Commit**

Stage только файлы задания и создать отдельный коммит реализации, не включая `web/platform/next-env.d.ts`.
