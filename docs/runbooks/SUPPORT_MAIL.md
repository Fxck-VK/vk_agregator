# Epifera support mail

The human support mailbox runs in a separate `epifera-mail` Compose project on
the existing VPS. Website verification/security emails continue through Resend
on `notify.epifera.com`; do not change those credentials or DNS records.

## Access and service ownership

- SSH: `deploy@2.26.91.39`; runtime: `/home/deploy/epifera-mail`.
- Source configuration: `deployments/mail/`.
- Mailbox/full login: `support@epifera.com`.
- Roundcube's default sender identity is `Epifera Support <support@epifera.com>`.
  The display name is stored in its persistent SQLite data and applies to new
  messages sent through webmail; other SMTP clients set their own From name.
- Webmail: <https://mail.epifera.com/> (Roundcube, not an account admin panel).
- IMAP: `mail.epifera.com:993`, implicit TLS.
- Authenticated SMTP: `mail.epifera.com:465`, implicit TLS, same credentials.
- SMTP 25 accepts incoming domain mail without offering AUTH or open relaying.
- Quota: 512 MiB; maximum SMTP message: 25 MiB.
- `postmaster@epifera.com` and `abuse@epifera.com` alias to support.

The owner-approved local access file is in the ignored `.cache/mail-support/`
directory with a restricted Windows ACL. Generated server credentials and
Roundcube's encryption key remain in private mode-600 files under the mode-700
runtime directory. Never print these files, pass passwords as CLI arguments,
or commit secrets, DKIM private keys, certificates or mail data.

Official Docker Mailserver 16.0.1, Roundcube 1.7.5, Nginx 1.31.3 and Certbot
5.8.0 images are pinned by digest. Rspamd/Valkey are enabled. ClamAV, Amavis and
Fail2ban are disabled to fit the current 2 GiB VPS without additional host
privileges or swap changes. The separate network is `172.29.42.0/24`; only
25/465/993/80/443 are published. Internal IMAP/SMTP verify the trusted public
certificate. There is no privileged container or Docker socket mount.

Use the empty private `.docker-public` Docker config when pulling these public
images, preserving the application's registry credentials:

```sh
cd "$HOME/epifera-mail"
docker --config "$PWD/.docker-public" compose pull
docker compose config --quiet
docker compose ps
```

This service is independent of Deploy DEV and its application Compose project.
Base server configuration is kept in `deployments/mail/`; provisioning and
mail-server configuration changes remain explicit operations. Routine UI updates
use the dedicated **Deploy Mail UI** workflow described below.

## Theme publication and rollback

The editable skin is `deployments/mail/skins/epifera/`, extending Elastic. See
`deployments/mail/README.md` for the collaborator's entry point. Packaging copies
the canonical website brand JSON, logo and favicon into a public artifact;
no mailbox data or credentials are read. Roundcube's identity display name is
user data in its existing SQLite database and is preserved during UI updates.

`.github/workflows/deploy-mail-ui.yml` runs validation on relevant pull requests
and on pushes to `dev-deploy`. Only the latter deploy, using the existing
development environment and `DEV_DEPLOY_*` SSH credentials with pinned host
keys. The exact 40-character source revision and SHA-256 artifact digest are
verified. Secret-free packages install in `ui-releases/<revision>/`; `ui-current`
points to the selected release. `compose.webmail.yaml` mounts the UI and selects
the skin. CI never provisions accounts or changes SMTP/IMAP, DNS or website data.

The deployment checks archive members and checksums before extraction, retains
the previous UI, recreates only webmail, and reloads the mail Nginx proxy to
resolve the new container address. HTTPS must render the release's stylesheet
and serve CSS/logo bytes with matching hashes. Failure restores the old UI and
reports failure. The daily certificate renewal script remains independent.

When manually recreating webmail, include the overlay:

```sh
cd "$HOME/epifera-mail"
docker compose -f compose.yaml -f compose.webmail.yaml up -d --no-deps webmail
docker compose exec -T web nginx -s reload </dev/null
```

For an intentional UI rollback, use the previous revision's already retained
archive and run the same script with its verified SHA-256 digest:

```sh
cd "$HOME/epifera-mail"
revision="$(cat ops/previous-ui-revision)"
digest="$(sha256sum "incoming/$revision.tar.gz" | awk '{print $1}')"
bash ops/deploy-mail-ui.sh "$revision" "$digest"
```

The previous-revision marker exists after a successful update from an already
versioned UI. Do not prune retained UI archives before their rollback window.

