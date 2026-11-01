DO $$ BEGIN
 RAISE EXCEPTION 'Web referral payment proof and reward ledger attribution are irreversible; roll back application code only';
END $$;
