# Workspace Header Clearance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Устранить пересечение плавающего селектора модели с центральной колонкой главной страницы.

**Architecture:** Существующий плавающий `WorkspaceHeader` сохраняется. Пересечение устраняется комбинацией компактного закрытого триггера, более узкой общей колонки и правого выравнивания этой колонки только в промежуточном десктопном диапазоне.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Vitest.

## Global Constraints

- Максимальная ширина закрытого селектора: `11.5rem`.
- Ширина центральной колонки и футера: `46rem`.
- Диапазон дополнительного зазора: `48rem <= width < 82rem`.
- Не изменять открытый список моделей, боковую панель и баланс.
- Не изменять `web/platform/next-env.d.ts`.

---

### Task 1: Контракт безопасной компоновки

**Files:**
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts`
- Modify: `web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`

**Interfaces:**
- Consumes: существующие CSS-классы `.trigger`, `.modelIcon`, `.contentFrame` и `.footerInner`.
- Produces: компактный селектор и центральную колонку с безопасным горизонтальным зазором.

- [ ] **Step 1: Write the failing style contracts**

Добавить в тест селектора проверку:

```ts
expect(triggerRule).toContain("max-inline-size: min(11.5rem, 42vw)");
expect(triggerRule).toContain("padding: var(--space-2)");
expect(modelIconRule).toContain("inline-size: 1.5rem");
```

Обновить контракт главной страницы:

```ts
expect(contentFrameRule).toContain("inline-size: min(100%, 46rem)");
expect(footerInnerRule).toContain("inline-size: min(100%, 46rem)");
expect(stylesheet).toMatch(/@media \(48rem <= width < 82rem\)[\s\S]*\.contentFrame/);
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `npm test -- src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Expected: FAIL на старых значениях `21rem` и `50rem` и отсутствии промежуточного медиазапроса.

- [ ] **Step 3: Implement the compact selector**

Изменить закрытый триггер:

```css
.trigger {
  gap: var(--space-1);
  max-inline-size: min(11.5rem, 42vw);
  padding: var(--space-2);
}

.modelIcon {
  inline-size: 1.5rem;
  block-size: 1.5rem;
}
```

- [ ] **Step 4: Implement the narrower collision-free frame**

Изменить общую ширину и добавить промежуточное выравнивание:

```css
.contentFrame,
.footerInner {
  inline-size: min(100%, 46rem);
}

@media (48rem <= width < 82rem) {
  .main {
    padding-inline-end: var(--space-4);
  }

  .contentFrame {
    margin-inline-end: 0;
  }
}
```

- [ ] **Step 5: Run focused and full verification**

Run: `npm test -- src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

Run: `npm test`

Run: `npm run typecheck`

Run: `npm run lint`

Expected: все команды завершаются успешно.

- [ ] **Step 6: Verify the rendered page**

Открыть `http://localhost:7158/app`, проверить селектор и левую границу формы, затем прокрутить страницу так, чтобы форма прошла рядом с плавающим селектором. Между ними должен оставаться видимый зазор.

- [ ] **Step 7: Commit**

Stage только четыре файла реализации и два документа задания; `web/platform/next-env.d.ts` не включать.
