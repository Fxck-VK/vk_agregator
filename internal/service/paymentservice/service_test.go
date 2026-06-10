package paymentservice_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	vatCode := int16(2)
	product := &domain.PaymentProduct{
		Code:           "credits_100",
		Title:          "100 credits",
		Amount:         9900,
		Currency:       domain.CurrencyRUB,
		Credits:        100,
		PriceVersion:   3,
		VATCode:        &vatCode,
		PaymentSubject: "service",
		PaymentMode:    "full_prepayment",
		IsActive:       true,
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
		first.Intent.PriceVersion != 3 ||
		first.Intent.ReceiptDescription != "100 credits" ||
		first.Intent.VATCode == nil ||
		*first.Intent.VATCode != vatCode ||
		first.Intent.PaymentSubject != "service" ||
		first.Intent.PaymentMode != "full_prepayment" {
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

func TestCreateIntentUsesReceiptSnapshotWhenProviderCreationIsRetried(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepo()
	vatOriginal := int16(2)
	vatChanged := int16(3)
	productID := uuid.New()
	repo.PutProduct(&domain.PaymentProduct{
		ID:             productID,
		Code:           "credits_100",
		Title:          "Current catalog title",
		Amount:         9900,
		Currency:       domain.CurrencyRUB,
		Credits:        100,
		PriceVersion:   2,
		VATCode:        &vatChanged,
		PaymentSubject: "commodity",
		PaymentMode:    "full_payment",
		IsActive:       true,
	})

	intent := &domain.PaymentIntent{
		UserID:             uuid.New(),
		ProductID:          &productID,
		Status:             domain.PaymentIntentCreated,
		Amount:             9900,
		Currency:           domain.CurrencyRUB,
		Credits:            100,
		PriceVersion:       1,
		ReceiptDescription: "Original fiscal position",
		VATCode:            &vatOriginal,
		PaymentSubject:     "service",
		PaymentMode:        "full_prepayment",
		Provider:           domain.PaymentProviderMock,
		IdempotencyKey:     "retry-intent-key",
		ReceiptEmail:       "user@example.com",
	}
	if err := repo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("create local intent: %v", err)
	}
	provider := &recordingPaymentProvider{code: domain.PaymentProviderMock}
	svc := paymentservice.New(repo, provider, paymentservice.Config{})

	result, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         intent.UserID,
		ProductCode:    "credits_100",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: "retry-intent-key",
	})
	if err != nil {
		t.Fatalf("replay intent: %v", err)
	}
	if result.Created {
		t.Fatal("replay should not create a second local intent")
	}
	if len(provider.createInputs) != 1 {
		t.Fatalf("create provider calls = %d, want 1", len(provider.createInputs))
	}
	input := provider.createInputs[0]
	if input.Description != "Original fiscal position" ||
		input.VATCode == nil ||
		*input.VATCode != vatOriginal ||
		input.PaymentSubject != "service" ||
		input.PaymentMode != "full_prepayment" {
		t.Fatalf("provider input used current catalog instead of snapshot: %+v", input)
	}
}

