#!/usr/bin/env bash
# Install a public UI artifact into the existing mail project; never provision mail.
set -Eeuo pipefail
umask 077
deploy_source="$(readlink -f "${BASH_SOURCE[0]}")"

revision="${1:-}"
digest="${2:-}"
if [[ ! "$revision" =~ ^[0-9a-f]{40}$ || ! "$digest" =~ ^[0-9a-f]{64}$ ]]; then
  echo 'Expected source revision and SHA-256 artifact digest.' >&2
  exit 1
fi
mail_root="$HOME/epifera-mail"
[[ -f "$mail_root/compose.yaml" && -f "$mail_root/webmail.env" ]] || {
  echo 'Existing mail installation is required.' >&2; exit 1;
}
cd "$mail_root"
install -d -m 700 ops ui-releases incoming
[[ ! -L ui-releases && ! -L incoming ]] || exit 1
exec 9>ops/ui-deploy.lock
flock -w 60 9
archive="$mail_root/incoming/$revision.tar.gz"
printf '%s  %s\n' "$digest" "$archive" | sha256sum --check --status

# Validate the entire artifact before writing public files. No links or escapes.
python3 - "$archive" "$mail_root/ui-releases/$revision" "$revision" <<'PY'
import hashlib,json,os,re,sys,tarfile,tempfile
from pathlib import Path,PurePosixPath
archive,destination,revision=sys.argv[1:]
payload={}
with tarfile.open(archive) as bundle:
    members=bundle.getmembers()
    if len(members)>2000 or sum(m.size for m in members)>16*1024*1024:
        raise ValueError('UI artifact exceeds limits')
    for member in members:
        name=member.name
        parts=PurePosixPath(name).parts
        allowed=(name.startswith('skin/') or name in ('config/brand.json','config/epifera.inc.php','compose.webmail.yaml','release.json'))
        if not allowed or not member.isfile() or name.startswith('/') or '..' in parts or '\\' in name or name in payload:
            raise ValueError('Unsafe artifact member')
        payload[name]=bundle.extractfile(member).read()
manifest=json.loads(payload['release.json'])
if manifest['revision']!=revision or set(manifest['files'])!=set(payload)-{'release.json'}:
    raise ValueError('Release manifest mismatch')
required={'skin/meta.json','skin/styles/epifera.css','skin/templates/includes/layout.html','skin/images/brand.svg','skin/images/favicon.png','config/brand.json','config/epifera.inc.php','compose.webmail.yaml'}
if not required<=payload.keys():raise ValueError('Incomplete UI artifact')
for name,digest in manifest['files'].items():
    if not re.fullmatch('[0-9a-f]{64}',digest) or hashlib.sha256(payload[name]).hexdigest()!=digest:
        raise ValueError('Artifact file checksum mismatch')
root=Path(destination)
if root.is_symlink():raise ValueError('Release directory cannot be a symlink')
if root.exists():
    for name,content in payload.items():
        if (root/name).is_symlink() or (root/name).read_bytes()!=content:
            raise ValueError('An immutable release already exists with different contents')
else:
    with tempfile.TemporaryDirectory(prefix='.stage-',dir=root.parent) as staging:
        staged=Path(staging)
        for name,content in payload.items():
            path=staged/name
            path.parent.mkdir(parents=True,exist_ok=True,mode=0o755)
            path.write_bytes(content);path.chmod(0o644)
        for directory in staged.rglob('*'):
            if directory.is_dir():directory.chmod(0o755)
        staged.chmod(0o755)
        os.replace(staged,root)
print('Verified public UI release '+revision)
PY

previous=''
if [[ -e ui-current || -L ui-current ]]; then
  [[ -L ui-current ]] || { echo 'ui-current must be a managed symlink.' >&2; exit 1; }
  previous="$(readlink -f ui-current)"
  [[ "$previous" == "$mail_root/ui-releases/"* && -d "$previous" ]] || exit 1
