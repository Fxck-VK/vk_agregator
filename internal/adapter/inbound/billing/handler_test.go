package billing_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestProductCatalogAdminCRUDAndIntentSnapshots(t *testing.T) {
	handler, users, payments, _, _ := setup(t)
	ctx := context.Background()

	createBody := []byte(`{"code":"credits_250","title":"NeiroHub 250 credits","amount":20000,"currency":"rub","credits":250,"vat_code":1,"payment_subject":"service","payment_mode":"full_prepayment"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/billing/payment-products", bytes.NewReader(createBody))
	createReq.Header.Set("X-Admin-Token", "secret")
	createRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create product: %d %s", createRec.Code, createRec.Body.String())
	}
	var product billing.PaymentProductDTO
	if err := json.Unmarshal(createRec.Body.Bytes(), &product); err != nil {
		t.Fatalf("decode product: %v", err)
	}
	if product.Code != "credits_250" || !product.IsActive || product.PriceVersion != 1 {
		t.Fatalf("unexpected product: %+v", product)
	}

	patchBody := []byte(`{"title":"NeiroHub 260 credits","amount":21000,"credits":260}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/billing/payment-products/"+product.ID.String(), bytes.NewReader(patchBody))
	patchReq.Header.Set("X-Admin-Token", "secret")
	patchRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch product: %d %s", patchRec.Code, patchRec.Body.String())
	}
	var patched billing.PaymentProductDTO
	if err := json.Unmarshal(patchRec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patched product: %v", err)
	}
	if patched.PriceVersion != 2 || patched.Amount != 21000 || patched.Credits != 260 {
		t.Fatalf("patch did not bump snapshot version: %+v", patched)
	}

	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	intentBody := []byte(`{"product_code":"credits_250","receipt_email":"user@example.com"}`)
	intentReq := httptest.NewRequest(http.MethodPost, "/billing/payment-intents", bytes.NewReader(intentBody))
	intentReq.Header.Set("X-Admin-Token", "secret")
	intentReq.Header.Set("X-User-ID", user.ID.String())
	intentReq.Header.Set("X-Idempotency-Key", "catalog-snapshot-key")
	intentRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(intentRec, intentReq)
	if intentRec.Code != http.StatusCreated {
		t.Fatalf("create intent: %d %s", intentRec.Code, intentRec.Body.String())
	}
	var intent billing.PaymentIntentDTO
	if err := json.Unmarshal(intentRec.Body.Bytes(), &intent); err != nil {
		t.Fatalf("decode intent: %v", err)
	}
	if intent.Amount != 21000 || intent.Credits != 260 || intent.PriceVersion != 2 {
		t.Fatalf("intent did not snapshot patched product: %+v", intent)
	}

	secondPatchReq := httptest.NewRequest(http.MethodPatch, "/billing/payment-products/"+product.ID.String(), bytes.NewReader([]byte(`{"amount":22000}`)))
	secondPatchReq.Header.Set("X-Admin-Token", "secret")
	secondPatchRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(secondPatchRec, secondPatchReq)
	if secondPatchRec.Code != http.StatusOK {
		t.Fatalf("second patch product: %d %s", secondPatchRec.Code, secondPatchRec.Body.String())
	}
	storedIntent, err := payments.GetIntentByID(ctx, intent.ID)
	if err != nil {
		t.Fatalf("get stored intent: %v", err)
	}
	if storedIntent.Amount != 21000 || storedIntent.Credits != 260 || storedIntent.PriceVersion != 2 {
		t.Fatalf("existing intent was mutated by catalog update: %+v", storedIntent)
	}

	disableReq := httptest.NewRequest(http.MethodPost, "/billing/payment-products/"+product.ID.String()+"/disable", nil)
	disableReq.Header.Set("X-Admin-Token", "secret")
	disableRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable product: %d %s", disableRec.Code, disableRec.Body.String())
	}
	var disabled billing.PaymentProductDTO
	if err := json.Unmarshal(disableRec.Body.Bytes(), &disabled); err != nil {
		t.Fatalf("decode disabled product: %v", err)
	}
	if disabled.IsActive || disabled.PriceVersion != 3 {
		t.Fatalf("disable should hide product without changing snapshot version: %+v", disabled)
	}

	activeReq := httptest.NewRequest(http.MethodGet, "/billing/payment-products?active=true", nil)
	activeReq.Header.Set("X-Admin-Token", "secret")
	activeRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(activeRec, activeReq)
	if activeRec.Code != http.StatusOK {
		t.Fatalf("list active products: %d %s", activeRec.Code, activeRec.Body.String())
	}
	if bytes.Contains(activeRec.Body.Bytes(), []byte("credits_250")) {
		t.Fatalf("disabled product leaked into active list: %s", activeRec.Body.String())
	}

	inactiveReq := httptest.NewRequest(http.MethodGet, "/billing/payment-products?active=false", nil)
	inactiveReq.Header.Set("X-Admin-Token", "secret")
	inactiveRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(inactiveRec, inactiveReq)
	if inactiveRec.Code != http.StatusOK {
		t.Fatalf("list inactive products: %d %s", inactiveRec.Code, inactiveRec.Body.String())
	}
	if !bytes.Contains(inactiveRec.Body.Bytes(), []byte("credits_250")) {
		t.Fatalf("inactive list missed disabled product: %s", inactiveRec.Body.String())
	}
}

