# Optimistic workspace logout design

## Goal

Make logout feel immediate without pretending that the server session has already been revoked. Private workspace state disappears before the first paint after the click, while server invalidation continues in a boundary that remains mounted.

## User-visible states

- `authenticated`: render the current private workspace unchanged.
- `pending`: immediately render the existing guest `/app` landing and both `Войти` actions. Cover the swap with a non-blocking 140 ms blur/fade veil.
- `confirmed`: keep the guest landing visible and silently replace the route with `/app` plus a server refresh.
- `failed`: keep all private UI unmounted and show a compact neutral notice with `Повторить выход`. Never restore the account, balance, conversations or private child route automatically.

The `Войти` controls remain visible during optimistic logout. If one is pressed before confirmation, the boundary remembers that intent; it waits for or retries logout and opens `/login` only after confirmation.

## Network behaviour

- Logout remains `POST /web/v1/auth/logout` through `webBrowserMutation`.
- The operation has a maximum visible retry window below five seconds: three transport attempts, each limited to 1.3 seconds, separated by 250 ms and 500 ms.
- Retry only rejected/aborted transport attempts. An HTTP response other than `204` is authoritative and moves directly to `failed` because the existing handler expires browser cookies even when server revocation reports an error.
- A successful `204` is the only confirmed state.
- Duplicate clicks and retries while a request is already in flight are deduplicated.

## Privacy and cache handling

- The controller lives above `WorkspaceFrame`, so switching to `pending` unmounts `WorkspaceAccountProvider`, `WorkspaceConversationListProvider`, `WorkspaceDataCacheProvider`, their consumers and the requested private page in one React update.
- Clear pending conversation prompt/title entries from `sessionStorage`; retain non-personal preferences such as theme and sidebar collapse.
- Do not change the backend, cookie format, DEV Basic Auth or session refresh flow.

## Multiple tabs

Use `BroadcastChannel` when available. Publish `logout-started`, `logout-confirmed` and `logout-failed` messages. Other authenticated tabs immediately enter the same guest-locked state. Lack of BroadcastChannel support must not block logout.

## Accessibility and motion

- The transition veil never accepts pointer input and has no accessible content.
- The failure notice uses a polite status region and a real retry button.
- Disable the blur/fade animation under `prefers-reduced-motion: reduce` while preserving the immediate state swap.

## Verification

- A deferred network response proves private content disappears synchronously after the click.
- Tests cover `204`, HTTP failure, timeout/retry exhaustion, retry recovery, login intent during pending logout, storage cleanup and cross-tab events.
- Existing authenticated, server-rendered guest, refresh-required and unavailable layout tests remain green.

