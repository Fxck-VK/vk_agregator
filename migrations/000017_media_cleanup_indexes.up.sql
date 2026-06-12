-- 000017_media_cleanup_indexes.up.sql
-- Bounded indexes for inactive media cleanup. They keep maintenance scans away
-- from active ready/stored artifacts and do not change data.

CREATE INDEX IF NOT EXISTS artifacts_media_cleanup_idx
    ON artifacts (status, media_type, updated_at)
    WHERE storage_bucket <> ''
      AND storage_key <> ''
      AND media_type IN ('image', 'video', 'audio', 'document');

CREATE INDEX IF NOT EXISTS artifact_variants_media_cleanup_idx
    ON artifact_variants (variant_type, updated_at, artifact_id)
    WHERE storage_bucket <> ''
      AND storage_key <> '';
