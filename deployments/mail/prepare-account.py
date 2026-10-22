"""Run once on the Ubuntu VPS as deploy; never prints mailbox credentials."""

import crypt
import json
import secrets
from pathlib import Path


root = Path.home() / "epifera-mail"
root.mkdir(mode=0o700, exist_ok=True)
root.chmod(0o700)
for relative in (
    "ops", "config/dms", "config/nginx", "config/roundcube",
    "data/mail", "data/state", "data/logs", "data/roundcube",
    "acme/certs", "acme/logs", "acme/webroot/.well-known/acme-challenge",
):
    (root / relative).mkdir(mode=0o700, parents=True, exist_ok=True)

credentials_file = root / "ops/credentials.json"
if credentials_file.exists():
    credentials = json.loads(credentials_file.read_text())
else:
    credentials = {
        "mailbox": "support@epifera.com",
        "password": secrets.token_urlsafe(24),
        "roundcube_key": secrets.token_urlsafe(18),
    }
    credentials_file.write_text(json.dumps(credentials))
credentials_file.chmod(0o600)

account_file = root / "config/dms/postfix-accounts.cf"
if not account_file.exists():
    password_hash = crypt.crypt(
        credentials["password"], crypt.mksalt(crypt.METHOD_SHA512, rounds=100000)
    )
    if not password_hash or not password_hash.startswith("$6$"):
        raise RuntimeError("SHA512-CRYPT password hashing failed")
    account_file.write_text(
        credentials["mailbox"] + "|{SHA512-CRYPT}" + password_hash + "\n"
    )
account_file.chmod(0o600)

webmail_env = root / "webmail.env"
if not webmail_env.exists():
    webmail_env.write_text(
        "EPIFERA_ROUNDCUBE_DES_KEY=" + credentials["roundcube_key"] + "\n"
    )
webmail_env.chmod(0o600)

aliases = root / "config/dms/postfix-virtual.cf"
if not aliases.exists():
    aliases.write_text(
        "postmaster@epifera.com support@epifera.com\n"
        "abuse@epifera.com support@epifera.com\n"
    )
aliases.chmod(0o600)

(root / "acme/webroot/.well-known/acme-challenge/epifera-preflight").write_text(
    "epifera-mail-ready\n"
)
for relative in (
    "config/roundcube", "config/nginx", "acme/webroot",
    "acme/webroot/.well-known", "acme/webroot/.well-known/acme-challenge",
):
    (root / relative).chmod(0o755)
(root / "acme/webroot/.well-known/acme-challenge/epifera-preflight").chmod(0o644)
print("Private mailbox configuration prepared; credentials were not printed.")
