package postgres_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func rewardPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	p := webReferralPool(t, ctx)
	root, _ := repoRoot()
	conn, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	for _, path := range webReferralMigrationUpFiles(t, root, "000066") {
		if filepath.Base(path)[:6] > "000056" {
			preparedRunMigrationFile(t, ctx, conn, path)
		}
	}
	return p
}

type rewardFixture struct {
	pool                                     *pgxpool.Pool
	owner, invited, relation, wallet, intent uuid.UUID
	at                                       time.Time
}

func newRewardFixture(t *testing.T, p *pgxpool.Pool, mode domain.WebReferralPaymentMode) rewardFixture {
	t.Helper()
	ctx := context.Background()
	f := rewardFixture{pool: p, owner: uuid.New(), invited: uuid.New(), relation: uuid.New(), intent: uuid.New(), at: time.Now().UTC().Truncate(time.Microsecond)}
	for _, id := range []uuid.UUID{f.owner, f.invited} {
		if _, err := p.Exec(ctx, `INSERT INTO accounts(id) VALUES($1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	a := &domain.CreditAccount{OwnerAccountID: f.owner, Currency: domain.CurrencyCredits}
	if err := postgres.NewBillingRepository(p).CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	f.wallet = a.ID
	if _, err := p.Exec(ctx, `INSERT INTO referrals(id,referrer_account_id,referred_account_id,referral_code,source) VALUES($1,$2,$3,'TEST','web')`, f.relation, f.owner, f.invited); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO payment_intents(id,account_id,status,amount,currency,credits,price_version,provider,provider_payment_id,idempotency_key,receipt_email) VALUES($1,$2,'succeeded',19900,'rub',400,1,'yookassa',$3,$3,'test@example.invalid')`, f.intent, f.invited, f.intent.String()); err != nil {
		t.Fatal(err)
	}
	err := postgres.RunInTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		return postgres.NewPaymentRepositoryTx(tx).RecordWebReferralPayment(ctx, domain.WebReferralPaymentEvidence{IntentID: f.intent, AccountID: f.invited, Provider: "yookassa", ProviderPaymentID: f.intent.String(), Amount: 19900, Currency: "rub", Mode: mode, VerifiedAt: f.at})
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f rewardFixture) process(t *testing.T, at time.Time) int64 {
	t.Helper()
	n, err := postgres.NewMaintenanceRepository(f.pool).ProcessWebReferralRewards(context.Background(), at, 100)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
func (f rewardFixture) refund(t *testing.T) {
	t.Helper()
	err := postgres.RunInTx(context.Background(), f.pool, func(ctx context.Context, tx pgx.Tx) error {
		return postgres.NewPaymentRepositoryTx(tx).RecordWebReferralRefund(ctx, domain.WebReferralRefundEvidence{IntentID: f.intent, Provider: "yookassa", ProviderPaymentID: f.intent.String(), ProviderRefundID: "refund-" + f.intent.String(), Amount: 100, Currency: "rub", VerifiedAt: f.at.Add(48 * time.Hour)})
	})
	if err != nil {
		t.Fatal(err)
	}
}
func (f rewardFixture) balance(t *testing.T, want int64) {
	t.Helper()
	ctx := context.Background()
	a, err := postgres.NewBillingRepository(f.pool).GetAccount(ctx, f.wallet)
	if err != nil || a.BalanceCached != want {
		t.Fatalf("balance got=%+v err=%v want=%d", a, err, want)
	}
	assertPostgresLotProjection(t, ctx, f.pool, f.wallet)
}

func TestWebReferralRewardsDelayAndRefund(t *testing.T) {
	p := rewardPool(t)
	f := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	f.process(t, f.at.Add(24*time.Hour-time.Microsecond))
	f.balance(t, 0)
	f.process(t, f.at.Add(24*time.Hour))
	f.balance(t, 50)
	f.process(t, f.at.Add(30*time.Hour))
	f.balance(t, 50)
	var permanent bool
	if err := p.QueryRow(context.Background(), `SELECT kind='bonus' AND expires_at IS NULL FROM credit_lots WHERE account_id=$1`, f.wallet).Scan(&permanent); err != nil || !permanent {
		t.Fatalf("permanent bonus %v %v", permanent, err)
	}
	f.refund(t)
	f.process(t, f.at.Add(49*time.Hour))
	f.balance(t, 0)
	f.process(t, f.at.Add(72*time.Hour))
	f.balance(t, 0)
	var state string
	_ = p.QueryRow(context.Background(), `SELECT status FROM web_referral_rewards WHERE referral_id=$1`, f.relation).Scan(&state)
	if state != "reversed" {
		t.Fatal(state)
	}
}
func TestWebReferralRewardsProofAndCancellation(t *testing.T) {
	p := rewardPool(t)
	for _, mode := range []domain.WebReferralPaymentMode{domain.WebReferralPaymentTest, domain.WebReferralPaymentUnknown, domain.WebReferralPaymentLive} {
		t.Run(string(mode), func(t *testing.T) {
			f := newRewardFixture(t, p, mode)
			if mode == domain.WebReferralPaymentLive {
				f.refund(t)
			}
			f.process(t, f.at.Add(72*time.Hour))
			f.balance(t, 0)
		})
	}
}
func TestWebReferralRewardsSpentOrHeldReversal(t *testing.T) {
	p := rewardPool(t)
	for _, terminal := range []string{"spent", "release", "capture"} {
		t.Run(terminal, func(t *testing.T) {
			ctx := context.Background()
			f := newRewardFixture(t, p, domain.WebReferralPaymentLive)
			f.process(t, f.at.Add(24*time.Hour))
			r := postgres.NewBillingRepository(p)
			// Purchased funds must not be reclassified or consumed by referral reversal.
			if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: f.wallet, Type: domain.LedgerTopup, Amount: 70, LotKind: domain.CreditLotPurchased, IdempotencyKey: uuid.NewString()}); err != nil {
				t.Fatal(err)
			}
			job := uuid.New()
			if _, err := p.Exec(ctx, `INSERT INTO jobs(id,account_id,operation_type,modality,idempotency_key) VALUES($1,$2,'text_generate','text',$3)`, job, f.owner, uuid.NewString()); err != nil {
				t.Fatal(err)
			}
			res := &domain.CreditReservation{AccountID: f.wallet, JobID: job, Amount: 30, IdempotencyKey: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour)}
			if err := r.ReserveForOwner(ctx, f.owner, res); err != nil {
				t.Fatal(err)
			}
			if terminal == "spent" {
				if err := r.CaptureForOwner(ctx, f.owner, res.ID, 30, uuid.NewString()); err != nil {
					t.Fatal(err)
				}
			}
			f.refund(t)
			f.process(t, f.at.Add(48*time.Hour))
			want := int64(70)
			if terminal == "spent" {
				want = 40
			}
			f.balance(t, want)
			if terminal == "release" {
				if err := r.ReleaseForOwner(ctx, f.owner, res.ID, uuid.NewString()); err != nil {
					t.Fatal(err)
				}
				f.balance(t, 70)
			}
			if terminal == "capture" {
				if err := r.CaptureForOwner(ctx, f.owner, res.ID, 30, uuid.NewString()); err != nil {
					t.Fatal(err)
				}
				f.balance(t, 40)
			}
			var purchased int64
			if err := p.QueryRow(ctx, `SELECT available_credits FROM credit_lots WHERE account_id=$1 AND kind='purchased'`, f.wallet).Scan(&purchased); err != nil || purchased != 70 {
				t.Fatalf("purchased changed %d %v", purchased, err)
			}
		})
	}
}
func TestWebReferralRewardsConcurrentAndRollback(t *testing.T) {
	p := rewardPool(t)
	f := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	ctx := context.Background()
	at := f.at.Add(24 * time.Hour)
	sentinel := errors.New("rollback")
	err := postgres.RunInTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		_, err := postgres.NewMaintenanceRepository(tx).ProcessWebReferralRewards(ctx, at, 100)
		if err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	f.balance(t, 0)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := postgres.NewMaintenanceRepository(p).ProcessWebReferralRewards(ctx, at, 100)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	f.balance(t, 50)
}

