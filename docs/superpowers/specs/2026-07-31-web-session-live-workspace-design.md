# NeiroHub Web Session and Live Workspace Design

Status: approved

Date: 2026-07-31

## Goal

Turn the existing static `/app` shell into a private, account-backed workspace:
users sign in through the existing neutral web-session contract, see only their
safe account label and their own web conversations in the fixed sidebar, can
create a durable empty conversation, and can safely sign out.

## Scope

- Reuse the existing cookie-only `/web/v1/auth/password/login`, refresh,
  logout, and `/web/v1/me` contract. This slice adds a password sign-in page
  only; account creation, password reset, identity linking, settings, chat
  messages, model calls, files, billing, and generation stay out of scope.
- Add a channel-neutral `web` conversation source. Web conversations are owned
  directly by `account_id`, never by a VK user id and never by the Mini App
  source.
- Add account-owner-checked `GET /web/v1/conversations` and idempotent
  `POST /web/v1/conversations` endpoints. The visible DTO exposes only a
  random conversation id, title, and timestamps.
- Keep the browser on same-origin `/web/v1/*`. A Next.js route handler proxies
  that route family to the internal Go origin, strips identity headers, and
  forwards cookies and every upstream `Set-Cookie` header.
- Make `/app/**` dynamic, `no-store`, and `noindex`. A valid session is
  required; no account, conversation, or profile data is rendered into `/`.

## Data Flow

```text
Browser
  -> same-origin /web/v1/* in Next.js
  -> server-only internal Go origin
  -> Account Layer / ConversationRepository
  -> Postgres
```

Server components load the profile and recent conversations with the incoming
cookie. Client mutations add the readable CSRF cookie as `X-CSRF-Token`; the
backend remains responsible for exact Origin, CSRF, session, and account-owner
checks.

## Backend Contract

The existing profile response remains the source of the account label. The UI
uses only the first verified, safe masked identity label and otherwise the
neutral `Аккаунт` label; it does not invent an avatar URL or expose an email,
phone, raw identity id, account id, or provider metadata.

`GET /web/v1/conversations?limit=<1..50>` returns:

```json
{"items":[{"id":"uuid","title":"string","updated_at":"RFC3339","created_at":"RFC3339"}]}
```

`POST /web/v1/conversations` requires a same-origin CSRF-validated request and
an `X-Idempotency-Key` UUID. That key is used only as the account-scoped web
thread reference. First use returns `201`; a repeat returns the same safe item
with `200`. Empty conversations receive their title only when the later chat
flow has a first user prompt.

The additive migration must permit `conversations.user_id` to be null for web
rows, allow `source = 'web'`, and add an active account/source/thread unique
index. Existing VK Bot and Mini App rows, constraints, and lookups remain
compatible.

## UI State

- Unauthenticated `/app/**` requests redirect to `/login` with a validated
  local return path. A Next.js `proxy.ts` records only the canonical pathname
  in a short-lived (five-minute), `HttpOnly`, `Secure`, `SameSite=Lax`,
  host-only return cookie; it is not rendered in a query string, browser
  storage, or API request to Go. Login reads and revalidates that cookie before
  client navigation, then falls back to `/app`.
- The sign-in form has submitting and safe failed states; raw backend errors
  are never surfaced.
- The authenticated sidebar receives feature content through props rather than
  importing account or conversation features. It remains fixed while the
  workspace is the only desktop scroll region.
- Creating a chat disables its trigger, shows progress, redirects to the new
  private placeholder route, and displays a recoverable error if the request
  fails.
- Logging out requires CSRF, clears the server session, and returns the user to
  `/login` only after the server reports success.
- A temporary upstream outage renders a private retryable service state rather
  than public content or a false authenticated shell.

## Security and Privacy

- Access and refresh tokens stay HttpOnly host-only Secure cookies; no token,
  account id, prompt, profile, or conversation data is written to local
  storage, URL query strings, analytics, or logs.
- The proxy strips Authorization, Cookie supplied by callers, and all identity
  headers before it chooses the request cookie itself. It permits only the
  `/web/v1/` route family and cannot be used as a general outbound proxy.
- Server storage queries for web conversations use exact `account_id` ownership
  rather than an account/user compatibility OR query.
- All private responses use `Cache-Control: no-store`; browser-side controls
  are presentation only and backend authorization remains mandatory.

## Verification

- Go handler, migration/repository, and security tests prove cookie-principal
  ownership, CSRF/origin rejection, idempotent create, safe DTO shape, and no
  Mini App/VK source reuse.
- Frontend tests prove same-origin proxy header handling, multiple `Set-Cookie`
  forwarding, CSRF mutation behavior, login failures, safe return-path handling
  without a query value or browser storage,
  session gates, sidebar data rendering, create-chat progress/error, and logout
  behavior.
- Run scoped Go tests, platform lint/typecheck/test/build, and a local browser
  smoke against an available dev API. No production deployment or live login is
  part of this slice.