func TestCreateIntentCanDisableProviderCaptureForOperatorSmoke(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepo()
	repo.PutProduct(&domain.PaymentProduct{
		Code:         "credits_100",
		Title:        "100 credits",
		Amount:       9900,
		Currency:     domain.CurrencyRUB,
		Credits:      100,
		PriceVersion: 1,
		IsActive:     true,
	})
	provider := &recordingPaymentProvider{code: domain.PaymentProviderMock}
	svc := paymentservice.New(repo, provider, paymentservice.Config{})
	capture := false

	result, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         uuid.New(),
		ProductCode:    "credits_100",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: "billing_payment:capture-false",
		Source:         "billing_admin",
		Capture:        &capture,
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if !result.Created {
		t.Fatal("intent should be created")
	}
	if len(provider.createInputs) != 1 {
		t.Fatalf("create provider calls = %d, want 1", len(provider.createInputs))
	}
	if provider.createInputs[0].Capture == nil || *provider.createInputs[0].Capture {
		t.Fatalf("provider capture = %#v, want false", provider.createInputs[0].Capture)
	}
	var metadata struct {
		Capture *bool `json:"capture"`
	}
	if err := json.Unmarshal(result.Intent.Metadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata.Capture == nil || *metadata.Capture {
		t.Fatalf("metadata capture = %#v, want false", metadata.Capture)
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

func TestCreateIntentReusesActiveWaitingIntentUnlessForceNew(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepo()
	repo.PutProduct(&domain.PaymentProduct{
		Code: "credits_100", Title: "100 credits", Amount: 9900,
		Currency: domain.CurrencyRUB, Credits: 100, PriceVersion: 1, IsActive: true,
	})
	repo.PutProduct(&domain.PaymentProduct{
		Code: "credits_500", Title: "500 credits", Amount: 39900,
		Currency: domain.CurrencyRUB, Credits: 500, PriceVersion: 1, IsActive: true,
	})
	provider := paymentmock.New()
	svc := paymentservice.New(repo, provider, paymentservice.Config{})
	userID := uuid.New()

	first, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    "credits_100",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: "payment:first",
		Source:         "vk_miniapp",
	})
	if err != nil {
		t.Fatalf("create first intent: %v", err)
	}

	reused, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    "credits_100",
		ReceiptEmail:   "new@example.com",
		IdempotencyKey: "payment:second",
		Source:         "vk_miniapp",
	})
	if err != nil {
		t.Fatalf("reuse active intent: %v", err)
	}
	if reused.Created || !reused.ReusedActive || reused.Intent.ID != first.Intent.ID {
		t.Fatalf("expected active intent reuse, got %+v first=%+v", reused, first)
	}

	differentProduct, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    "credits_500",
		ReceiptEmail:   "new@example.com",
		IdempotencyKey: "payment:different-product",
		Source:         "vk_miniapp",
	})
	if err != nil {
		t.Fatalf("create different product intent: %v", err)
	}
	if !differentProduct.Created || differentProduct.ReusedActive || differentProduct.Intent.ID == first.Intent.ID || differentProduct.Intent.Credits != 500 {
		t.Fatalf("expected new intent for different product, got %+v first=%+v", differentProduct, first)
	}

	forced, err := svc.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    "credits_100",
		ReceiptEmail:   "new@example.com",
		IdempotencyKey: "payment:third",
		Source:         "vk_miniapp",
		ForceNew:       true,
	})
	if err != nil {
		t.Fatalf("force new intent: %v", err)
	}
	if !forced.Created || forced.ReusedActive || forced.Intent.ID == first.Intent.ID {
		t.Fatalf("expected new forced intent, got %+v first=%+v", forced, first)
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

	repeated, err := processor.ReconcilePendingOlderThan(ctx, 10, -time.Nanosecond)
	if err != nil {
		t.Fatalf("repeat reconcile pending: %v", err)
	}
	if repeated.Checked != 0 || repeated.Processed != 0 || repeated.Mismatches != 0 {
		t.Fatalf("unexpected repeated reconciliation result: %+v", repeated)
	}
	acc, err = billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after repeated reconcile: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after repeated reconcile = %d, want 100", acc.BalanceCached)
	}
}

func TestWebhookProcessorLateSucceededWebhookAfterReconciliationDoesNotDoubleTopup(t *testing.T) {
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
		IdempotencyKey: "late-webhook-intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set provider status: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	if _, err := processor.ReconcilePendingOlderThan(ctx, 10, -time.Nanosecond); err != nil {
		t.Fatalf("reconcile pending: %v", err)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after reconcile = %d, want 100", acc.BalanceCached)
	}

	if _, _, err := processor.IngestWebhook(ctx, []byte(`{"event_type":"payment.succeeded","provider_payment_id":"`+created.Intent.ProviderPaymentID+`"}`), nil); err != nil {
		t.Fatalf("ingest late webhook: %v", err)
	}
	processed, err := processor.ProcessBatch(ctx, 10)
	if err != nil {
		t.Fatalf("process late webhook: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	acc, err = billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after late webhook: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after late webhook = %d, want unchanged 100", acc.BalanceCached)
	}
	events, err := repo.ListUnprocessedEvents(ctx, domain.PaymentProviderMock, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("unprocessed events = %d, want 0", len(events))
	}
}

