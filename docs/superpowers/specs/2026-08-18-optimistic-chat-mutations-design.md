# Optimistic Chat Mutations Design

## Goal

Make the three most frequent chat mutations feel immediate without weakening server authority:

- rename a conversation immediately and roll back on failure;
- hide a deleted conversation immediately and restore it on failure;
- show a sent message and the assistant waiting indicator in the same frame as Enter, with an in-place retry on failure.

## Scope and constraints

- The backend, API routes, database, and billing logic do not change.
- The server remains the source of truth. Optimistic state is a temporary client projection.
- Existing conversation creation remains unchanged.
- Only one outgoing turn can be active in a conversation at a time, matching the current UI contract.
- A message retry must reuse the original `X-Idempotency-Key`.
- `/app`, mobile navigation, current themes, and existing polling semantics must keep working.

## Rename flow

1. Preserve the previous `ConversationItem`.
2. Update the shared `WorkspaceConversationList` before sending `PATCH`.
3. On success, replace the optimistic item with the canonical server response.
4. On failure, restore the preserved item, keep the proposed title in the editor, and expose `Повторить`.

This keeps every sidebar consumer synchronized and avoids a full route refresh.

## Delete flow

1. Mark the row as locally hidden before sending `DELETE`.
2. Keep the row component mounted so it can observe the request result.
3. On success, notify the parent to remove the canonical item and redirect only if the deleted conversation is active.
4. On failure, reveal the row in its original position and keep the archive panel open with retry feedback.

No tombstone is written to the shared list until the server confirms deletion.

## Send-message flow

`ConversationHistory` owns the full turn lifecycle:

```ts
type PendingTurn = {
  id: string;
  baselineSeq: number;
  prompt: string;
  idempotencyKey: string;
  status: "sending" | "accepted" | "failed";
};
```

1. `ConversationComposer` validates the draft, clears it immediately, and calls `onSubmit(prompt)`.
2. History creates a pending user bubble with status `sending` and renders the assistant three-dot indicator.
3. History sends the API request with the generated idempotency key.
4. A valid response changes the turn to `accepted` and starts the existing history poll.
5. Failure changes it to `failed`: dots disappear, the bubble remains, and `Не отправлено` plus `Повторить` appear.
6. Retry sends the same prompt with the same idempotency key and returns the same bubble to `sending`.
7. When persisted history contains the matching user message and assistant response, the optimistic turn disappears in favor of canonical history.

## Error behavior

- No global page reload or route transition is used for recoverable mutation errors.
- Errors stay next to the affected entity.
- Retry buttons are disabled only while their mutation is active.
- Stale responses cannot overwrite a newer local attempt.

## Tests

- Deferred rename proves the title changes before the request settles, then proves rollback and retry copy.
- Deferred delete proves the row disappears before the request settles, then returns on failure.
- Deferred send proves the draft clears and the bubble/dots render before the request settles.
- Failed send proves the in-place error and retry reuse the exact idempotency key.
- Existing success, polling, session restoration, and mobile sidebar tests remain green.

## Out of scope

- Optimistic chat creation (already implemented).
- Likes/dislikes persistence (no backend endpoint exists yet).
- Image generation billing or job lifecycle.
- Multiple concurrent outgoing turns in one conversation.
