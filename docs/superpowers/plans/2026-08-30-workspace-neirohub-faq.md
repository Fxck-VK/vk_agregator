# Workspace NeiroHub FAQ Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the workspace FAQ with five approved NeiroHub questions and match the supplied rounded accordion-row treatment.

**Architecture:** Keep the existing `frequentlyAskedQuestions` data source and native `<details>` rendering. Change only the FAQ data, its focused workspace tests, and the FAQ-specific CSS rules.

**Tech Stack:** React server rendering, native HTML details/summary, CSS Modules, Vitest.

## Global Constraints

- Render exactly five FAQ rows.
- Use the approved questions verbatim.
- Do not claim that NeiroHub trained every available model or currently sells subscriptions.
- Keep native `<details>` behavior and the rotating right-side chevron.
- Use full-width surface rows with `1.5rem` radius, `5rem` minimum height, and no visible resting border.

---

### Task 1: Replace FAQ copy

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/workspace-home-content.ts`

**Interfaces:**
- Consumes: `frequentlyAskedQuestions: WorkspaceHomeFaq[]`.
- Produces: five question/answer objects rendered by the existing `WorkspaceLanding` details loop.

- [ ] **Step 1: Write the failing content test**

Add a home-route test that extracts the FAQ region and requires these five summaries:

```ts
const faqRegion = screen.getByRole("region", { name: "Частые вопросы" });
expect(within(faqRegion).getAllByRole("group")).toHaveLength(5);
for (const question of [
  "Что такое NeiroHub?",
  "Что такое собственные нейросети NeiroHub?",
  "Что такое токены и подписка?",
  "Как купить подписку?",
  "Есть ли бесплатный доступ?",
]) {
  expect(within(faqRegion).getByText(question)).toBeInTheDocument();
}
```

Also assert that the four removed questions are absent.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npx vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Expected: FAIL because four approved questions are absent and four old questions remain.

- [ ] **Step 3: Replace the data entries**

Keep the existing first answer and replace the remaining entries with truthful copy:

```ts
{
  question: "Что такое собственные нейросети NeiroHub?",
  answer: "Так мы называем модели, доступные через единый интерфейс NeiroHub. Для каждой модели показаны её возможности, параметры и актуальная стоимость запуска.",
},
{
  question: "Что такое токены и подписка?",
  answer: "В NeiroHub звёзды используются как единицы баланса для запуска нейросетей. Отдельная подписка для базовой работы с платформой сейчас не требуется.",
},
{
  question: "Как купить подписку?",
  answer: "Сейчас подписка не продаётся. Для платных запусков достаточно пополнить баланс в профиле и выбрать подходящую модель.",
},
{
  question: "Есть ли бесплатный доступ?",
  answer: "Открывать рабочее пространство и изучать каталог можно бесплатно. Для запуска платных моделей потребуется достаточный баланс.",
},
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run: `npx vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Expected: PASS.

### Task 2: Match the rounded accordion treatment

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`

**Interfaces:**
- Consumes: existing `.faqList details`, `.faqList summary`, and hover/focus selectors.
- Produces: full-width rounded surface rows with subtle interactive borders.

- [ ] **Step 1: Write the failing style contract**

```ts
it("styles FAQ rows like the approved rounded reference", () => {
  const detailsRule = stylesheet.match(/\.faqList details\s*\{[^}]*\}/s)?.[0] ?? "";
  const summaryRule = stylesheet.match(/\.faqList summary\s*\{[^}]*\}/s)?.[0] ?? "";

  expect(detailsRule).toContain("border: 0.0625rem solid transparent");
  expect(detailsRule).toContain("border-radius: 1.5rem");
  expect(summaryRule).toContain("min-block-size: 5rem");
  expect(summaryRule).toContain("font-size: var(--font-size-supporting)");
  expect(summaryRule).toContain("font-weight: var(--font-weight-semibold)");
  expect(stylesheet).toMatch(/\.faqList details:hover,[\s\S]*\.faqList details:focus-within\s*\{[^}]*border-color:\s*var\(--color-border\);/s);
});
```

- [ ] **Step 2: Run the stylesheet test and verify RED**

Run: `npx vitest run src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Expected: FAIL because current rows use a `1rem` radius, visible resting border, and no fixed minimum height or supporting type role.

- [ ] **Step 3: Implement the approved FAQ row styles**

Use these declarations:

```css
.faqList details {
  border: 0.0625rem solid transparent;
  border-radius: 1.5rem;
  background: var(--color-surface);
  transition: border-color var(--motion-fast);
}

.faqList details:hover,
.faqList details:focus-within {
  border-color: var(--color-border);
}

.faqList summary {
  min-block-size: 5rem;
  padding: var(--space-5) var(--space-6);
  font-size: var(--font-size-supporting);
  font-weight: var(--font-weight-semibold);
}
```

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `npx vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Expected: both files PASS.

- [ ] **Step 5: Run full verification**

Run from `web/platform`:

```bash
npm test
npm run typecheck
npm run lint
npm run build
npm run test:packaging
```

Expected: every command exits `0`.

- [ ] **Step 6: Commit**

```bash
git add web/platform/src/features/workspace/WorkspaceLanding/workspace-home-content.ts web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts
git commit -m "feat(platform): update workspace NeiroHub FAQ"
```
