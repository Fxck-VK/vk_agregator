# Inspiration Example Card Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver one interactive inspiration card with a fullscreen, accessible detail dialog and real client-side actions.

**Architecture:** `WorkspaceHome` delegates only its inspiration branch to a focused client component. Public example data and UI state stay inside that feature; the image generator receives a model, quality, and prompt through its existing query-string contract.

**Tech Stack:** TypeScript, React 19, Next.js 16, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Use one local project-owned image and no remote CDN.
- Do not add backend/API requests or new dependencies.
- Do not auto-start a paid generation.
- Preserve existing workspace routes and shell behavior.

---

### Task 1: Specify gallery behavior with failing tests

**Files:**
- Create: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx`
- Create: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.styles.test.ts`

**Interfaces:**
- Consumes: `ru.inspiration` labels and the `/app/image` query-string contract.
- Produces: behavioral expectations for `InspirationGallery`.

- [ ] Add tests for closed initial state, card opening, named modal dialog, Escape closing with focus restoration, prompt copying, download link, and recreate URL parameters.
- [ ] Add a stylesheet test for fixed viewport overlay and the `<60rem` single-column breakpoint.
- [ ] Run the focused tests and confirm they fail because the component and stylesheet do not exist.

### Task 2: Implement the example gallery

**Files:**
- Create: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.tsx`
- Create: `web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.module.css`
- Create: `web/platform/public/inspiration/paper-crane-cloud.png`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- `InspirationGallery(): JSX.Element` renders one card and owns dialog state.
- Recreate link: `/app/image?model=gpt-image-2&quality=1K&prompt=<encoded prompt>`.

- [ ] Copy the generated image into the public inspiration directory.
- [ ] Add Russian UI labels and the public example prompt.
- [ ] Implement the card, dialog, focus restoration, scroll lock, clipboard, share fallback, download and recreate actions.
- [ ] Route only `section="inspiration"` through `InspirationGallery`.
- [ ] Run focused tests until green.

### Task 3: Verify and deploy

**Files:**
- Verify all files changed in Tasks 1–2.

- [ ] Run `npm.cmd test`, `npm.cmd run typecheck`, `npm.cmd run lint`, and `next build --webpack` from `web/platform`.
- [ ] Review `git diff --check` and the final diff.
- [ ] Commit, push `HEAD:dev-deploy`, wait for signed images, run manual DEV deploy, and verify smoke success.
