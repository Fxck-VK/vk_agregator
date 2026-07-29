# NeiroHub Web Platform

Status: accepted

Date: 2026-07-29

## Goal

Build a standalone browser platform for NeiroHub on TypeScript, React, and
Next.js. The platform should provide the complete product experience—chat,
model catalog, image and video workflows, files, history, account, balance,
and payments—while making its public model, tool, prompt, comparison, and
editorial pages easy for Google and Yandex to crawl, understand, and rank.

The platform must be production-shaped from its first vertical slice. Search
visibility, performance, accessibility, security, observability, independent
deployment, and rollback are release requirements rather than later
optimizations.

## Reference Policy

StudyAI/Study24 is a product-structure and SEO reference. NeiroHub may reproduce
the useful interaction patterns and information architecture:

- persistent navigation for chats, files, models, and inspiration;
- a universal prompt entry point;
- a searchable model and tool catalog;
- dedicated task-oriented model/tool pages;
- public explanations, instructions, examples, FAQ, and related links;
- editorial clusters for guides, prompts, comparisons, and model updates.

NeiroHub must not copy StudyAI source code, private APIs, text, logos, images,
illustrations, or pixel-identical visual design. All implementation, content,
assets, naming, metadata, and component code must be original. The first
release may use a neutral temporary visual system that can later be replaced
with the final NeiroHub identity through design tokens and shared components.

## Architecture Decision

Use one independently deployable Next.js application with two explicit route
surfaces:

```text
Browser
  |
  v
NeiroHub Next.js platform
  |-- public/indexable surface: SSG or ISR
  |-- authenticated product surface: dynamic, no-store, noindex
  |-- same-origin BFF/session boundary
  |
  v
Versioned Go web API (/web/v1)
  |
  +-- Account Layer and web sessions
  +-- product catalog and pricing
  +-- job orchestration and workers
  +-- ledger-backed billing and payments
  +-- owner-checked artifacts
```

Next.js is a presentation and session-facing layer. The existing Go backend
remains the source of truth for identity, authorization, prices, balances,
payments, jobs, moderation, artifacts, and provider routing.

### Alternatives Considered

1. **One hybrid Next.js platform—selected.** Public and authenticated route
   groups share a domain, navigation, components, and internal linking while
   retaining different rendering and cache policies.
2. **Separate SEO site and product SPA.** This isolates failures but duplicates
   navigation and design, complicates authentication, and splits search and
   product signals across deployments or domains.
3. **Client-only React SPA with prerendering.** Rejected because critical
   content, links, metadata, error status codes, and crawl behavior would be
   more fragile and performance would depend on unnecessary browser
   JavaScript.

## Product Decomposition

The complete platform is split into independently verifiable subprojects. Each
subproject receives its own implementation plan and vertical-slice release.

1. Web identity, session verification, and `/web/v1` API foundation.
2. Next.js runtime, platform shell, universal chat, catalog, and one working
   generation flow.
3. Image and video workflows with asynchronous job progress and artifacts.
4. Conversation history, files, account settings, balance, and payments.
5. Public content engine for model/tool pages, prompts, comparisons, guides,
   and inspiration.
6. Editorial operations, search analytics, experimentation, and advanced
   NeiroHub workflows.

Framework scaffolding alone is not a deliverable. The first frontend change
must ship with one authenticated, tested vertical slice and its production
packaging.

## Route and Indexing Model

### Public, Indexable Routes

Suggested stable route families:

```text
/
/models
/models/[model-slug]
/tools
/tools/[tool-slug]
/prompts
/prompts/[prompt-slug]
/compare/[comparison-slug]
/guides/[article-slug]
/blog/[article-slug]
/pricing
/help/[article-slug]
```

Public routes must:

- return meaningful server-rendered HTML without requiring user interaction;
- return HTTP 200 only for real pages and HTTP 404 for absent or invalid pages;
- contain a single descriptive H1 and semantic heading hierarchy;
- expose important navigation through crawlable `<a href>` links;
- have stable, lowercase, human-readable URLs;
- declare an absolute self-canonical unless a different canonical is
  intentionally selected;
- include unique title, description, Open Graph, and applicable JSON-LD;
- link to relevant categories, tools, models, prompts, and guides;
- render the same substantive content to users and search crawlers.

### Authenticated, Non-Indexable Routes

Suggested route families:

```text
/app
/app/chat/[conversation-id]
/app/images
/app/videos
/app/files
/app/history
/account
/account/billing
/account/security
/payments/*
```

