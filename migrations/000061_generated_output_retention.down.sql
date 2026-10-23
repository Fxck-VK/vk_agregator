-- Disables future automatic deadline assignment only. Existing expirations
-- remain: restoring deleted bytes or changing agreed dates requires a separate
-- reviewed operation and cannot be achieved by an automatic schema rollback.
DROP TRIGGER IF EXISTS generated_output_expiry_cascade ON artifacts;
DROP FUNCTION IF EXISTS generated_output_expiry_cascade();
DROP TRIGGER IF EXISTS generated_output_variant_expiry ON artifact_variants;
DROP FUNCTION IF EXISTS generated_output_variant_expiry();
DROP TRIGGER IF EXISTS generated_output_expiry ON artifacts;
DROP FUNCTION IF EXISTS generated_output_expiry();
