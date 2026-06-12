-- 000016_video_media_metadata.up.sql
-- Add safe media probe metadata for the future video/media pipeline.

BEGIN;

ALTER TABLE artifacts
    ADD COLUMN codec TEXT NOT NULL DEFAULT '',
    ADD COLUMN container TEXT NOT NULL DEFAULT '',
    ADD COLUMN bitrate_bps BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN probe_status TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE artifacts
    ADD CONSTRAINT artifacts_bitrate_bps_nonnegative CHECK (bitrate_bps >= 0),
    ADD CONSTRAINT artifacts_probe_status_check CHECK (
        probe_status IN ('unknown', 'pending', 'passed', 'failed', 'skipped')
    );

ALTER TABLE artifact_variants
    ADD COLUMN codec TEXT NOT NULL DEFAULT '',
    ADD COLUMN container TEXT NOT NULL DEFAULT '',
    ADD COLUMN bitrate_bps BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN probe_status TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE artifact_variants
    ADD CONSTRAINT artifact_variants_bitrate_bps_nonnegative CHECK (bitrate_bps >= 0),
    ADD CONSTRAINT artifact_variants_probe_status_check CHECK (
        probe_status IN ('unknown', 'pending', 'passed', 'failed', 'skipped')
    );

COMMIT;
