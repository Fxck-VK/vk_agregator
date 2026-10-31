BEGIN;
-- Attribution rollback never changes financial ledger entries or balances.
DROP TABLE credit_reservation_lots;
DROP INDEX credit_reservations_id_account_unique;
DROP TABLE credit_lots;
ALTER TABLE ledger_entries DROP CONSTRAINT ledger_lot_expiry_kind, DROP COLUMN lot_expires_at, DROP COLUMN lot_kind;
COMMIT;
