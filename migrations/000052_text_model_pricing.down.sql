BEGIN;
-- Refuse rollback if it would discard text pricing. Retain the additive
-- schema for runtime rollback; no automatic schema rollback is supported.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM runtime_generation_prices WHERE text_model_id <> '') THEN
  RAISE EXCEPTION 'text pricing exists; keep additive schema during rollback';
 END IF;
END $$;
ALTER TABLE runtime_generation_prices DROP CONSTRAINT runtime_generation_prices_key_unique;
ALTER TABLE runtime_generation_prices DROP CONSTRAINT runtime_generation_prices_public_key_shape;
ALTER TABLE runtime_generation_prices ADD CONSTRAINT runtime_generation_prices_key_unique UNIQUE (catalog_version_id,operation,modality,image_model_id,video_route_alias,quality,resolution,duration_sec);
ALTER TABLE runtime_generation_prices ADD CONSTRAINT runtime_generation_prices_public_key_shape CHECK (
 (operation IN ('image_generate','image_edit','image_upscale') AND modality = 'image' AND image_model_id <> '' AND video_route_alias = '' AND quality <> '' AND resolution = '' AND duration_sec = 0)
 OR (operation IN ('video_generate','video_image_to_video','video_extend') AND modality = 'video' AND image_model_id = '' AND video_route_alias <> '' AND quality = '' AND resolution <> '' AND duration_sec > 0)
);
ALTER TABLE runtime_generation_prices DROP COLUMN text_model_id;
COMMIT;
