CREATE TABLE web_referral_payment_evidence (
 intent_id UUID PRIMARY KEY REFERENCES payment_intents(id),
 account_id UUID NOT NULL REFERENCES accounts(id),
 provider TEXT NOT NULL CHECK(provider='yookassa'),
 provider_payment_id TEXT NOT NULL,
 amount BIGINT NOT NULL CHECK(amount>0),
 currency TEXT NOT NULL CHECK(currency='rub'),
 mode TEXT NOT NULL CHECK(mode IN ('live','test','unknown','historical')),
 verified_at TIMESTAMPTZ NOT NULL,
 UNIQUE(provider,provider_payment_id)
);
CREATE INDEX web_referral_payment_first_idx ON web_referral_payment_evidence(account_id,verified_at,intent_id) WHERE mode<>'test';

CREATE TABLE web_referral_refund_evidence (
 provider TEXT NOT NULL CHECK(provider='yookassa'),
 provider_refund_id TEXT NOT NULL CHECK(btrim(provider_refund_id)<>''),
 intent_id UUID NOT NULL REFERENCES payment_intents(id),
 amount BIGINT NOT NULL CHECK(amount>0),
 verified_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(provider,provider_refund_id)
);
CREATE INDEX web_referral_refund_intent_idx ON web_referral_refund_evidence(intent_id);

CREATE TABLE web_referral_rewards (
 referral_id UUID PRIMARY KEY REFERENCES referrals(id),
 referred_account_id UUID NOT NULL UNIQUE REFERENCES accounts(id),
 referrer_account_id UUID NOT NULL REFERENCES accounts(id),
 intent_id UUID NOT NULL UNIQUE REFERENCES web_referral_payment_evidence(intent_id),
 policy_version INTEGER NOT NULL CHECK(policy_version>0),
 stars BIGINT NOT NULL CHECK(stars>0),
 qualified_at TIMESTAMPTZ NOT NULL,
 due_at TIMESTAMPTZ NOT NULL,
 status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','applied','cancelled','reversed')),
 grant_entry_id UUID UNIQUE REFERENCES ledger_entries(id),
 reversal_entry_id UUID UNIQUE REFERENCES ledger_entries(id),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(referred_account_id<>referrer_account_id),
 CHECK(due_at>=qualified_at),
 CHECK((status IN ('applied','reversed'))=(grant_entry_id IS NOT NULL)),
 CHECK((status='reversed')=(reversal_entry_id IS NOT NULL))
);
CREATE INDEX web_referral_rewards_due_idx ON web_referral_rewards(due_at,referral_id) WHERE status='pending';

-- Past paid intents cannot earn a second first-payment reward. Their immutable
-- settled ledger timestamp is retained, without inventing live/test proof or
-- retroactively granting rewards. No mutable intent.updated_at is used.
INSERT INTO web_referral_payment_evidence(intent_id,account_id,provider,provider_payment_id,amount,currency,mode,verified_at)
SELECT p.id,p.account_id,p.provider,p.provider_payment_id,p.amount,p.currency,'historical',l.created_at
FROM payment_intents p JOIN ledger_entries l
 ON l.idempotency_key='topup:'||p.provider||':'||p.provider_payment_id
 AND l.owner_account_id=p.account_id AND l.status='committed' AND l.amount>0
WHERE p.provider='yookassa' AND p.account_id IS NOT NULL
 AND p.status IN ('succeeded','refunded','partially_refunded')
ON CONFLICT DO NOTHING;
