# Sidebar Shared-Scroll Layout Design

## Goal

Make the workspace sidebar a three-zone layout: a fixed brand/header, one shared scrollable navigation-and-conversations region, and a fixed account footer. Remove the duplicate "Create chat" control from recent conversations.

## Agreed behaviour

- The NeiroHub brand and desktop-collapse control stay visually fixed at the top of the sidebar.
- Navigation links and recent conversations scroll together in one middle region; recent conversations no longer own a separate scrollbar.
- The account section remains fixed at the bottom on desktop and in the open mobile drawer.
- The recent-conversations "Create chat" button is absent. The existing "New chat" navigation link remains available and continues to lead to `/app/chats`.
- On small screens, choosing a chat or the "New chat" navigation link still closes the drawer.

## Architecture

`Sidebar` gains a `scrollArea` wrapper around its navigation and conversations slot. The panel stays a height-constrained flex column with `overflow: hidden`; `scrollArea` becomes its only vertical overflow owner. Existing custom scrollbar tokens move to that wrapper.

`SidebarConversations` stops rendering `NewConversationButton`. Its archive-focus fallback moves to the stable `sidebar-new-chat` navigation link provided by `Sidebar`, so deleting the last relevant chat does not leave keyboard focus on a removed element.

## Scope and non-goals

- Frontend layout and interaction only; no API, server, account, or conversation data changes.
- Do not change what `/app/chats` does or introduce a new create-conversation endpoint.
- Keep existing rename/archive behavior and desktop/mobile drawer semantics intact.

## Verification

- Component tests prove the create button is absent, the "New chat" link remains, and the fallback focus target is valid.
- Stylesheet contract tests prove only the middle `scrollArea` owns vertical scrolling.
- Existing narrow-drawer tests prove a navigation selection closes the drawer.