func TestProductCatalogAdminRejectsInvalidAndDuplicateProducts(t *testing.T) {
	handler, _, _, _, _ := setup(t)

	invalidReq := httptest.NewRequest(http.MethodPost, "/billing/payment-products", bytes.NewReader([]byte(`{"code":"Bad Code","title":"Bad","amount":100,"currency":"rub","credits":1}`)))
	invalidReq.Header.Set("X-Admin-Token", "secret")
	invalidRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid product: %d %s", invalidRec.Code, invalidRec.Body.String())
	}

	duplicateReq := httptest.NewRequest(http.MethodPost, "/billing/payment-products", bytes.NewReader([]byte(`{"code":"credits_100","title":"Duplicate","amount":100,"currency":"rub","credits":1}`)))
	duplicateReq.Header.Set("X-Admin-Token", "secret")
	duplicateRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(duplicateRec, duplicateReq)
	if duplicateRec.Code != http.StatusConflict {
		t.Fatalf("duplicate product: %d %s", duplicateRec.Code, duplicateRec.Body.String())
	}
}

func TestOperatorListsPendingIntents(t *testing.T) {
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
	req.Header.Set("X-Idempotency-Key", "pending-key")
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create intent: %d %s", rec.Code, rec.Body.String())
	}

	time.Sleep(2 * time.Millisecond)
	listReq := httptest.NewRequest(http.MethodGet, "/billing/payment-intents/pending?stale_after=1ms&stale_only=true", nil)
	listReq.Header.Set("X-Admin-Token", "secret")
	listRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list pending intents: %d %s", listRec.Code, listRec.Body.String())
	}
	var list struct {
		Items []billing.PaymentIntentDTO `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode pending list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected one pending intent, got %+v", list.Items)
	}
	if list.Items[0].Status != string(domain.PaymentIntentWaitingForUser) || !list.Items[0].Stale {
		t.Fatalf("unexpected pending intent dto: %+v", list.Items[0])
	}
}

func TestOperatorListsUnprocessedPaymentEventsWithoutRawPayload(t *testing.T) {
	handler, _, payments, _, _ := setup(t)
	created, err := payments.CreateEvent(context.Background(), &domain.PaymentEvent{
		Provider:          domain.PaymentProviderMock,
		EventType:         "payment.succeeded",
		ProviderPaymentID: "mock-pay-operator-list",
		DedupKey:          "webhook:mock:payment.succeeded:mock-pay-operator-list",
		Payload:           json.RawMessage(`{"secret":"do-not-return","provider_payment_id":"mock-pay-operator-list"}`),
	})
	if err != nil || !created {
		t.Fatalf("create event created=%v err=%v", created, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/billing/payment-events/unprocessed?provider=mock", nil)
	req.Header.Set("X-Admin-Token", "secret")
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list events: %d %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("payload")) || bytes.Contains(rec.Body.Bytes(), []byte("do-not-return")) {
		t.Fatalf("operator event list leaked raw payload: %s", rec.Body.String())
	}
	var list struct {
		Items []billing.PaymentEventDTO `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode event list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ProviderPaymentID != "mock-pay-operator-list" || list.Items[0].EventType != "payment.succeeded" {
		t.Fatalf("unexpected event dto: %+v", list.Items)
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

	replayReq := httptest.NewRequest(http.MethodPost, "/billing/payment-intents/"+intent.ID.String()+"/refund", bytes.NewReader([]byte(`{"reason":"operator replay"}`)))
	replayReq.Header.Set("X-Admin-Token", "secret")
	replayReq.Header.Set("X-Idempotency-Key", "refund-key")
	replayRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(replayRec, replayReq)
	if replayRec.Code != http.StatusOK {
		t.Fatalf("refund replay: %d %s", replayRec.Code, replayRec.Body.String())
	}
	var replay struct {
		Intent billing.PaymentIntentDTO `json:"intent"`
		Refund billing.PaymentRefundDTO `json:"refund"`
	}
	if err := json.Unmarshal(replayRec.Body.Bytes(), &replay); err != nil {
		t.Fatalf("decode refund replay: %v", err)
	}
	if replay.Refund.ID != refund.Refund.ID {
		t.Fatalf("replay refund id = %s, want %s", replay.Refund.ID, refund.Refund.ID)
	}
	acc, err = billingRepo.GetAccountByUser(ctx, user.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after refund replay: %v", err)
	}
	if acc.BalanceCached != 0 {
		t.Fatalf("balance after refund replay = %d, want 0", acc.BalanceCached)
	}
}

func TestRefundPaymentIntentRejectsSpentCredits(t *testing.T) {
	handler, users, _, provider, billingRepo := setup(t)
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	body := []byte(`{"product_code":"credits_100","receipt_email":"user@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/payment-intents", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "secret")
	req.Header.Set("X-User-ID", user.ID.String())
	req.Header.Set("X-Idempotency-Key", "spent-refund-key")
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
	acc, err := billingRepo.GetAccountByUser(ctx, user.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if err := billingRepo.AppendEntry(ctx, &domain.LedgerEntry{
		AccountID:      acc.ID,
		Type:           domain.LedgerAdjustment,
		Amount:         -10,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: "operator-test-spend",
		Reason:         "test spend after top-up",
	}); err != nil {
		t.Fatalf("append spend: %v", err)
	}
	if err := billingRepo.AppendEntry(ctx, &domain.LedgerEntry{
		AccountID:      acc.ID,
		Type:           domain.LedgerAdjustment,
		Amount:         10,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: "operator-test-later-grant",
		Reason:         "test later grant",
	}); err != nil {
		t.Fatalf("append later grant: %v", err)
	}

	refundReq := httptest.NewRequest(http.MethodPost, "/billing/payment-intents/"+intent.ID.String()+"/refund", bytes.NewReader([]byte(`{"reason":"operator test"}`)))
	refundReq.Header.Set("X-Admin-Token", "secret")
	refundReq.Header.Set("X-Idempotency-Key", "spent-refund-key")
	refundRec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(refundRec, refundReq)
	if refundRec.Code != http.StatusConflict {
		t.Fatalf("refund spent credits: %d %s", refundRec.Code, refundRec.Body.String())
	}
	acc, err = billingRepo.GetAccountByUser(ctx, user.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account after rejected refund: %v", err)
	}
	if acc.BalanceCached != 100 {
		t.Fatalf("balance after rejected refund = %d, want unchanged 100", acc.BalanceCached)
	}
}
