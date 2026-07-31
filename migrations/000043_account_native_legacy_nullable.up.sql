ALTER TABLE jobs ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE jobs ALTER COLUMN vk_peer_id DROP NOT NULL;
ALTER TABLE artifacts ALTER COLUMN owner_user_id DROP NOT NULL;
ALTER TABLE credit_accounts ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE payment_intents ALTER COLUMN user_id DROP NOT NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM (
            SELECT owner_account_id, currency
            FROM credit_accounts
            WHERE owner_account_id IS NOT NULL
            GROUP BY owner_account_id, currency
            HAVING COUNT(*) > 1
        ) AS duplicate_owner_currency
    ) THEN
        RAISE EXCEPTION
            'cannot create credit_accounts owner_account_id/currency uniqueness index: duplicate canonical owner/currency rows exist';
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS credit_accounts_owner_account_currency_unique
    ON credit_accounts (owner_account_id, currency)
    WHERE owner_account_id IS NOT NULL;
