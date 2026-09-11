-- Run with psql -v ON_ERROR_STOP=1 in a disposable PostgreSQL database.
-- Checks the reviewed 000052 migration against existing image/video prices.
CREATE SCHEMA text_pricing_migration_review;
SET search_path TO text_pricing_migration_review, public;
\ir ../../migrations/000001_init_schema.up.sql
\ir ../../migrations/000027_job_pricing_snapshot.up.sql
\ir ../../migrations/000028_runtime_pricing_catalog.up.sql

INSERT INTO runtime_pricing_catalog_versions (id, price_version, status)
VALUES ('00000000-0000-0000-0000-000000000001', 1, 'active');
INSERT INTO runtime_generation_prices (
    catalog_version_id, operation, modality, image_model_id, video_route_alias,
    quality, resolution, duration_sec, floor_amount, floor_unit,
    multiplier_numerator, multiplier_denominator, enabled
) VALUES
('00000000-0000-0000-0000-000000000001', 'image_generate', 'image', 'image-test', '', '1k', '', 0, 1000, 'usd_micros', 3, 1, true),
('00000000-0000-0000-0000-000000000001', 'video_generate', 'video', '', 'video-test', '', '720p', 5, 2000, 'usd_micros', 3, 1, true);
CREATE TEMP TABLE prices_before AS SELECT to_jsonb(p) AS value FROM runtime_generation_prices p;
CREATE TEMP TABLE versions_before AS SELECT to_jsonb(v) AS value FROM runtime_pricing_catalog_versions v;

\ir ../../migrations/000052_text_model_pricing.up.sql

DO $$ BEGIN
    IF (SELECT count(*) FROM runtime_generation_prices) <> 2 OR EXISTS (
        SELECT value FROM prices_before
        EXCEPT SELECT to_jsonb(p) - 'text_model_id' FROM runtime_generation_prices p
    ) THEN RAISE EXCEPTION 'Existing prices changed or disappeared'; END IF;
    IF EXISTS (
        SELECT value FROM versions_before
        EXCEPT SELECT to_jsonb(v) FROM runtime_pricing_catalog_versions v
    ) OR (SELECT count(*) FROM runtime_pricing_catalog_versions) <> 1 THEN
        RAISE EXCEPTION 'Catalog activation changed';
    END IF;
    IF EXISTS (SELECT 1 FROM runtime_generation_prices WHERE text_model_id <> '') THEN
        RAISE EXCEPTION 'Migration activated text pricing';
    END IF;
END $$;

INSERT INTO runtime_generation_prices (
    catalog_version_id, operation, modality, text_model_id,
    floor_amount, floor_unit, multiplier_numerator, multiplier_denominator
) VALUES
('00000000-0000-0000-0000-000000000001', 'text_generate', 'text', 'model-a', 1000, 'usd_micros', 3, 1),
('00000000-0000-0000-0000-000000000001', 'text_generate', 'text', 'model-b', 1000, 'usd_micros', 3, 1);

DO $$ BEGIN
    BEGIN
        INSERT INTO runtime_generation_prices (catalog_version_id, operation, modality, text_model_id, floor_amount, floor_unit, multiplier_numerator, multiplier_denominator)
        VALUES ('00000000-0000-0000-0000-000000000001', 'text_generate', 'text', 'model-a', 1000, 'usd_micros', 3, 1);
        RAISE EXCEPTION 'Duplicate text model accepted';
    EXCEPTION WHEN unique_violation THEN NULL; END;
    BEGIN
        UPDATE runtime_generation_prices SET text_model_id = 'mixed-model' WHERE modality = 'image';
        RAISE EXCEPTION 'Mixed image/text dimensions accepted';
    EXCEPTION WHEN check_violation THEN NULL; END;
    BEGIN
        UPDATE runtime_generation_prices SET text_model_id = '' WHERE text_model_id = 'model-a';
        RAISE EXCEPTION 'Empty text model accepted';
    EXCEPTION WHEN check_violation THEN NULL; END;
    BEGIN
        INSERT INTO runtime_generation_prices (catalog_version_id, operation, modality, image_model_id, quality, floor_amount, floor_unit, multiplier_numerator, multiplier_denominator)
        VALUES ('00000000-0000-0000-0000-000000000001', 'image_generate', 'image', 'image-test', '1k', 1000, 'usd_micros', 3, 1);
        RAISE EXCEPTION 'Duplicate image price accepted';
    EXCEPTION WHEN unique_violation THEN NULL; END;
END $$;
SELECT 'Text pricing migration preserves existing data and enforces constraints' AS result;
