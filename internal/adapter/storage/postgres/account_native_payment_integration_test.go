package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/paymentservice"
)

// TestAccountNativePaymentIntentPostgresIntegration covers nullable legacy
// provenance, strict canonical owner reads, global idempotency, and the
// non-aborting conflict path used by transaction-bound repositories.
func TestAccountNativePaymentIntentPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	owner, foreign := uuid.New(), uuid.New()
	for _, accountID := range []uuid.UUID{owner, foreign} {
		if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, accountID); err != nil {
			t.Fatalf("insert account %s: %v", accountID, err)
		}
	}
	repo := postgres.NewPaymentRepository(pool)
	key := "native-payment-" + uuid.NewString()
	intent := &domain.PaymentIntent{
		AccountID: owner, Status: domain.PaymentIntentWaitingForUser,
		Amount: 9900, Currency: domain.CurrencyRUB, Credits: 100,
		CreditDenominationVersion: domain.CurrentCreditDenominationVersion,
		PriceVersion:              1,
		ReceiptDescription:        "100 credits",
		PaymentSubject:            "service",
		PaymentMode:               "full_prepayment",
		Provider:                  domain.PaymentProviderMock, ProviderPaymentID: "provider-" + uuid.NewString(),
		IdempotencyKey: key, ReceiptEmail: "account@example.com",
		Metadata: json.RawMessage(`{"source":"web","return_url":"https://example.test/return","capture":true}`),
	}
	if err := repo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("create native payment intent: %v", err)
	}
	var legacyUser *uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT user_id FROM payment_intents WHERE id = $1`, intent.ID).Scan(&legacyUser); err != nil {
		t.Fatalf("read nullable legacy user: %v", err)
	}
	if legacyUser != nil || intent.UserID != uuid.Nil {
		t.Fatalf("native intent must keep legacy user NULL/nil: db=%v domain=%s", legacyUser, intent.UserID)
	}
	if got, err := repo.GetIntentByIDForAccount(ctx, owner, intent.ID); err != nil || got.ID != intent.ID || got.AccountID != owner || got.UserID != uuid.Nil {
		t.Fatalf("owner intent read = %#v, %v", got, err)
	}
	if got, err := repo.GetIntentByIdempotencyKeyForAccount(ctx, owner, key); err != nil || got.ID != intent.ID {
		t.Fatalf("owner intent key read = %#v, %v", got, err)
	}
	for _, accountID := range []uuid.UUID{foreign, uuid.Nil} {
		if _, err := repo.GetIntentByIDForAccount(ctx, accountID, intent.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("foreign/nil strict intent read for %s = %v, want not found", accountID, err)
		}
		if _, err := repo.GetIntentByIdempotencyKeyForAccount(ctx, accountID, key); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("foreign/nil strict key read for %s = %v, want not found", accountID, err)
		}
	}
	list, err := repo.ListIntentsByAccount(ctx, owner, 10, 0)
	if err != nil || len(list) != 1 || list[0].ID != intent.ID {
		t.Fatalf("strict intent list = %#v, %v", list, err)
	}

	if err := postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		txRepo := postgres.NewPaymentRepositoryTx(tx)
		collision := *intent
		collision.ID = uuid.New()
		collision.AccountID = foreign
		collision.ProviderPaymentID = "provider-" + uuid.NewString()
		if err := txRepo.CreateIntent(ctx, &collision); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("foreign global idempotency collision = %v, want conflict", err)
		}
		if _, err := tx.Exec(ctx, `SELECT 1`); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("outer payment transaction remained unusable after expected conflict: %v", err)
	}
}

func TestCreateAccountIntentConcurrentIdempotencyRacePostgresIntegration(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	owner := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, owner); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	baseRepo := postgres.NewPaymentRepository(pool)
	product := &domain.PaymentProduct{
		Code: "native-race-" + uuid.NewString(), Title: "Concurrent native payment",
		Amount: 9900, Currency: domain.CurrencyRUB, Credits: 100,
		CreditDenominationVersion: domain.CurrentCreditDenominationVersion,
		PriceVersion:              1,
		PaymentSubject:            "service",
		PaymentMode:               "full_prepayment",
		IsActive:                  true,
	}
	if err := baseRepo.CreateProduct(ctx, product); err != nil {
		t.Fatalf("create payment product: %v", err)
	}
	input := paymentservice.CreateAccountIntentInput{
		AccountID: owner, ProductCode: product.Code, ReceiptEmail: "account@example.com",
		IdempotencyKey: "native-payment-race-" + uuid.NewString(),
	}
	barrier := newPaymentIntentLookupBarrier(input.IdempotencyKey)
	provider := paymentmock.New()

	txs := make([]pgx.Tx, 2)
	for i := range txs {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin transaction %d: %v", i, err)
		}
		txs[i] = tx
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	}
	type result struct {
		intent  *domain.PaymentIntent
		created bool
		tx      pgx.Tx
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, tx := range txs {
		tx := tx
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			repo := paymentIntentLookupBarrierRepo{PaymentRepository: postgres.NewPaymentRepositoryTx(tx), barrier: barrier}
			created, err := paymentservice.New(repo, provider, paymentservice.Config{ReturnURL: "https://example.test/payments/return"}).CreateAccountIntent(ctx, input)
			results <- result{intent: created.Intent, created: created.Created, tx: tx, err: err}
		}()
	}
	close(start)

	first := <-results
	if first.err != nil || !first.created || first.intent == nil {
		t.Fatalf("first concurrent create = %#v, want successful insert", first)
	}
	if _, err := first.tx.Exec(ctx, `SELECT 1`); err != nil {
		t.Fatalf("winning transaction unusable before commit: %v", err)
	}
	if err := first.tx.Commit(ctx); err != nil {
		t.Fatalf("commit winning transaction: %v", err)
	}

	second := <-results
	wg.Wait()
	if second.err != nil || second.created || second.intent == nil {
		t.Fatalf("conflict replay = %#v, want existing intent", second)
	}
	if second.intent.ID != first.intent.ID || second.intent.ProviderPaymentID != first.intent.ProviderPaymentID {
		t.Fatalf("concurrent replay differs: first=%#v second=%#v", first.intent, second.intent)
	}
	if _, err := second.tx.Exec(ctx, `SELECT 1`); err != nil {
		t.Fatalf("conflict transaction unusable after ON CONFLICT/re-read: %v", err)
	}
	if err := second.tx.Commit(ctx); err != nil {
		t.Fatalf("commit conflict replay transaction: %v", err)
	}
	if got := barrier.InitialMisses(); got != 2 {
		t.Fatalf("initial idempotency lookup misses = %d, want 2", got)
	}
	if got := barrier.FallbackLookups(); got != 1 {
		t.Fatalf("conflict fallback lookups = %d, want 1", got)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM payment_intents WHERE account_id = $1 AND idempotency_key = $2`, owner, input.IdempotencyKey).Scan(&count); err != nil {
		t.Fatalf("count raced payment intents: %v", err)
	}
	if count != 1 {
		t.Fatalf("raced payment intents = %d, want 1", count)
	}
}

