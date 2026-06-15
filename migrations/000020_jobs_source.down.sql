DROP INDEX IF EXISTS jobs_source_created_idx;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS source;
