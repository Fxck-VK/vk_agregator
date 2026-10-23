# Final legal edit and DEV publication plan

Status: in progress. The user explicitly requested final proofreading and website
publication after selecting Russian hosting as the launch architecture.

**Goal:** publish the reviewed legal revision and its existing consent integration
on the website's DEV contour, preserving unrelated local development.

**Scope:** four core documents plus optional marketing consent, effective date,
runtime requisites, migration 000062, reproducible DEV configuration and smoke.
The schema release includes prepared additive migrations 000059-000060 to keep
the existing migration sequence complete, the already-applied retention policy
000061 and legal evidence 000062. The first adds a lookup index; the second adds
guards against reuse/resurrection of deleted artifacts. Neither deletes files or
publishes its unfinished UI. No main deployment, pricing activation, recurring charge, provider call,
server relocation or assertion that a planned relocation has already happened.

- [ ] Proofread all five texts, remove internal notes/inactive proposed tariff
      annex, record effective date and preserve actual payment/refund behavior.
- [ ] Verify source checksum/parity and explicit publication rendering; prepare
      fail-closed DEV runtime configuration without putting requisites in Git.
- [ ] Prepare a scoped release from the current DEV head, preserving all unrelated
      working files; verify locally and through exact-commit CI/build.
- [ ] Publish to DEV with immutable images and normal migrations; verify document
      responses, revision/flag parity, legal API protection and server health.

Acceptance has already been authorized in this conversation as a product design;
this task does not accept an agreement on behalf of a real user. Unknown external
legal/infrastructure facts stay in the private readiness report, not as invented
public assurances. Existing protected identity, CSRF, job and payment boundaries
remain mandatory. No optional design or publication reapproval is requested.
