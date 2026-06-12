-- 000016_video_media_metadata.down.sql
-- Remove safe media probe metadata columns.

BEGIN;

ALTER TABLE artifact_variants
    DROP CONSTRAINT IF EXISTS artifact_variants_probe_status_check,
    DROP CONSTRAINT IF EXISTS artifact_variants_bitrate_bps_nonnegative,
    DROP COLUMN IF EXISTS probe_status,
    DROP COLUMN IF EXISTS bitrate_bps,
    DROP COLUMN IF EXISTS container,
    DROP COLUMN IF EXISTS codec;

ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_probe_status_check,
    DROP CONSTRAINT IF EXISTS artifacts_bitrate_bps_nonnegative,
    DROP COLUMN IF EXISTS probe_status,
    DROP COLUMN IF EXISTS bitrate_bps,
    DROP COLUMN IF EXISTS container,
    DROP COLUMN IF EXISTS codec;

COMMIT;
