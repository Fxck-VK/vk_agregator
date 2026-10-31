-- Additive attribution of the existing ledger; no credit balance is reset.
BEGIN;

-- Block billing writes throughout the snapshot and hold-allocation backfill.
LOCK TABLE credit_accounts, credit_reservations, ledger_entries IN SHARE ROW EXCLUSIVE MODE;

ALTER TABLE ledger_entries
    ADD COLUMN lot_kind TEXT NOT NULL DEFAULT '' CHECK (lot_kind IN ('', 'legacy', 'purchased', 'subscription', 'bonus')),
    ADD COLUMN lot_expires_at TIMESTAMPTZ,
    ADD CONSTRAINT ledger_lot_expiry_kind CHECK (lot_expires_at IS NULL OR lot_kind IN ('subscription', 'bonus'));

CREATE TABLE credit_lots (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES credit_accounts(id),
    source_entry_id UUID UNIQUE REFERENCES ledger_entries(id),
    kind TEXT NOT NULL CHECK (kind IN ('legacy', 'purchased', 'subscription', 'bonus')),
    available_credits BIGINT NOT NULL,
    reserved_credits BIGINT NOT NULL DEFAULT 0 CHECK (reserved_credits >= 0),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, account_id),
    CHECK (available_credits >= 0 OR (kind = 'legacy' AND expires_at IS NULL)),
    CHECK (expires_at IS NULL OR kind IN ('subscription', 'bonus'))
);
CREATE INDEX credit_lots_fefo_idx ON credit_lots(account_id, expires_at, created_at, id);
CREATE INDEX credit_lots_expiry_idx ON credit_lots(expires_at, account_id) WHERE expires_at IS NOT NULL AND available_credits > 0;

CREATE UNIQUE INDEX credit_reservations_id_account_unique ON credit_reservations(id, account_id);
CREATE TABLE credit_reservation_lots (
    reservation_id UUID NOT NULL,
    lot_id UUID NOT NULL,
    account_id UUID NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    PRIMARY KEY (reservation_id, lot_id),
    FOREIGN KEY (reservation_id, account_id) REFERENCES credit_reservations(id, account_id),
    FOREIGN KEY (lot_id, account_id) REFERENCES credit_lots(id, account_id)
);
CREATE INDEX credit_reservation_lots_lot_idx ON credit_reservation_lots(lot_id);

-- Legacy provenance is deliberately coarse. Subtract holds only from the
-- available portion; total available + reserved remains exactly balance_cached,
-- including historical negative corrections or overcommitted accounts.
INSERT INTO credit_lots(id, account_id, kind, available_credits, reserved_credits, created_at)
SELECT a.id, a.id, 'legacy', a.balance_cached - COALESCE(h.amount, 0), COALESCE(h.amount, 0), a.created_at
FROM credit_accounts a
LEFT JOIN (
    SELECT account_id, SUM(amount)::BIGINT AS amount FROM credit_reservations
    WHERE status = 'reserved' GROUP BY account_id
) h ON h.account_id = a.id;

INSERT INTO credit_reservation_lots(reservation_id, lot_id, account_id, amount)
SELECT id, account_id, account_id, amount FROM credit_reservations WHERE status = 'reserved' AND amount > 0;
COMMIT;
