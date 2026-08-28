# Frameless Featured Model Icons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Убрать декоративную рамку и подложку вокруг базовых иконок популярных моделей, сохранив их геометрию и поведение.

**Architecture:** Изменение локализуется в CSS-модуле `FeaturedModelShortcuts`. Отдельный тест читает итоговый CSS и защищает точные визуальные ограничения от регрессии.

**Tech Stack:** CSS Modules, TypeScript, Vitest.

## Global Constraints

- Изменяется только оформление популярных ярлыков на главной странице.
- Размер иконки остаётся `3.5rem × 3.5rem`.
- Сетка, подписи, интервалы, ссылки и общий `ModelIcon` не меняются.

---

### Task 1: Удалить рамку популярных иконок

**Files:**
- Create: `web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.styles.test.ts`
- Modify: `web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.module.css`

**Interfaces:**
- Consumes: CSS-класс `.icon`, применяемый к `ModelIcon` внутри `FeaturedModelShortcuts`.
- Produces: прозрачная иконка без рамки и подложки размером `3.5rem × 3.5rem` с прежним hover-подъёмом.

- [ ] **Step 1: Write the failing CSS test**

```ts
const iconRule = stylesheet.match(/\.icon\s*\{(?<body>[\s\S]*?)\}/)?.groups?.body ?? "";
expect(iconRule).toContain("inline-size: 3.5rem");
expect(iconRule).toContain("block-size: 3.5rem");
expect(iconRule).not.toMatch(/\bborder(?:-radius)?:/);
expect(iconRule).not.toContain("background:");
expect(stylesheet).not.toContain(".shortcut:hover .icon {\n  translate: 0 -0.2rem;\n  border-color:");
```

- [ ] **Step 2: Run test and verify RED**

Run: `npm exec vitest -- run src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.styles.test.ts`

Expected: FAIL because `.icon` currently defines `border`, `border-radius`, `background`, and hover `border-color`.

- [ ] **Step 3: Apply the minimal CSS change**

```css
.icon {
  inline-size: 3.5rem;
  block-size: 3.5rem;
  transition: translate var(--motion-fast);
}

.shortcut:hover .icon {
  translate: 0 -0.2rem;
}
```

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `npm exec vitest -- run src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.styles.test.ts src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx`

Expected: both test files PASS.

- [ ] **Step 5: Run static verification**

Run: `npm run lint`, `npm run typecheck`, and `git diff --check`.

Expected: all commands exit 0; the final diff contains only the CSS module, its test, and this plan.