func TestWebhookProcessorReconcilePendingCanceledMarksIntentWithoutTopup(t *testing.T) {
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
		IdempotencyKey: "reconcile-canceled-intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentCanceled); err != nil {
		t.Fatalf("set canceled: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	result, err := processor.ReconcilePendingOlderThan(ctx, 10, -time.Nanosecond)
	if err != nil {
		t.Fatalf("reconcile canceled: %v", err)
	}
	if result.Checked != 1 || result.Processed != 1 || result.Mismatches != 0 {
		t.Fatalf("unexpected reconciliation result: %+v", result)
	}
	intent, err := repo.GetIntentByID(ctx, created.Intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if intent.Status != domain.PaymentIntentCanceled {
		t.Fatalf("intent status = %s, want canceled", intent.Status)
	}
	if _, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("billing account err = %v, want ErrNotFound", err)
	}
}

func TestWebhookProcessorReconcileUserClosedPaymentKeepsWaitingWithoutTopup(t *testing.T) {
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
		IdempotencyKey: "user-closed-payment-intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	result, err := processor.ReconcilePendingOlderThan(ctx, 10, -time.Nanosecond)
	if err != nil {
		t.Fatalf("reconcile user-closed payment: %v", err)
	}
	if result.Checked != 1 || result.Processed != 1 || result.Mismatches != 0 {
		t.Fatalf("unexpected reconciliation result: %+v", result)
	}
	intent, err := repo.GetIntentByID(ctx, created.Intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if intent.Status != domain.PaymentIntentWaitingForUser {
		t.Fatalf("intent status = %s, want waiting_for_user", intent.Status)
	}
	if _, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("billing account err = %v, want ErrNotFound", err)
	}
}

func TestWebhookProcessorCanceledMarksIntentWithoutTopup(t *testing.T) {
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
		IdempotencyKey: "canceled-intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentCanceled); err != nil {
		t.Fatalf("set canceled: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	if _, _, err := processor.IngestWebhook(ctx, []byte(`{"event_type":"payment.canceled","provider_payment_id":"`+created.Intent.ProviderPaymentID+`"}`), nil); err != nil {
		t.Fatalf("ingest canceled: %v", err)
	}
	processed, err := processor.ProcessBatch(ctx, 10)
	if err != nil {
		t.Fatalf("process canceled: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	intent, err := repo.GetIntentByID(ctx, created.Intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if intent.Status != domain.PaymentIntentCanceled {
		t.Fatalf("intent status = %s, want canceled", intent.Status)
	}
	if _, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("billing account err = %v, want ErrNotFound", err)
	}
}

func TestWebhookProcessorRefundSucceededIsInboxOnly(t *testing.T) {
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
		IdempotencyKey: "refund-intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set succeeded: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	if _, _, err := processor.IngestWebhook(ctx, []byte(`{"event_type":"payment.succeeded","provider_payment_id":"`+created.Intent.ProviderPaymentID+`"}`), nil); err != nil {
		t.Fatalf("ingest succeeded: %v", err)
	}
	if _, err := processor.ProcessBatch(ctx, 10); err != nil {
		t.Fatalf("process succeeded: %v", err)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after topup = %d, want 100", acc.BalanceCached)
	}

	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentRefunded); err != nil {
		t.Fatalf("set refunded: %v", err)
	}
	if _, _, err := processor.IngestWebhook(ctx, []byte(`{"event_type":"refund.succeeded","provider_payment_id":"`+created.Intent.ProviderPaymentID+`","provider_refund_id":"refund-1"}`), nil); err != nil {
		t.Fatalf("ingest refund: %v", err)
	}
	processed, err := processor.ProcessBatch(ctx, 10)
	if err != nil {
		t.Fatalf("process refund: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed refund = %d, want 1", processed)
	}
	acc, err = billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after refund webhook: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after refund webhook = %d, want unchanged 100", acc.BalanceCached)
	}
	intent, err := repo.GetIntentByID(ctx, created.Intent.ID)
	if err != nil {
		t.Fatalf("get intent after refund webhook: %v", err)
	}
	if intent.Status != domain.PaymentIntentSucceeded {
		t.Fatalf("intent status after refund webhook = %s, want succeeded until manual refund policy applies", intent.Status)
	}
	events, err := repo.ListUnprocessedEvents(ctx, domain.PaymentProviderMock, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("unprocessed events = %d, want 0", len(events))
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

func TestRefundIntentRejectsSpentTopupCreditsWithoutFIFO(t *testing.T) {
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
		IdempotencyKey: "spent-refund-intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set provider status: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	if _, err := processor.ReconcilePendingOlderThan(ctx, 10, -time.Nanosecond); err != nil {
		t.Fatalf("reconcile pending: %v", err)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if err := billingRepo.AppendEntry(ctx, &domain.LedgerEntry{
		AccountID:      acc.ID,
		Type:           domain.LedgerAdjustment,
		Amount:         -10,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: "test-spend-after-topup",
		Reason:         "test spend after top-up",
	}); err != nil {
		t.Fatalf("append spend: %v", err)
	}
	if err := billingRepo.AppendEntry(ctx, &domain.LedgerEntry{
		AccountID:      acc.ID,
		Type:           domain.LedgerAdjustment,
		Amount:         10,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: "test-later-grant",
		Reason:         "test later grant",
	}); err != nil {
		t.Fatalf("append later grant: %v", err)
	}

	_, err = processor.RefundIntent(ctx, paymentservice.RefundIntentInput{
		IntentID:       created.Intent.ID,
		IdempotencyKey: "refund-spent-key",
		Reason:         "operator refund",
	})
	if !errors.Is(err, paymentservice.ErrRefundCreditsSpent) {
		t.Fatalf("refund err = %v, want ErrRefundCreditsSpent", err)
	}
	acc, err = billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after rejected refund: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after rejected refund = %d, want unchanged 100", acc.BalanceCached)
	}
}