These routes must require a valid server-verifiable session and emit
`noindex, nofollow` defense-in-depth metadata. Authentication, not robots.txt,
is the privacy boundary. User prompts, responses, files, balances, payment
state, and artifact URLs must never appear in public HTML, metadata, sitemap,
structured data, or analytics payloads.

### Catalog Filters and Pagination

- Normal search, sort, and filter combinations are user interface state and
  are not indexable pages.
- Unneeded query-parameter combinations are excluded from crawling.
- Only curated, useful filter combinations may become dedicated static landing
  pages with unique copy and canonical URLs.
- Empty, nonsensical, duplicated, or out-of-range combinations return 404.
- Paginated content uses unique URLs, self-canonicals, and sequential
  crawlable links; infinite scroll is an enhancement, not the only discovery
  mechanism.
- UTM and other tracking parameters never create separate canonical pages.

## Search Architecture

### Metadata and Discovery

Next.js Metadata APIs generate:

- per-page title and description;
- absolute canonical URL;
- Open Graph and social preview images;
- robots directives;
- icons and web app manifest;
- `robots.txt`;
- segmented sitemap indexes for models, tools, editorial content, images, and
  video where appropriate.

Sitemaps contain only canonical, public, HTTP-200 URLs. They are submitted to
Google Search Console and Yandex Webmaster before launch.

### Structured Data

Use JSON-LD only when the visible page content supports it. Candidate types
include:

- `Organization` and `WebSite` for site identity;
- `BreadcrumbList` for hierarchical pages;
- `SoftwareApplication` or the closest supported type for genuine tools;
- `Article` for editorial pages;
- supported media markup for public image or video pages.

Structured data must pass validation and must not contain hidden reviews,
invented ratings, misleading availability, private user data, or unsupported
claims. Rich-result display is never treated as guaranteed.

### Content Quality

Every indexable model or tool page must provide original user value:

- a clear task and audience;
- supported inputs, outputs, limitations, and price explanation;
- real instructions and workflow steps;
- examples produced or reviewed by NeiroHub;
- related tools and decision guidance;
- visible update/review date;
- author or reviewer attribution where users reasonably expect it;
- sources for factual or comparative claims.

Mass-generated doorway pages, copied competitor text, trivial keyword
variations, fake freshness updates, and unreviewed AI content are prohibited.
Content is created for users first and mapped to search intent second.

### Google and Yandex Verification

Before releasing a new page template:

- inspect the rendered result in Google URL Inspection;
- verify final title, canonical, main text, links, and structured data;
- verify mobile rendering and that required CSS/JavaScript is crawlable;
- verify HTTP status and page rendering in Yandex Webmaster;
- add representative high-value URLs to important-page monitoring;
- test robots and sitemap behavior in both webmaster products.

Russian-only launch pages do not emit `hreflang`. If localized equivalents are
added later, every language variant must reference itself and all equivalents.

## Rendering and Caching

| Data or route | Rendering | Cache policy |
|---|---|---|
| Global shell and stable public copy | Static | CDN immutable assets |
| Catalog taxonomy and model/tool pages | SSG/ISR | Tagged revalidation |
| Editorial pages | SSG/ISR | Revalidate on publish |
| Public pricing explanation | SSG/ISR | Invalidate after catalog update |
| Account/profile | Dynamic server rendering | Private, `no-store` |
| Balance, payments, jobs, history | Dynamic | Private, `no-store` |
| Live job status | Authenticated polling or SSE | Never shared |
| Owner-checked artifact links | Dynamic and short-lived | Never shared |

Server Components are the default. Client Components are limited to genuinely
interactive regions such as composers, uploads, media controls, dialogs, and
live job updates. Large editors, media tooling, and history views are loaded
on demand.

Cache keys and invalidation tags must never mix account-scoped data. A write
that changes catalog content, user state, balance, payment state, or job state
invalidates the relevant cache or uses `no-store` as specified above.

## Performance Requirements

Core Web Vitals must pass at the 75th percentile, measured separately for
mobile and desktop users:

- LCP <= 2.5 seconds;
- INP <= 200 milliseconds;
- CLS <= 0.1.

Initial engineering budgets for representative public routes:

