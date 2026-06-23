BEGIN;

-- Enable slow-query aggregation where the database role is allowed to create
-- extensions. Managed Postgres installations may need this to be enabled by
-- the provider/admin; do not fail the whole app migration in that case.
DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
EXCEPTION
    WHEN insufficient_privilege THEN
        RAISE NOTICE 'pg_stat_statements extension was not created: insufficient privileges';
    WHEN undefined_file THEN
        RAISE NOTICE 'pg_stat_statements extension was not created: extension files are unavailable';
END
$$;

-- The generic artifact retention index includes artifact_tier and is useful for
-- tier-specific lifecycle checks. Media object deletion candidates scan by
-- expires_at plus storage coordinates, so keep this narrower index for the
-- maintenance worker cleanup path.
CREATE INDEX IF NOT EXISTS artifacts_media_cleanup_expiry_idx
    ON artifacts (expires_at, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL
      AND storage_bucket <> ''
      AND storage_key <> '';

CREATE INDEX IF NOT EXISTS artifact_variants_media_cleanup_expiry_idx
    ON artifact_variants (expires_at, artifact_id, id)
    WHERE deleted_at IS NULL
      AND expires_at IS NOT NULL
      AND storage_bucket <> ''
      AND storage_key <> '';

COMMIT;
