package admin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/inbound/admin"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
)

func setup(t *testing.T) (*admin.Handler, *memory.JobRepo, *memory.UserRepo, *memory.DeliveryRepo, *billingservice.Service) {
	t.Helper()
	jobs := memory.NewJobRepo()
	users := memory.NewUserRepo()
	deliveries := memory.NewDeliveryRepo()
	billingRepo := memory.NewBillingRepo()
	billing := billingservice.New(billingRepo)
	h := admin.NewHandler(admin.Config{}, admin.Deps{
		Jobs:       jobs,
		Users:      users,
		Deliveries: deliveries,
		Referrals:  memory.NewReferralRepo(),
		Billing:    billingRepo,
	})
	return h, jobs, users, deliveries, billing
}

func do(t *testing.T, h *admin.Handler, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	var body map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
	}
	return rec, body
}

func TestOverviewReadOnlySafeDTO(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	payments := memory.NewPaymentRepo()
	h := admin.NewHandler(admin.Config{}, admin.Deps{
		Jobs:       jobs,
		Users:      memory.NewUserRepo(),
		Deliveries: memory.NewDeliveryRepo(),
		Referrals:  memory.NewReferralRepo(),
		Payment:    payments,
	})
	userID := uuid.New()
	for _, status := range []domain.JobStatus{
		domain.JobStatusQueued,
		domain.JobStatusProviderProcessing,
		domain.JobStatusFailedRetryable,
	} {
		if err := jobs.Create(ctx, &domain.Job{
			ID:             uuid.New(),
			UserID:         userID,
			OperationType:  domain.OperationVideoGenerate,
			Modality:       domain.ModalityVideo,
			Status:         status,
			IdempotencyKey: uuid.NewString(),
		}); err != nil {
			t.Fatalf("create job: %v", err)
		}
	}
	staleAt := time.Now().Add(-10 * time.Minute)
	if err := payments.CreateIntent(ctx, &domain.PaymentIntent{
		ID:             uuid.New(),
		UserID:         userID,
		Status:         domain.PaymentIntentProviderPending,
		Amount:         100,
		Currency:       domain.CurrencyRUB,
		Credits:        10,
		Provider:       domain.PaymentProviderYooKassa,
		IdempotencyKey: "test-payment-key",
		CreatedAt:      staleAt,
		UpdatedAt:      staleAt,
	}); err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if _, err := payments.CreateEvent(ctx, &domain.PaymentEvent{
		ID:                uuid.New(),
		Provider:          domain.PaymentProviderYooKassa,
		EventType:         "payment.succeeded",
		ProviderPaymentID: "provider-payment-id",
		DedupKey:          "dedup-key",
		Payload:           json.RawMessage(`{"secret":"raw-payload"}`),
	}); err != nil {
		t.Fatalf("create payment event: %v", err)
	}

	rec, _ := do(t, h, "/admin/overview")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var overview admin.OverviewDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &overview); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	required := map[string]bool{
		"api":                    false,
		"vk_bot":                 false,
		"miniapp":                false,
		"workers":                false,
		"provider_webhook":       false,
		"queue_backlog":          false,
		"active_alerts":          false,
		"provider_health":        false,
		"media_safety":           false,
		"payment_reconciliation": false,
	}
	for _, card := range overview.Cards {
		if _, ok := required[card.ID]; ok {
			required[card.ID] = true
		}
		if card.Status == "" || card.Title == "" || card.Summary == "" {
			t.Fatalf("overview card must be bounded and displayable: %+v", card)
		}
	}
	for id, seen := range required {
		if !seen {
			t.Fatalf("missing overview card %q in %+v", id, overview.Cards)
		}
	}
	raw := rec.Body.String()
	for _, forbidden := range []string{
		"user_id",
		"vk_user_id",
		"provider_payment_id",
		"confirmation_url",
		"idempotency",
		"dedup",
		"payload",
		"raw-payload",
		"test-payment-key",
		"provider-payment-id",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("overview DTO leaked forbidden field/value %q: %s", forbidden, raw)
		}
	}
}

