# Subscription renewal retries: approved 14-day policy

Status: implementation authorized by the owner's 2026-10-29 reply to the daily
retry proposal, replacing seven days with fourteen and requiring document updates.

The canonical policy is version1, a24-hour minimum interval, a14-day window from
the first authenticated insufficient-funds cancellation, and at most14 total
failed attempts including the first. The window is half-open: no new submit at or
after its deadline. Delayed workers do not catch up multiple missed attempts.
Prices, periods, star grants and layouts are unchanged. No push/deploy/live-money
operation is authorized.

Only an authenticated provider GET proving canceled, unpaid/uncaptured,
insufficient_funds with exact amount/currency can schedule a new attempt. A
notification's reason or an immediate CreatePayment response is insufficient.
Other reasons, invalid/missing proof, explicit cancellation and exhausted policy
stop further attempts. Existing ambiguous-request idempotency and23-hour submit
retention remain: unknown outcomes never receive a new payment key.

New checkout consent includes the retry policy version. The immutable purchase
and private subscription state retain the accepted policy. Missing version means
legacy no-retry behavior; an old opt-in never silently gains extra attempts.
Retry attempts retain the first cycle's price/plan snapshot and canonical owner,
use one durable intent/key per verified failure number, and preserve provider
GET/reconciliation/inbox checks. Successful renewal grants once, starts its paid
period at settlement and clears recovery state. Cancellation clears retry state
under the existing account lock; late settlement cannot re-enable canceled or
exhausted renewal. Historical grants/consents remain immutable.

The account DTO exposes the actual next attempt through existing renewal_at;
no private method ID or raw provider reason enters the browser. Existing consent
text discloses the policy. User agreement and payment/refund terms state reason,
interval, window, stop conditions, unchanged price and no grants/access before
successful payment. Legal revision/date/hash advance; marketing/privacy bodies
and operator data are unchanged. Changes use the existing catalog/renderers.

Validation covers exact24-hour/deadline boundaries,14-attempt cap, success after
failure, replay, concurrency/cancellation, rollback, stale events, unknown or
hard declines, legacy consent, immutable terms, DTO/copy/legal coherence and
provider normalization. No requirement to reopen a public tunnel or repeat
already-proven hosted card entry for offline schedule tests.
