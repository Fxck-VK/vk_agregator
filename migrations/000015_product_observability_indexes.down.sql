-- 000015_product_observability_indexes.down.sql

BEGIN;

DROP INDEX IF EXISTS jobs_created_surface_idx;
DROP INDEX IF EXISTS jobs_created_at_idx;

COMMIT;
