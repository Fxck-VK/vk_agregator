-- Add Yandex login bindings and audit records without changing ownership.
BEGIN;
ALTER TABLE account_identities DROP CONSTRAINT account_identities_provider_check;
ALTER TABLE account_identities ADD CONSTRAINT account_identities_provider_check CHECK (
    provider IN ('vk', 'telegram', 'google', 'apple', 'yandex', 'email', 'phone', 'password')
);
ALTER TABLE account_links_audit DROP CONSTRAINT account_links_audit_provider_check;
ALTER TABLE account_links_audit ADD CONSTRAINT account_links_audit_provider_check CHECK (
    provider = '' OR provider IN ('vk', 'telegram', 'google', 'apple', 'yandex', 'email', 'phone', 'password')
);
COMMIT;
