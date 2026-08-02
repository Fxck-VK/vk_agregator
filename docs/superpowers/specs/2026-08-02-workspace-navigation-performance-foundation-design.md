# Workspace Navigation Performance Foundation Design

## Goal

Make navigation inside the authenticated workspace feel immediate without
weakening account isolation or creating a fan-out of unnecessary requests at
one million monthly users. This is the first, frontend-only slice of a larger
performance programme.

## Evidence and Root Cause

The observed delay is not only a DEV-tunnel effect.

- `app/app/chat/[conversationId]/page.tsx` waits for a server-side request for
  up to 100 messages before it can render the chat route.
- The shared workspace layout is intentionally dynamic and loads `/me` and
  then `/conversations?limit=20`, both through `cache: "no-store"`. It is
  retained across ordinary sibling navigation by the App Router, but remains
  expensive on cold navigation and every explicit `router.refresh()`.
- There is no `loading.tsx` beneath `/app`, so a dynamic route has no
  immediate right-panel fallback while its RSC payload is pending.
- The model catalogue reloads `/web/v1/image-models` on every mount, and its
  CTA currently used a native anchor, which bypassed Next client navigation.

The current backend already uses Redis-backed job queues and keyset
pagination for image jobs. Changing its database architecture does not solve
the visible tab-switch delay first.

## Approaches Considered

1. **Backend-first caching.** Add Redis caches and database migrations before
   changing the UI. This is useful later, but leaves the first visible render
   blocked and risks putting account data in the wrong cache tier.
2. **Full client cache immediately.** Move session, conversations and chat
   histories into one new client data layer. This can make chat-to-chat
   navigation instant, but requires carefully designed account identity,
   mutation invalidation and bounded memory. It is too broad for a first
   performance fix.
3. **Staged navigation foundation (chosen).** Give every dynamic workspace
   route an immediate fallback, eliminate the known document navigation,
   deduplicate only the safe model-catalogue data in tab memory, then add an
   account-scoped chat-history LRU cache as the next independently reviewed
   slice.

## First Slice: Frontend Navigation Foundation

### 1. Immediate route feedback

Add `web/platform/src/app/app/loading.tsx` and an isolated CSS module. The
fallback renders only in the existing right workspace region; it never
recreates the sidebar, account control, or page shell. It is visible while a
dynamic child route such as a conversation is loading.

The fallback must be semantic (`role="status"`), use existing design tokens,
and be visually neutral. It must not expose an account id, request data, or
call the API.

### 2. Safe model catalogue cache

Create a focused `features/models` data module used by both `ModelsCatalog`
and `ImageGenerationPanel`.

- Cache only the validated `ImageModelList` DTO in JavaScript memory for the
  current tab.
- Use a 60-second TTL and a single in-flight promise, so concurrent mounts
  issue one request.
- Do not use `localStorage`, service-worker storage, Redis, CDN caching, or a
  public HTTP cache for this slice.
- Do not cache a rejected response, a non-200 response, or invalid DTO data.
- Keep the public API explicit: `loadImageModelCatalog()` and
  `resetImageModelCatalogCacheForTests()`.

The model DTO contains configuration facts only. No profile, conversation,
job, payment, image artifact, token, or account-derived response is placed in
this shared module cache.

### 3. Preserve client-side navigation

Replace the model-card native anchor with Next `Link`, retaining exactly the
encoded `/app/image?model=<id>` destination. This lets the App Router retain
the workspace shell and apply its normal production prefetch policy.

Do not force-prefetch every chat. At one million MAU, prefetching 20 private
histories per sidebar would multiply authenticated RSC/API requests and work
against the performance goal.

### 4. Measurement boundary

This slice adds no analytics vendor and sends no browser timing data to the
backend. It establishes testable immediate feedback. A later observability
slice will add sampled, privacy-reviewed navigation metrics before any
external reporting endpoint is enabled.

## Next Slice: Account-Scoped Chat Cache

After the foundation is verified, introduce a client provider with a bounded
LRU cache keyed by `account_id + conversation_id`. It will show a cached chat
immediately, revalidate in the background, and invalidate only the affected
conversation after message creation, archive or logout. The server remains
the authorization source for every revalidation. This needs its own design
because the current chat page blocks in a Server Component and because cache
invalidation must be proven across account changes.

## Later Backend Scale Slice

Keep private responses out of public CDN cache. Before high-volume rollout:

- replace conversation-list offset pagination with a cursor;
- add an index aligned with the active-conversation predicate;
- measure PostgreSQL, Redis and queue p95s under representative load;
- choose a privacy-reviewed telemetry sink for sampled navigation metrics.

These changes are intentionally out of the first frontend slice.

## Testing and Acceptance Criteria

- Tests prove the loading fallback is a right-panel status without client
  data fetches.
- Tests prove a model link remains a Next client link with the exact encoded
  destination.
- Cache tests prove fresh reuse, TTL expiry, concurrent single-flight fetch,
  rejected-response eviction and invalid-DTO eviction.
- Both catalogue and generator consume the same cache API.
- Existing auth, chat, mutation and mobile-drawer behaviour remains covered
  by the existing test suite.
- No Go, database, API-contract, authentication, deployment or CDN
  configuration files change in this slice.
