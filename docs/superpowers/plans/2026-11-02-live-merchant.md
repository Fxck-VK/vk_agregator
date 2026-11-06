# Live merchant implementation plan

**Goal:** Prepare the existing payment pipeline for explicit production merchant
enablement while retaining test-only DEV and independent recurring consent.
**Architecture:** Shared config policy for API and renewal worker; existing
websession/payment/ledger contracts. No dependencies or migration.
**Approved design:** ../specs/2026-11-02-live-merchant-design.md.

- [x] Add failing environment/merchant/recurring configuration tests.
- [x] Implement flags, default-off parsing and validation. Wire API and renewal
  worker to the same policy; rename internal test-only handler availability.
- [x] Add early env admission/parity checks and synthetic regressions.
- [x] Prepare ignored merchant input and sanitized read-only validation tooling.
  Let owner fill credentials; never log the key or raw API response.
- [x] Run focused Go/config/payment/webhook tests, vet and affected env checks.
- [x] Independently review runtime separation and recurring behavior, fix issues.
- [x] Document exact go-live steps and distinguish local preparation from live
  configuration/deployment. Do not create real payments.

## Final local evidence

- Owner entered live credentials into the ignored input; one official read-only
  `GET /v3/me` confirmed matching shop, test=false, status=enabled, available
  payment methods. Fiscalization=false is handled by owner-confirmed NPD manual
  mode. No raw response/key/shop identifier is copied into tracked evidence.
- Nine affected Go packages passed: config, payment factory/mock/YooKassa,
  API app, websession, paymentservice, cmd/api and cmd/provider-webhook. Go vet
  passed on the same scope. Logs: `.cache/live-merchant-go-final.log` and
  `.cache/live-merchant-vet-final.log`.
- Fresh govulncheck: 0 reachable vulnerabilities in this code/imported packages;
  3 module advisories reported outside used packages/call paths. No dependencies
  were added or changed. `.cache/live-merchant-security.log`.
- Bash/PowerShell production environment regression tests, DEV environment
  regressions and env-name parity tests passed. Earlier new admission tests
  reproduced missing recurring-dependency enforcement. Review found flag parser
  and live-key-prefix mismatches; canonical tokens plus matching whitespace
  normalization and malformed/mixed-case key tests now cover them.
- Independent reviewer final recheck clear. Default fiscal receipt tests remain;
  NPD tests cover initial/recurring/refund body omission and preserved contact
  rejection before any mocked provider request.
- Existing checkout UI/prices/layout and financial schema unchanged. No build,
  UI width rerun or real-money lifecycle test was required/performed for these
  backend settings. Exact-commit release preflight/native Linux container gates
  were not run: there is no authorized release, and Docker engine is stopped.

## Operational steps still pending

- Owner initially reported the legacy DEV notification URL. A later owner
  screenshot shows `https://epifera.com/billing/webhooks/yookassa` configured with
  payment.succeeded/payment.canceled/refund.succeeded. The owner changed this
  setting; no agent merchant UI/API write occurred. Delivery remains unverified.
- Production `https://epifera.com/billing/webhooks/yookassa` HEAD and IPv4 GET
  probes timed out; browser reader also could not reach it. This establishes
  unverified reachability, not a diagnosis of the server/network cause.
- Apply production runtime inputs only through an explicitly authorized reviewed
  release of API/provider-webhook; preserve other payment settings. No GitHub
  secret/VPS environment/merchant setting was changed in this preparation.
- Once production ingress works, verify delivery using the owner's configured
  production notification URL and its three selected events.
- Confirm recurring support in YooKassa before setting its independent flag.
  Manual NPD receipt issuance remains the operator's task.
- Small live payment/refund/replay smoke is still outstanding and requires owner
  authorization for that transaction. Read-only merchant validation alone is
  not full payment integration verification.
