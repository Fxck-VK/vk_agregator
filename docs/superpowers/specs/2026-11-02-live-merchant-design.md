# Live YooKassa merchant preparation

Approved by owner: configure the activated live merchant for production. Existing
DEV/PROD preview remains a display profile and never chooses a payment merchant.
No real payment, commit, push or deployment is included in this local work.

The config layer owns one environment-bound payment policy consumed by the API
and subscription renewal process. Existing development/staging test merchants
retain their behavior. Production checkout needs explicit WEB_LIVE_PAYMENTS_ENABLED,
a live key, shop ID, safe HTTPS origin and the official YooKassa API endpoint.
WEB_LIVE_RECURRING_ENABLED is a separate default-off opt-in; it also needs live
checkout and confirmation that the merchant supports recurring payments.
Wrong-environment flags or incomplete/unsafe live inputs fail startup.

The website API advertises checkout availability from that server policy. Account
ownership, verified receipt email, CSRF, idempotency, amount verification, webhook
inbox/provider verification and the append-only ledger stay unchanged. No UI
redesign, new tariffs, generation prices or schema changes.

Owner confirmed an NPD self-employed merchant. `YOOKASSA_RECEIPT_MODE=npd_manual`
omits the 54-FZ receipt body for payments/refunds while retaining verified buyer
contact validation. NPD receipts remain manual through My Tax; no automatic
fiscalization is claimed. The default `yookassa` receipt mode is unchanged.

Merchant credentials are entered by the owner in ignored
`.runtime/payments/live-shop.env`, not in chat or tracked configuration. A bounded
read-only GET /v3/me can check credentials and merchant test/status flags, without
creating payments. Only non-sensitive booleans/statuses may be reported. The
permanent HTTPS webhook is configured after its actual reachability is verified.
Production publication remains a separate authorized release procedure.