func TestListJobsPaginationAndFilter(t *testing.T) {
	h, jobs, _, _, _ := setup(t)
	ctx := context.Background()
	userA := uuid.New()
	for i := 0; i < 3; i++ {
		_ = jobs.Create(ctx, &domain.Job{
			ID: uuid.New(), UserID: userA, OperationType: domain.OperationTextGenerate,
			Modality: domain.ModalityText, Status: domain.JobStatusQueued,
			IdempotencyKey: uuid.NewString(),
		})
	}
	_ = jobs.Create(ctx, &domain.Job{
		ID: uuid.New(), UserID: uuid.New(), OperationType: domain.OperationVideoGenerate,
		Modality: domain.ModalityVideo, Status: domain.JobStatusSucceeded,
		IdempotencyKey: uuid.NewString(),
	})

	// Pagination: limit 2 of the 4 jobs -> has_more true.
	rec, body := do(t, h, "/admin/jobs?limit=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	items := body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	pg := body["pagination"].(map[string]any)
	if pg["has_more"] != true {
		t.Fatalf("expected has_more true, got %v", pg["has_more"])
	}

	// Filter by status.
	_, body = do(t, h, "/admin/jobs?status=succeeded")
	if got := len(body["items"].([]any)); got != 1 {
		t.Fatalf("status filter: expected 1, got %d", got)
	}

	// Filter by user.
	_, body = do(t, h, "/admin/jobs?user_id="+userA.String())
	if got := len(body["items"].([]any)); got != 3 {
		t.Fatalf("user filter: expected 3, got %d", got)
	}
}

func TestGetJobAndNotFound(t *testing.T) {
	h, jobs, _, _, _ := setup(t)
	ctx := context.Background()
	job := &domain.Job{
		ID: uuid.New(), UserID: uuid.New(), OperationType: domain.OperationImageGenerate,
		Modality: domain.ModalityImage, Status: domain.JobStatusQueued, IdempotencyKey: uuid.NewString(),
	}
	_ = jobs.Create(ctx, job)

	rec, body := do(t, h, "/admin/jobs/"+job.ID.String())
	if rec.Code != http.StatusOK || body["id"] != job.ID.String() {
		t.Fatalf("get job failed: %d %v", rec.Code, body)
	}

	rec, _ = do(t, h, "/admin/jobs/"+uuid.NewString())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	rec, _ = do(t, h, "/admin/jobs/not-a-uuid")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad id, got %d", rec.Code)
	}
}

func TestGetUserIncludesBalance(t *testing.T) {
	h, _, users, _, billing := setup(t)
	ctx := context.Background()
	user := &domain.User{VKUserID: 42, Role: domain.RoleUser, Status: domain.StatusActive}
	_ = users.Create(ctx, user)
	if _, err := billing.EnsureAccount(ctx, user.ID); err != nil {
		t.Fatalf("ensure account: %v", err)
	}

	rec, body := do(t, h, "/admin/users/"+user.ID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if body["balance_credits"].(float64) != 1000 {
		t.Fatalf("expected balance 1000, got %v", body["balance_credits"])
	}
}

func TestGetDelivery(t *testing.T) {
	h, _, _, deliveries, _ := setup(t)
	ctx := context.Background()
	del := &domain.Delivery{
		ID: uuid.New(), JobID: uuid.New(), UserID: uuid.New(), VKPeerID: 1,
		Type: domain.DeliveryTypePhoto, Status: domain.DeliveryStatusSent,
		VKRandomID: 123, IdempotencyKey: uuid.NewString(),
	}
	_ = deliveries.Create(ctx, del)

	rec, body := do(t, h, "/admin/deliveries/"+del.ID.String())
	if rec.Code != http.StatusOK || body["id"] != del.ID.String() {
		t.Fatalf("get delivery failed: %d %v", rec.Code, body)
	}
}

func TestAuthTokenRequired(t *testing.T) {
	jobs := memory.NewJobRepo()
	h := admin.NewHandler(admin.Config{Token: "secret"}, admin.Deps{Jobs: jobs, Users: memory.NewUserRepo(), Deliveries: memory.NewDeliveryRepo()})

	req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	req.Header.Set("X-Admin-Token", "secret")
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", rec.Code)
	}
}

