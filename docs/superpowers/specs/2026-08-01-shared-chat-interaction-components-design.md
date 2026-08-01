# Shared chat interaction components

## Goal

Make the chat input and the assistant waiting state consistent for every present and future text-dialogue model in the web platform.

## Approved behaviour

1. A chat input has a fixed height and cannot be resized with a mouse.
2. `Enter` submits a non-empty enabled draft; `Shift+Enter` inserts a line break; IME composition never submits.
3. While a submitted message waits for a reply, three animated dots appear in the conversation stream below the newest user message, aligned as an assistant message on the left. The composer itself stays free of waiting dots.

These rules apply to the start-a-chat surface and to every conversation composer. They do not apply to the image-generation prompt: it is a configuration field whose Enter key must not bypass its pricing confirmation.

## Components

### `ChatTextInput`

`web/platform/src/components/chat/ChatTextInput/ChatTextInput.tsx` is a controlled textarea primitive. It owns only keyboard semantics and fixed textarea styling. Its API accepts `value`, `onChange`, `onSend`, `disabled`, `placeholder`, `rows`, and a fixed-height variant. It does not know an API endpoint, model, account, idempotency key, route, or error text.

`WorkspacePrompt` and `ConversationComposer` retain their own submit, retry, error, and navigation logic, and both render this primitive. Thus a new model-specific composer gets the same interaction behaviour by composition rather than copied event handlers and CSS.

### `AssistantTypingIndicator`

`web/platform/src/components/chat/AssistantTypingIndicator/AssistantTypingIndicator.tsx` renders exactly three decorative dots with an accessible live label and reduced-motion support. It has no polling, model, or request logic. A thread supplies its assistant label and places the component as an assistant-aligned list item.

## Conversation state

`ConversationHistory` remains the owner of polling. Once the message request is safely accepted, it begins the existing bounded refresh and displays an optimistic user bubble followed by `AssistantTypingIndicator`. This guarantees that the dots are visibly below the question immediately, including before the history endpoint has returned the persisted user message. When a matching persisted user message arrives, the optimistic bubble is removed so the question is rendered exactly once. The indicator disappears only when the matching refresh stops (assistant reply observed, timeout, or unmount).

The first prompt from `WorkspacePrompt` crosses the client-side route transition through session-scoped browser storage keyed by the new conversation ID. It is read without deletion and cleared only once that prompt is visibly optimistic or already persisted in history; this makes the handoff safe under React Strict Mode effect replay. The prompt never appears in a URL or in a backend contract.

No backend endpoint, API response, authentication flow, model routing, query string, or billing logic changes.

## Testing

- Unit-test the shared input for Enter, Shift+Enter, IME composition, and its non-resizable fixed-height class.
- Test `WorkspacePrompt` uses the shared Enter behaviour without changing its create-conversation/idempotency sequence.
- Test `ConversationHistory` renders the optimistic user bubble followed immediately by the left-aligned three-dot indicator, replaces the optimistic bubble with the server message without duplication, and removes the indicator on the assistant reply. Cover an already-persisted first prompt, Strict Mode replay, and newly arrived server messages that must remain before the optimistic turn.
- Preserve existing retry, polling, idempotency, and safe-response tests.
