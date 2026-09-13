# Chat Input Auto-Resize Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Grow every shared chat input through nine text rows, then scroll, with a manual expand/collapse action.

**Architecture:** Keep sizing state and DOM measurement inside `ChatTextInput`; keep `ScrollArea` as the native textarea viewport. Drive root and viewport height through one CSS custom property, and render the expand action as an overlay sibling so the textarea reserves its space.

**Tech Stack:** React, TypeScript, CSS Modules, Vitest, Testing Library

## Global Constraints

- Automatic growth stops at exactly nine computed text rows.
- Text must not overlap either the vertical scrollbar or expand control.
- Manual expansion must preserve the draft, selection, focus, and keyboard submission semantics.
- No new runtime dependency.

---

### Task 1: Specify shared input resizing behaviour

**Files:**
- Modify: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.test.tsx`
- Modify: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.styles.test.ts`

**Interfaces:**
- Consumes: existing `ChatTextInput` props and `ScrollArea` textarea viewport
- Produces: behavioural and CSS contracts for automatic and manual sizing

- [ ] **Step 1: Write failing component tests**

Add tests that mock textarea `scrollHeight`, verify the root `--chat-text-input-height` clamps to the nine-line maximum, and verify clicking `Развернуть поле ввода` changes `data-manually-expanded` plus its label to `Свернуть поле ввода` without changing the value.

- [ ] **Step 2: Write the failing style contract**

Assert that the stylesheet declares `--chat-text-input-max-auto-rows: 9`, reserves an inline action gutter, and defines the responsive manual expanded height.

- [ ] **Step 3: Verify RED**

Run `npx vitest run src/components/chat/ChatTextInput` from `web/platform`. Expect failures because the size custom property and manual action do not exist.

### Task 2: Implement automatic and manual resizing

**Files:**
- Modify: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.tsx`
- Modify: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.module.css`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.module.css`

**Interfaces:**
- Consumes: textarea ref, `scrollHeight`, computed `line-height`, current controlled value
- Produces: `--chat-text-input-height`, `data-manually-expanded`, and accessible toggle labels

- [ ] **Step 1: Add textarea measurement**

Use a layout effect and a shared viewport ref. Temporarily set the textarea height to the configured minimum, read `scrollHeight`, compute the nine-line ceiling from computed line height and block padding, then set the clamped height on the `ScrollArea` root.

- [ ] **Step 2: Add the expand action**

Render a `button type="button"` inside the input wrapper with `aria-label` and `aria-expanded`. Toggle local manual state, keep focus on the textarea, and switch between diagonal expand/collapse line icons.

- [ ] **Step 3: Update layout styles**

Use the height custom property for both the root and textarea, define a responsive manual height, reserve separate gutters for the action and scroll track, and remove the landing variants' one-line overflow override.

- [ ] **Step 4: Verify GREEN**

Run `npx vitest run src/components/chat/ChatTextInput src/components/chat/ChatComposer src/features/conversations/ConversationComposer src/features/image-generation/ImageGenerationComposer` from `web/platform`. Expect all selected tests to pass.

### Task 3: Verify integration and browser behaviour

**Files:**
- Verify only

**Interfaces:**
- Consumes: completed shared component
- Produces: verification evidence

- [ ] **Step 1: Run static checks**

Run `npm run typecheck` and `npm run lint` from `web/platform`; both must exit with code 0.

- [ ] **Step 2: Inspect browser growth**

Enter one, nine, and ten lines in the conversation composer. Confirm growth through line nine, fixed height plus visible branded scrollbar at line ten, and no text overlap.

- [ ] **Step 3: Inspect manual expansion**

Toggle the upper-right action in both directions and confirm the draft remains unchanged, focus stays in the textarea, the icon changes, and the composer returns to its content-sized height.
