# Mail UI in the shared repository

Approved scope: the owner accepted committing the existing mail configuration,
an editable Roundcube theme and automatic theme delivery to dev-deploy so a
collaborator can continue styling in the single project.

The theme extends the installed Elastic skin. Its templates and CSS live in
deployments/mail/skins/epifera. Packaging copies the canonical website brand JSON,
logo and favicon into a public, secret-free archive. No runtime mail, passwords,
SQLite databases, DKIM private keys or TLS certificates enter the repository.

A dedicated GitHub Actions workflow validates and packages the exact source
revision. Pushes on dev-deploy deploy through existing DEV SSH secrets and pinned
host keys. Pull requests only validate. A separate Compose overlay mounts the
versioned UI release and selects the Epifera skin. Deployment recreates only
Roundcube, checks HTTPS and the actual rendered stylesheet/logo, and restores
the previous UI release on failure. It never initializes accounts or modifies
mailbox storage, SMTP/IMAP services, website data or DNS.

Mail-only paths do not trigger application image builds. Public configuration
for the original server setup remains documented separately from routine theme
updates. The runbook describes editable files, publication and rollback. Existing
unrelated local changes are preserved and excluded from this commit. Main is
unchanged; only the explicitly authorized dev-deploy branch receives the tested
mail work. Other branches require a separately authorized integration.