- no unnecessary client hydration for static content;
- compressed first-load JavaScript target <= 170 KB;
- no unbounded third-party scripts;
- fixed image/video dimensions to prevent layout shift;
- responsive AVIF/WebP images with correct `sizes`;
- optimized, subsetted, preferably self-hosted fonts;
- no critical console, hydration, or resource-loading errors;
- Lighthouse CI performance target >= 90 and accessibility/SEO target >= 95,
  used as regression gates rather than ranking guarantees.

Real-user Web Vitals are the release truth. Laboratory scores are diagnostic
signals. Budgets are reviewed when a measured product requirement justifies a
larger payload.

## Web Identity and API Prerequisites

The platform must not launch against VK launch-parameter authentication or
permanently reuse `/miniapp/*`.

The Go backend must provide:

- a persisted, server-verifiable opaque web session;
- hashed session/token storage;
- short-lived access and rotating refresh semantics where applicable;
- explicit expiration, revocation, logout, and active-session controls;
- account-scoped authorization on every protected request;
- a channel-neutral versioned `/web/v1` contract;
- safe DTOs for profile, identities, catalog, estimate, jobs, balance,
  payments, and artifacts;
- stable idempotency behavior for repeat-sensitive writes.

The Next.js boundary keeps authentication material in secure, `HttpOnly`,
`Secure`, appropriately scoped `SameSite` cookies. Secrets and bearer tokens
must not be stored in query strings, browser storage, client logs, analytics,
or rendered markup.

## Generation and Job UX

Every generative action becomes a durable backend Job before provider work.
The platform:

- obtains backend-calculated price and availability;
- attaches a stable client-intent idempotency key;
- submits the request to `/web/v1`;
- shows safe states such as queued, running, retrying, succeeded, failed, and
  canceled;
- resumes status tracking after refresh or reconnection;
- uses bounded polling with backoff and ETag or an authenticated SSE channel;
- retrieves artifacts only through owner-checked endpoints or short-lived
  signed URLs;
- never exposes provider task IDs, raw provider errors, internal routes, or
  private object-storage URLs.

Double clicks, retries, browser refreshes, and temporary network failures must
not create duplicate paid jobs or payments.

## Security Requirements

- Strict Content Security Policy with a minimal documented allowlist.
- HSTS, MIME sniffing protection, referrer policy, and appropriate frame
  policy.
- CSRF and Origin validation for cookie-authenticated writes.
- Same-origin browser API where practical; no broad production CORS.
- Server-side authorization for every account, artifact, job, and payment
  object.
- Rate limits and abuse controls on authentication, search, uploads,
  generation, and payments.
- File type, size, content, moderation, and malware controls before files
  become user-visible or provider-bound.
- Safe text rendering; no trusted rendering of user/provider HTML.
- PII-free structured logging and client telemetry.
- Locked dependencies, vulnerability and secret scanning, SBOM, provenance,
  and signed immutable images.

The frontend never calls AI providers, payment providers, Postgres, Redis, or
S3 directly and never mutates trusted prices, balances, ownership, moderation,
or job status.

## Reliability and Failure Behavior

The platform is a stateless independently deployable service. It provides
private liveness/readiness endpoints and graceful shutdown.

Required behaviors:

- typed normalized error responses with safe user messages;
- page and component error boundaries;
- retry only where the operation is safe or idempotent;
- timeouts and cancellation for downstream calls;
- useful loading, empty, degraded, and reconnecting states;
- catalog/public content fallback to last valid cached version when backend
  content refresh fails;
- no false success after failed generation or payment writes;
- no loss of durable jobs when the browser or Next.js process restarts.

Initial service targets:

- monthly web-platform availability >= 99.9%, excluding declared maintenance;
- synthetic checks for home, catalog, representative tool page, login, and
  authenticated API boundary;
- alerting on elevated 5xx, SSR latency, authentication failures, stale job
  progress, and public-route failures.

## Accessibility

WCAG 2.2 AA is a release target:

- semantic landmarks, headings, lists, links, buttons, and forms;
- complete keyboard navigation and visible focus;
- labelled inputs and understandable validation errors;
- sufficient contrast and non-color-only status communication;
- reduced-motion support;
- focus restoration for dialogs and route transitions;
- accessible live-region updates for asynchronous job state;
- captions or alternatives for instructional media where applicable.

## Testing and CI Gates

Every platform change runs:

- locked dependency installation;
- ESLint with zero warnings;
- strict TypeScript typecheck;
- unit and component tests;
- API schema/contract tests against `/web/v1`;
- accessibility checks;
- production build;
- Playwright tests for public discovery, login/session, generation,
  reconnection, artifact access, and payment entry;
