BEGIN;

DROP INDEX IF EXISTS artifact_variants_media_cleanup_expiry_idx;
DROP INDEX IF EXISTS artifacts_media_cleanup_expiry_idx;

-- pg_stat_statements is intentionally left installed. It is an environment
-- diagnostic extension and may be shared with other databases or managed by the
-- hosting provider.

COMMIT;
