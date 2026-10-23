-- Generated files expire two UTC calendar months after their original creation.
-- Explicit expirations are consumed by the existing maintenance worker, across
-- standard/free/paid tiers. Text conversations and uploaded inputs are excluded.
-- Do not reset an earlier expiry: user deletion and previous cleanup must win.
CREATE OR REPLACE FUNCTION generated_output_expiry() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE deadline timestamptz;
BEGIN
    IF NEW.kind = 'output' AND NEW.media_type IN ('image', 'video', 'audio', 'document') THEN
        deadline := ((NEW.created_at AT TIME ZONE 'UTC') + interval '2 months') AT TIME ZONE 'UTC';
        IF TG_OP = 'UPDATE' THEN
            NEW.expires_at := LEAST(NEW.expires_at, OLD.expires_at, deadline);
        ELSE
            NEW.expires_at := LEAST(NEW.expires_at, deadline);
        END IF;
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS generated_output_expiry ON artifacts;
CREATE TRIGGER generated_output_expiry
BEFORE INSERT OR UPDATE OF created_at, kind, media_type, expires_at ON artifacts
FOR EACH ROW EXECUTE FUNCTION generated_output_expiry();

-- A preview created much later has the parent's original deadline. Keep the
-- parent stable while admitting the variant, as in the artifact deletion guard.
CREATE OR REPLACE FUNCTION generated_output_variant_expiry() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE parent artifacts%ROWTYPE;
DECLARE deadline timestamptz;
BEGIN
    SELECT * INTO parent FROM artifacts WHERE id = NEW.artifact_id FOR SHARE;
    IF parent.kind = 'output' AND parent.media_type IN ('image', 'video', 'audio', 'document') THEN
        deadline := LEAST(parent.expires_at,
            ((parent.created_at AT TIME ZONE 'UTC') + interval '2 months') AT TIME ZONE 'UTC');
        IF TG_OP = 'UPDATE' THEN
            NEW.expires_at := LEAST(NEW.expires_at, OLD.expires_at, deadline);
        ELSE
            NEW.expires_at := LEAST(NEW.expires_at, deadline);
        END IF;
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS generated_output_variant_expiry ON artifact_variants;
CREATE TRIGGER generated_output_variant_expiry
BEFORE INSERT OR UPDATE OF artifact_id, expires_at ON artifact_variants
FOR EACH ROW EXECUTE FUNCTION generated_output_variant_expiry();

-- If the original is explicitly expired sooner, existing previews follow it.
CREATE OR REPLACE FUNCTION generated_output_expiry_cascade() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.kind = 'output' AND NEW.media_type IN ('image', 'video', 'audio', 'document')
       AND NEW.expires_at IS DISTINCT FROM OLD.expires_at THEN
        UPDATE artifact_variants
        SET expires_at = NEW.expires_at
        WHERE artifact_id = NEW.id AND deleted_at IS NULL
          AND (expires_at IS NULL OR expires_at > NEW.expires_at);
    END IF;
    RETURN NULL;
END $$;

DROP TRIGGER IF EXISTS generated_output_expiry_cascade ON artifacts;
CREATE TRIGGER generated_output_expiry_cascade
AFTER UPDATE OF expires_at ON artifacts
FOR EACH ROW EXECUTE FUNCTION generated_output_expiry_cascade();

-- Backfill existing bytes, preserving timestamps and already earlier expiries.
-- Blob removal remains the existing idempotent maintenance operation.
UPDATE artifacts
SET expires_at = LEAST(expires_at,
    ((created_at AT TIME ZONE 'UTC') + interval '2 months') AT TIME ZONE 'UTC')
WHERE kind = 'output'
  AND media_type IN ('image', 'video', 'audio', 'document')
  AND deleted_at IS NULL
  AND (expires_at IS NULL OR expires_at >
    ((created_at AT TIME ZONE 'UTC') + interval '2 months') AT TIME ZONE 'UTC');

UPDATE artifact_variants v
SET expires_at = LEAST(v.expires_at, a.expires_at,
    ((a.created_at AT TIME ZONE 'UTC') + interval '2 months') AT TIME ZONE 'UTC')
FROM artifacts a
WHERE a.id = v.artifact_id
  AND a.kind = 'output'
  AND a.media_type IN ('image', 'video', 'audio', 'document')
  AND v.deleted_at IS NULL
  AND (v.expires_at IS NULL OR v.expires_at > LEAST(a.expires_at,
    ((a.created_at AT TIME ZONE 'UTC') + interval '2 months') AT TIME ZONE 'UTC'));
