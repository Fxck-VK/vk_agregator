-- 000018_media_lifecycle.up.sql
-- Additive media lifecycle metadata and indexes for safe reference-image dedupe
-- and batched cleanup. Existing artifacts get conservative lifecycle classes;
-- validation_policy_version stays empty so older uploads are not silently reused
-- under newer validation rules.

BEGIN;

ALTER TABLE artifacts
    ADD COLUMN IF NOT EXISTS validation_policy_version TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS lifecycle_class TEXT NOT NULL DEFAULT 'provider_original';

ALTER TABLE artifact_variants
    ADD COLUMN IF NOT EXISTS lifecycle_class TEXT NOT NULL DEFAULT 'delivery_variant';

UPDATE artifacts
SET lifecycle_class = CASE
    WHEN status IN ('failed', 'deleted') THEN 'failed_deleted'
    WHEN kind = 'input' AND media_type = 'image' THEN 'input_reference'
    WHEN kind IN ('output', 'intermediate') THEN 'provider_original'
    ELSE 'temp_upload'
END;

UPDATE artifact_variants
SET lifecycle_class = 'delivery_variant';

ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_lifecycle_class_check,
    ADD CONSTRAINT artifacts_lifecycle_class_check CHECK (
        lifecycle_class IN (
            'temp_upload',
            'input_reference',
            'provider_original',
            'delivery_variant',
            'failed_deleted'
        )
    );

ALTER TABLE artifact_variants
    DROP CONSTRAINT IF EXISTS artifact_variants_lifecycle_class_check,
    ADD CONSTRAINT artifact_variants_lifecycle_class_check CHECK (
        lifecycle_class IN (
            'temp_upload',
            'input_reference',
            'provider_original',
            'delivery_variant',
            'failed_deleted'
        )
    );

CREATE INDEX IF NOT EXISTS artifacts_input_reference_dedupe_idx
    ON artifacts (owner_user_id, sha256, validation_policy_version, mime_type, created_at)
    WHERE lifecycle_class = 'input_reference'
      AND kind = 'input'
      AND media_type = 'image'
      AND status = 'ready'
      AND storage_bucket <> ''
      AND storage_key <> ''
      AND sha256 <> ''
      AND validation_policy_version <> '';

CREATE INDEX IF NOT EXISTS artifacts_lifecycle_cleanup_idx
    ON artifacts (lifecycle_class, status, updated_at, id)
    WHERE storage_bucket <> ''
      AND storage_key <> '';

CREATE INDEX IF NOT EXISTS artifact_variants_lifecycle_cleanup_idx
    ON artifact_variants (lifecycle_class, updated_at, artifact_id, id)
    WHERE storage_bucket <> ''
      AND storage_key <> '';

CREATE INDEX IF NOT EXISTS jobs_input_artifact_ids_gin_idx
    ON jobs USING GIN (input_artifact_ids);

CREATE INDEX IF NOT EXISTS jobs_output_artifact_ids_gin_idx
    ON jobs USING GIN (output_artifact_ids);

CREATE INDEX IF NOT EXISTS deliveries_artifact_id_idx
    ON deliveries (artifact_id)
    WHERE artifact_id IS NOT NULL;

COMMIT;
