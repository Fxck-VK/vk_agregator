-- Roll back only when no account-native rows require nullable legacy IDs.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM jobs WHERE user_id IS NULL OR vk_peer_id IS NULL)
       OR EXISTS (SELECT 1 FROM artifacts WHERE owner_user_id IS NULL)
       OR EXISTS (SELECT 1 FROM credit_accounts WHERE user_id IS NULL)
       OR EXISTS (SELECT 1 FROM payment_intents WHERE user_id IS NULL) THEN
        RAISE EXCEPTION
            'cannot roll back 000043: account-native rows require nullable legacy columns';
    END IF;
END $$;

ALTER TABLE jobs
    ALTER COLUMN user_id SET NOT NULL,
    ALTER COLUMN vk_peer_id SET NOT NULL;

ALTER TABLE artifacts
    ALTER COLUMN owner_user_id SET NOT NULL;

ALTER TABLE credit_accounts
    ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE payment_intents
    ALTER COLUMN user_id SET NOT NULL;

DROP INDEX IF EXISTS credit_accounts_owner_account_currency_unique;
