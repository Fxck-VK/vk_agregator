# Remove Section Kickers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Удалить четыре указанные служебные подписи с главной страницы, не затронув остальные метки и контент.

**Architecture:** Изменение ограничено JSX главной страницы. Существующий тест `WorkspaceHome.test.tsx` фиксирует пользовательский текстовый контракт.

**Tech Stack:** React 19, Next.js 16, TypeScript, Vitest.

## Global Constraints

- Удалить только четыре согласованные строки.
- Сохранить «Аккаунт и баланс» и «Идеи и примеры».
- Не изменять `web/platform/next-env.d.ts`.

---

### Task 1: Remove editorial kicker labels

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`

**Interfaces:**
- Consumes: `WorkspaceHome` и существующую разметку `WorkspaceLanding`.
- Produces: главную страницу без четырёх указанных строк.

- [ ] **Step 1: Write the failing test**

```tsx
const markup = renderToStaticMarkup(<WorkspaceHome />);

for (const removedLabel of [
  "Коротко о главном",
  "Не только обычный чат",
  "Начните с готовой идеи",
  "Помощь по платформе",
]) {
  expect(markup).not.toContain(removedLabel);
}
expect(markup).toContain("Аккаунт и баланс");
expect(markup).toContain("Идеи и примеры");
```

- [ ] **Step 2: Verify RED**

Run: `npx vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Expected: FAIL because the four labels are still rendered.

- [ ] **Step 3: Remove the labels**

Delete these four JSX nodes from `WorkspaceLanding.tsx`:

```tsx
<p className={styles.kicker}>Коротко о главном</p>
<p className={styles.kicker}>Не только обычный чат</p>
<p className={styles.kicker}>Начните с готовой идеи</p>
<p className={styles.kicker}>Помощь по платформе</p>
```

- [ ] **Step 4: Verify GREEN and project quality**

Run: `npx vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Run: `npm run typecheck`

Run: `npm run lint`

Expected: all commands exit successfully.

- [ ] **Step 5: Commit**

```bash
git add web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx
git commit -m "style(platform): remove section kicker labels"
```
