CREATE TABLE account_subscriptions (
 owner_account_id uuid PRIMARY KEY REFERENCES accounts(id),
 revision bigint NOT NULL DEFAULT 0 CHECK(revision>=0),
 state jsonb NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(state)='object'),
 due_at timestamptz,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX account_subscriptions_due ON account_subscriptions(due_at) WHERE due_at IS NOT NULL;
COMMENT ON COLUMN account_subscriptions.state IS 'Private subscription/consent and provider-method snapshots; never expose as public JSON.';
