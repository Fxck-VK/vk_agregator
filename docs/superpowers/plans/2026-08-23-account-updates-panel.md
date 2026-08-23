# Account Updates Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить к пункту «Что нового?» отдельную адаптивную панель обновлений без навигации и изменения размеров меню аккаунта.

**Architecture:** `AccountMenu` владеет состоянием и событиями закрытия. Новый `AccountUpdatesPanel` отвечает только за доступную разметку и визуальное представление, а строки берутся из типизированного русского словаря.

**Tech Stack:** TypeScript, React, Next.js, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Не менять маршруты и серверные API.
- Не затрагивать несвязанный `next-env.d.ts`.
- Действие «Предложить» пока не выполняет внешних действий.

---

### Task 1: Behaviour contract

**Files:**
- Modify: `web/platform/src/features/account/AccountControl/AccountControl.test.tsx`

**Interfaces:**
- Consumes: существующий `AccountControl` и меню аккаунта.
- Produces: тестовый контракт для `aria-expanded`, панели, повторного клика, `Escape` и клика снаружи.

- [x] **Step 1: Write failing integration tests**
- [x] **Step 2: Run the targeted test and confirm failure because the updates action is disabled**

### Task 2: Updates panel component

**Files:**
- Create: `web/platform/src/features/account/AccountUpdatesPanel/AccountUpdatesPanel.tsx`
- Create: `web/platform/src/features/account/AccountUpdatesPanel/AccountUpdatesPanel.module.css`
- Modify: `web/platform/src/i18n/ru.ts`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.tsx`

**Interfaces:**
- Consumes: `id: string` and localized strings from `ru.account`.
- Produces: a labelled `region` rendered beside the account menu.

- [x] **Step 1: Add localized copy and the presentational panel**
- [x] **Step 2: Add toggle, outside click and Escape handling to `AccountMenu`**
- [x] **Step 3: Add desktop and mobile positioning without changing menu geometry**
- [x] **Step 4: Run the targeted tests and confirm they pass**

### Task 3: Verification

**Files:**
- Verify only; do not modify unrelated files.

**Interfaces:**
- Consumes: completed panel implementation.
- Produces: evidence that tests, TypeScript and lint all pass.

- [x] **Step 1: Run account tests**
- [x] **Step 2: Run TypeScript validation**
- [x] **Step 3: Run lint**
- [ ] **Step 4: Review `git diff` and confirm `next-env.d.ts` is excluded from this work**
