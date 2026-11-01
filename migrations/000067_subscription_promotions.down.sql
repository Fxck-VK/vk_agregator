-- Commercial purchase snapshots and consumption history must survive rollbacks.
DO $$ BEGIN RAISE EXCEPTION 'subscription promotion migration is irreversible; roll back stateless code only'; END $$;