fi
had_overlay=0
if [[ -f compose.webmail.yaml ]]; then
  cp -p compose.webmail.yaml ops/ui-overlay-before.yaml
  had_overlay=1
fi
activated=0
rollback() {
  status=$?
  trap - ERR
  set +e
  if (( activated )); then
    rollback_ok=1
    if [[ -n "$previous" ]]; then
      ln -s "$previous" ".ui-rollback-$revision"
      mv -Tf ".ui-rollback-$revision" ui-current
    else
      rm -f ui-current
    fi
    if (( had_overlay )); then
      cp ops/ui-overlay-before.yaml compose.webmail.yaml
      docker compose -f compose.yaml -f compose.webmail.yaml up -d --no-deps --force-recreate webmail </dev/null || rollback_ok=0
    else
      rm -f compose.webmail.yaml
      docker compose -f compose.yaml up -d --no-deps --force-recreate webmail </dev/null || rollback_ok=0
    fi
    docker compose exec -T web nginx -s reload </dev/null || rollback_ok=0
    rollback_ready=0
    for ((check=1; check<=15; check++)); do
      if curl --fail --silent --max-time 5 https://mail.epifera.com/ >/dev/null; then rollback_ready=1; break; fi
      sleep 2
    done
    if (( rollback_ok && rollback_ready )); then
      echo 'Previous mail UI restored; deployment failed.' >&2
    else
      echo 'Mail UI rollback could not be verified; inspect webmail and Nginx.' >&2
    fi
  fi
  exit "$status"
}
trap rollback ERR
activated=1
ln -s "$mail_root/ui-releases/$revision" ".ui-next-$revision"
mv -Tf ".ui-next-$revision" ui-current
cp "ui-current/compose.webmail.yaml" compose.webmail.yaml
docker compose -f compose.yaml -f compose.webmail.yaml config --quiet
docker compose exec -T webmail php -l <ui-current/config/epifera.inc.php
docker compose -f compose.yaml -f compose.webmail.yaml up -d --no-deps --force-recreate webmail </dev/null
docker compose exec -T web nginx -t </dev/null
docker compose exec -T web nginx -s reload </dev/null

expected_css="$(sha256sum ui-current/skin/styles/epifera.css | awk '{print $1}')"
expected_logo="$(sha256sum ui-current/skin/images/brand.svg | awk '{print $1}')"
smoke() {
  local html css_hash logo_hash
  html="$(curl --fail --silent --show-error --max-time 8 https://mail.epifera.com/)" || return 1
  [[ "$html" == *"skins/epifera/styles/epifera.css?v=$revision"* ]] || return 1
  css_hash="$(curl --fail --silent --show-error --max-time 8 "https://mail.epifera.com/static.php/skins/epifera/styles/epifera.css?v=$revision" | sha256sum | awk '{print $1}')" || return 1
  logo_hash="$(curl --fail --silent --show-error --max-time 8 "https://mail.epifera.com/static.php/skins/epifera/images/brand.svg?v=$revision" | sha256sum | awk '{print $1}')" || return 1
  [[ "$css_hash" == "$expected_css" && "$logo_hash" == "$expected_logo" ]]
}
attempts="${EPIFERA_MAIL_SMOKE_ATTEMPTS:-30}"
[[ "$attempts" =~ ^[1-9][0-9]?$ ]]
passed=0
for ((attempt=1; attempt<=attempts; attempt++)); do
  if smoke; then passed=1; break; fi
  sleep 2
done
[[ "$passed" == 1 ]]
trap - ERR
if [[ -n "$previous" && "$previous" != "$mail_root/ui-releases/$revision" ]]; then
  printf '%s\n' "${previous##*/}" >ops/previous-ui-revision
fi
if [[ -f "$deploy_source" && "$deploy_source" != "$mail_root/ops/deploy-mail-ui.sh" ]]; then
  install -m 700 "$deploy_source" ops/deploy-mail-ui.sh
fi
printf 'Mail UI deployed and verified: %s\n' "$revision"
