package paymentservice_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/paymentservice"
)

func TestCreateIntentCreatesProviderPaymentAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepo()
	product := &domain.PaymentProduct{
		Code:         "credits_100",
		Title:        "100 credits",
		Amount:       9900,
		Currency:     domain.CurrencyRUB,
		Credits:      100,
		PriceVersion: 3,
		IsActive:     true,
	}
	repo.PutProduct(product)
	provider := paymentmock.New()
	svc := paymentservice.New(repo, provider, paymentservice.Config{ReturnURL: "https://neiirohub.ru/payments/return"})
	userID := uuid.New()

	first, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    "credits_100",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: "miniapp_payment:777:client-key",
		Source:         "vk_miniapp",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if !first.Created {
		t.Fatal("first call should create local intent")
	}
	if first.Intent.Status != domain.PaymentIntentWaitingForUser ||
		first.Intent.ProviderPaymentID == "" ||
		first.Intent.ConfirmationURL == "" ||
		first.Intent.Amount != 9900 ||
		first.Intent.Credits != 100 ||
		first.Intent.PriceVersion != 3 {
		t.Fatalf("unexpected first intent: %+v", first.Intent)
	}

	second, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    "credits_100",
		ReceiptEmail:   "other@example.com",
		IdempotencyKey: "miniapp_payment:777:client-key",
		Source:         "vk_miniapp",
	})
	if err != nil {
		t.Fatalf("replay intent: %v", err)
	}
	if second.Created {
		t.Fatal("replay should return existing intent")
	}
	if second.Intent.ID != first.Intent.ID || second.Intent.ProviderPaymentID != first.Intent.ProviderPaymentID {
		t.Fatalf("replay returned different provider payment: first=%+v second=%+v", first.Intent, second.Intent)
	}
}

func TestCreateIntentRequiresReceiptContact(t *testing.T) {
	repo := memory.NewPaymentRepo()
	repo.PutProduct(&domain.PaymentProduct{
		Code: "credits_100", Title: "100 credits", Amount: 9900,
		Currency: domain.CurrencyRUB, Credits: 100, PriceVersion: 1, IsActive: true,
	})
	svc := paymentservice.New(repo, paymentmock.New(), paymentservice.Config{})
	_, err := svc.CreateIntent(context.Background(), paymentservice.CreateIntentInput{
		UserID:         uuid.New(),
		ProductCode:    "credits_100",
		IdempotencyKey: "key",
	})
	if !errors.Is(err, paymentservice.ErrReceiptContactRequired) {
		t.Fatalf("expected ErrReceiptContactRequired, got %v", err)
	}
}

func TestCreateIntentRejectsIdempotencyKeyFromAnotherUser(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepo()
	repo.PutProduct(&domain.PaymentProduct{
		Code: "credits_100", Title: "100 credits", Amount: 9900,
		Currency: domain.CurrencyRUB, Credits: 100, PriceVersion: 1, IsActive: true,
	})
	svc := paymentservice.New(repo, paymentmock.New(), paymentservice.Config{})
	key := "shared-key"
	if _, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         uuid.New(),
		ProductCode:    "credits_100",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: key,
	}); err != nil {
		t.Fatalf("create first: %v", err)
	}
	_, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         uuid.New(),
		ProductCode:    "credits_100",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: key,
	})
	if !errors.Is(err, paymentservice.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestWebhookProcessorIngestIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepo()
	provider := paymentmock.New()
	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	raw := []byte(`{"event_type":"payment.succeeded","provider_payment_id":"mock-pay-1"}`)

	_, created, err := processor.IngestWebhook(ctx, raw, nil)
	if err != nil {
		t.Fatalf("ingest first: %v", err)
	}
	if !created {
		t.Fatal("first ingest should create event")
	}
	_, created, err = processor.IngestWebhook(ctx, raw, nil)
	if err != nil {
		t.Fatalf("ingest duplicate: %v", err)
	}
	if created {
		t.Fatal("duplicate ingest should be dedup no-op")
	}
	events, err := repo.ListUnprocessedEvents(ctx, domain.PaymentProviderMock, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}

	refundA := []byte(`{"event_type":"refund.succeeded","provider_payment_id":"mock-pay-1","provider_refund_id":"refund-a"}`)
	refundB := []byte(`{"event_type":"refund.succeeded","provider_payment_id":"mock-pay-1","provider_refund_id":"refund-b"}`)
	if _, created, err := processor.IngestWebhook(ctx, refundA, nil); err != nil || !created {
		t.Fatalf("ingest refund A created=%v err=%v", created, err)
	}
	if _, created, err := processor.IngestWebhook(ctx, refundA, nil); err != nil || created {
		t.Fatalf("duplicate refund A created=%v err=%v", created, err)
	}
	if _, created, err := processor.IngestWebhook(ctx, refundB, nil); err != nil || !created {
		t.Fatalf("ingest refund B created=%v err=%v", created, err)
	}
	events, err = repo.ListUnprocessedEvents(ctx, domain.PaymentProviderMock, 10)
	if err != nil {
		t.Fatalf("list events after refunds: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events len after refunds = %d, want 3", len(events))
	}
}

