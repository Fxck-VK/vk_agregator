"""Export the embedded public policy deterministically; --check never writes."""
import argparse
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[2]
DEFAULT_SOURCE = ROOT / 'internal/service/economycatalog/policy.json'
DEFAULT_TARGET = ROOT / 'web/platform/src/config/economy.generated.json'

# Explicit schemas make this public export fail closed if private fields are
# accidentally introduced into the backend's canonical input later.
SCHEMAS = {
    'policy': {'schema_version','version','research_date','credit_denomination_version','star_nominal_kopecks','signup_bonus_stars','amount_unit','currency','renewal_retries','sources','verification','plans','models','operation_facts','gaps'},
    'renewal_retries': {'version','window_days','interval_hours','max_attempts'},
    'plan': {'code','name','amount','stars','period_days','trial','next_plan_code'},
    'model': {'id','name','category','url','source_date','pricing_source_url','catalog_source_url','minimum_stars','catalog_card_stars','price_conflict','unit','displayed_unit','free_category','mapping'},
    'mapping': {'status','product_ids','qualities','unit','reason'},
    'operation': {'id','name','minimum_stars','unit','url','source_date','mapping_status','reason'},
    'gap': {'code','reason'},
}


def validate_shape(value, schema):
    if not isinstance(value, dict) or set(value) != SCHEMAS[schema]:
        raise ValueError(f'unexpected public {schema} fields')


def export(source=DEFAULT_SOURCE, target=DEFAULT_TARGET, *, check=False):
    policy = json.loads(source.read_text(encoding='utf-8'))
    validate_shape(policy, 'policy')
    validate_shape(policy['renewal_retries'], 'renewal_retries')
    if policy['renewal_retries'] != {'version': 1, 'window_days': 14, 'interval_hours': 24, 'max_attempts': 14}:
        raise ValueError('unexpected renewal retry policy')
    for key, schema in [('plans','plan'),('models','model'),('operation_facts','operation'),('gaps','gap')]:
        for value in policy[key]:
            validate_shape(value, schema)
    for model in policy['models']:
        validate_shape(model['mapping'], 'mapping')
    expected = (json.dumps(policy, ensure_ascii=False, indent=2) + '\n').encode('utf-8')
    if check:
        if not target.exists() or target.read_bytes() != expected:
            print('Economic policy export differs; run python scripts/economy/export-web.py', file=sys.stderr)
            return 1
        return 0
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(expected)
    return 0


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    raise SystemExit(export(check=args.check))
