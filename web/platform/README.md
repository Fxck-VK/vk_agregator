# NeiroHub Web Platform

Standalone TypeScript, React and Next.js application for the NeiroHub browser
platform. It is an independently buildable frontend over the shared Go backend;
it does not replace the backend's identity, billing, jobs, artifacts, or
provider boundaries.

## Local commands

Run commands from the repository root:

```powershell
npm --prefix web/platform ci
npm --prefix web/platform run dev
npm --prefix web/platform run lint
npm --prefix web/platform run typecheck
npm --prefix web/platform run test
npm --prefix web/platform run test:packaging
npm --prefix web/platform run build
docker build -f Dockerfile.platform -t neirohub-platform:local .
```

## Local platform setup

The platform and the shared Go API are two separate local processes. Start the
configured development API first, then run the platform process. Set
`WEB_API_INTERNAL_ORIGIN` to that API's local HTTP(S) origin; this is a
server-only platform setting and must never use a `NEXT_PUBLIC_` name.
`config.example` documents the safe local value; copy it only into an ignored
local environment file when needed.

Browser requests stay same-origin and use only `/web/v1/*`. The platform route
handler forwards those requests to `WEB_API_INTERNAL_ORIGIN`; browser code does
not call the API origin directly.

A real local sign-in requires an existing verified password identity in the
development API. Do not add credentials, sessions, API origins containing
secrets, or other secrets to this repository. Keep them in local, ignored
environment configuration instead.

The web-session cookie contract remains `Secure`, host-only, and
`SameSite=Lax`. An end-to-end local sign-in therefore needs a compatible HTTPS
and origin setup. An HTTP preview is useful for non-authenticated UI work, but
must not weaken the cookie contract to make sign-in work.

`test:packaging` verifies that the production Dockerfile retains its pinned,
non-root standalone runtime contract. The container exposes only its internal
application port and checks `GET /health` locally.

## API and deployment boundary

Browser requests may use only same-origin `/web/v1/*` routes with
server-managed cookie sessions. The browser must not receive backend origins,
tokens, provider credentials, or trusted account and billing state.

The approved DEV-only remote DEV deployment hostname is
`https://dev-web.neiirohub.ru`. Its dashboard-managed Cloudflare route is
`dev-web.neiirohub.ru/* -> http://127.0.0.1:8088`; Nginx applies a gateway
before forwarding every path to this platform. The DEV API must set exactly
`WEB_ORIGIN=https://dev-web.neiirohub.ru`, preserving the same-origin
`/web/v1/*` rule required by host-only session cookies.

This hostname is supplied by the remote DEV deployment's
`docker-compose.dev-web.yml` overlay. The standard local DEV stack does not
start that overlay, so it cannot satisfy the gateway smoke without explicitly
including the platform and gateway configuration.

The gateway value is the distinct Development secret
`DEV_WEB_BASIC_AUTH_HTPASSWD`: one single pre-hashed htpasswd entry, never a
plaintext password. It is not an application login credential and must not be
read, sent or logged by smoke checks.

Operator acceptance is deliberately two-stage: verify an unauthenticated
`401`, clear the outer Basic Auth gate, then verify `/web/v1/me -> 401` from
the anonymous BFF and a protected administrative path -> 404. Restore the
gateway and complete password login. Confirm `Secure`, host-only,
`SameSite=Lax` cookies, `/web/v1/me`, CSRF rejection, and deep-link return
after login. This route is DEV-only; production remains unchanged.

## Repository layout

```text
web/
  miniapp/   VK Mini App
  admin/     Operator application
  platform/  Standalone web application
```

The platform must not access Postgres, Redis, S3, AI providers, payment
providers, or `/miniapp/*` directly. It uses the shared Account Layer and the
channel-neutral `/web/v1` API as those server contracts become available.