- SEO assertions for status, metadata, canonical, robots, sitemap, links, and
  structured data;
- Lighthouse CI on representative route templates;
- Docker image build and health smoke;
- dependency, secret, and image security scans.

DEV/staging smoke precedes production rollout. Production uses immutable signed
images, post-deploy public/private smoke checks, and stateless image rollback.
Schema rollback remains a separate reviewed backup-first action.

## Observability

Collect and alert on:

- real-user LCP, INP, and CLS by route template, device, and release;
- Next.js SSR latency, 4xx/5xx, resource saturation, and cache behavior;
- Go API latency and safe error classes;
- login/session success and failure;
- job submission success, queue age, job freshness, and terminal outcomes;
- artifact access failures;
- frontend exceptions and failed resources;
- deployment revision and correlated trace/request IDs.

Monitoring must not collect prompts, generated private content, raw identity
values, auth headers, payment secrets, or private artifact URLs.

Operational search monitoring includes Search Console and Yandex Webmaster
ownership, submitted sitemaps, weekly indexing/query review, Core Web Vitals,
manual/security actions, and alerts for important-page status changes.

## Content Operations

Content is accessed through a typed server-only content interface so the
storage mechanism can change without altering route contracts.

The initial implementation uses schema-validated repository content for the
first model/tool pages and guides. Publishing requires review, preview, build
validation, and explicit release. A later editorial UI or headless CMS must use
the same validation and publishing contract; it must not expose backend or CMS
secrets to the browser.

Every published page has ownership, status, slug, title, description, body,
related entities, author/reviewer metadata where applicable, timestamps, and
explicit indexing policy.

## Deployment Boundary

`web/platform` receives its own:

- package manifest and lock file;
- Next.js configuration and source tree;
- test and accessibility configuration;
- production multi-stage Dockerfile;
- non-root, read-only runtime hardening where supported;
- compose service and private health route;
- reverse-proxy hostname/upstream and strict public route allowlist;
- CI checks and immutable release image;
- deploy, smoke, rollback, and observability integration.

The platform deploys independently from `web/miniapp` and `web/admin`. Public
metrics, debug, admin, broad billing, and internal health routes remain blocked
at the edge.

## Definition of Done

The first production vertical slice is complete only when:

1. A user can create, verify, use, refresh, and revoke a web session.
2. A public catalog and at least one model/tool page render useful indexable
   HTML with valid metadata, canonical, links, and structured data.
3. An authenticated user can submit one real generation flow through
   `/web/v1`, survive refresh, observe status, and retrieve an owner-checked
   artifact.
4. Duplicate submission is proven idempotent.
5. Private routes and data are absent from sitemaps and protected by auth plus
   `noindex`.
6. Core Web Vitals budgets are verified in lab and real-user collection is
   active.
7. Accessibility, typecheck, lint, unit, contract, E2E, SEO, build, image, and
   smoke gates pass.
8. The service has dashboards, alerts, immutable deployment, and tested
   stateless rollback.
9. Search Console and Yandex Webmaster are configured and representative pages
   pass rendered/indexing inspection.
10. No client path can call providers, storage, payments, or trusted billing
    state directly.

## Residual Risks

- Search engines never guarantee crawling, indexing, canonical selection,
  ranking, or rich-result display.
- The current web-session verifier and channel-neutral API are launch blockers
  until implemented and security-tested.
- Frequent high-quality publishing requires editorial ownership and review,
  not only technical SEO.
- Real provider and payment flows require separately approved live smoke tests.
- The current single-VPS data contour remains operationally weaker than
  managed Postgres, Redis, and S3 and requires backup/restore drills.

## Primary References

- Next.js App Router and caching:
  https://nextjs.org/docs/app
- Google technical requirements:
  https://developers.google.com/search/docs/essentials/technical
- Google JavaScript SEO:
  https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics
- Google people-first content:
  https://developers.google.com/search/docs/fundamentals/creating-helpful-content
- Google faceted navigation:
  https://developers.google.com/crawling/docs/faceted-navigation
- Core Web Vitals:
  https://web.dev/articles/vitals
- Yandex mobile recommendations:
  https://yandex.ru/support/webmaster/ru/recommendations/mobile-site
- Yandex canonical guidance:
  https://yandex.ru/support/webmaster/ru/robot-workings/canonical
- Yandex Webmaster monitoring:
  https://yandex.ru/support/webmaster/ru/service/tracking-url
