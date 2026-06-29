BEGIN;

DROP INDEX IF EXISTS provider_tasks_console_provider_job_idx;
DROP INDEX IF EXISTS jobs_console_error_created_id_idx;
DROP INDEX IF EXISTS jobs_console_user_created_id_idx;
DROP INDEX IF EXISTS jobs_console_modality_created_id_idx;
DROP INDEX IF EXISTS jobs_console_operation_created_id_idx;
DROP INDEX IF EXISTS jobs_console_status_created_id_idx;
DROP INDEX IF EXISTS jobs_console_created_id_idx;

COMMIT;
