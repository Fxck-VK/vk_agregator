#!/usr/bin/env bash
set -euo pipefail
input=""; output=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --input) input="${2:-}"; shift 2 ;;
    --output) output="${2:-}"; shift 2 ;;
    *) echo 'Unsupported DEV legal configuration argument' >&2; exit 2 ;;
  esac
done
if [[ ! -f "${input}" || -z "${output}" || "${input}" == "${output}" ]]; then
  echo 'DEV legal configuration needs separate input and output files' >&2; exit 2
fi
umask 077
python3 - "${input}" "${output}" <<'PY'
from pathlib import Path
import os, re, sys

flag=os.environ.get('DEV_LEGAL_DOCUMENTS_PUBLISHED','false').strip() or 'false'
name=os.environ.get('DEV_LEGAL_OPERATOR_NAME','').strip()
inn=os.environ.get('DEV_LEGAL_OPERATOR_INN','').strip()
if flag not in ('true','false'):
    raise SystemExit('Invalid DEV legal publication flag')
valid_name=bool(name) and len(name.encode('utf-8'))<=256 and all(ch.isalpha() or ch in ' .-’' for ch in name)
valid_inn=re.fullmatch(r'[0-9]{12}',inn) is not None and inn!='000000000000'
if flag=='true' and not (valid_name and valid_inn):
    raise SystemExit('DEV legal publication requires complete valid operator configuration')
if name and not valid_name or inn and not valid_inn:
    raise SystemExit('Invalid DEV legal operator configuration')
keys={'LEGAL_DOCUMENTS_PUBLISHED','LEGAL_OPERATOR_NAME','LEGAL_OPERATOR_INN'}
lines=[line for line in Path(sys.argv[1]).read_text(encoding='utf-8').splitlines() if line.split('=',1)[0] not in keys]
lines.extend(['LEGAL_DOCUMENTS_PUBLISHED='+flag,'LEGAL_OPERATOR_NAME='+name,'LEGAL_OPERATOR_INN='+inn])
target=Path(sys.argv[2])
target.write_text('\n'.join(lines)+'\n',encoding='utf-8')
target.chmod(0o600)
print('DEV legal publication '+('enabled' if flag=='true' else 'disabled'))
PY
