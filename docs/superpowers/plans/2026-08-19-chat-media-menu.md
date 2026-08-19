# Chat Media Menu Implementation Plan

**Goal:** Replace the disabled media button in the shared chat composer with an accessible reusable media-source menu without pretending that unsupported server-side attachments already exist.

**Architecture:** `ChatComposer` remains the single composer used by workspace, new-chat, and conversation screens. A dedicated `ChatMediaMenu` owns open/close behavior, the native file input, keyboard handling, and media-source callbacks. Host screens can later connect uploaded/generated libraries through typed callbacks without changing the composer layout.

**Tech Stack:** React 19, TypeScript, CSS Modules, Vitest, Testing Library.

---

### Task 1: Specify the shared interaction

**Files:**
- Create: `web/platform/src/components/chat/ChatMediaMenu/ChatMediaMenu.test.tsx`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.test.tsx`

1. Test that the media trigger is enabled.
2. Test that it opens the three requested actions.
3. Test native file selection and uploaded/generated callbacks.
4. Test Escape and outside-click dismissal.
5. Run the focused tests and confirm they fail before implementation.

### Task 2: Implement the reusable menu

**Files:**
- Create: `web/platform/src/components/chat/ChatMediaMenu/ChatMediaMenu.tsx`
- Create: `web/platform/src/components/chat/ChatMediaMenu/ChatMediaMenu.module.css`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.tsx`
- Modify: `web/platform/src/components/chat/ChatComposer/ChatComposer.module.css`
- Modify: `web/platform/src/i18n/ru.ts`

1. Add the accessible popup trigger and menu.
2. Add a hidden native file input with safe media accept rules.
3. Expose typed callbacks for file, uploaded-library, and generated-library choices.
4. Close on selection, Escape, and outside click.
5. Match the current NeiroHub design tokens and mobile layout.

### Task 3: Preserve every current composer host

**Files:**
- Modify: `web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx`
- Modify: `web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx`
- Modify related tests only where the old disabled assumption exists.

1. Pass the shared menu labels from the typed Russian dictionary.
2. Keep current text submission unchanged.
3. Do not send unsupported attachment fields to the backend.

### Task 4: Verify

1. Run focused component tests.
2. Run the full platform test suite.
3. Run lint, typecheck, and production build.
4. Review the diff for unrelated changes.
