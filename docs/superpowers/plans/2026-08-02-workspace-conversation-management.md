# Workspace Conversation Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Give an authenticated web user a safe, complete sidebar chat workflow: active state, creation, rename, archive-as-delete, empty states, and correct mobile behavior.

**Architecture:** Keep the authenticated server layout responsible for initial data, Sidebar responsible for drawer state/focus, and a client conversation-list boundary responsible for local row interactions. Add account-scoped backend repository operations and protected PATCH/DELETE handlers; archive preserves historical data and hides the row from active lists. Client mutations use the existing same-origin proxy and refresh route data only after a successful response.

**Tech Stack:** Go 1.24 HTTP handler/domain repositories/PostgreSQL/in-memory test repo; Next.js App Router, React 19, TypeScript, Zod, Vitest and Testing Library; CSS Modules.

## Global Constraints

- Apply only to exact account-owned `web` conversations; do not change VK or Mini App behavior.
- `PATCH /web/v1/conversations/{conversationID}` accepts strict `{ "title": string }`; trim it, require 1–120 Unicode code points, and return only the existing safe DTO.
- `DELETE /web/v1/conversations/{conversationID}` archives rather than hard-deletes; own archived retry is `204`, foreign/missing/wrong-source is `404`.
- Reuse existing `requireUnsafePrincipal` for Origin/CSRF/session protection; no client-side ownership claims.
- The sidebar list renders active web conversations only. It has no optimistic rename/delete state that can diverge from the server.
- Preserve the component-folder convention: each new React component has its own folder, TSX, CSS Module and test file.
- Keep `Sidebar` as the mobile drawer and focus-trap owner; normal chat navigation closes it at <48rem, action buttons do not.
- Use Russian copy through `web/platform/src/i18n/ru.ts`; do not hard-code user-facing strings.
- Preserve current unrelated worktree changes.

---

### Task 1: Account-scoped conversation rename and archive API

**Files:**
- Modify: `internal/domain/repositories.go`
- Modify: `internal/adapter/storage/postgres/conversation.go`
- Modify: `internal/adapter/storage/memory/conversation.go`
- Modify: `internal/adapter/inbound/websession/handler.go`
- Modify: `internal/adapter/inbound/websession/handler_test.go`
- Create: `internal/adapter/storage/postgres/conversation_management_test.go`
- Modify if compilation requires it: concrete test spies embedding `domain.ConversationRepository`

**Interfaces:**
- Consumes: `domain.ConversationRepository`, `requireUnsafePrincipal`, `decodeJSON`, `safeConversation`, `domain.ErrNotFound`.
- Produces:

~~~
ListActiveByAccountSource(ctx context.Context, accountID uuid.UUID, source ConversationSource, limit, offset int) ([]*Conversation, error)
RenameActiveConversationForAccount(ctx context.Context, accountID, conversationID uuid.UUID, source ConversationSource, title string) (*Conversation, error)
ArchiveConversationForAccount(ctx context.Context, accountID, conversationID uuid.UUID, source ConversationSource) error
~~~

- Routes:

~~~
mux.HandleFunc("PATCH /web/v1/conversations/{conversationID}", h.requireUnsafePrincipal(h.renameConversation))
mux.HandleFunc("DELETE /web/v1/conversations/{conversationID}", h.requireUnsafePrincipal(h.archiveConversation))
~~~

- The existing list handler switches from `ListByAccountSource` to `ListActiveByAccountSource`.

- [ ] **Step 1: Write failing Go tests for externally visible behavior**

In `handler_test.go`, add table-driven cases that first assert the new routes are absent/fail, then cover:

~~~
func TestWebConversationRename(t *testing.T) {
  // own active web conversation + PATCH {"title":"  План запуска  "} => 200
  // response has id/title/created_at/updated_at only, title == "План запуска"
  // empty, 121-rune and unknown-field bodies => 400
  // foreign, Mini App/VK and archived IDs => 404
  // unsafe request without CSRF/Origin never calls the repository
}

func TestWebConversationArchive(t *testing.T) {
  // own active web conversation + DELETE => 204 and does not appear in GET list
  // repeat DELETE own archived row => 204
  // foreign, wrong source and absent IDs => 404
  // archived row still rejects POST messages via existing contract
}
~~~

Add PostgreSQL repository tests that seed active + archived rows and assert the active-list filter, rename predicate and archive idempotence use exact account/source ownership.

- [ ] **Step 2: Run RED tests**

Run:

~~~powershell
go test ./internal/adapter/inbound/websession ./internal/adapter/storage/postgres
~~~

Expected: compile/test failure because the route and repository methods do not yet exist.

- [ ] **Step 3: Add minimal repository operations**

Extend `ConversationRepository`; implement it in memory and PostgreSQL. Use account + source predicates in every query. PostgreSQL active list must be equivalent to:

