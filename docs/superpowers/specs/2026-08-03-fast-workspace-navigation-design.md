# Fast Private Workspace Navigation

Status: approved in conversation

Date: 2026-08-03

## Goal

Make switching among the five fixed authenticated NeiroHub sections feel
immediate without restoring a global loading screen, exposing private data, or
creating unbounded work for a high-traffic account.

## Confirmed Root Cause

The previous global `/app/loading.tsx` was removed because it replaced the
workspace with a visually inconsistent loading surface. With the fallback gone,
dynamic App Router routes wait for their route payload when users click them.
Conversation pages additionally wait for private history in their server page,
and the Files route loads its first private page only after the client mounts.

## Chosen Design

### Bounded route prefetch

The fixed sidebar links `/app`, `/app/chats`, `/app/files`, `/app/models`, and
`/app/inspiration` opt into full Next.js route prefetch. These are the only
routes preloaded: recent conversation links remain on-demand so opening the
workspace cannot trigger a request for every chat history.

### Account-scoped in-memory data cache

`WorkspaceFrame` owns a client-only cache provider keyed by `accountId`. It
lives only for the open browser tab and is discarded when the account changes
or the tab closes. It stores only:

- at most eight ready conversation-history first pages, evicted least recently
  used;
- one ready first page of the current account's image-job file list.

It does not use `localStorage`, cookies, module-global account state, CDN
caching, result artifacts, prompts in telemetry, or cached balances/prices.

### Stale-while-revalidate private views

The Files workspace renders a cached first page synchronously when available
and revalidates the first page in the background. A conversation route renders
a cached ready history synchronously and refreshes it in the background. On a
cold chat, only the chat content displays its existing neutral private state;
the persistent shell stays visible. `404` and transient failures are never
cached as ready data.

### DEV-only timing evidence

A client-only timing collector records a bounded set of anonymous navigation
and private-view data-load durations only on local and DEV hosts. Records use
route categories (`workspace`, `files`, `models`, `inspiration`, `chats`, or
`conversation`) rather than conversation IDs, prompts, account identifiers, or
URLs with query values. They stay in `window` memory for browser-console
diagnosis and are never sent to an analytics service.

## Data Flow

```text
fixed sidebar link enters viewport
  -> Next.js full route prefetch for one of five known routes
  -> user clicks link: App Router reuses the workspace shell
  -> private route uses account-tab cache if a ready page exists
  -> same-origin /web/v1 request quietly revalidates the page
  -> cache and visible view receive only safe parsed DTOs
```

## Scale and Privacy Boundaries

- The prefetch budget is exactly five fixed routes per active workspace shell;
  it does not grow with chat count.
- Conversation history cache capacity is capped at eight first pages per
  account/tab, preventing a long chat list from growing browser memory without
  bound.
- Files cache retains one 12-item list page, not files themselves or artifact
  bytes. Artifact metadata and previews remain owner-checked requests.
- The backend remains the authority for session, account ownership, messages,
  files, jobs, pricing, and balances. Cache data is presentation-only.
- All browser requests remain same-origin `/web/v1/*`; no provider, storage,
  payment, database, or VK call is introduced.

## Failure Behavior

- A failed revalidation leaves an already visible cached view intact and does
  not turn it into a global loading/error screen.
- A cold failure shows the existing safe unavailable/not-found state only in
  the affected private view.
- Route prefetch failure is non-fatal: the later click performs normal route
  navigation.

## Verification

- Unit/component tests prove that only the five fixed links receive forced
  prefetch, while chat links do not.
- Cache tests prove per-provider isolation, LRU eviction, and no caching of
  failure states.
- Files and history loader tests prove cached content renders before a delayed
  revalidation resolves and that a fresh response replaces it.
- DEV metrics tests prove data-free bounded records and no operation outside
  DEV/local hostnames.
- Platform lint, strict typecheck, test suite, production build, package test,
  Docker image workflow, DEV deploy, and DEV smoke must pass before handoff.