func TestRefundIntentReplayIsIdempotent(t *testing.T) {
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
		IdempotencyKey: "idempotent-refund-intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set provider status: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	if _, err := processor.ReconcilePendingOlderThan(ctx, 10, -time.Nanosecond); err != nil {
		t.Fatalf("reconcile pending: %v", err)
	}
	first, err := processor.RefundIntent(ctx, paymentservice.RefundIntentInput{
		IntentID:       created.Intent.ID,
		IdempotencyKey: "refund-idempotent-key",
		Reason:         "operator refund",
	})
	if err != nil {
		t.Fatalf("refund first: %v", err)
	}
	second, err := processor.RefundIntent(ctx, paymentservice.RefundIntentInput{
		IntentID:       created.Intent.ID,
		IdempotencyKey: "refund-idempotent-key",
		Reason:         "operator refund replay",
	})
	if err != nil {
		t.Fatalf("refund replay: %v", err)
	}
	if first.Refund.ID != second.Refund.ID || first.Refund.ProviderRefundID != second.Refund.ProviderRefundID {
		t.Fatalf("replay returned different refund: first=%+v second=%+v", first.Refund, second.Refund)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after replay: %v", err)
	}
	if acc.BalanceCached != 0 {
		t.Fatalf("balance after replay = %d, want 0", acc.BalanceCached)
	}
}