~~~sql
SELECT id, user_id, account_id, source, vk_peer_id, external_thread_id, status, title, created_at, updated_at
FROM conversations
WHERE account_id = $1 AND source = $2 AND status = 'active'
ORDER BY updated_at DESC, created_at DESC
LIMIT $3 OFFSET $4;
~~~

Rename uses a single `UPDATE … WHERE id/account_id/source/status='active' RETURNING …`, mapping no row to `domain.ErrNotFound`. Archive accepts only `active` or `archived` rows under the same account/source and leaves an already archived row archived; missing/wrong-source maps to `domain.ErrNotFound`.

- [ ] **Step 4: Add protected handlers**

Add the two route registrations and handlers. Use this validation shape:

~~~go
const maxConversationTitleRunes = 120
type renameConversationRequest struct { Title string }

func validConversationTitle(raw string) (string, bool) {
  title := strings.TrimSpace(raw)
  return title, title != "" && utf8.RuneCountInString(title) <= maxConversationTitleRunes
}
~~~

`renameConversation` parses a non-nil UUID, calls `decodeJSON`, validates title, invokes `RenameActiveConversationForAccount(..., domain.ConversationSourceWeb, title)`, and writes `200 newSafeConversation(*conversation)`. `archiveConversation` parses the UUID, invokes `ArchiveConversationForAccount(..., domain.ConversationSourceWeb)`, and writes `204`. Both map `domain.ErrNotFound` to neutral `404 conversation not found`; all other repository errors become neutral `503 conversations unavailable`.

- [ ] **Step 5: Run GREEN tests and format**

Run:

~~~powershell
gofmt -w internal/domain/repositories.go internal/adapter/storage/postgres/conversation.go internal/adapter/storage/memory/conversation.go internal/adapter/inbound/websession/handler.go internal/adapter/inbound/websession/handler_test.go internal/adapter/storage/postgres/conversation_management_test.go
go test ./internal/adapter/inbound/websession ./internal/adapter/storage/postgres
~~~

Expected: all focused backend tests pass.

- [ ] **Step 6: Commit only Task 1 files**

~~~powershell
git add internal/domain/repositories.go internal/adapter/storage/postgres/conversation.go internal/adapter/storage/memory/conversation.go internal/adapter/inbound/websession/handler.go internal/adapter/inbound/websession/handler_test.go internal/adapter/storage/postgres/conversation_management_test.go
git commit -m "feat: manage account web conversations"
~~~

### Task 2: Active sidebar list and new-chat empty state

**Files:**
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.tsx`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.module.css`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.test.tsx`
- Modify: `web/platform/src/features/conversations/NewConversationButton/NewConversationButton.tsx`
- Modify: `web/platform/src/features/conversations/NewConversationButton/NewConversationButton.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `ConversationItem[]`, `usePathname()`, existing `NewConversationButton` and `webBrowserMutation`.
- Produces a client `SidebarConversations` that passes an exact active boolean to row rendering and always exposes create action.

- [ ] **Step 1: Write failing frontend behavior tests**

Add tests that mock `usePathname` and prove:

~~~tsx
render(<SidebarConversations conversations={conversations} />)
expect(screen.getByRole("link", { name: "Подготовить макет" })).toHaveAttribute("aria-current", "page")
expect(screen.getByRole("button", { name: ru.conversations.createLabel })).toBeEnabled()

render(<SidebarConversations conversations={[]} />)
expect(screen.getByText(ru.conversations.empty)).toBeInTheDocument()
expect(screen.getByRole("button", { name: ru.conversations.createLabel })).toBeEnabled()
~~~

Add a NewConversationButton test that verifies its successful navigation also calls `router.refresh()` only after a safely parsed 200/201 DTO.

- [ ] **Step 2: Run RED tests**

Run:

~~~powershell
npm.cmd --prefix web/platform test -- --run src/features/conversations/SidebarConversations/SidebarConversations.test.tsx src/features/conversations/NewConversationButton/NewConversationButton.test.tsx
~~~

Expected: failing active-state/create-action expectations.

- [ ] **Step 3: Implement sidebar presentation without changing drawer ownership**

Mark `SidebarConversations.tsx` with `"use client"`; use `usePathname` to derive an exact `/app/chat/` + ID match. Render the existing NewConversationButton above the recent list, preserve unnamed fallback and list semantics, and set `aria-current="page"` only on the active link. Add only visual active/hover/focus styles through the component CSS module.

Extend `NewConversationButton` as follows:

~~~tsx
// only after parseConversationList succeeds:
router.refresh()
router.push("/app/chat/" + conversation.id)
~~~

Do not introduce a client cache or repeat POST logic.

- [ ] **Step 4: Run GREEN tests and typecheck**

Run the focused Vitest command from Step 2, then:

~~~powershell
npm.cmd --prefix web/platform run typecheck
npm.cmd --prefix web/platform run lint
~~~

Expected: all focused tests, TypeScript and ESLint pass.

- [ ] **Step 5: Commit only Task 2 files**

