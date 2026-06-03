package admin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
