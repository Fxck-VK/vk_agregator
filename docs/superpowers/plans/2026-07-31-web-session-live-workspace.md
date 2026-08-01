# Web Session and Live Workspace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver an account-native, cookie-session-backed `/app` workspace with a safe live sidebar and durable creation of empty web conversations.

**Architecture:** Keep Go as the single source of identity and conversation ownership. Add a small `/web/v1` adapter contract for account-owned `web` conversations, then let the standalone Next.js frontend fetch it only through same-origin cookie requests and a server-only BFF proxy. Components receive feature content through composition so shared layout stays independent.

**Tech Stack:** Go `net/http`, PostgreSQL migrations, existing Account Layer/session service, Next.js 16.2.9 App Router, React 19.2.7, TypeScript strict, Zod, CSS Modules, Vitest and Testing Library.

## Global Constraints

- Do not call `/miniapp/*`, VK, providers, storage, payments, or a database from `web/platform`.
- Do not store or render access/refresh tokens, account IDs, raw identity details, prompts, or private data in browser storage, query strings, logs, or public pages.
- Keep Go session cookies `Secure`, `HttpOnly` except the CSRF cookie, host-only, and `SameSite=Lax`; never weaken the production cookie contract for local preview.
- All unsafe `/web/v1` writes require the existing exact Origin and CSRF validation. New conversation creation uses an account-scoped UUID `X-Idempotency-Key`.
- Use exact `account_id` ownership for web conversations; preserve existing VK Bot and Mini App compatibility behavior.
- Every new React component owns its folder, TSX, CSS Module, and test when interactive. Route files remain thin compositions.
- `/app/**` is authenticated, dynamic, private, `no-store`, `noindex`, and `nofollow`; `/` remains public with no account state.
- Do not commit, push, deploy, add a production hostname, or change real provider/payment behavior.

---

### Task 1: Add account-native web conversations to the Go contract

**Files:**
- Modify: `internal/domain/conversation.go`, `internal/domain/repositories.go`
- Modify: `internal/adapter/storage/memory/conversation.go`, `internal/adapter/storage/memory/conversation_test.go`
- Modify: `internal/adapter/storage/postgres/conversation.go` and its focused tests
- Create: `migrations/000046_web_account_conversations.up.sql`, `migrations/000046_web_account_conversations.down.sql`

**Interfaces:**
- Add `domain.ConversationSourceWeb = "web"`.
- Add exact owner methods `GetByIDForAccount(ctx, accountID, conversationID)` and `ListByAccountSource(ctx, accountID, source, limit, offset)` to `domain.ConversationRepository`.
- `CreateConversation` accepts an account-only conversation with nil legacy `UserID` only when `AccountID` is non-nil.

- [ ] **Step 1: Write failing repository tests.**
  Prove that a web row with `UserID == uuid.Nil` is created and listed only for
  its exact account, that another account cannot read it, and that legacy
  Mini App/VK source behavior is unchanged.
- [ ] **Step 2: Run focused Go tests and observe the missing source/method failure.**
  Run: `go test ./internal/adapter/storage/memory ./internal/adapter/storage/postgres -run 'Web|Conversation' -count=1`
  Expected: FAIL because `web` and account-native methods do not exist.
- [ ] **Step 3: Implement the minimal domain/repository behavior.**
  Keep legacy source methods unchanged; add exact account queries and use SQL
  `NULL` for a nil legacy owner on account-native inserts.
- [ ] **Step 4: Add the reversible additive migration.**
  Permit nullable `conversations.user_id`, replace the source check with the
  three allowed sources, and create/drop the active `(account_id, source,
  external_thread_id)` web unique index in the paired migration.
- [ ] **Step 5: Run focused tests and formatting.**
  Run: `gofmt -w internal/domain/conversation.go internal/domain/repositories.go internal/adapter/storage/memory/conversation.go internal/adapter/storage/memory/conversation_test.go internal/adapter/storage/postgres/conversation.go` and the focused Go test command.
  Expected: PASS.

