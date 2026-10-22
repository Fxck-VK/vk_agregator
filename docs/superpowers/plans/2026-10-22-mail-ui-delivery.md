# Mail UI delivery implementation plan

> For agentic workers: execute the already approved scope inline using the
> executing-plans workflow. Preserve the one active checkout and local server.

**Goal:** Let the collaborator edit the mail client in the shared repository and
publish the exact dev-deploy revision to the existing mailbox server.

**Architecture:** A secret-free UI artifact and versioned server release extend
Roundcube Elastic through a Compose overlay. Only webmail is recreated, with
checks and rollback; canonical brand assets are copied during packaging.

**Tech stack:** Python standard library, Bash, Docker Compose, Roundcube, GitHub
Actions and existing pinned SSH credentials.

## Global constraints

- Work only in epifera-domain-cutover, preserving other local changes.
- No secret, message, key, certificate or SQLite exports into Git.
- Do not modify main, SMTP/IMAP containers, website release or DNS during UI deploy.
- Keep TLS verification, pinned host keys and existing DEV environment protection.

## Task 1: Public package and editable skin

- [ ] Add packaging regression tests that build an artifact in a temporary
  directory, verify canonical branding and reject symlink/path escapes.
- [ ] Run `python -m unittest discover -s scripts/mail -p 'test_*.py'`; confirm
  failure for the missing packager, then implement the packager and theme.
- [ ] Test packaging without credentials or runtime files. Validate PHP and the
  rendered theme against the installed official Roundcube image.

## Task 2: Delivery and rollback

- [ ] Add a tested Compose overlay and deployment script accepting a 40-character
  source revision plus a SHA-256 artifact digest. Validate before extraction.
- [ ] Preserve the previous release; update the UI symlink and only webmail.
  Check HTTPS and rendered assets; restore the prior overlay/release on error.
- [ ] Add push/PR workflow with validation before secret-bearing deployment,
  contents-read permission, pinned checkout and non-cancelling deployment lock.
- [ ] Exclude mail-only paths from unrelated application image builds.
- [ ] Validate shell/YAML and deploy a local package using the existing SSH key.

## Task 3: Publish and verify

- [ ] Update the mail runbook, documentation routing and active context.
- [ ] Review only the staged mail/config/workflow diff, run focused checks and
  secret-scope checks, commit, and push to dev-deploy without force.
- [ ] Verify automatic Deploy Mail UI success for the exact commit and inspect
  the live rendered theme. Publish only to the authorized dev-deploy branch.
- [ ] Provide commit, editable theme path and deployment status to the owner.
