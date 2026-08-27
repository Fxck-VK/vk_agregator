# Remove the blue dialog eyebrow

## Goal

Remove the blue uppercase `Диалог` label from every chat page state without changing messages, the model selector, chat behavior, or unrelated labels.

## Scope

- Remove the label from a loaded conversation history.
- Remove the label from the pending conversation bootstrap state.
- Remove the empty header wrappers so they do not preserve spacing for a label that no longer exists.
- Remove only the now-unused `historyEyebrow` translation and the shared `.header` and `.eyebrow` rules.
- Preserve conversation accessibility labels, message content, loading and error states, controls, and data flow.

## Implementation

Delete the two header blocks that render `ru.conversations.historyEyebrow` in `ConversationHistory` and `PendingConversationBootstrap`. Remove the translation property and the CSS rules used only by those blocks. No new component or conditional rendering is needed.

## Verification

- Add focused assertions covering both loaded and pending chat states.
- Prove the assertions fail before implementation and pass afterward.
- Run lint, typecheck, the full test suite, and a production build.
- Confirm the final diff preserves the already-uncommitted brand-eyebrow work and contains no unrelated changes.
