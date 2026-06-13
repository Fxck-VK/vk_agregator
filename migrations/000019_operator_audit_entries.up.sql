-- 000019_operator_audit_entries.up.sql
-- Sanitized protected-operator audit log. Rows contain bounded refs only:
-- no raw tokens, request bodies, prompts, PII, URLs or provider/payment payloads.

BEGIN;

CREATE TABLE IF NOT EXISTS operator_audit_entries (
    id UUID PRIMARY KEY,
    actor_ref TEXT NOT NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_ref TEXT NOT NULL DEFAULT '',
    result TEXT NOT NULL,
    request_ref TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT operator_audit_entries_action_check CHECK (
        action ~ '^[a-zA-Z0-9_.:-]{1,128}$'
    ),
    CONSTRAINT operator_audit_entries_target_type_check CHECK (
        target_type ~ '^[a-zA-Z0-9_.:-]{1,64}$'
    ),
    CONSTRAINT operator_audit_entries_result_check CHECK (
        result IN ('success', 'error')
    )
);

CREATE INDEX IF NOT EXISTS operator_audit_entries_created_idx
    ON operator_audit_entries (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS operator_audit_entries_filter_idx
    ON operator_audit_entries (target_type, action, result, created_at DESC);

COMMIT;
