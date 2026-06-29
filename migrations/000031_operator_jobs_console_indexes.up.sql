BEGIN;

CREATE INDEX IF NOT EXISTS jobs_console_created_id_idx
    ON jobs (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS jobs_console_status_created_id_idx
    ON jobs (status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS jobs_console_operation_created_id_idx
    ON jobs (operation_type, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS jobs_console_modality_created_id_idx
    ON jobs (modality, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS jobs_console_user_created_id_idx
    ON jobs (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS jobs_console_error_created_id_idx
    ON jobs (error_code, created_at DESC, id DESC)
    WHERE error_code <> '';

CREATE INDEX IF NOT EXISTS provider_tasks_console_provider_job_idx
    ON provider_tasks (provider, job_id);

COMMIT;
