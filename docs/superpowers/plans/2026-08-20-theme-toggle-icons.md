# Theme Toggle Icons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the account-menu theme controls' inline glyphs with the three approved SVG assets while preserving all behavior and accessibility.

**Architecture:** Add the SVGs to the existing public asset library, expose stable paths, and wrap each asset with the existing theme-aware `AssetIcon` component. The account menu consumes those wrappers without changing its state or event handlers.

**Tech Stack:** TypeScript, React 19, CSS Modules, Vitest, Testing Library, Next.js 16.

## Global Constraints

- Do not change theme-selection behavior or accessible labels.
- Reuse the existing `AssetIcon` CSS-mask infrastructure.
- Keep each icon wrapper independent and theme-aware through `currentColor`.

---

### Task 1: Specify the approved theme icon contract

**Files:**
- Modify: `web/platform/src/features/account/AccountControl/AccountControl.test.tsx`
- Modify: `web/platform/src/components/icons/icons.test.tsx`
- Modify: `web/platform/src/assets/asset-paths.test.ts`

**Interfaces:**
- Consumes: existing account-menu theme buttons and `assetPaths`.
- Produces: failing tests requiring `monitor`, `sun`, and `moon` shared icons.

- [x] **Step 1: Write failing tests for the three icon markers and stable asset URLs.**
- [x] **Step 2: Run the focused tests and confirm they fail because the new icons and paths do not exist.**

### Task 2: Add the shared theme assets and icon wrappers

**Files:**
- Create: `web/platform/public/assets/icons/theme/monitor.svg`
- Create: `web/platform/public/assets/icons/theme/sun.svg`
- Create: `web/platform/public/assets/icons/theme/moon.svg`
- Create: `web/platform/src/components/icons/MonitorIcon/MonitorIcon.tsx`
- Create: `web/platform/src/components/icons/MonitorIcon/index.ts`
- Create: `web/platform/src/components/icons/SunIcon/SunIcon.tsx`
- Create: `web/platform/src/components/icons/SunIcon/index.ts`
- Create: `web/platform/src/components/icons/MoonIcon/MoonIcon.tsx`
- Create: `web/platform/src/components/icons/MoonIcon/index.ts`
- Modify: `web/platform/src/assets/asset-paths.ts`

**Interfaces:**
- Consumes: `AssetIcon` and `assetPaths.icons.theme`.
- Produces: `MonitorIcon`, `SunIcon`, and `MoonIcon` React components.

- [x] **Step 1: Copy the approved SVG sources into the validated public asset library.**
- [x] **Step 2: Add stable theme asset paths.**
- [x] **Step 3: Add one small wrapper component per approved icon.**

### Task 3: Replace only the account-menu theme artwork

**Files:**
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.tsx`
- Modify: `web/platform/src/features/account/AccountMenu/AccountMenu.module.css`

**Interfaces:**
- Consumes: `MonitorIcon`, `SunIcon`, and `MoonIcon`.
- Produces: unchanged theme buttons displaying approved artwork.

- [x] **Step 1: Replace the three inline theme glyphs with shared icon components.**
- [x] **Step 2: Include asset icons in the existing theme-button size rule.**
- [x] **Step 3: Run focused tests and confirm they pass.**
- [x] **Step 4: Run asset validation, lint, typecheck, the full test suite, and the production build.**
