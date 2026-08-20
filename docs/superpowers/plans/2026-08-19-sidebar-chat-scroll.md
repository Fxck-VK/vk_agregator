# Sidebar Chat Scroll Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Сделать список чатов единственным прокручиваемым участком Sidebar и переименовать его в «Чаты».

**Architecture:** `Sidebar` остаётся flex-каркасом с фиксированными брендом, навигацией и аккаунтом. `SidebarConversations` получает ограниченную высоту и владеет прокруткой только своего `<ul>`, сохраняя заголовок неподвижным.

**Tech Stack:** TypeScript, React, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Не менять API и серверное состояние чатов.
- Не включать `web/platform/next-env.d.ts` в изменения.
- Сохранять работу desktop и mobile Sidebar.

---

### Task 1: Зафиксировать поведение тестами

**Files:**
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.test.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.scrollbar.test.ts`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.test.tsx`

- [ ] Изменить ожидание заголовка на «Чаты» и добавить проверки классов отдельного слота и списка.
- [ ] Запустить целевые тесты и подтвердить ожидаемое падение из-за старой общей прокрутки.

### Task 2: Перенести владельца прокрутки

**Files:**
- Modify: `web/platform/src/i18n/ru.ts`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.module.css`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.module.css`

- [ ] Переименовать заголовок и связанные пустое состояние/подтверждение удаления.
- [ ] Сделать внутреннюю область Sidebar flex-контейнером без `overflow-y: auto`.
- [ ] Выделить `conversationsSlot` с доступной высотой.
- [ ] Сделать список чатов единственным `overflow-y: auto` владельцем и оформить hover/focus scrollbar.
- [ ] Запустить целевые тесты до зелёного состояния.

### Task 3: Проверить отсутствие регрессий

**Files:**
- No production file changes expected.

- [ ] Выполнить `npm test` в `web/platform`.
- [ ] Выполнить `npm run typecheck` и `npm run lint`.
- [ ] Проверить `git diff --check` и убедиться, что `next-env.d.ts` не включён в задачу.
