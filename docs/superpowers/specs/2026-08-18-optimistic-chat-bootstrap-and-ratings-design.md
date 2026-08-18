# Optimistic Chat Bootstrap And Ratings Design

## Goal

Complete two high-frequency chat interactions without making the interface wait for the network:

- open a newly submitted conversation immediately under its client-generated UUID, show the first user message and assistant waiting indicator, and replace the temporary route with the canonical server conversation after the create-and-send workflow succeeds;
- persist assistant-message likes and dislikes on the server while changing the selected icon immediately and rolling back only the latest failed choice.

## Chosen architecture

### New-conversation bootstrap

The existing two-request API remains unchanged. `WorkspacePrompt` becomes a synchronous launcher:

1. normalize the prompt and generate stable conversation and message idempotency UUIDs;
2. store a validated bootstrap intent in `sessionStorage`;
3. insert the pending conversation into `WorkspaceConversationList`;
4. clear the composer and navigate to `/app/chat/{clientUUID}?pending=1` in the same event turn.

The pending route renders `PendingConversationBootstrap`, which owns the network workflow and survives the launcher unmount:

1. show the optimistic user bubble and three-dot assistant indicator immediately;
2. create the conversation with the stored conversation idempotency key;
3. reconcile the sidebar item with the canonical server conversation;
4. send the first prompt with the stored message idempotency key;
5. save the prompt for the existing canonical history refresh, clear the bootstrap intent, and call `router.replace` with the server UUID;
6. on failure keep the same route and message, replace the dots with `Не отправлено` and `Повторить`, and reuse the exact same keys.

Persisting the intent makes refresh and retry safe. A malformed or absent intent produces the existing unavailable state without sending a request. Logout clears all bootstrap intents with the other private session data.

### Assistant-message ratings

Ratings are account-owned server data. A nullable `rating` column is added to `conversation_messages` with a database check permitting only `like`, `dislike`, or `NULL`. The message primary key already supplies the required lookup index, so no additional index or per-rating table is needed.

The authenticated endpoint is:

```text
PUT /web/v1/conversations/{conversationID}/messages/{messageID}/rating
{"rating":"like" | "dislike" | null}
```

It succeeds only when the conversation belongs to the exact account, is an active Web conversation, and the message is an assistant message inside that conversation. It returns the canonical rating. Message-list DTOs include `rating`, allowing reloads to restore the controls.

The client changes the icon immediately and serializes writes per rendered message. Serial ordering prevents older requests from overwriting a newer click on the server. A failed latest write restores the last confirmed rating. Copying and user-message recreation remain independent.

## Rejected alternatives

- A combined create-and-send endpoint would reduce requests but would duplicate the established idempotent job workflow and broaden backend scope.
- Putting the first prompt in the URL would leak user content into history, logs, analytics, and referrers.
- Storing ratings only in `localStorage` would not synchronize across devices and would not satisfy server persistence.
- Sending rating writes in parallel could leave the server with a stale final choice when requests arrive out of order.

## Scale and safety

- Conversation bootstrap adds no server round trips and reuses existing idempotency behavior.
- Ratings add one indexed row update per user action and no new read query; ratings travel with the existing history response.
- The rating endpoint is protected by the existing unsafe-principal CSRF/session guard and exact account ownership checks.
- Session storage is treated as untrusted input and validated before use.
- Stale async completions cannot replace a newer local rating.

## Tests

- A deferred create request proves navigation, draft clearing, the user bubble, and dots happen before any response.
- Bootstrap refresh and retry tests prove the same UUID keys are reused and malformed storage never triggers API calls.
- Handler tests cover authentication, malformed input, account isolation, non-assistant rejection, set/change/clear, and list persistence.
- Memory and PostgreSQL repository tests cover ownership and canonical update semantics.
- Client tests prove immediate selection, serialized writes, rollback, and persisted initial rating.

## Out of scope

- Ratings for user messages or image-generation artifacts.
- Aggregate public like counts.
- Multiple concurrent first-message submissions for one temporary conversation.
- Changing job execution, billing, title generation, or provider selection.
