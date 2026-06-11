-- 000015_product_observability_indexes.up.sql
-- Low-load product observability aggregates over jobs.

BEGIN;

CREATE INDEX IF NOT EXISTS jobs_created_at_idx ON jobs (created_at);
CREATE INDEX IF NOT EXISTS jobs_created_surface_idx ON jobs (created_at, command_id, operation_type, modality, user_id);

COMMIT;
