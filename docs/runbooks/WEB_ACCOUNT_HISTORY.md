# Account purchase and star history

Status: local implementation, not a verified deployment. The profile overview's
existing “Purchases and debits” section reads `GET /web/v1/billing/history`.
Normal deployment must include the ordered migration chain through 000066;
this read feature adds no schema migration and performs no balance repair.

## Ownership and transport

The cookie-authenticated principal supplies the canonical account ID. Query
parameters accept only one `limit` (1–50, default 20) and one optional `cursor`.
No account/user selector or legacy VK fallback exists. All responses are
no-store and use the existing account-bound browser transport. Pagination uses
immutable created_at/source/UUID keysets, not offsets or mutable updated_at.
The versioned opaque cursor is bound to the authenticated owner and only
selects a read position; it never establishes identity or permissions.

The response is `{items:[...],next_cursor:string|null}`. Public fields are
opaque namespaced row ID, kind, timestamp, signed current-denomination star
delta, normalized status, optional stored RUB amount in kopecks, allowlisted
plan/operation/modality and verified optional test-mode flag. Raw ledger
reasons/keys, provider payment/refund IDs, receipt contacts, prompts, Job params,
private URLs and complete metadata never leave the projection.

## Financial semantics

- Payment orders use their immutable stored RUB amount and creation date.
  Their current status may change between reads. Granted stars come only from
  that owner's matching committed payment top-up; a pending/failed order does
  not invent a credit from its promised package. The same grant is excluded
  from the separate ledger branch.
- Committed nonzero ledger movements show actual generation capture, job
  refunds, bonus grants, referral awards/reversals, expiry and adjustments.
  Reserve/release and pending/cancelled entries are excluded as final charges.
  Unknown adjustments receive a generic label. Safe generation labels use an
  owned web Job only; missing/deleted/foreign jobs retain the financial entry
  without leaking task content.
- Historical ledger denominations convert through domain.CurrentCreditAmount;
  current model/tariff catalog prices are not used to reconstruct old amounts.
- Money-return rows use stored manual refund requests and authenticated refund
  evidence. A shared provider-refund binding produces one money row. Verified
  successful proof overrides a stale manual pending/failed status and amount,
  retaining the manual row's stable ID/date. External-only proofs use an opaque
  derived ID; raw provider IDs remain private.
- A refund's actual star correction is a separate ledger fact from returned
  RUB. Failed refund compensation remains visible as its own positive star
  movement. The history does not initiate refunds or change existing principal
  refund processing. No provider API request is made while reading.
- Older orders may carry refunded status without an individual refund record
  if no durable exact refund evidence exists. Do not fabricate refund dates or
  amounts from today's prices or unverified webhook bodies.

## Client lifecycle and preview

The account-keyed component keeps financial data only in component memory.
Session abort/revision and account changes clear visible records and discard
late requests/JSON. Loading another page preserves existing rows; failed page
reads offer retry; repeated IDs/cursors cannot duplicate or loop the list.
Initial failure/empty/loading use shared state components. No persistent or
cross-account history cache exists.

Local preview shows an explicit unavailable/example notice and performs no
financial backend read. The BFF's preview GET returns an empty read-only page
without forwarding; preview writes remain blocked. Synthetic populated browser
fixtures are QA only, not real account financial evidence.

## Verification

Focused tests cover SQL ownership and related-object guards, price/grant
deduplication, denomination, all four tied keyset sources, new inserts, refunds,
referral reversal, read-only transactions, service cursor/DTO validation and
cookie handler boundaries. CI includes TestAccountHistoryPostgres. Browser
coverage is the affected profile at 360,390,768,1024,1440,1920,2560x1440 and
3840x2160 CSS pixels. Evidence lives under the ignored billing-history and
responsive-qa cache directories; local tests do not establish DEV deployment.