func TestReferralStatsByCodeSafeDTO(t *testing.T) {
	ctx := context.Background()
	refs := memory.NewReferralRepo()
	h := admin.NewHandler(admin.Config{}, admin.Deps{
		Jobs:       memory.NewJobRepo(),
		Users:      memory.NewUserRepo(),
		Deliveries: memory.NewDeliveryRepo(),
		Referrals:  refs,
	})
	referrerID := uuid.New()
	if err := refs.CreateCode(ctx, &domain.ReferralCode{UserID: referrerID, Code: "SAFE2345"}); err != nil {
		t.Fatalf("create code: %v", err)
	}
	for _, referral := range []domain.Referral{
		{ReferrerUserID: referrerID, ReferredUserID: uuid.New(), ReferralCode: "SAFE2345", Source: domain.ReferralSourceVKBot, Status: domain.ReferralStatusRegistered, RewardStatus: domain.ReferralRewardPending},
		{ReferrerUserID: referrerID, ReferredUserID: uuid.New(), ReferralCode: "SAFE2345", Source: domain.ReferralSourceVKMiniApp, Status: domain.ReferralStatusActivated, RewardStatus: domain.ReferralRewardPending},
		{ReferrerUserID: referrerID, ReferredUserID: uuid.New(), ReferralCode: "SAFE2345", Source: domain.ReferralSourceVKBot, Status: domain.ReferralStatusRewarded, RewardStatus: domain.ReferralRewardApplied},
	} {
		referral := referral
		if err := refs.CreateReferral(ctx, &referral); err != nil {
			t.Fatalf("create referral: %v", err)
		}
	}

	rec, body := do(t, h, "/admin/referrals/codes/safe2345/stats")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body["code"] != "SAFE2345" ||
		body["invited_count"].(float64) != 3 ||
		body["registered_count"].(float64) != 1 ||
		body["activated_count"].(float64) != 1 ||
		body["rewarded_count"].(float64) != 1 {
		t.Fatalf("unexpected referral stats dto: %#v", body)
	}
	raw := rec.Body.String()
	if strings.Contains(raw, "vk_user_id") || strings.Contains(raw, "user_id") {
		t.Fatalf("referral stats DTO must not expose user ids: %s", raw)
	}

	rec, _ = do(t, h, "/admin/referrals/codes/MISSING1/stats")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing code, got %d", rec.Code)
	}
}

func TestSuspiciousReferralListAndFreezeFutureFlag(t *testing.T) {
	ctx := context.Background()
	refs := memory.NewReferralRepo()
	h := admin.NewHandler(admin.Config{Token: "secret"}, admin.Deps{
		Jobs:       memory.NewJobRepo(),
		Users:      memory.NewUserRepo(),
		Deliveries: memory.NewDeliveryRepo(),
		Referrals:  refs,
	})
	referrerID := uuid.New()
	if err := refs.CreateCode(ctx, &domain.ReferralCode{UserID: referrerID, Code: "SPAM2345"}); err != nil {
		t.Fatalf("create code: %v", err)
	}
	for i := 0; i < 2; i++ {
		referral := &domain.Referral{
			ReferrerUserID: referrerID,
			ReferredUserID: uuid.New(),
			ReferralCode:   "SPAM2345",
			Source:         domain.ReferralSourceVKBot,
			Status:         domain.ReferralStatusRegistered,
			RewardStatus:   domain.ReferralRewardPending,
		}
		if err := refs.CreateReferral(ctx, referral); err != nil {
			t.Fatalf("create suspicious referral: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/referrals/suspicious?min_registered=2&min_total=99", nil)
	req.Header.Set("X-Admin-Token", "secret")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []admin.SuspiciousReferralDTO `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode suspicious list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Code != "SPAM2345" || list.Items[0].RegisteredCount != 2 {
		t.Fatalf("unexpected suspicious list: %+v", list.Items)
	}
	if len(list.Items[0].Reasons) != 1 || list.Items[0].Reasons[0] != "many_registered_not_activated" {
		t.Fatalf("unexpected suspicious reasons: %+v", list.Items[0].Reasons)
	}
	if strings.Contains(rec.Body.String(), "vk_user_id") || strings.Contains(rec.Body.String(), "user_id") {
		t.Fatalf("suspicious referral DTO must not expose user ids: %s", rec.Body.String())
	}

	freezeReq := httptest.NewRequest(http.MethodPost, "/admin/referrals/codes/SPAM2345/freeze", nil)
	freezeReq.Header.Set("X-Admin-Token", "secret")
	freezeRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(freezeRec, freezeReq)
	if freezeRec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 future flag, got %d: %s", freezeRec.Code, freezeRec.Body.String())
	}
	if !strings.Contains(freezeRec.Body.String(), `"enabled":false`) || !strings.Contains(freezeRec.Body.String(), "future_flag") {
		t.Fatalf("unexpected freeze future flag response: %s", freezeRec.Body.String())
	}
}
