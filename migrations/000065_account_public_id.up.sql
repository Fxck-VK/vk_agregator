-- Existing rows are backfilled by PostgreSQL; future account INSERTs allocate
-- the number independently of login methods, legacy users and application code.
ALTER TABLE accounts
    ADD COLUMN public_number BIGINT GENERATED ALWAYS AS IDENTITY,
    ADD CONSTRAINT accounts_public_number_positive CHECK (public_number > 0),
    ADD CONSTRAINT accounts_public_number_unique UNIQUE (public_number);

CREATE FUNCTION guard_account_public_number() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.public_number IS DISTINCT FROM OLD.public_number THEN
        RAISE EXCEPTION 'account public number is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER accounts_public_number_immutable
BEFORE UPDATE OF public_number ON accounts
FOR EACH ROW EXECUTE FUNCTION guard_account_public_number();