### Task 2: Expose owner-checked web conversation endpoints

**Files:**
- Modify: `internal/adapter/inbound/websession/handler.go`, `internal/adapter/inbound/websession/handler_test.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- `GET /web/v1/conversations?limit=...` returns `{"items": [...]}` with only
  id, title, created_at and updated_at.
- `POST /web/v1/conversations` requires a UUID `X-Idempotency-Key`, valid
  cookie principal, origin, and CSRF; first create is `201`, replay is `200`.

- [ ] **Step 1: Write failing handler tests.**
  Test unauthenticated read rejection, exact-account list isolation, absent or
  invalid CSRF/origin rejection before repository access, idempotent create,
  and safe DTO serialization without account/source/thread keys.
- [ ] **Step 2: Run the handler test and observe the missing route behavior.**
  Run: `go test ./internal/adapter/inbound/websession -run 'Conversation|Web' -count=1`
  Expected: FAIL because the routes and dependency are absent.
- [ ] **Step 3: Implement the adapter only.**
  Add the repository dependency, bounded pagination, safe DTO mapping,
  `Cache-Control: no-store`, and source `web`. Reuse the existing
  `requirePrincipal` and `requireUnsafeRequest` guards; do not trust a request
  account id or expose the idempotency/thread reference.
- [ ] **Step 4: Wire the shared repository in `cmd/api/main.go`.**
  Pass `core.Conversations` to the web handler without changing providers,
  jobs, billing, or Mini App wiring.
- [ ] **Step 5: Run focused Go tests and API package build.**
  Run: `go test ./internal/adapter/inbound/websession ./cmd/api -count=1`
  Expected: PASS.

### Task 3: Build the same-origin web API boundary and typed session data

**Files:**
- Create: `web/platform/src/app/web/v1/[...path]/route.ts`
- Create: `web/platform/src/lib/web-api/internal-origin.ts`, `proxy.ts`, and focused tests
- Modify: `web/platform/src/lib/web-api/browser.ts`, `browser.test.ts`, `contracts.ts`, `contracts.test.ts`, `server.ts`, `server.test.ts`
- Create: `web/platform/src/features/session/session-data.ts` and tests

**Interfaces:**
- Browser mutations use `webBrowserMutation(path, init)` which supplies the
  CSRF header from `nh_csrf`; generic browser fetch still strips forged
  identity headers.
- The proxy only accepts canonical `/web/v1/*` paths, forwards selected
  request headers/cookies to `WEB_API_INTERNAL_ORIGIN`, and appends all
  upstream `Set-Cookie` values to its response.
- `loadWorkspaceSession()` returns exactly `authenticated`, `unauthenticated`,
  or `unavailable`, with parsed safe profile and conversation list only.

- [ ] **Step 1: Write failing TypeScript tests.**
  Assert CSRF mutation headers, proxy stripping of caller identity headers,
  repeated `Set-Cookie` forwarding, strict Zod conversation parsing, and
  server state mapping for 401 and upstream failure.
- [ ] **Step 2: Run focused platform tests and observe missing modules.**
  Run: `npm.cmd --prefix web/platform run test -- src/lib/web-api src/features/session`
  Expected: FAIL because the mutation/proxy/session modules do not exist.
- [ ] **Step 3: Implement the minimal secure boundary.**
  Factor internal-origin validation into a server-only helper, preserve
  `no-store`, do not forward raw Authorization or caller Cookie headers, and
  use `Headers.getSetCookie()` plus `append` for upstream cookies.
- [ ] **Step 4: Implement session data loading.**
  Fetch `/web/v1/me` and `/web/v1/conversations?limit=20`, parse only
  documented DTOs, and convert backend errors into internal state rather than
  exposing raw text.
- [ ] **Step 5: Run focused tests.**
  Run: `npm.cmd --prefix web/platform run test -- src/lib/web-api src/features/session`
  Expected: PASS.

### Task 4: Make the application layout, account control, and sidebar live

**Files:**
- Create: `web/platform/src/app/login/page.tsx`, `page.module.css`, `layout.tsx`
- Create: `web/platform/src/features/auth/LoginForm/*`
- Create: `web/platform/src/features/account/AccountControl/*`
- Create: `web/platform/src/features/conversations/SidebarConversations/*`, `NewConversationButton/*`
- Create: `web/platform/src/app/app/chat/[conversationId]/page.tsx`
- Modify: `web/platform/src/app/app/layout.tsx`, `web/platform/src/components/layout/Sidebar/Sidebar.tsx`, `Sidebar.module.css`, and tests
- Modify: `web/platform/src/i18n/ru.ts`, `web/platform/src/features/workspace/WorkspaceHome/*`, relevant route composition/tests

**Interfaces:**
- `Sidebar` accepts account and conversation React nodes through props; it
  continues to own only layout/navigation/drawer behavior.
- `LoginForm` posts only email/password to the same-origin API and redirects
  only to a validated local `/app` path after success.
- `AccountControl` renders a safe primary identity label and CSRF-protected
  logout.
- `NewConversationButton` creates a UUID idempotency key, posts through the
  mutation helper, and navigates to `/app/chat/<safe-id>` after success.

- [ ] **Step 1: Write failing component/route tests.**
  Cover login submitting/failure/safe redirect, account logout CSRF behavior,
  empty and populated recent chat lists, create progress/error/success, and
  preservation of mobile-sidebar focus behavior with composed feature content.
- [ ] **Step 2: Run focused tests and observe the missing components.**
  Run: `npm.cmd --prefix web/platform run test -- src/features src/components/layout/Sidebar`
  Expected: FAIL because the components and dynamic layout are absent.
- [ ] **Step 3: Implement feature-local components and dictionary copy.**
  Use CSS Modules and no browser storage. Keep raw backend errors hidden and
  do not introduce a speculative user name/avatar contract.
- [ ] **Step 4: Make `/app` dynamic and session-gated.**
  On unauthenticated state redirect to `/login`; on unavailable state render a
  private retry state; on authenticated state compose the safe profile and
  conversations into the current fixed sidebar. Keep all metadata `noindex,
  nofollow`.
- [ ] **Step 5: Run focused tests, lint, and typecheck.**
  Run: `npm.cmd --prefix web/platform run lint`; `npm.cmd --prefix web/platform run typecheck`; `npm.cmd --prefix web/platform run test`.
  Expected: PASS.

### Task 5: Verify integration boundaries and document local requirements

**Files:**
- Modify: `web/platform/README.md`, `web/platform/.env.example`, `docs/INDEX.md`
- Test: focused Go and platform suites, production platform build, migration validation where the existing migration baseline allows it

- [ ] **Step 1: Write any missing failure-oriented boundary test.**
  Add a regression test for a rejected non-`/web/v1` proxy path or a forged
  owner header if it is not already covered by Task 3.
- [ ] **Step 2: Update the local setup documentation.**
  State the two local processes/origin prerequisites and that a real sign-in
  requires a configured dev API and an existing verified password identity;
  do not put credentials in the repository.
- [ ] **Step 3: Run the complete scoped gates.**
  Run: `go test ./internal/adapter/inbound/websession ./internal/adapter/storage/memory ./internal/adapter/storage/postgres ./cmd/api -count=1`; `npm.cmd --prefix web/platform run lint`; `npm.cmd --prefix web/platform run typecheck`; `npm.cmd --prefix web/platform run test`; `npm.cmd --prefix web/platform run build`; `git diff --check`.
  Expected: all new scoped checks pass. Report Docker/migration-suite failures only if they are unrelated baseline blockers.

## Review Checklist

- A valid browser cookie principal is the only identity input to every new web
  endpoint.
- No path lets one account list or create data in another account’s scope.
- No frontend code touches `/miniapp/*`, provider/payment/storage systems, or
  client persistence for account state.
- Existing fixed-sidebar/right-scroll and mobile focus behavior survives.
- New private pages have noindex/no-store behavior and the public home remains
  account-free.
