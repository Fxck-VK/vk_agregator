# Website invitations

The website now uses canonical `account_id` referral codes and relations. Migration 000056 allows account-owned rows without synthetic legacy VK users and adds anonymous invitation visits. Existing account codes are reused across surfaces.

`GET /web/v1/referrals` returns the authenticated account's code and cumulative website funnel counters. Visits count saved browser invitation bindings; registrations include later activated/rewarded website relations. Legacy VK relations are excluded from these website counters. It exposes no invited-user identities or payment information.

`/{locale}/invite/{code}` records an invitation through same-origin `POST /web/v1/referrals/visit`. The first invitation stays in a Secure, HttpOnly, SameSite=Lax cookie for 30 days. Only its SHA-256 hash enters PostgreSQL. Resetting cookies or using another browser can create another visit; visits are not unique people. Transient per-cookie limits run before the aggregate anonymous write limit, without storing IP addresses. Resetting cookies can evade the per-cookie limit; the shared global cap still bounds writes.

`POST /web/v1/referrals/accept` requires the cookie session and CSRF. It derives the invited account from the server principal. A transaction locks the intent and account, checks expiry, account creation time and self-invitation, then inserts at most one referral per invited account. An old account cannot become a new referral. Replays for the same account are safe; a different referrer cannot replace the first relation.

Email registration and OAuth login callbacks attempt acceptance after issuing the session. A storage outage leaves the invitation cookie intact and does not turn a successful sign-in into an error. Workspace initialization retries acceptance at most three times; another page load retries again. Terminal ineligibility clears the cookie.

The maintenance worker deletes expired visit token hashes in batches of at most 1000. The same transaction adds those visits to a per-referrer aggregate, so cumulative statistics remain stable. Accepted referral relations remain intact. Cleanup must run through a transaction-capable PostgreSQL pool; a non-transactional maintenance querier fails closed. Expired hashes can remain until the next maintenance sweep or while a backlog is being drained.

Website activation qualification and rewards are **disabled** until their rules are chosen. No website referral credit grant is wired to billing. Existing VK reward configuration remains separate; VK activation ignores `source=web` and relations without complete legacy user ownership. Account-native codes and relationships stay usable for attribution without sending nil owners to legacy billing. The UI shows the pending rules explicitly, without guessed bonus amounts.

Google adapters and stored bindings remain, but `ACCOUNT_OAUTH_GOOGLE_ENABLED` defaults to `false`. Stored credentials alone do not enable it. Google login/link buttons are hidden, and a disabled Google binding does not permit removal of the last working login. Profile/menu labels prefer the verified primary email. DEV rollout checks Google is disabled even if its credential overlay exists.

Before rollout, apply the normal ordered migrations and deploy API and website together. Migration rollback refuses to discard collected web invitation data; retain migration 000056 when rolling code back. Local preview performs no real referral requests and shows no invented counts.
