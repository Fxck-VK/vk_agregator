-- Public origin-model metadata is read from durable Jobs, not paginated messages.
-- This bounds the lookup to one account/conversation without scanning job history.
CREATE INDEX IF NOT EXISTS jobs_web_conversation_first_model_idx
    ON jobs (account_id, (params->>'conversation_id'), created_at, id)
    WHERE source = 'web' AND params->>'conversation_source' = 'web'
      AND status NOT IN ('prepared', 'validated', 'awaiting_payment');
