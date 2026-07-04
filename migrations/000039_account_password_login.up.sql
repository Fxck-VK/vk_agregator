CREATE UNIQUE INDEX IF NOT EXISTS account_credentials_account_type_key
    ON account_credentials (account_id, credential_type);

ALTER TABLE account_links_audit
    DROP CONSTRAINT IF EXISTS account_links_audit_action_check;

ALTER TABLE account_links_audit
    ADD CONSTRAINT account_links_audit_action_check CHECK (
        action IN (
            'linked',
            'unlinked',
            'login',
            'merge_requested',
            'merge_completed',
            'password_set',
            'password_reset'
        )
    );
