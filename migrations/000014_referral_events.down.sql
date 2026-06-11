-- 000014_referral_events.down.sql

BEGIN;

DROP INDEX IF EXISTS referral_events_referral_id_idx;
DROP INDEX IF EXISTS referral_events_referrer_type_created_idx;
DROP TABLE IF EXISTS referral_events;

COMMIT;