~~~powershell
git add web/platform/src/features/conversations/SidebarConversations web/platform/src/features/conversations/NewConversationButton web/platform/src/i18n/ru.ts
git commit -m "feat: show active workspace conversations"
~~~

### Task 3: Conversation-row rename/archive actions and mobile route close

**Files:**
- Create: `web/platform/src/features/conversations/ConversationRow/ConversationRow.tsx`
- Create: `web/platform/src/features/conversations/ConversationRow/ConversationRow.module.css`
- Create: `web/platform/src/features/conversations/ConversationRow/ConversationRow.test.tsx`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.tsx`
- Modify: `web/platform/src/features/conversations/SidebarConversations/SidebarConversations.test.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.tsx`
- Modify: `web/platform/src/components/layout/Sidebar/Sidebar.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `ConversationItem`, `isActive`, `webBrowserMutation`, `useRouter`, `usePathname`.
- Produces:

~~~tsx
type ConversationRowProps = {
  conversation: ConversationItem;
  isActive: boolean;
};
export function ConversationRow({ conversation, isActive }: ConversationRowProps): JSX.Element;
~~~

- `Sidebar` closes an open narrow drawer after a pathname change without focus restoration; it retains its existing direct close for normal chat links.

- [ ] **Step 1: Write failing row and mobile tests**

Add a ConversationRow suite that asserts:

~~~tsx
// active link uses aria-current
// menu opens by accessible label; Escape/cancel closes it
// rename form starts with fallback/name; Enter sends PATCH JSON and router.refresh()
// bad response keeps typed text and reveals neutral role=alert
// delete requires confirmation; DELETE success refreshes non-active list
// delete success on active row router.replace("/app")
// bad delete keeps row and shows neutral role=alert
~~~

Add Sidebar tests with a real action button inside conversations slot to prove action clicks do not close the narrow drawer, and a pathname transition test to prove a successful active-chat delete does close the narrow drawer without returning focus to its trigger.

- [ ] **Step 2: Run RED tests**

Run:

~~~powershell
npm.cmd --prefix web/platform test -- --run src/features/conversations/ConversationRow/ConversationRow.test.tsx src/features/conversations/SidebarConversations/SidebarConversations.test.tsx src/components/layout/Sidebar/Sidebar.test.tsx
~~~

Expected: failing imports and missing controls/mutation behavior.

- [ ] **Step 3: Implement isolated row interaction**

Create `ConversationRow` as a client component. It renders the chat link, a labelled menu button, a rename form and inline archive confirmation. Its mutation paths are exactly:

~~~tsx
await webBrowserMutation("/web/v1/conversations/" + conversation.id, {
  method: "PATCH",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ title: nextTitle }),
});

await webBrowserMutation("/web/v1/conversations/" + conversation.id, { method: "DELETE" });
~~~

For PATCH, require `response.status === 200` and parse its DTO before closing edit state and calling `router.refresh()`. For DELETE, require `response.status === 204`; on `isActive`, call `router.replace("/app")`, otherwise `router.refresh()`. Never render raw server errors. All pending states disable only the row controls, and `event.stopPropagation()` is used only on the action control container so its buttons cannot be treated as a chat-link activation.

Use CSS Module rules for truncation, active row, non-hover-visible desktop actions, keyboard focus, compact mobile target sizes, inline confirmation and alert. Pass each row from `SidebarConversations`.

- [ ] **Step 4: Close narrow drawer when route changes**

In `Sidebar.tsx`, consume `usePathname` and add an effect that observes a pathname change after mount. If the viewport is narrow and the drawer is open, call the existing close state path with `restoreFocusRef.current = false`. This catches `router.replace("/app")` after deleting the active chat without changing desktop state, trigger semantics or the focus trap.

- [ ] **Step 5: Run GREEN tests and full frontend validation**

Run the focused command from Step 2, then:

~~~powershell
npm.cmd --prefix web/platform test -- --run
npm.cmd --prefix web/platform run typecheck
npm.cmd --prefix web/platform run lint
npm.cmd --prefix web/platform run build
npm.cmd --prefix web/platform run test:packaging
~~~

Expected: all frontend checks pass.

- [ ] **Step 6: Commit only Task 3 files**

~~~powershell
git add web/platform/src/features/conversations/ConversationRow web/platform/src/features/conversations/SidebarConversations web/platform/src/components/layout/Sidebar web/platform/src/i18n/ru.ts
git commit -m "feat: manage chats from workspace sidebar"
~~~

## Final verification

- [ ] Run `go test ./...` and `npm.cmd --prefix web/platform test -- --run`.
- [ ] Run typecheck, lint, production build, `test:packaging`, and `git diff --check` from the pre-task base through HEAD.
- [ ] Run independent task and final review; resolve all Critical/Important findings.
- [ ] Push `dev-deploy`, require green CI and signed images, manually dispatch existing DEV deployment if necessary, wait for DEV smoke, and confirm `https://dev-web.neiirohub.ru/` returns unauthenticated `401 Basic realm="NeiroHub development"`.
