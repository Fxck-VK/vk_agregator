# Instant Sidebar Conversation Insertion Design

## Root cause

The first-message flow in `WorkspacePrompt` creates a conversation, submits a message, and navigates to `/app/chat/:id`, but it does not update the persistent workspace layout. The layout receives `session.conversations` as server-rendered props once, and `SidebarConversations` only renders that prop. A client-side route transition preserves the layout, so the new conversation is absent until a document reload fetches the list again.

## Goal

Show a newly created conversation in the sidebar immediately after the server accepts its creation, without waiting for a page reload and without changing the backend.

## Chosen architecture

Add an account-scoped in-memory conversation-list provider inside `WorkspaceFrame`.

- The server still supplies the initial authoritative `ConversationItem[]` list.
- `WorkspaceFrame` passes the authenticated account id and initial list to `WorkspaceConversationListProvider` and renders `SidebarConversations` inside it.
- `WorkspacePrompt` calls `upsertConversation(createdConversation)` immediately after a valid create response. The sidebar observes the shared state and renders the chat at the top while the first message request proceeds.
- When a future Server Component refresh delivers a new initial list, the provider replaces its list with that server-authoritative list. Keying the provider by account id prevents state from one account appearing under another account.

## Behaviour and failure handling

- The server must first return a valid conversation DTO. No provisional id is invented by the browser.
- A valid created conversation remains in the sidebar even if the first message request fails, because the server has already created it and the user can retry in that conversation later.
- Repeated upserts for the same id replace and move that item to the top rather than creating duplicates.
- Existing list rendering outside the workspace provider continues to accept direct `conversations` props for isolated tests and reuse.

## Scope

- Frontend TypeScript/React only.
- No API, schema, session, or backend changes.
- Preserve existing create/message idempotency, pending prompt storage, route navigation, mobile sidebar behaviour, and rename/archive controls.
- Do not introduce global browser events, persistent browser storage, or an unbounded cross-account cache.

## Verification

- Provider tests cover initial rendering, idempotent upsert-and-move-to-top, server-list reconciliation, and account boundary reset.
- An integration test renders `WorkspacePrompt` and `SidebarConversations` inside the provider and proves the returned conversation link appears before route navigation/reload.
- Existing prompt failure tests prove malformed creation responses do not insert a sidebar item.
