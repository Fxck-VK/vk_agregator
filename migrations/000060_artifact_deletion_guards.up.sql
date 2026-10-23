-- Serialize new consumers and variants with the artifact's deletion lock.
-- Existing job, financial, message and artifact metadata is preserved.
CREATE OR REPLACE FUNCTION guard_deleted_job_inputs() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE a record;
BEGIN
 IF TG_OP='UPDATE' AND NEW.input_artifact_ids IS NOT DISTINCT FROM OLD.input_artifact_ids
 AND (NEW.params->>'source_job_id') IS NOT DISTINCT FROM (OLD.params->>'source_job_id')
 AND (NEW.params->'sources') IS NOT DISTINCT FROM (OLD.params->'sources')
 AND (NEW.params->>'persona_job_id') IS NOT DISTINCT FROM (OLD.params->>'persona_job_id')
 AND (NEW.params->>'custom_model_job_id') IS NOT DISTINCT FROM (OLD.params->>'custom_model_job_id') THEN RETURN NEW; END IF;
 FOR a IN SELECT id,status FROM artifacts
 WHERE id=ANY(NEW.input_artifact_ids)
 OR id IN (SELECT unnest(output_artifact_ids) FROM jobs WHERE account_id=NEW.account_id AND
  (id::text=(NEW.params->>'source_job_id') OR id::text=(NEW.params->>'persona_job_id') OR id::text=(NEW.params->>'custom_model_job_id')
   OR id::text IN (SELECT source.value->>'job_id' FROM jsonb_array_elements(CASE WHEN jsonb_typeof(NEW.params->'sources')='array' THEN NEW.params->'sources' ELSE '[]'::jsonb END) AS source(value))))
 ORDER BY id FOR SHARE LOOP
  IF a.status='deleted' THEN
   RAISE EXCEPTION 'artifact unavailable' USING ERRCODE='23514', CONSTRAINT='artifact_input_available';
  END IF;
 END LOOP;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_deleted_job_inputs BEFORE INSERT OR UPDATE OF input_artifact_ids,params ON jobs
 FOR EACH ROW EXECUTE FUNCTION guard_deleted_job_inputs();

CREATE OR REPLACE FUNCTION guard_deleted_artifact_variant() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE parent_status text;
BEGIN
 SELECT status INTO parent_status FROM artifacts WHERE id=NEW.artifact_id FOR SHARE;
 IF parent_status='deleted' THEN
  RAISE EXCEPTION 'artifact unavailable' USING ERRCODE='23514', CONSTRAINT='artifact_input_available';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_deleted_artifact_variant BEFORE INSERT ON artifact_variants
 FOR EACH ROW EXECUTE FUNCTION guard_deleted_artifact_variant();

CREATE OR REPLACE FUNCTION guard_artifact_tombstone() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.status='deleted' AND NEW.status<>'deleted' THEN
  RAISE EXCEPTION 'artifact unavailable' USING ERRCODE='23514', CONSTRAINT='artifact_input_available';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_artifact_tombstone BEFORE UPDATE OF status ON artifacts
 FOR EACH ROW EXECUTE FUNCTION guard_artifact_tombstone();
