BEGIN;
ALTER TABLE runtime_generation_prices ADD COLUMN text_model_id TEXT NOT NULL DEFAULT '' CHECK (length(text_model_id) <= 128);
ALTER TABLE runtime_generation_prices DROP CONSTRAINT runtime_generation_prices_key_unique;
ALTER TABLE runtime_generation_prices ADD CONSTRAINT runtime_generation_prices_key_unique UNIQUE (
 catalog_version_id,operation,modality,text_model_id,image_model_id,video_route_alias,quality,resolution,duration_sec
);
ALTER TABLE runtime_generation_prices DROP CONSTRAINT runtime_generation_prices_public_key_shape;
ALTER TABLE runtime_generation_prices ADD CONSTRAINT runtime_generation_prices_public_key_shape CHECK (
 (operation = 'text_generate' AND modality = 'text' AND text_model_id <> '' AND image_model_id = '' AND video_route_alias = '' AND quality = '' AND resolution = '' AND duration_sec = 0)
 OR (operation IN ('image_generate','image_edit','image_upscale') AND modality = 'image' AND text_model_id = '' AND image_model_id <> '' AND video_route_alias = '' AND quality <> '' AND resolution = '' AND duration_sec = 0)
 OR (operation IN ('video_generate','video_image_to_video','video_extend') AND modality = 'video' AND text_model_id = '' AND image_model_id = '' AND video_route_alias <> '' AND quality = '' AND resolution <> '' AND duration_sec > 0)
);
-- Price activation remains an explicit, audited operator action. Never modify
-- existing versions or historical Job snapshots during a schema migration.
COMMIT;
