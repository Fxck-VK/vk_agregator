#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
task_dir="$(mktemp -d)"
trap 'rm -rf -- "${task_dir}"' EXIT
printf 'WEB_ORIGIN=https://dev-web.example.test\nUNRELATED_FLAG=preserved\n' > "${task_dir}/input"
run_prepare() { bash "${script_dir}/prepare-dev-legal.sh" --input "${task_dir}/input" --output "${task_dir}/output"; }

DEV_LEGAL_DOCUMENTS_PUBLISHED=false run_prepare > "${task_dir}/log"
grep -qx 'UNRELATED_FLAG=preserved' "${task_dir}/output"
grep -qx 'LEGAL_DOCUMENTS_PUBLISHED=false' "${task_dir}/output"

DEV_LEGAL_DOCUMENTS_PUBLISHED=true DEV_LEGAL_OPERATOR_NAME='Synthetic Test Operator' DEV_LEGAL_OPERATOR_INN=123456789012 run_prepare > "${task_dir}/log"
grep -qx 'LEGAL_DOCUMENTS_PUBLISHED=true' "${task_dir}/output"
grep -q '^LEGAL_OPERATOR_NAME=' "${task_dir}/output"
if grep -qE 'Synthetic|123456789012' "${task_dir}/log"; then echo 'Requisites leaked to logs' >&2; exit 1; fi
if DEV_LEGAL_DOCUMENTS_PUBLISHED=true DEV_LEGAL_OPERATOR_NAME='' DEV_LEGAL_OPERATOR_INN='' run_prepare > /dev/null 2>&1; then echo 'Missing requisites did not block publication' >&2; exit 1; fi
if DEV_LEGAL_DOCUMENTS_PUBLISHED=true DEV_LEGAL_OPERATOR_NAME=$'Synthetic\nINJECTED=true' DEV_LEGAL_OPERATOR_INN=123456789012 run_prepare > /dev/null 2>&1; then echo 'Multiline name accepted' >&2; exit 1; fi
if DEV_LEGAL_DOCUMENTS_PUBLISHED=invalid run_prepare > /dev/null 2>&1; then echo 'Invalid publication flag accepted' >&2; exit 1; fi
echo 'DEV legal configuration tests passed'
