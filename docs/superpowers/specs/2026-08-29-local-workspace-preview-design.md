# Local workspace preview

## Goal

Allow the real `/app` workspace layout to be reviewed locally without starting the Go API, database, Redis, MinIO, or Docker containers.

## Root cause

The workspace layout loads its authenticated session through `WEB_API_INTERNAL_ORIGIN`. During the frontend-only local run this variable is absent and no API listens on `127.0.0.1:8080`, so the server-side session loader returns `unavailable` and renders the neutral fallback.

## Design

Add an explicit server-only local preview flag named `NEIROHUB_LOCAL_WORKSPACE_PREVIEW`. When its value is `1` and `NODE_ENV` is exactly `development`, the session loader returns a fixed preview session containing a non-sensitive placeholder profile, balance, and conversation titles. The same guarded mode returns a fixed read-only response for `GET /web/v1/image-models` so the model selector, shortcuts, and featured cards render. In every other environment the existing API-backed session and proxy flows remain unchanged.

Enable the flag only in the ignored `web/platform/.env.development.local` file on this workstation. The variable must not use the `NEXT_PUBLIC_` prefix and must never be exposed to browser code.

## Safety and scope

- Production, DEV deployment, Docker, CI/CD, authentication, cookies, and API routes remain unchanged.
- Preview activation requires both development mode and the explicit flag.
- Preview values contain no real account identifiers, email addresses, credentials, tokens, or conversations.
- Only `GET /web/v1/image-models` is served locally; every other API request retains the existing proxy behavior.
- Backend-dependent mutations remain unavailable; the preview is only for layout and style review.
- The existing unavailable state remains unchanged when preview mode is disabled.

## Verification

- Add unit tests proving preview session and model-catalog activation in development and rejection in production.
- Preserve the existing tests for authenticated, unauthenticated, and unavailable API states.
- Restart `npm run dev` so Next.js loads `.env.development.local`.
- Reload `http://localhost:7158/app` and verify that the workspace shell, landing content, placeholder account, balance, conversations, model selector, shortcuts, and featured model cards render instead of the unavailable fallback.