func TestWebhookProcessorVerifiedSuccessGrantsOnce(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepo()
	repo.PutProduct(&domain.PaymentProduct{
		Code: "credits_100", Title: "100 credits", Amount: 9900,
		Currency: domain.CurrencyRUB, Credits: 100, PriceVersion: 1, IsActive: true,
	})
	provider := paymentmock.New()
	intentSvc := paymentservice.New(repo, provider, paymentservice.Config{})
	userID := uuid.New()
	created, err := intentSvc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    "credits_100",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: "intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set provider status: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	raw := []byte(`{"event_type":"payment.succeeded","provider_payment_id":"` + created.Intent.ProviderPaymentID + `"}`)
	if _, _, err := processor.IngestWebhook(ctx, raw, nil); err != nil {
		t.Fatalf("ingest webhook: %v", err)
	}
	processed, err := processor.ProcessBatch(ctx, 10)
	if err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	intent, err := repo.GetIntentByID(ctx, created.Intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if intent.Status != domain.PaymentIntentSucceeded {
		t.Fatalf("intent status = %s, want succeeded", intent.Status)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance = %d, want 100", acc.BalanceCached)
	}

	rawSecond := []byte(`{"event_type":"payment.succeeded.retry","provider_payment_id":"` + created.Intent.ProviderPaymentID + `"}`)
	if _, _, err := processor.IngestWebhook(ctx, rawSecond, nil); err != nil {
		t.Fatalf("ingest second webhook: %v", err)
	}
	if _, err := processor.ProcessBatch(ctx, 10); err != nil {
		t.Fatalf("process second batch: %v", err)
	}
	acc, err = billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after replay: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after replay = %d, want 100", acc.BalanceCached)
	}
}

func TestWebhookProcessorReconcilePendingSyncsSucceededIntent(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepo()
	repo.PutProduct(&domain.PaymentProduct{
		Code: "credits_100", Title: "100 credits", Amount: 9900,
		Currency: domain.CurrencyRUB, Credits: 100, PriceVersion: 1, IsActive: true,
	})
	provider := paymentmock.New()
	intentSvc := paymentservice.New(repo, provider, paymentservice.Config{})
	userID := uuid.New()
	created, err := intentSvc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    "credits_100",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: "reconcile-intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set provider status: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	result, err := processor.ReconcilePendingOlderThan(ctx, 10, -time.Nanosecond)
	if err != nil {
		t.Fatalf("reconcile pending: %v", err)
	}
	if result.Checked != 1 || result.Processed != 1 || result.Mismatches != 0 {
		t.Fatalf("unexpected reconciliation result: %+v", result)
	}
	intent, err := repo.GetIntentByID(ctx, created.Intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if intent.Status != domain.PaymentIntentSucceeded {
		t.Fatalf("intent status = %s, want succeeded", intent.Status)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance = %d, want 100", acc.BalanceCached)
	}
}

func TestWebhookProcessorLateCanceledDoesNotRollbackSucceeded(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepo()
	repo.PutProduct(&domain.PaymentProduct{
		Code: "credits_100", Title: "100 credits", Amount: 9900,
		Currency: domain.CurrencyRUB, Credits: 100, PriceVersion: 1, IsActive: true,
	})
	provider := paymentmock.New()
	intentSvc := paymentservice.New(repo, provider, paymentservice.Config{})
	userID := uuid.New()
	created, err := intentSvc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    "credits_100",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: "intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set succeeded: %v", err)
	}
	if _, _, err := processor.IngestWebhook(ctx, []byte(`{"event_type":"payment.succeeded","provider_payment_id":"`+created.Intent.ProviderPaymentID+`"}`), nil); err != nil {
		t.Fatalf("ingest succeeded: %v", err)
	}
	if _, err := processor.ProcessBatch(ctx, 10); err != nil {
		t.Fatalf("process succeeded: %v", err)
	}

	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentCanceled); err != nil {
		t.Fatalf("set canceled: %v", err)
	}
	if _, _, err := processor.IngestWebhook(ctx, []byte(`{"event_type":"payment.canceled","provider_payment_id":"`+created.Intent.ProviderPaymentID+`"}`), nil); err != nil {
		t.Fatalf("ingest canceled: %v", err)
	}
	if _, err := processor.ProcessBatch(ctx, 10); err != nil {
		t.Fatalf("process canceled: %v", err)
	}
	intent, err := repo.GetIntentByID(ctx, created.Intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if intent.Status != domain.PaymentIntentSucceeded {
		t.Fatalf("late canceled rolled back intent to %s", intent.Status)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance = %d, want 100", acc.BalanceCached)
	}
	events, err := repo.ListUnprocessedEvents(ctx, domain.PaymentProviderMock, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("unprocessed events = %d, want 0", len(events))
	}
}

func newTestWebhookProcessor(repo *memory.PaymentRepo, provider *paymentmock.Provider, billingRepo *memory.BillingRepo) *paymentservice.WebhookProcessor {
	billing := billingservice.New(billingRepo, billingservice.WithStartingBalance(0))
	tx := paymentservice.TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		return fn(ctx, repo, billingRepo)
	})
	return paymentservice.NewWebhookProcessor(repo, provider, billing, tx)
}
