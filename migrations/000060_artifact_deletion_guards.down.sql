DROP TRIGGER IF EXISTS guard_artifact_tombstone ON artifacts;
DROP FUNCTION IF EXISTS guard_artifact_tombstone();
DROP TRIGGER IF EXISTS guard_deleted_artifact_variant ON artifact_variants;
DROP FUNCTION IF EXISTS guard_deleted_artifact_variant();
DROP TRIGGER IF EXISTS guard_deleted_job_inputs ON jobs;
DROP FUNCTION IF EXISTS guard_deleted_job_inputs();
