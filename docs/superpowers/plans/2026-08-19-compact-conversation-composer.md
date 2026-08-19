# Compact Conversation Composer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the nested conversation form appearance with the approved single-surface compact composer while preserving existing chat behavior.

**Architecture:** Keep `ConversationComposer` as the behavior owner and extend `ChatTextInput` with a dedicated composer appearance. Use native accessible buttons and CSS Modules; do not add backend coupling.

**Tech Stack:** TypeScript, React, CSS Modules, Vitest, Testing Library

---

### Task 1: Lock the approved structure with tests

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.test.tsx`
- Create: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.styles.test.ts`

- [x] Add component expectations for the media control, icon-only submit action, and note below the composer.
- [x] Add focused CSS expectations for a single rounded surface, hidden visual label, embedded controls, circular submit action, and borderless textarea appearance.
- [x] Run the focused tests and confirm they fail before implementation.

### Task 2: Implement the compact composer

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.module.css`
- Modify: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.tsx`
- Modify: `web/platform/src/components/chat/ChatTextInput/ChatTextInput.module.css`
- Modify: `web/platform/src/i18n/ru.ts`

- [x] Hide the visual label while retaining textarea accessibility.
- [x] Add the embedded media control and circular arrow submit action.
- [x] Add the truthful compact pricing/disclaimer line.
- [x] Add the borderless composer textarea appearance.
- [x] Preserve existing keyboard and pending-state behavior.

### Task 3: Verify the platform

- [x] Run the focused component and style tests.
- [x] Run the complete web platform test suite.
- [x] Run the production build.
- [x] Review the final diff for unrelated changes.
