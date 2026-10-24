# Registration legal notice

Status: completed locally; not committed, pushed or deployed. Supersedes 2026-10-24-legal-action-consent.md.

Goal: match the observed Magnific pattern: small linked notice on registration,
no legal wall or first-action dialog, no notice on ordinary login/recovery.

Architecture: remove the workspace boundary, mutation preflight and API legal
precondition. Keep account authentication, Origin/CSRF, billing and idempotency.
Record the displayed exact revision only through registration, using the existing
account-owned immutable consent repository. Never backfill existing accounts.
Preserve separate voluntary marketing preferences in profile. No automatic grant.

Work in the single active checkout, use localhost:7159. No commit/push/deploy.

- [x] Regression tests: login/registration placement, no action precondition,
      registration exact revision, proof ownership, retry and cookie failure.
- [x] Remove rejected action dialog/transport plumbing and backend gate.
- [x] Registration notice, registration entry, exact-revision evidence.
- [x] Update active UI/architecture/runbook context, mark old plan superseded.
- [x] Focused tests, typecheck/lint/build/packaging/docs and browser desktop/mobile.

Verification: 166 tests in 15 frontend files passed. Backend websession,
legalacceptance, accountoauth, accountregistration, shared legal catalogue and
API bootstrap tests passed. Go vet, frontend typecheck/lint, production build,
packaging, documentation validation and diff whitespace checks passed.
Browser: 390x844 and 1280x800 signup; return to ordinary login removes the notice;
workspace music opens without a legal gate. Real accounts, paid generation and
marketing subscriptions were not created for browser verification.
Evidence: ignored .cache/registration-* logs and screenshots.

Subsequent authorization (2026-10-24): user requested pushing all current changes
to dev-deploy together. Release evidence is kept in ignored
.cache/release-2026-10-24-all/status.json; do not infer deployment from this plan.
