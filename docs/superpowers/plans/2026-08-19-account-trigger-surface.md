# Sidebar Account Trigger Surface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Сделать прямоугольный триггер аккаунта прозрачным в покое, подсвечивать его при взаимодействии и добавить слева круг с инициалами `NH`.

**Architecture:** Существующий `AccountMenu` остаётся владельцем разметки и состояния открытия. Новая декоративная иконка добавляется внутрь текущей кнопки, а все визуальные состояния задаются CSS без новых запросов, контекстов или зависимостей.

**Tech Stack:** TypeScript, React, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Геометрия триггера остаётся прямоугольной, без овальной капсулы.
- Покой использует прозрачный фон.
- Hover, `:focus-visible` и `data-open="true"` используют текущую подложку `--color-surface-raised`.
- Размеры и отступы не меняются между состояниями.
- Иконка декоративная, не получает роль изображения и не меняет доступное имя кнопки.
- Логика открытия меню, смены темы и выхода не меняется.

---

### Task 1: AccountMenu trigger appearance

**Files:**
- Modify: `web/platform/src/features/account/AccountControl/AccountControl.test.tsx`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.styles.test.ts`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.tsx`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.module.css`

**Interfaces:**
- Consumes: `AccountMenu({ identityLabel, isLogoutPending, logoutFailure, onLogout })`.
- Produces: существующий доступный trigger аккаунта с декоративным элементом `data-account-avatar="true"` и текстом `NH`.

- [ ] **Step 1: Write failing component and CSS tests**

Добавить в `AccountControl.test.tsx` проверку:

```tsx
const avatar = container.querySelector('[data-account-avatar="true"]');
expect(avatar).toHaveTextContent("NH");
expect(avatar).toHaveAttribute("aria-hidden", "true");
expect(screen.queryByRole("img")).not.toBeInTheDocument();
```

Добавить в `AccountMenu.styles.test.ts` проверки:

```ts
expect(triggerRule).toContain("grid-template-columns: 2.5rem minmax(0, 1fr) 1.5rem");
expect(triggerRule).toContain("background: transparent");
expect(stylesheet).toMatch(/\.trigger:hover,\s*\.trigger:focus-visible,\s*\.trigger\[data-open="true"\][^{]*\{[^}]*background:\s*var\(--color-surface-raised\)/s);
expect(stylesheet).toMatch(/\.avatar\s*\{[^}]*border-radius:\s*50%/s);
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
npm exec vitest run src/features/account/AccountControl/AccountControl.test.tsx src/features/account/AccountMenu/AccountMenu.styles.test.ts
```

Expected: FAIL because the trigger has two columns, a raised resting background, no `.avatar`, and no `data-account-avatar` element.

- [ ] **Step 3: Implement minimal markup and styles**

Добавить первым дочерним элементом trigger в `AccountMenu.tsx`:

```tsx
<span aria-hidden="true" className={styles.avatar} data-account-avatar="true">
  NH
</span>
```

Изменить trigger в CSS:

```css
.trigger {
  grid-template-columns: 2.5rem minmax(0, 1fr) 1.5rem;
  background: transparent;
}

.trigger:hover,
.trigger:focus-visible,
.trigger[data-open="true"] {
  background: var(--color-surface-raised);
}

.avatar {
  display: grid;
  inline-size: 2.5rem;
  block-size: 2.5rem;
  place-items: center;
  border-radius: 50%;
  background: var(--color-accent);
  color: var(--color-on-accent);
  font-size: 0.75rem;
  font-weight: 800;
}
```

- [ ] **Step 4: Run targeted tests and verify GREEN**

Run the same targeted Vitest command. Expected: both files pass.

- [ ] **Step 5: Run full frontend verification**

Run `npm test`, `npm run typecheck`, and `npm run lint`. Expected: all commands exit with code `0` and ESLint reports no warnings.

- [ ] **Step 6: Review the diff**

Run `git diff --check` and inspect the four files listed above. Expected: no whitespace errors and only the approved account-trigger behavior is added.
