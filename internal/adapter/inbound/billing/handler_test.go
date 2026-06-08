package billing_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vk-ai-aggregator/internal/adapter/inbound/billing"
	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/paymentservice"
)

func setup(t *testing.T) (*billing.Handler, *memory.UserRepo, *memory.PaymentRepo, *paymentmock.Provider, *memory.BillingRepo) {
	t.Helper()
	users := memory.NewUserRepo()
	payments := memory.NewPaymentRepo()
	billingRepo := memory.NewBillingRepo()
	provider := paymentmock.New()
	payments.PutProduct(&domain.PaymentProduct{
		Code:         "credits_100",
		Title:        "100 credits",
		Amount:       9900,
		Currency:     domain.CurrencyRUB,
		Credits:      100,
		PriceVersion: 1,
		IsActive:     true,
	})
	service := paymentservice.New(payments, provider, paymentservice.Config{
		ReturnURL: "https://neiirohub.ru/payments/return",
	})
	billingSvc := billingservice.New(billingRepo, billingservice.WithStartingBalance(0))
	tx := paymentservice.TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		return fn(ctx, payments, billingRepo)
	})
	ops := paymentservice.NewWebhookProcessor(payments, provider, billingSvc, tx)
	handler := billing.NewHandler(billing.Config{Token: "secret"}, billing.Deps{
		Users:      users,
		Payment:    service,
		PaymentOps: ops,
	})
	return handler, users, payments, provider, billingRepo
}

func TestBillingAuthFailClosed(t *testing.T) {
	handler := billing.NewHandler(billing.Config{}, billing.Deps{})
	req := httptest.NewRequest(http.MethodGet, "/billing/payment-history", nil)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when token is not configured, got %d", rec.Code)
	}

	handler, _, _, _, _ = setup(t)
	req = httptest.NewRequest(http.MethodGet, "/billing/payment-history", nil)
	rec = httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token header, got %d", rec.Code)
	}
}

func TestCreatePaymentIntentUsesTrustedUserIDAndIsIdempotent(t *testing.T) {
	handler, users, _, _, _ := setup(t)
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	body := []byte(`{"user_id":"00000000-0000-0000-0000-000000000000","product_code":"credits_100","receipt_email":"user@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/payment-intents", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "secret")
	req.Header.Set("X-User-ID", user.ID.String())
	req.Header.Set("X-Idempotency-Key", "client-key")
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var first billing.PaymentIntentDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if first.UserID != user.ID || first.ProviderPaymentID == "" || first.ConfirmationURL == "" {
		t.Fatalf("unexpected operator payment dto: %+v", first)
	}

	replay := httptest.NewRequest(http.MethodPost, "/billing/payment-intents", bytes.NewReader(body))
	replay.Header.Set("X-Admin-Token", "secret")
	replay.Header.Set("X-User-ID", user.ID.String())
	replay.Header.Set("X-Idempotency-Key", "client-key")
	replayRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(replayRec, replay)
	if replayRec.Code != http.StatusOK {
		t.Fatalf("expected replay 200, got %d: %s", replayRec.Code, replayRec.Body.String())
	}
	var second billing.PaymentIntentDTO
	if err := json.Unmarshal(replayRec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode replay: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("replay created a new intent: %s != %s", second.ID, first.ID)
	}
}

func TestPaymentHistoryListsIntents(t *testing.T) {
	handler, users, _, _, _ := setup(t)
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	body := []byte(`{"product_code":"credits_100","receipt_email":"user@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/payment-intents", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "secret")
	req.Header.Set("X-User-ID", user.ID.String())
	req.Header.Set("X-Idempotency-Key", "history-key")
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create intent: %d %s", rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/billing/payment-history?user_id="+user.ID.String(), nil)
	listReq.Header.Set("X-Admin-Token", "secret")
	listRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var list struct {
		Items []billing.PaymentIntentDTO `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].UserID != user.ID {
		t.Fatalf("unexpected list: %+v", list.Items)
	}
}

func TestSyncAndRefundPaymentIntent(t *testing.T) {
	handler, users, payments, provider, billingRepo := setup(t)
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	body := []byte(`{"product_code":"credits_100","receipt_email":"user@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/payment-intents", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "secret")
	req.Header.Set("X-User-ID", user.ID.String())
	req.Header.Set("X-Idempotency-Key", "sync-refund-key")
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create intent: %d %s", rec.Code, rec.Body.String())
	}
	var intent billing.PaymentIntentDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &intent); err != nil {
		t.Fatalf("decode intent: %v", err)
	}
	if err := provider.SetPaymentStatus(intent.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set provider success: %v", err)
	}

	syncReq := httptest.NewRequest(http.MethodPost, "/billing/payment-intents/"+intent.ID.String()+"/sync", nil)
	syncReq.Header.Set("X-Admin-Token", "secret")
	syncRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(syncRec, syncReq)
	if syncRec.Code != http.StatusOK {
		t.Fatalf("sync intent: %d %s", syncRec.Code, syncRec.Body.String())
	}
	var synced billing.PaymentIntentDTO
	if err := json.Unmarshal(syncRec.Body.Bytes(), &synced); err != nil {
		t.Fatalf("decode synced: %v", err)
	}
	if synced.Status != string(domain.PaymentIntentSucceeded) {
		t.Fatalf("synced status = %s", synced.Status)
	}
	acc, err := billingRepo.GetAccountByUser(ctx, user.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after sync = %d, want 100", acc.BalanceCached)
	}

	refundReq := httptest.NewRequest(http.MethodPost, "/billing/payment-intents/"+intent.ID.String()+"/refund", bytes.NewReader([]byte(`{"reason":"operator test"}`)))
	refundReq.Header.Set("X-Admin-Token", "secret")
	refundReq.Header.Set("X-Idempotency-Key", "refund-key")
	refundRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(refundRec, refundReq)
	if refundRec.Code != http.StatusOK {
		t.Fatalf("refund intent: %d %s", refundRec.Code, refundRec.Body.String())
	}
	var refund struct {
		Intent billing.PaymentIntentDTO `json:"intent"`
		Refund billing.PaymentRefundDTO `json:"refund"`
	}
	if err := json.Unmarshal(refundRec.Body.Bytes(), &refund); err != nil {
		t.Fatalf("decode refund: %v", err)
	}
	if refund.Intent.Status != string(domain.PaymentIntentRefunded) || refund.Refund.Status != string(domain.PaymentRefundSucceeded) {
		t.Fatalf("unexpected refund response: %+v", refund)
	}
	acc, err = billingRepo.GetAccountByUser(ctx, user.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after refund: %v", err)
	}
	if acc.BalanceCached != 0 {
		t.Fatalf("balance after refund = %d, want 0", acc.BalanceCached)
	}
	if _, err := payments.GetRefundByIdempotencyKey(ctx, "billing_refund:"+intent.ID.String()+":refund-key"); err != nil {
		t.Fatalf("refund row not stored: %v", err)
	}
}
