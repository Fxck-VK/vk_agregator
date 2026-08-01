package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
)

// TestEnsureAccountForAccountConflictKeepsOuterTransactionUsable proves that a
// concurrent native account creation is recoverable inside a caller-managed
// transaction. The second insert must not issue a unique-violation error: that
// would abort the outer transaction before EnsureAccountForAccount can re-read
// the winning row.
func TestEnsureAccountForAccountConflictKeepsOuterTransactionUsable(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ensureNativeCreditAccountSchema(t, ctx, pool)

	ownerAccountID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, ownerAccountID); err != nil {
		t.Fatalf("create canonical owner: %v", err)
	}

	billing := postgres.NewBillingRepository(pool)
	svc := billingservice.New(billing)
	var wantID uuid.UUID
	if err := postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		txBilling := postgres.NewBillingRepositoryTx(tx)
		raced := &createAfterInitialOwnerMissRepository{
			BillingRepository: txBilling,
			beforeCreate: func() error {
				winner := &domain.CreditAccount{
					OwnerAccountID: ownerAccountID,
					Currency:       domain.CurrencyCredits,
				}
				if err := billing.CreateAccount(ctx, winner); err != nil {
					return err
				}
				wantID = winner.ID
				return nil
			},
		}

		account, err := svc.EnsureAccountForAccount(ctx, raced, ownerAccountID, domain.CurrencyCredits)
		if err != nil {
			return err
		}
		if account.ID != wantID {
			t.Fatalf("conflict recovery returned account %s, want %s", account.ID, wantID)
		}
		if account.UserID != uuid.Nil || account.OwnerAccountID != ownerAccountID {
			t.Fatalf("conflict recovery returned wrong ownership: %#v", account)
		}

		// This statement fails with SQLSTATE 25P02 when the losing INSERT has
		// aborted the caller-owned transaction.
		if _, err := tx.Exec(ctx, `SELECT 1`); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("ensure native account after concurrent create: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM credit_accounts
		WHERE owner_account_id = $1 AND currency = $2`, ownerAccountID, domain.CurrencyCredits).Scan(&count); err != nil {
		t.Fatalf("count native owner/currency accounts: %v", err)
	}
	if count != 1 {
		t.Fatalf("native owner/currency account count = %d, want 1", count)
	}
}

type createAfterInitialOwnerMissRepository struct {
	domain.BillingRepository
	beforeCreate func() error
	missed       bool
}

func (r *createAfterInitialOwnerMissRepository) GetAccountByOwner(ctx context.Context, ownerAccountID uuid.UUID, currency domain.Currency) (*domain.CreditAccount, error) {
	if !r.missed {
		r.missed = true
		return nil, domain.ErrNotFound
	}
	return r.BillingRepository.GetAccountByOwner(ctx, ownerAccountID, currency)
}

func (r *createAfterInitialOwnerMissRepository) CreateAccount(ctx context.Context, account *domain.CreditAccount) error {
	if r.beforeCreate != nil {
		if err := r.beforeCreate(); err != nil {
			return err
		}
		r.beforeCreate = nil
	}
	return r.BillingRepository.CreateAccount(ctx, account)
}

func ensureNativeCreditAccountSchema(t *testing.T, ctx context.Context, pool interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}) {
	t.Helper()
	for _, statement := range []string{
		`ALTER TABLE jobs ALTER COLUMN user_id DROP NOT NULL`,
		`ALTER TABLE jobs ALTER COLUMN vk_peer_id DROP NOT NULL`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts (id) ON DELETE SET NULL`,
		`ALTER TABLE credit_accounts ALTER COLUMN user_id DROP NOT NULL`,
		`ALTER TABLE credit_accounts ADD COLUMN IF NOT EXISTS owner_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL`,
		`ALTER TABLE credit_accounts ADD COLUMN IF NOT EXISTS credit_denomination_version SMALLINT NOT NULL DEFAULT 2 CHECK (credit_denomination_version IN (1, 2))`,
		`ALTER TABLE credit_reservations ADD COLUMN IF NOT EXISTS owner_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL`,
		`ALTER TABLE credit_reservations ADD COLUMN IF NOT EXISTS credit_denomination_version SMALLINT NOT NULL DEFAULT 2 CHECK (credit_denomination_version IN (1, 2))`,
		`ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS owner_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL`,
		`ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS credit_denomination_version SMALLINT NOT NULL DEFAULT 2 CHECK (credit_denomination_version IN (1, 2))`,
		`CREATE UNIQUE INDEX IF NOT EXISTS credit_accounts_owner_account_currency_unique
			ON credit_accounts (owner_account_id, currency)
			WHERE owner_account_id IS NOT NULL`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("prepare account-native credit schema: %v", err)
		}
	}
}

