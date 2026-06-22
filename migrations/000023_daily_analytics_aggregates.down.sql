-- 000023_daily_analytics_aggregates.down.sql

BEGIN;

DROP TABLE IF EXISTS daily_funnel_stats;
DROP TABLE IF EXISTS daily_retention_stats;
DROP TABLE IF EXISTS daily_referral_stats;
DROP TABLE IF EXISTS daily_revenue_stats;
DROP TABLE IF EXISTS daily_provider_stats;
DROP TABLE IF EXISTS daily_generation_stats;
DROP TABLE IF EXISTS daily_user_activity;

COMMIT;
