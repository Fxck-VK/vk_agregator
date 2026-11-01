CREATE TABLE subscription_promotions (
 id uuid PRIMARY KEY,
 code text NOT NULL UNIQUE CHECK(code ~ '^[A-Z0-9_-]{3,64}$'),
 revision bigint NOT NULL CHECK(revision>0),
 config jsonb NOT NULL CHECK(jsonb_typeof(config)='object'),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE subscription_promotion_uses (
 intent_id uuid PRIMARY KEY REFERENCES payment_intents(id) DEFERRABLE INITIALLY DEFERRED,
 promotion_id uuid NOT NULL REFERENCES subscription_promotions(id),
 account_id uuid NOT NULL REFERENCES accounts(id),
 state text NOT NULL CHECK(state IN ('reserved','consumed','released')),
 snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX subscription_promotion_account_code ON subscription_promotion_uses(promotion_id,account_id) WHERE state IN ('reserved','consumed');
CREATE UNIQUE INDEX subscription_promotion_pending_account ON subscription_promotion_uses(account_id) WHERE state='reserved';
CREATE INDEX subscription_promotion_capacity ON subscription_promotion_uses(promotion_id,state);
CREATE TABLE subscription_promotion_mutations (
 actor_ref text NOT NULL,
 request_key uuid NOT NULL,
 fingerprint text NOT NULL CHECK(length(fingerprint)=64),
 promotion_id uuid NOT NULL REFERENCES subscription_promotions(id),
 before_snapshot jsonb,
 after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(actor_ref,request_key)
);
COMMENT ON TABLE subscription_promotion_mutations IS 'Durable safe operator configuration audit and idempotent response. Never stores credentials, customer PII or provider payloads.';

-- All payment writers, including rolling old binaries, serialize with the
-- account-first promo reservation gate before inserting payable intents.
CREATE FUNCTION guard_subscription_promotion_payment() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE owner_id uuid;
BEGIN
 owner_id := COALESCE(NEW.account_id,(SELECT account_id FROM users WHERE id=NEW.user_id));
 IF owner_id IS NULL THEN RETURN NEW; END IF;
 PERFORM id FROM accounts WHERE id=owner_id FOR UPDATE;
 IF EXISTS(SELECT 1 FROM subscription_promotion_uses WHERE account_id=owner_id AND state='reserved' AND intent_id<>NEW.id)
 THEN RAISE EXCEPTION 'pending first purchase' USING ERRCODE='23505'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_subscription_promotion_payment BEFORE INSERT ON payment_intents FOR EACH ROW EXECUTE FUNCTION guard_subscription_promotion_payment();
