-- Public identifiers may already have been shared with users and support.
-- Preserve the mapping/sequence and migration record on application rollback.
DO $$
BEGIN
    RAISE EXCEPTION 'account public IDs are permanent; roll back application binaries while retaining migration 000065';
END;
$$;