The workflow file is introduced on `dev-deploy`, so automatic push deployment
works there. GitHub manual dispatch may be unavailable until that file is also
present on the default branch; do not modify `main` just to enable dispatch.

## DNS and delivery

| Type | Name | Value |
| --- | --- | --- |
| A, DNS only | `mail.epifera.com` | `2.26.91.39` |
| MX, priority 10 | `epifera.com` | `mail.epifera.com` |
| TXT | `epifera.com` | `v=spf1 ip4:2.26.91.39 -all` |
| TXT | `mail._domainkey.epifera.com` | Generated public DKIM key |
| TXT | `_dmarc.epifera.com` | `v=DMARC1; p=none; sp=none; adkim=s; aspf=s` |
| PTR, at SERV.HOST | `2.26.91.39` | `mail.epifera.com` |

DMARC is monitoring only. A quarantine/reject policy was not approved. Selector
`mail` uses the actual generated public value from `ops/dkim-public.txt`; its
private signing key stays in `config/dms/rspamd/dkim/` on the server.

The previous apex MX records were priorities 10/20 pointing to
`mx1.beget.com`/`mx2.beget.com`, with SPF `v=spf1 redirect=beget.com`. The apex
fallback was removed to avoid routing mail to another service during outages.
Unrelated website, `www` and Resend DNS records were preserved. The manual
client settings above are authoritative; old autoconfig/autodiscover records
still refer to Beget.

PTR belongs to the VPS provider, not Cloudflare. Confirm forward/reverse
agreement and an explicitly authorized external-recipient delivery test before
claiming delivery to Gmail/Yandex verified. Local SMTP acceptance is insufficient.

## Certificates

The HTTP ACME webroot is `acme/webroot`; certificates in `acme/certs` are mounted
read-only in Nginx and Docker Mailserver. The certificate issued on 2026-10-21
expires on 2027-01-19.

Deploy's crontab runs `ops/renew-certificates.sh` daily at 03:23 in the host's
timezone (UTC at installation), appending to `ops/certificate-renewal.log`.
Other cron entries are preserved; the private pre-install backup is
`ops/crontab-before-mail.txt`. Nginx is validated/reloaded after renewal;
Docker Mailserver's change detector reloads renewed certificates automatically.

```sh
cd "$HOME/epifera-mail"
bash ops/renew-certificates.sh
docker --config "$PWD/.docker-public" compose --profile maintenance run \
  --rm --no-deps certbot renew --dry-run --no-random-sleep-on-renew \
  --webroot --webroot-path /var/www/acme
```

Preserve HTTP port 80 and its ACME exception. Do not disable TLS verification
or substitute self-signed certificates to bypass renewal failures.

## Creating another mailbox

Use an interactive SSH terminal. Omit the password argument so the CLI asks
for it privately instead of storing it in shell history/process arguments:

```sh
cd "$HOME/epifera-mail"
docker compose exec mailserver setup email add name@epifera.com
docker compose exec mailserver setup quota set name@epifera.com 512M
```

It uses the same webmail URL and full-address login. A new independently
passworded mailbox differs from an alias forwarding into an existing mailbox.
No additional domain MX/DKIM records are needed for another `@epifera.com` user.

## Installation evidence and maintenance

On 2026-10-22 authenticated Roundcube login and trusted HTTPS worked. An
authenticated SMTP self-test reached support's IMAP INBOX with a DKIM signature.
From the owner's computer, strict-TLS SMTP 465 and IMAP 993 authentication
succeeded; SMTP 25 STARTTLS accepted support (250). Unauthenticated SMTP 25 did
not advertise AUTH, rejected an unknown local recipient (550), and rejected an
external relay recipient (554), without sending to another person. Public DNS
resolved the actual mail A, apex MX, SPF and public DKIM values.

Let's Encrypt's simulated renewal succeeded (confirmed in the rotated Certbot
log), and the installed renewal script passed Nginx validation/reload. Public
DMARC resolves with the monitoring policy above. At handoff, both 1.1.1.1 and
8.8.8.8 returned NXDOMAIN for the PTR lookup; the owner requested the PTR change
from SERV.HOST support. No external-recipient message was sent.

Check `docker compose ps` and `docker compose exec mailserver setup quota list`.
Preserve mail data, mail state, Roundcube SQLite data, DKIM keys and certificates
across updates. Do not prune this project's data during website deployments.

No off-server mail backup or external inbox delivery test is configured by this
installation. Arrange encrypted off-server backup and restore testing before
using this as the sole copy of important correspondence. The VPS is shared with
DEV: monitor free RAM/disk; a VPS outage also interrupts support mail.
