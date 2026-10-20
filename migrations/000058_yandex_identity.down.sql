-- Never remove identities or historical audit records to permit a downgrade.
BEGIN;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM account_identities WHERE provider = 'yandex')
       OR EXISTS (SELECT 1 FROM account_links_audit WHERE provider = 'yandex') THEN
        RAISE EXCEPTION 'Cannot downgrade Yandex identity schema while Yandex bindings or audit records exist; retain the additive schema';
    END IF;
END $$;
ALTER TABLE account_identities DROP CONSTRAINT account_identities_provider_check;
ALTER TABLE account_identities ADD CONSTRAINT account_identities_provider_check CHECK (
    provider IN ('vk', 'telegram', 'google', 'apple', 'email', 'phone', 'password')
);
ALTER TABLE account_links_audit DROP CONSTRAINT account_links_audit_provider_check;
ALTER TABLE account_links_audit ADD CONSTRAINT account_links_audit_provider_check CHECK (
    provider = '' OR provider IN ('vk', 'telegram', 'google', 'apple', 'email', 'phone', 'password')
);
COMMIT;
