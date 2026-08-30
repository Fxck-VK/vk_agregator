# Workspace Prompt Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the wide workspace prompt promotion with one reusable Inspiration example card and the approved copy.

**Architecture:** `WorkspaceLanding` selects `inspirationExamples[0]` and renders the existing `InspirationExampleCard` when present. A small workspace-owned wrapper controls the desktop width and mobile expansion without changing the shared Inspiration component or its dialog behavior.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Heading: `Библиотека промптов`.
- Description: `Собрали промпты для любых задач и идей`.
- Do not render `Все идеи`, the old category label, editorial title, or `Посмотреть пример`.
- Render only `inspirationExamples[0]` through `InspirationExampleCard`; render no placeholder when the collection is empty.
- Cap the card wrapper at `25rem` on desktop and let it fill the available width below `36rem` while the shared card retains its `2 / 3` aspect ratio.
- Do not modify Inspiration example data, the gallery, or dialog behavior.

---

### Task 1: Replace the workspace prompt promotion with the shared card

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`

**Interfaces:**
- Consumes: `inspirationExamples: readonly InspirationExample[]` and `InspirationExampleCard({ example, priority?, sizes? })`.
- Produces: workspace section copy plus at most one shared, interactive Inspiration card.

- [ ] **Step 1: Write the failing workspace behavior test**

Add assertions to the home-route test that require the approved description, reject the four removed strings, and require exactly one button with the first example's `openLabel`. Add an interaction test that clicks that button and expects the shared dialog to open.

```tsx
expect(text).toContain("Собрали промпты для любых задач и идей");
for (const removedText of [
  "Все идеи",
  "Промпт для изображения",
  "Воздушная бумажная скульптура среди мягких облаков",
  "Посмотреть пример",
]) {
  expect(text).not.toContain(removedText);
}

const promptCard = screen.getByRole("button", { name: inspirationExamples[0].openLabel });
expect(screen.getAllByRole("button", { name: inspirationExamples[0].openLabel })).toHaveLength(1);
await userEvent.click(promptCard);
expect(screen.getByRole("dialog", { name: ru.inspiration.dialogLabel })).toBeInTheDocument();
```

- [ ] **Step 2: Run the focused behavior test and verify RED**

Run: `npm test -- src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Expected: FAIL because the new description and shared Inspiration card are absent while the old promotion remains.

- [ ] **Step 3: Implement the minimal shared-card section**

Import the shared data and component, select the first example once, replace the old link card and action with:

```tsx
const promptExample = inspirationExamples[0];

<section aria-labelledby="workspace-prompts-title" className={`${styles.section} ${styles.contentFrame}`}>
  <div className={styles.sectionHeading}>
    <div>
      <h2 id="workspace-prompts-title">Библиотека промптов</h2>
      <p>Собрали промпты для любых задач и идей</p>
    </div>
  </div>
  {promptExample ? (
    <div className={styles.promptExample} data-testid="workspace-prompt-example">
      <InspirationExampleCard example={promptExample} priority sizes="(max-width: 36rem) 100vw, 25rem" />
    </div>
  ) : null}
</section>
```

- [ ] **Step 4: Run the focused behavior test and verify GREEN**

Run: `npm test -- src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Expected: PASS with exactly one shared card and its existing dialog behavior.

- [ ] **Step 5: Commit the behavior**

```bash
git add web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx
git commit -m "feat(platform): reuse inspiration card on workspace home"
```

### Task 2: Constrain the single card responsively

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`

**Interfaces:**
- Consumes: `styles.promptExample` from Task 1 and the shared card's intrinsic `aspect-ratio: 2 / 3`.
- Produces: a `25rem` desktop wrapper that expands to `100%` below `36rem`.

- [ ] **Step 1: Write the failing stylesheet test**

```ts
it("keeps the workspace prompt card narrow on desktop and fluid below 36rem", () => {
  const promptExampleRule = stylesheet.match(/\.promptExample\s*\{[^}]*\}/s)?.[0] ?? "";

  expect(promptExampleRule).toContain("inline-size: min(100%, 25rem)");
  expect(stylesheet).toMatch(
    /@media \(width < 36rem\)[\s\S]*\.promptExample\s*\{[^}]*inline-size:\s*100%;/s,
  );
  expect(componentSource).toContain('sizes="(max-width: 36rem) 100vw, 25rem"');
  expect(stylesheet).not.toContain(".promptFeature");
  expect(stylesheet).not.toContain(".promptImage");
});
```

- [ ] **Step 2: Run the focused stylesheet test and verify RED**

Run: `npm test -- src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Expected: FAIL because `.promptExample` does not yet define the approved desktop and mobile widths and obsolete prompt styles remain.

- [ ] **Step 3: Implement the responsive wrapper and remove obsolete styles**

Replace `.promptFeature`, `.promptImage`, and their descendant rules with:

```css
.promptExample {
  inline-size: min(100%, 25rem);
}

@media (width < 36rem) {
  .promptExample {
    inline-size: 100%;
  }
}
```

Remove obsolete `.promptFeature`/`.promptImage` references from the existing `48rem` and `32rem` media rules.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `npm test -- src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Expected: PASS for both behavior and responsive layout checks.

- [ ] **Step 5: Run full verification**

Run from `web/platform`:

```bash
npm test
npm run typecheck
npm run lint
npm run build
npm run test:packaging
```

Expected: every command exits `0` with no test failures or lint errors.

- [ ] **Step 6: Commit the responsive layout**

```bash
git add web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css
git commit -m "style(platform): constrain workspace prompt card"
```