func (f rewardFixture) nextPayment(t *testing.T, mode domain.WebReferralPaymentMode, at time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO payment_intents(id,account_id,status,amount,currency,credits,price_version,provider,provider_payment_id,idempotency_key,receipt_email) VALUES($1,$2,'succeeded',54900,'rub',1250,1,'yookassa',$3,$3,'test@example.invalid')`, id, f.invited, id.String()); err != nil {
		t.Fatal(err)
	}
	err := postgres.RunInTx(ctx, f.pool, func(ctx context.Context, tx pgx.Tx) error {
		return postgres.NewPaymentRepositoryTx(tx).RecordWebReferralPayment(ctx, domain.WebReferralPaymentEvidence{IntentID: id, AccountID: f.invited, Provider: "yookassa", ProviderPaymentID: id.String(), Amount: 54900, Currency: "rub", Mode: mode, VerifiedAt: at})
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func TestWebReferralRewardsFirstPaymentAndLateAcceptance(t *testing.T) {
	p := rewardPool(t)
	ctx := context.Background()
	for _, mode := range []domain.WebReferralPaymentMode{domain.WebReferralPaymentUnknown, domain.WebReferralPaymentTest, domain.WebReferralPaymentLive} {
		t.Run(string(mode), func(t *testing.T) {
			f := newRewardFixture(t, p, mode)
			f.nextPayment(t, domain.WebReferralPaymentLive, f.at.Add(time.Hour))
			f.process(t, f.at.Add(48*time.Hour))
			want := int64(50)
			if mode == domain.WebReferralPaymentUnknown {
				want = 0
			}
			f.balance(t, want)
			if mode == domain.WebReferralPaymentLive {
				f.refund(t)
				f.process(t, f.at.Add(49*time.Hour))
				f.nextPayment(t, domain.WebReferralPaymentLive, f.at.Add(50*time.Hour))
				f.process(t, f.at.Add(100*time.Hour))
				f.balance(t, 0)
			}
		})
	}
	f := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	if _, err := p.Exec(ctx, `DELETE FROM referrals WHERE id=$1`, f.relation); err != nil {
		t.Fatal(err)
	}
	f.process(t, f.at.Add(24*time.Hour))
	f.balance(t, 0)
	if _, err := p.Exec(ctx, `INSERT INTO referrals(id,referrer_account_id,referred_account_id,referral_code,source) VALUES($1,$2,$3,'TEST','web')`, f.relation, f.owner, f.invited); err != nil {
		t.Fatal(err)
	}
	f.process(t, f.at.Add(25*time.Hour))
	f.balance(t, 50)
	second := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	if _, err := p.Exec(ctx, `UPDATE referrals SET referrer_account_id=$2 WHERE id=$1`, second.relation, f.owner); err != nil {
		t.Fatal(err)
	}
	f.process(t, second.at.Add(25*time.Hour))
	f.balance(t, 100)
}
func TestWebReferralRewardsEvidenceBindingHistoryAndRollback(t *testing.T) {
	p := rewardPool(t)
	ctx := context.Background()
	f := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	if _, err := p.Exec(ctx, `DELETE FROM web_referral_payment_evidence WHERE intent_id=$1`, f.intent); err != nil {
		t.Fatal(err)
	}
	a := &domain.CreditAccount{OwnerAccountID: f.invited, Currency: domain.CurrencyCredits}
	billing := postgres.NewBillingRepository(p)
	if err := billing.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := billing.AppendEntry(ctx, &domain.LedgerEntry{AccountID: a.ID, Type: domain.LedgerTopup, Amount: 400, LotKind: domain.CreditLotPurchased, IdempotencyKey: "topup:yookassa:" + f.intent.String()}); err != nil {
		t.Fatal(err)
	}
	f.nextPayment(t, domain.WebReferralPaymentLive, f.at.Add(time.Hour))
	f.process(t, f.at.Add(48*time.Hour))
	f.balance(t, 0)
	var mode string
	if err := p.QueryRow(ctx, `SELECT mode FROM web_referral_payment_evidence WHERE intent_id=$1`, f.intent).Scan(&mode); err != nil || mode != "historical" {
		t.Fatalf("history %s %v", mode, err)
	}
	g := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	proof := domain.WebReferralRefundEvidence{IntentID: g.intent, Provider: "yookassa", ProviderPaymentID: g.intent.String(), ProviderRefundID: uuid.NewString(), Amount: 100, Currency: "rub", VerifiedAt: g.at.Add(time.Hour)}
	sentinel := errors.New("evidence rollback")
	err := postgres.RunInTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if err := postgres.NewPaymentRepositoryTx(tx).RecordWebReferralRefund(ctx, proof); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	g.process(t, g.at.Add(24*time.Hour))
	g.balance(t, 50)
	if err := postgres.NewPaymentRepository(p).RecordWebReferralRefund(ctx, proof); err == nil {
		t.Fatal("bare querier accepted financial evidence")
	}
	proof.ProviderPaymentID = "foreign"
	err = postgres.RunInTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		return postgres.NewPaymentRepositoryTx(tx).RecordWebReferralRefund(ctx, proof)
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("foreign binding %v", err)
	}
	g.balance(t, 50)
}

func TestWebReferralRewardsConcurrentMixedOwnerBatches(t *testing.T) {
	p := rewardPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	first := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	second := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	for i := 0; i < 10; i++ {
		f := newRewardFixture(t, p, domain.WebReferralPaymentLive)
		owner := first.owner
		if i%2 == 1 {
			owner = second.owner
		}
		if _, err := p.Exec(ctx, `UPDATE referrals SET referrer_account_id=$2 WHERE id=$1`, f.relation, owner); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 12; j++ {
				_, err := postgres.NewMaintenanceRepository(p).ProcessWebReferralRewards(ctx, time.Now().Add(25*time.Hour), 2)
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	first.balance(t, 300)
	second.balance(t, 300)
}
func TestWebReferralRewardsSpentDebtBlocksFurtherSpending(t *testing.T) {
	p := rewardPool(t)
	ctx := context.Background()
	f := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	f.process(t, f.at.Add(24*time.Hour))
	r := postgres.NewBillingRepository(p)
	job := uuid.New()
	if _, err := p.Exec(ctx, `INSERT INTO jobs(id,account_id,operation_type,modality,idempotency_key) VALUES($1,$2,'text_generate','text',$3)`, job, f.owner, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	res := &domain.CreditReservation{AccountID: f.wallet, JobID: job, Amount: 50, IdempotencyKey: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := r.ReserveForOwner(ctx, f.owner, res); err != nil {
		t.Fatal(err)
	}
	if err := r.CaptureForOwner(ctx, f.owner, res.ID, 50, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	f.refund(t)
	f.process(t, f.at.Add(48*time.Hour))
	f.balance(t, -50)
	if _, err := r.ExpireCreditLots(ctx, f.at.AddDate(2, 0, 0), 100); err != nil {
		t.Fatal(err)
	}
	f.balance(t, -50)
	if err := r.AppendEntry(ctx, &domain.LedgerEntry{AccountID: f.wallet, Type: domain.LedgerTopup, Amount: 60, LotKind: domain.CreditLotPurchased, IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	f.balance(t, 10)
	job = uuid.New()
	if _, err := p.Exec(ctx, `INSERT INTO jobs(id,account_id,operation_type,modality,idempotency_key) VALUES($1,$2,'text_generate','text',$3)`, job, f.owner, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	res = &domain.CreditReservation{AccountID: f.wallet, JobID: job, Amount: 11, IdempotencyKey: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := r.ReserveForOwner(ctx, f.owner, res); err == nil {
		t.Fatal("debt was ignored for new spending")
	}
	f.balance(t, 10)
}

func TestWebReferralRewardsRefundSerializesWithDueGrant(t *testing.T) {
	p := rewardPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	f.process(t, f.at.Add(23*time.Hour))
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	proof := domain.WebReferralRefundEvidence{IntentID: f.intent, Provider: "yookassa", ProviderPaymentID: f.intent.String(), ProviderRefundID: uuid.NewString(), Amount: 100, Currency: "rub", VerifiedAt: f.at.Add(24 * time.Hour)}
	if err = postgres.NewPaymentRepositoryTx(tx).RecordWebReferralRefund(ctx, proof); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := postgres.NewMaintenanceRepository(p).ProcessWebReferralRewards(ctx, f.at.Add(24*time.Hour), 100)
		done <- err
	}()
	waiting := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err = p.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%web_referral_payment_evidence%FOR UPDATE%'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			waiting = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("due grant did not serialize with authenticated refund proof")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	f.balance(t, 0)
	var state string
	if err = p.QueryRow(ctx, `SELECT status FROM web_referral_rewards WHERE referral_id=$1`, f.relation).Scan(&state); err != nil || state != "cancelled" {
		t.Fatalf("refund lost race %s %v", state, err)
	}
}
