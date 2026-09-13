# Shared Input Surface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the model search field and chat composer one reusable transparent visual shell without coupling their behavior.

**Architecture:** Add a presentational `InputSurface` component that renders a styled `div`, forwards normal `div` props, and merges consumer layout classes. Move shared border, transparent background, radius, and `focus-within` styling into it. Keep search filtering and composer submission/media state inside their existing feature components.

**Tech Stack:** React, TypeScript, CSS Modules, Vitest, Testing Library.

## Global Constraints

- The surface uses the same `rgb(8 8 12 / 92%)` toner as `ModeSwitchPanel`.
- The surface owns the border, `1rem` radius token, and focus-within ring.
- Search and composer behavior remain independent.
- Existing consumer-specific layout, spacing, and responsive behavior remain in their current CSS modules.

---

### Task 1: Add and adopt `InputSurface`

**Files:**
- Create: `web/platform/src/components/ui/InputSurface/InputSurface.tsx`
- Create: `web/platform/src/components/ui/InputSurface/InputSurface.module.css`
- Create: `web/platform/src/components/ui/InputSurface/InputSurface.test.tsx`
- Create: `web/platform/src/components/ui/InputSurface/InputSurface.styles.test.ts`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.tsx`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.module.css`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.test.tsx`
- Modify: `web/platform/src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx`
- Modify: `web/platform/src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.module.css`
- Modify: `web/platform/src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.test.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.styles.test.ts`

**Interfaces:**
- Consumes: standard `React.ComponentPropsWithoutRef<"div">` properties.
- Produces: `InputSurface`, a `div` carrying `data-ui="input-surface"`, shared surface styles, and an optional consumer `className`.

- [x] **Step 1: Write failing component, consumer, and style tests**

Assert that `InputSurface` merges classes and forwards attributes, that both consumers render it, and that its CSS owns the transparent background, border, radius, and focus-within state.

- [x] **Step 2: Run tests and verify the expected failures**

Run: `npx vitest run src/components/ui/InputSurface src/components/chat/ChatComposer/ChatComposer.test.tsx src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.test.tsx src/features/conversations/ConversationComposer/ConversationComposer.styles.test.ts`

Expected: FAIL because `InputSurface` does not exist and consumers do not expose the shared surface.

- [x] **Step 3: Implement the shared surface and replace consumer wrappers**

Create the small wrapper component and CSS module, replace the two outer `div` elements, then remove duplicated visual declarations from consumer CSS while retaining layout declarations.

- [x] **Step 4: Run focused tests**

Run the command from Step 2.

Expected: PASS.

- [x] **Step 5: Verify the application**

Run `npm run typecheck`, `npm run lint`, and visually inspect both the chat composer and model search field in the local application.