func TestAccountNativeReservationReplayKeepsOuterTransactionUsable(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ensureNativeCreditAccountSchema(t, ctx, pool)

	ownerAccountID, jobID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, ownerAccountID); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO jobs (id, account_id, operation_type, modality, status, idempotency_key)
		VALUES ($1, $2, 'image_generate', 'image', 'queued', $3)`, jobID, ownerAccountID, "job:"+uuid.NewString()); err != nil {
		t.Fatalf("create account-native job: %v", err)
	}
	billing := postgres.NewBillingRepository(pool)
	account := &domain.CreditAccount{
		OwnerAccountID: ownerAccountID,
		Currency:       domain.CurrencyCredits,
		BalanceCached:  100,
	}
	if err := billing.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create credit account: %v", err)
	}
	initial := &domain.CreditReservation{
		AccountID:      account.ID,
		OwnerAccountID: ownerAccountID,
		JobID:          jobID,
		Amount:         40,
		IdempotencyKey: "resv:" + uuid.NewString(),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	if err := billing.ReserveForOwner(ctx, ownerAccountID, initial); err != nil {
		t.Fatalf("initial reserve: %v", err)
	}

	if err := postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		txBilling := postgres.NewBillingRepositoryTx(tx)
		replay := &domain.CreditReservation{
			AccountID:      account.ID,
			OwnerAccountID: ownerAccountID,
			JobID:          jobID,
			Amount:         40,
			IdempotencyKey: initial.IdempotencyKey,
			ExpiresAt:      time.Now().Add(time.Hour),
		}
		if err := txBilling.ReserveForOwner(ctx, ownerAccountID, replay); err != nil {
			return err
		}
		if replay.ID != initial.ID || replay.OwnerAccountID != ownerAccountID {
			t.Fatalf("exact replay = %#v, want original %s", replay, initial.ID)
		}
		changed := *replay
		changed.ID = uuid.New()
		changed.Amount = 41
		if err := txBilling.ReserveForOwner(ctx, ownerAccountID, &changed); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("changed replay error = %v, want ErrConflict", err)
		}
		if _, err := tx.Exec(ctx, `SELECT 1`); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("reservation replay transaction: %v", err)
	}
}

func TestAccountNativeReservationTerminalIntegrity(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ensureNativeCreditAccountSchema(t, ctx, pool)

	owner, foreignOwner := uuid.New(), uuid.New()
	for _, accountID := range []uuid.UUID{owner, foreignOwner} {
		if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, accountID); err != nil {
			t.Fatalf("create canonical account %s: %v", accountID, err)
		}
	}
	newJob := func() uuid.UUID {
		t.Helper()
		jobID := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO jobs (id, account_id, operation_type, modality, status, idempotency_key)
			VALUES ($1, $2, 'image_generate', 'image', 'queued', $3)`, jobID, owner, "job:"+uuid.NewString()); err != nil {
			t.Fatalf("create account-native job: %v", err)
		}
		return jobID
	}
	billing := postgres.NewBillingRepository(pool)
	account := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyCredits, BalanceCached: 100}
	if err := billing.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create primary credit account: %v", err)
	}
	ownerAlternateAccount := &domain.CreditAccount{OwnerAccountID: owner, Currency: domain.CurrencyRUB, BalanceCached: 100}
	if err := billing.CreateAccount(ctx, ownerAlternateAccount); err != nil {
		t.Fatalf("create alternate owner credit account: %v", err)
	}
	reserve := func(amount int64) *domain.CreditReservation {
		t.Helper()
		res := &domain.CreditReservation{
			AccountID:      account.ID,
			JobID:          newJob(),
			Amount:         amount,
			IdempotencyKey: "resv:" + uuid.NewString(),
			ExpiresAt:      time.Now().Add(time.Hour),
		}
		if err := billing.ReserveForOwner(ctx, owner, res); err != nil {
			t.Fatalf("reserve: %v", err)
		}
		return res
	}
	assertReservation := func(reservationID uuid.UUID, want domain.ReservationStatus) {
		t.Helper()
		got, err := billing.GetReservation(ctx, reservationID)
		if err != nil || got.Status != want {
			t.Fatalf("reservation %s = %#v, %v; want status %q", reservationID, got, err, want)
		}
	}
	assertBalance := func(want int64) {
		t.Helper()
		got, err := billing.GetAccount(ctx, account.ID)
		if err != nil || got.BalanceCached != want {
			t.Fatalf("balance = %#v, %v; want %d", got, err, want)
		}
	}
	assertTerminalLedgerCount := func(reservationID uuid.UUID, entryType domain.LedgerEntryType, want int) {
		t.Helper()
		var got int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE reservation_id = $1 AND type = $2`, reservationID, entryType).Scan(&got); err != nil {
			t.Fatalf("count %s ledger entries: %v", entryType, err)
		}
		if got != want {
			t.Fatalf("%s ledger entries for reservation %s = %d, want %d", entryType, reservationID, got, want)
		}
	}

	invalidCapture := reserve(40)
	for _, amount := range []int64{0, -1, 39, 41} {
		if err := billing.CaptureForOwner(ctx, owner, invalidCapture.ID, amount, "cap:"+uuid.NewString()); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("capture %d error = %v, want ErrConflict", amount, err)
		}
		assertReservation(invalidCapture.ID, domain.ReservationReserved)
		assertTerminalLedgerCount(invalidCapture.ID, domain.LedgerCapture, 0)
		assertBalance(100)
	}
	if err := billing.ReleaseForOwner(ctx, owner, invalidCapture.ID, "rel:cleanup:"+invalidCapture.ID.String()); err != nil {
		t.Fatalf("release invalid capture reservation: %v", err)
	}
	assertReservation(invalidCapture.ID, domain.ReservationReleased)
	assertTerminalLedgerCount(invalidCapture.ID, domain.LedgerRelease, 1)
	assertBalance(100)

	captured := reserve(40)
	captureKey := "cap:" + captured.ID.String()
	if err := billing.CaptureForOwner(ctx, owner, captured.ID, 40, captureKey); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if err := billing.CaptureForOwner(ctx, owner, captured.ID, 40, captureKey); err != nil {
		t.Fatalf("exact capture replay: %v", err)
	}
	assertReservation(captured.ID, domain.ReservationCaptured)
	assertTerminalLedgerCount(captured.ID, domain.LedgerCapture, 1)
	assertBalance(60)
	if err := billing.ReleaseForOwner(ctx, owner, captured.ID, "rel:"+captured.ID.String()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("release after capture error = %v, want ErrConflict", err)
	}
	assertReservation(captured.ID, domain.ReservationCaptured)
	assertTerminalLedgerCount(captured.ID, domain.LedgerRelease, 0)
	assertBalance(60)

	released := reserve(10)
	releaseKey := "rel:" + released.ID.String()
	if err := billing.ReleaseForOwner(ctx, owner, released.ID, releaseKey); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := billing.ReleaseForOwner(ctx, owner, released.ID, releaseKey); err != nil {
		t.Fatalf("exact release replay: %v", err)
	}
	assertReservation(released.ID, domain.ReservationReleased)
	assertTerminalLedgerCount(released.ID, domain.LedgerRelease, 1)
	assertBalance(60)
	if err := billing.CaptureForOwner(ctx, owner, released.ID, 10, "cap:"+released.ID.String()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("capture after release error = %v, want ErrConflict", err)
	}
	assertReservation(released.ID, domain.ReservationReleased)
	assertTerminalLedgerCount(released.ID, domain.LedgerCapture, 0)
	assertBalance(60)

	captureCollision := reserve(10)
	captureCollisionKey := "cap:" + captureCollision.ID.String()
	if err := billing.AppendEntry(ctx, &domain.LedgerEntry{
		AccountID: account.ID, OwnerAccountID: owner, Type: domain.LedgerTopup,
		Status: domain.LedgerStatusPending, IdempotencyKey: captureCollisionKey,
	}); err != nil {
		t.Fatalf("preinsert capture key collision: %v", err)
	}
	if err := billing.CaptureForOwner(ctx, owner, captureCollision.ID, 10, captureCollisionKey); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("capture key collision error = %v, want ErrConflict", err)
	}
	assertReservation(captureCollision.ID, domain.ReservationReserved)
	assertTerminalLedgerCount(captureCollision.ID, domain.LedgerCapture, 0)
	assertBalance(60)
	if err := billing.ReleaseForOwner(ctx, owner, captureCollision.ID, "rel:cleanup:"+captureCollision.ID.String()); err != nil {
		t.Fatalf("release capture collision reservation: %v", err)
	}
	assertReservation(captureCollision.ID, domain.ReservationReleased)
	assertTerminalLedgerCount(captureCollision.ID, domain.LedgerRelease, 1)
	assertBalance(60)

	releaseCollision := reserve(10)
	releaseCollisionKey := "rel:" + releaseCollision.ID.String()
	if err := billing.AppendEntry(ctx, &domain.LedgerEntry{
		AccountID: account.ID, OwnerAccountID: owner, Type: domain.LedgerTopup,
		Status: domain.LedgerStatusPending, IdempotencyKey: releaseCollisionKey,
	}); err != nil {
		t.Fatalf("preinsert release key collision: %v", err)
	}
	if err := billing.ReleaseForOwner(ctx, owner, releaseCollision.ID, releaseCollisionKey); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("release key collision error = %v, want ErrConflict", err)
	}
	assertReservation(releaseCollision.ID, domain.ReservationReserved)
	assertTerminalLedgerCount(releaseCollision.ID, domain.LedgerRelease, 0)
	assertBalance(60)
	if err := billing.ReleaseForOwner(ctx, owner, releaseCollision.ID, "rel:cleanup:"+releaseCollision.ID.String()); err != nil {
		t.Fatalf("release release collision reservation: %v", err)
	}
	assertReservation(releaseCollision.ID, domain.ReservationReleased)
	assertTerminalLedgerCount(releaseCollision.ID, domain.LedgerRelease, 1)
	assertBalance(60)

	replay := reserve(10)
	exactReplay := *replay
	if err := billing.ReserveForOwner(ctx, owner, &exactReplay); err != nil {
		t.Fatalf("exact reservation replay: %v", err)
	}
	for name, changed := range map[string]domain.CreditReservation{
		"account":      {AccountID: ownerAlternateAccount.ID},
		"job":          {JobID: newJob()},
		"denomination": {CreditDenominationVersion: 1},
		"owner":        {OwnerAccountID: foreignOwner},
	} {
		t.Run("replay changed "+name, func(t *testing.T) {
			attempt := *replay
			if changed.AccountID != uuid.Nil {
				attempt.AccountID = changed.AccountID
			}
			if changed.JobID != uuid.Nil {
				attempt.JobID = changed.JobID
			}
			if changed.CreditDenominationVersion != 0 {
				attempt.CreditDenominationVersion = changed.CreditDenominationVersion
			}
			if changed.OwnerAccountID != uuid.Nil {
				attempt.OwnerAccountID = changed.OwnerAccountID
			}
			if err := billing.ReserveForOwner(ctx, owner, &attempt); !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("changed %s replay error = %v, want ErrConflict", name, err)
			}
		})
	}
	assertReservation(replay.ID, domain.ReservationReserved)
	assertBalance(60)
}
