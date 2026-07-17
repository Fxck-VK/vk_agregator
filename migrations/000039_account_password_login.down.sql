ALTER TABLE account_links_audit
    DROP CONSTRAINT IF EXISTS account_links_audit_action_check;

ALTER TABLE account_links_audit
    ADD CONSTRAINT account_links_audit_action_check CHECK (
        action IN ('linked', 'unlinked', 'login', 'merge_requested', 'merge_completed')
    );

DROP INDEX IF EXISTS account_credentials_account_type_key;
