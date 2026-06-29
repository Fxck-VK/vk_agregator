CREATE INDEX IF NOT EXISTS payment_intents_console_user_created_id_idx
    ON payment_intents (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS payment_intents_console_status_created_id_idx
    ON payment_intents (status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS payment_intents_console_provider_payment_created_idx
    ON payment_intents (provider, provider_payment_id, created_at DESC)
    WHERE provider_payment_id IS NOT NULL AND btrim(provider_payment_id) <> '';

CREATE INDEX IF NOT EXISTS payment_events_console_provider_payment_received_idx
    ON payment_events (provider, provider_payment_id, received_at DESC)
    WHERE provider_payment_id IS NOT NULL AND btrim(provider_payment_id) <> '';

CREATE INDEX IF NOT EXISTS payment_refunds_console_intent_created_id_idx
    ON payment_refunds (intent_id, created_at DESC, id DESC);
