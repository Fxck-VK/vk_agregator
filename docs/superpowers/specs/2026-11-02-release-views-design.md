# DEV / PROD release views

Owner approved the six-part proposal on2026-11-02. Implement in the single active
checkout/localhost7159, preserving all pending loading improvements. No release.

One versioned registry `web/platform/release/features.json` owns readiness and
first-release inclusion. Status ready describes implemented functionality, not
live PROD verification; each row explicitly records remaining acceptance and
verification wording without invented dates. Existing model/provider admission,
pricing, billing and authorization remain authoritative. A release flag can
restrict these, never grant capability. Public DTOs contain no routing secrets.

The existing development panel gains DEV/PROD controls and readiness rows with
links, current work, remaining work, inclusion and verification. Shared global
profile context covers public/workspace UI. Local simulation remains local;
protected DEV production builds expose only release controls, no simulator.

Profile is a server-owned preference cookie. Only loopback local preview or
the existing verified DEV gateway may read/change a DEV view. A same-origin POST
with bounded validated JSON changes only that cookie, then refreshes the tree.
Production/unknown deployments always resolve PROD and offer no switch; forged
cookies/headers/query/localStorage never expand production. Do not switch data,
credentials, merchant, jobs, balances or providers.

Server page guards block excluded features and unpublished public content, with
legal documents preserved by their existing independent publication rules.
All24 SEO records currently draft: PROD preview will expose that unfinished
release state honestly, while DEV retains the existing original/derived pages.
Blocked links/promises are removed from navigations and controls without CSS
redesign. The developer launcher must remain available on blocked pages to
return to DEV. Authentication/account/history/downloads remain available.

Next forwards a sanitized restrictive view to Go. Trusted Go environment always
forces PROD outside explicit development. Apply one predicate to public and
legacy model catalogues, paid message/image/video preparations and stored-job
activation/retry. DEV-smoke/pending candidates and excluded audio operations
are unavailable in PROD view. Never mutate a process-global admission registry
per request; never hide existing owned results merely for changing profile.

Caches are keyed/invalidated by profile as well as account. Delayed DEV catalogue
results must not repopulate a PROD view, including server/cache/session snapshots.
Switching back cannot revive an expired or unauthenticated account.

Validation: RED/GREEN pure policy, same-origin/dev-gate preference endpoints,
direct pages/API, forged production overrides, catalogue/activation parity,
late catalogue races, UI filtering/inventory. Eight required viewport widths,
production build/packaging and current security checks. No paid calls or live
merchant/deployment mutation. Document remaining real-server verification.
