-- 000018_media_lifecycle.down.sql

BEGIN;

DROP INDEX IF EXISTS deliveries_artifact_id_idx;
DROP INDEX IF EXISTS jobs_output_artifact_ids_gin_idx;
DROP INDEX IF EXISTS jobs_input_artifact_ids_gin_idx;
DROP INDEX IF EXISTS artifact_variants_lifecycle_cleanup_idx;
DROP INDEX IF EXISTS artifacts_lifecycle_cleanup_idx;
DROP INDEX IF EXISTS artifacts_input_reference_dedupe_idx;

ALTER TABLE artifact_variants
    DROP CONSTRAINT IF EXISTS artifact_variants_lifecycle_class_check,
    DROP COLUMN IF EXISTS lifecycle_class;

ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_lifecycle_class_check,
    DROP COLUMN IF EXISTS lifecycle_class,
    DROP COLUMN IF EXISTS validation_policy_version;

COMMIT;