func TestRefundIntentProviderFailureCompensatesDebit(t *testing.T) {
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
		IdempotencyKey: "provider-fail-refund-intent-key",
	})
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := provider.SetPaymentStatus(created.Intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set provider status: %v", err)
	}

	billingRepo := memory.NewBillingRepo()
	processor := newTestWebhookProcessor(repo, provider, billingRepo)
	if _, err := processor.ReconcilePendingOlderThan(ctx, 10, -time.Nanosecond); err != nil {
		t.Fatalf("reconcile pending: %v", err)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance before refund = %d, want 100", acc.BalanceCached)
	}

	failingProvider := &recordingPaymentProvider{
		code:      domain.PaymentProviderMock,
		refundErr: errors.New("provider refund unavailable"),
	}
	failingProcessor := newTestWebhookProcessorWithProvider(repo, failingProvider, billingRepo)
	_, err = failingProcessor.RefundIntent(ctx, paymentservice.RefundIntentInput{
		IntentID:       created.Intent.ID,
		IdempotencyKey: "refund-provider-failure-key",
		Reason:         "operator refund",
	})
	if err == nil {
		t.Fatal("expected provider refund error")
	}
	acc, err = billingRepo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after provider failure: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after provider failure = %d, want compensated 100", acc.BalanceCached)
	}
	refund, err := repo.GetRefundByIdempotencyKey(ctx, "refund-provider-failure-key")
	if err != nil {
		t.Fatalf("get refund: %v", err)
	}
	if refund.Status != domain.PaymentRefundFailed {
		t.Fatalf("refund status = %s, want failed", refund.Status)
	}
}

func newTestWebhookProcessor(repo *memory.PaymentRepo, provider *paymentmock.Provider, billingRepo *memory.BillingRepo) *paymentservice.WebhookProcessor {
	return newTestWebhookProcessorWithProvider(repo, provider, billingRepo)
}

func newTestWebhookProcessorWithProvider(repo *memory.PaymentRepo, provider domain.PaymentProvider, billingRepo *memory.BillingRepo) *paymentservice.WebhookProcessor {
	billing := billingservice.New(billingRepo, billingservice.WithStartingBalance(0))
	tx := paymentservice.TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		return fn(ctx, repo, billingRepo)
	})
	return paymentservice.NewWebhookProcessor(repo, provider, billing, tx)
}

type recordingPaymentProvider struct {
	code         domain.PaymentProviderCode
	createInputs []domain.CreatePaymentInput
	refundInputs []domain.CreateRefundInput
	refundErr    error
}

func (p *recordingPaymentProvider) Code() domain.PaymentProviderCode {
	if p.code == "" {
		return domain.PaymentProviderMock
	}
	return p.code
}

func (p *recordingPaymentProvider) CreatePayment(_ context.Context, in domain.CreatePaymentInput) (domain.CreatePaymentResult, error) {
	p.createInputs = append(p.createInputs, in)
	return domain.CreatePaymentResult{
		ProviderPaymentID: "recording-pay-" + in.IntentID.String(),
		ConfirmationURL:   "https://payments.local/" + in.IntentID.String(),
		Status:            domain.PaymentIntentWaitingForUser,
	}, nil
}

func (p *recordingPaymentProvider) GetPayment(context.Context, string) (domain.ProviderPayment, error) {
	return domain.ProviderPayment{}, domain.ErrNotFound
}

func (p *recordingPaymentProvider) CancelPayment(context.Context, string) error {
	return nil
}

func (p *recordingPaymentProvider) CreateRefund(_ context.Context, in domain.CreateRefundInput) (domain.RefundResult, error) {
	p.refundInputs = append(p.refundInputs, in)
	if p.refundErr != nil {
		return domain.RefundResult{}, p.refundErr
	}
	return domain.RefundResult{
		ProviderRefundID: "recording-refund-" + in.RefundID.String(),
		Status:           domain.PaymentRefundSucceeded,
		Amount:           in.Amount,
		Currency:         in.Currency,
	}, nil
}

func (p *recordingPaymentProvider) ParseWebhook(context.Context, []byte, http.Header) (domain.WebhookEvent, error) {
	return domain.WebhookEvent{}, domain.ErrNotFound
}
