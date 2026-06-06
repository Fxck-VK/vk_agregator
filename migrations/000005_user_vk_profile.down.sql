-- 000005_user_vk_profile.down.sql

ALTER TABLE users
    DROP COLUMN IF EXISTS welcome_name_sent_at,
    DROP COLUMN IF EXISTS vk_profile_synced_at,
    DROP COLUMN IF EXISTS vk_last_name,
    DROP COLUMN IF EXISTS vk_first_name;
