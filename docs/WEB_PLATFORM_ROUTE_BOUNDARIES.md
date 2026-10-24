# Web Platform Route Boundaries

Status: active architecture contract

## Ownership

`web/platform` is one Next.js application with explicit public, authentication,
private product, and same-origin API boundaries:

```text
web/platform/src/app/(public) -> public/indexable pages; route group is absent from URLs
web/platform/src/app/app      -> authenticated product pages (internal /app)
web/platform/src/app/login    -> public authentication entry; noindex
web/platform/src/app/web/v1   -> same-origin BFF proxy; not a page surface
```

## Invariants

- Route-group folder names organize source ownership and never appear in URLs.
- All UI URLs use `/ru` or `/en`. The proxy validates the prefix and rewrites to
  the shared internal route tree, preserving client state during locale changes.
  The URL determines page language; preference cookies cannot override it.
- The public layout permits indexing; individual future public templates own
  their titles, descriptions, canonicals, and structured data.
- Implemented public pages own co-located server-only publication declarations.
  Build/start discovery statically imports these into the sitemap; metadata
  reads the same status/indexing/locale paths. Draft/noindex/scheduled and
  unimplemented editorial records never become sitemap entries. See the
  [publication contract](../web/platform/docs/locale-routing.md#публикация-новой-seo-страницы).
- The `/app` layout verifies the server session before rendering account data,
  forces dynamic rendering, disables revalidation, and emits
  `noindex, nofollow` as defense in depth.
- Authentication, not `robots.txt` or metadata, is the privacy boundary.
- Prompts, responses, files, balances, payment state, session state, and private
  artifact URLs must not enter public HTML, metadata, sitemaps, or analytics.
- The same-origin `/web/v1` proxy remains a server boundary and is never an
  indexable page family.
- Public model, tool, prompt, guide, comparison, pricing, and editorial pages
  may be added later without restructuring or redesigning the existing `/app`.

## Current URL Contract

The current external URL contract is:

```text
/en                  -> rebuilt English public home; draft/noindex, self-canonical
/ru                  -> empty home during rebuild; draft/noindex, self-canonical
/ru/login, /en/login -> authentication page; noindex
/ru/app, /en/app     -> authenticated workspace; noindex
/ru/app/*, /en/app/* -> existing private pages; noindex
/web/v1/*            -> same-origin BFF; no locale prefix
/health, /assets/*   -> unchanged technical endpoints
/robots.txt          -> public crawler rules
/sitemap.xml         -> published public locale pages only
```

The former homepage was retired on both locales. Only the English replacement
is implemented; no Russian translation is supplied. Home publication remains
draft/noindex, with no language alternates or home sitemap entries until its
content is admitted for publication. Other public and application routes keep
their existing ownership and locale behaviour.

Unknown page URLs, both inside and outside `/app`, use the `[...missing]` 404
route with the existing workspace navigation and header. Its server layout is
dynamic, uncacheable and noindex; only a successfully verified workspace session
may supply account/history data. Otherwise the same missing-page content uses
the guest workspace frame. Navigation to a real private route retains that
route's existing authentication/refresh checks. The root `not-found` fallback
never reads account data, since it can be serialized alongside valid public
pages. Missing technical assets/endpoints skip the account-aware frame, and
proxy validation of unsupported locales and localized service URLs is unchanged.

Legacy page URLs without a language temporarily redirect (307, private/no-store)
to the preference locale or default Russian, retaining the query. Unsupported
locale prefixes and prefixed technical endpoints return 404. API, auth callbacks,
payment notifications and static resources retain their technical URLs; old
payment return page URLs enter the same legacy page redirect.

Internal Next.js routes stay stable. Proxy-attested request headers initialize
server translations; a client locale boundary follows browser history. UI links
and router calls use the shared locale adapters, while route comparisons use
unprefixed paths. Public absolute URLs derive only from configured `WEB_ORIGIN`.

See [the language routing contract](../web/platform/docs/locale-routing.md) for
state preservation, auth returns, SEO, service exclusions and verification.
