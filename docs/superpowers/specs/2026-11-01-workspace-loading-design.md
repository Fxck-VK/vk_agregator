# Workspace loading without presentation changes

Approved scope: the owner accepted the four-part proposal on 2026-11-01:
private DEV static caching, a lighter deferred video poster, one catalogue load
with retained display state, and separate identity/catalogue/balance/chat timings.
Implementation is local; this approval does not authorize a commit or deployment.

## Boundaries

- Use the existing checkout and localhost:7159. Preserve Russian-only URLs and
  all current geometry, typography, controls, hover states and billing behavior.
- Session validation remains required before rendering private workspace data.
  Browser caches never establish identity, eligibility, price or admission.
- Do not persist tokens, profiles, balances, messages, payment data or private URLs.
- No providers, real payments, schema changes or new dependencies.

## DEV static resources

The Nginx DEV gate continues checking every request, including conditional
requests. Successful public-build assets under `/_next/static/` and `/assets/`
can use a private browser cache that must revalidate before reuse. Shared caches
remain forbidden. HTML, account/API responses, failures and access challenges
stay `private, no-store`. Preserve ETag/Last-Modified and valid 304 behavior.

## Catalogue delivery and retained presentation

After successful session verification, the layout starts the existing bounded
server catalogue read. It passes an available successful snapshot and the
pending read to the existing client provider. A slow read is streamed rather
than abandoned after 200ms. Client adapters and selectors share that read;
transport failure may fall back once to the existing authenticated browser API.

Keep the last validated non-personal catalogue visible during revalidation.
Freshness remains 60 seconds. A bounded sessionStorage snapshot may restore
display after a full refresh: at most 15 minutes old, at most 1.5 MB, schema
validated, no account-specific fields. Restore after hydration and verified
workspace mounting; preserve its original age. Clear on logout/session change.
Old requests must not repopulate cache after invalidation. Backend quotes and
authorization remain authoritative for every generation and payment.

## Poster

Retain the original neon composition and old PNG URL for compatibility. Add an
optimized WebP asset and use an opt-in viewport-deferred poster on the existing
VideoPlayer only for the workspace promotional video. Default player consumers
keep their behavior, dimensions, aspect ratio and playback controls. MP4 remains
preload=none. A visible/keyboard-operated player must still load and play.

## Measurements

Extend the existing local/DEV-only bounded workspace diagnostics. Record only
finite operation names, source, success/failure and numeric elapsed time for
identity, catalogue, balance and conversations. Measure server identity/seed
delivery and complete browser reads including decoding. No external analytics,
payloads, paths with IDs, credentials or persistent diagnostic logs.

## Acceptance

Regressions cover conditional caching under the gate, a delayed server seed with
no duplicate browser call, retained state across refresh/stale reads, corrupted
storage and invalidation races, opt-in poster loading, playback and safe metrics.
Verify affected workspace states at 360,390,768,1024,1440,1920,2560x1440 and
3840x2160 CSS pixels. Compare cold/warm local loading against the saved baseline;
distinguish preview fixtures from authenticated DEV measurements.
