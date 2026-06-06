-- 000005_user_vk_profile.up.sql
-- Cache VK profile fields used for first-start personalization.

ALTER TABLE users
    ADD COLUMN vk_first_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN vk_last_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN vk_profile_synced_at TIMESTAMPTZ,
    ADD COLUMN welcome_name_sent_at TIMESTAMPTZ;
