# DeepSeek Conversation Title Design

## Goal

Give each newly created Web chat a short, meaningful title generated from its first user message by the already configured `deepseek-ai/DeepSeek-V4-Flash` model. The title must not delay the answer, consume user stars, or overwrite a title chosen by the user.

## Scope

- Applies only to active `web` conversations.
- Leaves VK Bot and Mini App title behaviour unchanged.
- Reuses the existing DeepInfra credential and worker deployment; no browser receives a provider credential.
- Does not create a visible user job, artifact, billing reservation, or delivery for title generation.

## Why a new flow is needed

The current dialog-context worker writes a deterministic 80-rune copy of the first prompt into an empty Web title. It is not semantic and the persistent client sidebar does not learn about that later write. Calling a model in the HTTP request would delay the chat and be lost on a restart; using a normal text job would pollute user history and billing.

## Chosen architecture

### 1. Immediate, durable fallback

New Web conversations begin with `title_origin = auto_pending`. When the normal text worker persists the first user message, it atomically records a trimmed, bounded prompt fallback and changes the origin to `auto_fallback`. The browser shows the same short fallback immediately, so it never needs to display `Без названия` while the title worker runs.

The database adds a private `title_origin` column with four values:

- `manual` — set by an explicit user rename; never changed automatically.
- `auto_pending` — a new Web chat is eligible for a generated title.
- `auto_fallback` — the first-prompt fallback is stored and may be replaced by a generated title.
- `auto_generated` — a validated DeepSeek title is stored.

Existing non-empty conversations migrate to `manual`; existing empty Web conversations migrate to `auto_pending`. The origin is not part of the public conversation DTO.

### 2. Durable internal title task

When an accepted Web text job belongs to a title-eligible conversation, its creation transaction records a second, independent `event.conversation_title.queued` outbox row. The existing `event.job.queued` path remains solely responsible for the normal answer. The title event publishes to a dedicated Redis stream, `stream:conversations:title`, and contains only job/account/correlation metadata—never the prompt.

The title worker loads the persisted job and conversation, verifies the exact account, `web` source and active status, and reads only the first stored user message from the database. If the normal worker has not yet stored that message, the title task is retryable. Repeated deliveries are safe: only the first-message job for a conversation may call the generator, and the final write is an atomic compare-and-set.

This keeps title work off the normal chat latency path: a Redis or DeepSeek failure retries or falls back only within the title path and cannot duplicate or delay the normal chat generation event. The title stream has a recovery lease longer than the bounded provider request timeout, so another worker cannot reclaim a live model call. A newly provisioned title consumer group replays retained title entries from the beginning, protecting a relay-first rollout. A failed title task never changes the success or failure state of the user chat job.

Each title-engine replica claims one task at a time. That keeps a Redis lease
aligned to exactly one bounded provider call; throughput scales by adding title
worker replicas rather than by pre-claiming a serial batch.

### 2.1 Rollout compatibility gate

`event.conversation_title.queued` is understood only by the new outbox relay.
Before an API cutover that can emit it, every separately managed
`WORKER_MODE=relay` process must be upgraded to the same image or stopped. The
managed deploy scripts make this an explicit `--relay-only-workers-upgraded`
gate, then start and wait for the new jobs worker before starting the API. The
DEV workflow can attest to the compose-managed topology; a production operator
must pass the gate only after completing the equivalent relay fleet rollout.
No rolling overlap with an old relay is permitted.

### 3. DeepSeek generator boundary

Add an internal `conversationtitle.Generator` interface. Its DeepInfra implementation uses the configured DeepSeek V4 Flash model with:

- a trusted system instruction to return a concise 1–6 word title in the user’s language, without quotes or explanations;
- the first prompt only as untrusted user content;
- a small output cap (32 tokens);
- deterministic provider idempotency key `conversation-title:<conversation-id>`.

The output is trimmed, collapsed to one line, stripped of wrapping quotes, limited to 80 Unicode runes and rejected if empty. The internal call is not a normal generation Job, so it cannot reserve or capture user stars. It still has provider cost, which is borne by the platform.

The database update succeeds only for the exact active account-owned Web conversation whose origin is `auto_pending` or `auto_fallback`. It sets `title_origin = auto_generated`. A manual rename sets `title_origin = manual`, so a late or retried title task cannot overwrite it.

### 4. Sidebar reconciliation

`WorkspacePrompt` immediately upserts a local DTO whose fallback title is derived from the first prompt. On the open chat page, a small account-scoped title-sync hook performs a bounded backoff refresh of only that conversation: 1, 2, 4, 8 and 15 seconds, then stops. It replaces the local fallback when the server returns the generated or manually changed title.

The refresh uses a protected `GET /web/v1/conversations/{conversationID}` endpoint returning the existing safe DTO only. It is indexed by account and conversation id, avoids refetching the full sidebar, and is bounded to at most five lightweight requests per newly created chat. Reloads always receive the persisted fallback or generated title from the normal list endpoint.

## Failure and privacy rules

- Missing, malformed or provider-failed titles leave the durable first-prompt fallback in place; the chat answer remains unaffected.
- Retries and duplicate stream deliveries cannot create multiple title writes.
- Prompt text is never included in outbox payloads, Redis task payloads, metrics, logs or public API responses.
- Only the first user message, not conversation history or assistant replies, is sent to DeepInfra.
- The UI keeps its existing manual rename control; manual input is authoritative.

## Verification

- Backend tests cover outbox fan-out/replay, no prompt in queue data, source/account validation, first-message race retry, malformed model output, no user billing, retry-safe compare-and-set, and rename-before-completion.
- Dialog-context tests prove Web fallback persistence and unchanged Mini App behaviour.
- Frontend tests prove immediate fallback display, eventual generated-title replacement, bounded polling, unmount cleanup, and account isolation.
- Full Go and frontend quality gates, CI image build and DEV smoke run before release.