type paymentIntentLookupBarrier struct {
	mu              sync.Mutex
	key             string
	initialMisses   int
	fallbackLookups int
	release         chan struct{}
}

func newPaymentIntentLookupBarrier(key string) *paymentIntentLookupBarrier {
	return &paymentIntentLookupBarrier{key: key, release: make(chan struct{})}
}

type paymentIntentLookupBarrierRepo struct {
	domain.PaymentRepository
	barrier *paymentIntentLookupBarrier
}

func (r paymentIntentLookupBarrierRepo) GetIntentByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentIntent, error) {
	intent, err := r.PaymentRepository.GetIntentByIdempotencyKey(ctx, key)
	if r.barrier == nil || key != r.barrier.key {
		return intent, err
	}
	r.barrier.mu.Lock()
	if errors.Is(err, domain.ErrNotFound) && r.barrier.initialMisses < 2 {
		r.barrier.initialMisses++
		if r.barrier.initialMisses == 2 {
			close(r.barrier.release)
		}
		r.barrier.mu.Unlock()
		select {
		case <-r.barrier.release:
			return intent, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err == nil {
		r.barrier.fallbackLookups++
	}
	r.barrier.mu.Unlock()
	return intent, err
}

func (b *paymentIntentLookupBarrier) InitialMisses() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.initialMisses
}

func (b *paymentIntentLookupBarrier) FallbackLookups() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.fallbackLookups
}
