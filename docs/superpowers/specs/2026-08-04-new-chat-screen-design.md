# New chat screen

## Goal

Turn `/app/chats` from a placeholder into the dedicated entry point for a normal NeiroHub conversation while preserving the already proven conversation, naming, retry, and navigation flow.

## Approved behaviour

1. The static workspace header continues to show the section name and the current star balance.
2. The content area no longer shows the placeholder heading “Чаты” or any copy about connecting a session.
3. A centered greeting and a new-chat prompt are shown instead. The prompt says “Задайте вопрос NeiroHub”.
4. The prompt has a fixed height, cannot be resized, submits with `Enter`, and inserts a line break with `Shift+Enter`.
5. Submitting creates a normal conversation through the existing idempotent create-then-message workflow.
6. The sidebar immediately uses the first prompt (up to the existing limit) as a temporary title. The existing asynchronous title sync later replaces it with the smart DeepSeek title.
7. After the first message is accepted, client navigation opens `/app/chat/{id}` without a full-page reload. The existing pending-prompt handoff renders the user question, the left-aligned three-dot assistant indicator, bounded polling, and automatic scroll behaviour.
8. The same content and controls remain usable on mobile.

## Architecture

`WorkspaceHome` owns which workspace section is presented. Its `chats` branch will render the dedicated centered new-chat state instead of the generic placeholder branch.

`WorkspacePrompt` remains the only owner of new-conversation creation. A presentation variant changes only the label, placeholder, support copy, and compact new-chat styling. It does not fork request logic or introduce another API client.

The existing shared components and state continue to provide the rest of the flow:

- `ChatTextInput` owns fixed sizing and Enter/Shift+Enter semantics.
- `WorkspaceConversationList` owns the immediate pending sidebar entry.
- `pending-conversation-prompt` hands the first question across the client route transition.
- `ConversationHistory` owns optimistic question rendering, the assistant typing indicator, polling, and scrolling.
- `pending-conversation-title-sync` and `ConversationTitleSync` own temporary and smart titles.

No backend endpoint, billing rule, model routing, automatic intent detection, session contract, or database schema changes.

## Responsive layout

Desktop content is vertically centered inside the available workspace and constrained to a readable width. Mobile uses smaller horizontal and vertical padding, keeps the textarea and submit control within the viewport, and preserves the same keyboard behaviour.

## Testing

- Prove `WorkspaceHome section="chats"` renders the greeting and new-chat prompt but not the old placeholder copy or unrelated quick actions.
- Prove the new-chat `WorkspacePrompt` variant exposes “Задайте вопрос NeiroHub” while preserving the existing create-then-message and client-navigation flow.
- Keep existing shared input, pending prompt, optimistic history, typing indicator, autoscroll, title sync, retry, and idempotency tests green.

