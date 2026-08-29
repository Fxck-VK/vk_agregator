# Transparent All-Models Arrow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the «Все нейросети» arrow container transparent by default and show only an accent border on hover or keyboard focus.

**Architecture:** Keep the existing markup and geometry. Change only the `.arrowIcon` default surface and the shortcut interaction selectors in the existing CSS Module, guarded by a stylesheet contract test.

**Tech Stack:** CSS Modules, Vitest

## Global Constraints

- Preserve the `3.5rem` square size, `1rem` radius, arrow size, label, link, and spacing.
- Keep the arrow visible in all states.
- Keep the background transparent in all states.
- Use `var(--color-accent)` only for the hover and focus-visible border.

---

### Task 1: Transparent arrow interaction

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css`
- Test: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts`

**Interfaces:**
- Consumes: `.allToolsShortcut` and its child `.arrowIcon`.
- Produces: transparent default geometry and an accent interaction border.

- [ ] **Step 1: Write the failing contract**

Add assertions requiring `border-color: transparent`, `background: transparent`, and a combined hover/focus-visible selector with `border-color: var(--color-accent)`.

- [ ] **Step 2: Verify RED**

Run `npm exec -- vitest run src/features/workspace/WorkspaceLanding/WorkspaceLanding.styles.test.ts` and confirm failure on the current surface and border.

- [ ] **Step 3: Apply the minimal CSS change**

Set the final `.arrowIcon` rule to transparent border and background. Extend the hover rule with `.allToolsShortcut:focus-visible .arrowIcon` while preserving translate and setting the accent border.

- [ ] **Step 4: Verify GREEN and regressions**

Run the focused test, `npm test`, `npm run lint`, and `npm run typecheck`.

- [ ] **Step 5: Verify locally and commit**

Measure default and hover/focus styles at `http://localhost:7158/app`, then commit the CSS and test as `style(platform): simplify all-models shortcut`.
