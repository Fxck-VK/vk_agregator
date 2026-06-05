package miniapp_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	miniappinbound "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
)

// ---------------------------------------------------------------------------
// Sign verification unit tests
// ---------------------------------------------------------------------------

// buildSignedParams constructs a valid VK launch-params query string.
func buildSignedParams(vkUserID int64, appSecret string, ts int64) string {
	params := url.Values{}
	params.Set("vk_user_id", fmt.Sprintf("%d", vkUserID))
	params.Set("vk_app_id", "123456")
	params.Set("vk_ts", fmt.Sprintf("%d", ts))
	params.Set("vk_platform", "desktop_web")
	return buildSignedParamsFromValues(params, appSecret)
}

func buildSignedParamsFromValues(params url.Values, appSecret string) string {
	// Compute sign over sorted vk_* params.
	vkParams := make(url.Values)
	for k, v := range params {
		if strings.HasPrefix(k, "vk_") {
			vkParams[k] = v
		}
	}
	toSign := vkParams.Encode()
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(toSign))
	sign := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	params.Set("sign", sign)

	// Build deterministic query string.
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+url.QueryEscape(params.Get(k)))
	}
	return strings.Join(parts, "&")
}

func TestVerifyLaunchParams_Valid(t *testing.T) {
	const secret = "test-secret"
	raw := buildSignedParams(777, secret, time.Now().Unix())

	params, err := miniappinbound.VerifyLaunchParams(raw, secret, time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if params.Get("vk_user_id") != "777" {
		t.Fatalf("expected vk_user_id=777, got %s", params.Get("vk_user_id"))
	}
}

func TestVerifyLaunchParams_InvalidSign(t *testing.T) {
	const secret = "test-secret"
	raw := buildSignedParams(777, "wrong-secret", time.Now().Unix())

	_, err := miniappinbound.VerifyLaunchParams(raw, secret, time.Hour)
	if err == nil {
		t.Fatal("expected error for invalid sign, got nil")
	}
}

func TestVerifyLaunchParams_Expired(t *testing.T) {
	const secret = "test-secret"
	oldTS := time.Now().Add(-2 * time.Hour).Unix()
	raw := buildSignedParams(777, secret, oldTS)

	_, err := miniappinbound.VerifyLaunchParams(raw, secret, time.Hour)
	if err == nil {
		t.Fatal("expected error for expired params, got nil")
	}
}

func TestVerifyLaunchParams_MissingTimestampWithMaxAge(t *testing.T) {
	const secret = "test-secret"
	params := url.Values{}
	params.Set("vk_user_id", "777")
	params.Set("vk_app_id", "123456")
	params.Set("vk_platform", "desktop_web")
	raw := buildSignedParamsFromValues(params, secret)

	_, err := miniappinbound.VerifyLaunchParams(raw, secret, time.Hour)
	if !errors.Is(err, miniappinbound.ErrMissingTimestamp) {
		t.Fatalf("expected ErrMissingTimestamp, got %v", err)
	}
}

func TestVerifyLaunchParams_InvalidTimestampWithMaxAge(t *testing.T) {
	const secret = "test-secret"
	params := url.Values{}
	params.Set("vk_user_id", "777")
	params.Set("vk_app_id", "123456")
	params.Set("vk_ts", "not-a-timestamp")
	params.Set("vk_platform", "desktop_web")
	raw := buildSignedParamsFromValues(params, secret)

	_, err := miniappinbound.VerifyLaunchParams(raw, secret, time.Hour)
	if !errors.Is(err, miniappinbound.ErrInvalidTimestamp) {
		t.Fatalf("expected ErrInvalidTimestamp, got %v", err)
	}
}

func TestVerifyLaunchParams_EmptySecret_SkipsSignCheck(t *testing.T) {
	// When appSecret is empty, signature check is skipped (dev/mock mode).
	raw := "vk_user_id=42&sign=whatever"
	params, err := miniappinbound.VerifyLaunchParams(raw, "", 0)
	if err != nil {
		t.Fatalf("expected no error in dev mode, got %v", err)
	}
	if params.Get("vk_user_id") != "42" {
		t.Fatalf("expected vk_user_id=42, got %s", params.Get("vk_user_id"))
	}
}

func TestVerifyLaunchParams_MissingUserID(t *testing.T) {
	_, err := miniappinbound.VerifyLaunchParams("sign=abc", "", 0)
	if err == nil {
		t.Fatal("expected error for missing vk_user_id")
	}
}

// ---------------------------------------------------------------------------
// Handler integration tests (in-memory adapters)
// ---------------------------------------------------------------------------

// newTestHandler wires a Handler with in-memory repositories.
func newTestHandler(appSecret string) *miniappinbound.Handler {
	return newTestHandlerWithLimiter(appSecret, nil)
}

func newTestHandlerWithLimiter(appSecret string, limiter interface{ Allow(string) bool }) *miniappinbound.Handler {
	userRepo := memory.NewUserRepo()
	jobRepo := memory.NewJobRepo()
	billingRepo := memory.NewBillingRepo()
	outboxRepo := memory.NewOutboxRepo()
	uowMgr := memory.NewUnitOfWork(jobRepo, outboxRepo, billingRepo)

	billing := billingservice.New(billingRepo)
	orch := joborchestrator.New(jobRepo, uowMgr, billing, 0)

	return miniappinbound.NewHandler(
		miniappinbound.Config{
			AppSecret:          appSecret,
			LaunchParamsMaxAge: time.Hour,
			JobRateLimiter:     limiter,
		},
		miniappinbound.Deps{
			Users:        userRepo,
			Jobs:         jobRepo,
			Billing:      billing,
			BillingRepo:  billingRepo,
			Orchestrator: orch,
		},
	)
}

type countingLimiter struct {
	burst  int
	counts map[string]int
}

func (l *countingLimiter) Allow(key string) bool {
	if l.counts == nil {
		l.counts = map[string]int{}
	}
	l.counts[key]++
	return l.counts[key] <= l.burst
}

func newArtifactHandler(t *testing.T, jobStatus domain.JobStatus, decision *domain.ModerationDecision) (http.Handler, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	userRepo := memory.NewUserRepo()
	jobRepo := memory.NewJobRepo()
	artifactRepo := memory.NewArtifactRepo()
	moderationRepo := memory.NewModerationRepo()
	objects := memory.NewObjectStore()

	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	job := &domain.Job{
		UserID:         user.ID,
		VKPeerID:       user.VKUserID,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         jobStatus,
		IdempotencyKey: "artifact-test:" + uuid.NewString(),
	}
	if err := jobRepo.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	artifact := &domain.Artifact{
		OwnerUserID:   user.ID,
		JobID:         &job.ID,
		Kind:          domain.ArtifactKindOutput,
		MediaType:     domain.MediaTypeText,
		MimeType:      "text/plain",
		StorageBucket: "artifacts",
		StorageKey:    "outputs/" + uuid.NewString() + ".txt",
		SHA256:        uuid.NewString(),
		SizeBytes:     int64(len("safe result")),
		Status:        domain.ArtifactStatusReady,
	}
	if err := artifactRepo.Create(ctx, artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	job.OutputArtifactIDs = []uuid.UUID{artifact.ID}
	if err := jobRepo.Update(ctx, job); err != nil {
		t.Fatalf("update job outputs: %v", err)
	}
	if err := objects.Put(ctx, artifact.StorageBucket, artifact.StorageKey, []byte("safe result"), artifact.MimeType); err != nil {
		t.Fatalf("put object: %v", err)
	}
	if decision != nil {
		artID := artifact.ID
		if err := moderationRepo.Create(ctx, &domain.ModerationResult{
			JobID:      job.ID,
			ArtifactID: &artID,
			Stage:      domain.ModerationStageOutput,
			Decision:   *decision,
			Provider:   "test",
		}); err != nil {
			t.Fatalf("create moderation result: %v", err)
		}
	}

	handler := miniappinbound.NewHandler(miniappinbound.Config{}, miniappinbound.Deps{
		Users:      userRepo,
		Jobs:       jobRepo,
		Artifacts:  artifactRepo,
		Moderation: moderationRepo,
		Objects:    objects,
	})
	return handler.Routes(), artifact.ID
}

// devLaunchParams returns a minimal dev-mode launch-params string (no secret).
func devLaunchParams(vkUserID int64) string {
	return fmt.Sprintf("vk_user_id=%d&vk_ts=%d", vkUserID, time.Now().Unix())
}

func TestHandler_CreateJob_OK(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "hello world",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp["id"] == nil {
		t.Fatal("expected job id in response")
	}
}

func TestHandler_CreateJob_InvalidOperation(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "unknown_op",
		"prompt":    "hello",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandler_CreateJob_MissingPrompt(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{"operation": "text_generate"})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandler_CreateJob_UnauthorizedNoParams(t *testing.T) {
	routes := newTestHandler("real-secret").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "hello",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-Launch-Params header, secret is set → must reject.

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandler_CreateJob_FailClosedOnInvalidVKTimestamp(t *testing.T) {
	const secret = "my-app-secret"

	cases := []struct {
		name string
		raw  string
	}{
		{
			name: "missing",
			raw: buildSignedParamsFromValues(url.Values{
				"vk_user_id":  {"777"},
				"vk_app_id":   {"123456"},
				"vk_platform": {"desktop_web"},
			}, secret),
		},
		{
			name: "invalid",
			raw: buildSignedParamsFromValues(url.Values{
				"vk_user_id":  {"777"},
				"vk_app_id":   {"123456"},
				"vk_ts":       {"not-a-timestamp"},
				"vk_platform": {"desktop_web"},
			}, secret),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			routes := newTestHandler(secret).Routes()
			body, _ := json.Marshal(map[string]string{
				"operation": "text_generate",
				"prompt":    "hello",
			})

			req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Launch-Params", tc.raw)
			req.Header.Set("X-Idempotency-Key", "client-key")
			w := httptest.NewRecorder()
			routes.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
			}
			var errResp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("invalid error response json: %v", err)
			}
			if errResp["error"] != "unauthorized" {
				t.Fatalf("unexpected error body: %s", w.Body.String())
			}

			listReq := httptest.NewRequest(http.MethodGet, "/miniapp/jobs", nil)
			listReq.Header.Set("X-Launch-Params", buildSignedParams(777, secret, time.Now().Unix()))
			listW := httptest.NewRecorder()
			routes.ServeHTTP(listW, listReq)
			if listW.Code != http.StatusOK {
				t.Fatalf("list after rejected create: expected 200, got %d: %s", listW.Code, listW.Body.String())
			}
			var listResp struct {
				Items []any `json:"items"`
			}
			if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
				t.Fatalf("invalid list response json: %v", err)
			}
			if len(listResp.Items) != 0 {
				t.Fatalf("rejected request must not create a job, got %d jobs", len(listResp.Items))
			}
		})
	}
}

func TestHandler_GetJob_OwnershipCheck(t *testing.T) {
	userRepo := memory.NewUserRepo()
	jobRepo := memory.NewJobRepo()
	billingRepo := memory.NewBillingRepo()
	outboxRepo := memory.NewOutboxRepo()
	uowMgr := memory.NewUnitOfWork(jobRepo, outboxRepo, billingRepo)
	billing := billingservice.New(billingRepo)
	orch := joborchestrator.New(jobRepo, uowMgr, billing, 0)

	handler := miniappinbound.NewHandler(
		miniappinbound.Config{},
		miniappinbound.Deps{
			Users:        userRepo,
			Jobs:         jobRepo,
			Billing:      billing,
			BillingRepo:  billingRepo,
			Orchestrator: orch,
		},
	)
	routes := handler.Routes()

	// Create two users.
	user1 := &domain.User{VKUserID: 100, Role: domain.RoleUser, Status: domain.StatusActive, Locale: "ru", Timezone: "UTC"}
	user2 := &domain.User{VKUserID: 200, Role: domain.RoleUser, Status: domain.StatusActive, Locale: "ru", Timezone: "UTC"}
	ctx := context.Background()
	_ = userRepo.Create(ctx, user1)
	_ = userRepo.Create(ctx, user2)

	// Create a job owned by user1.
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         user1.ID,
		VKPeerID:       100,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "test-job-ownership",
	}
	_ = jobRepo.Create(ctx, job)

	// User2 tries to access user1's job → 404.
	req := httptest.NewRequest(http.MethodGet, "/miniapp/jobs/"+job.ID.String(), nil)
	req.Header.Set("X-Launch-Params", devLaunchParams(200))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-user job access, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_GetArtifact_GuardsSucceededAndModerationPassed(t *testing.T) {
	allow := domain.ModerationAllow
	block := domain.ModerationBlock
	tests := []struct {
		name     string
		status   domain.JobStatus
		decision *domain.ModerationDecision
		wantCode int
		wantBody string
	}{
		{
			name:     "result ready is not enough",
			status:   domain.JobStatusResultReady,
			decision: &allow,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "succeeded without moderation is hidden",
			status:   domain.JobStatusSucceeded,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "succeeded blocked moderation is hidden",
			status:   domain.JobStatusSucceeded,
			decision: &block,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "succeeded allowed moderation returns bytes",
			status:   domain.JobStatusSucceeded,
			decision: &allow,
			wantCode: http.StatusOK,
			wantBody: "safe result",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			routes, artifactID := newArtifactHandler(t, tc.status, tc.decision)
			req := httptest.NewRequest(http.MethodGet, "/miniapp/artifacts/"+artifactID.String(), nil)
			req.Header.Set("X-Launch-Params", devLaunchParams(777))
			w := httptest.NewRecorder()

			routes.ServeHTTP(w, req)

			if w.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, w.Code, w.Body.String())
			}
			if tc.wantBody != "" && w.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", w.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestHandler_GetBalance(t *testing.T) {
	routes := newTestHandler("").Routes()

	req := httptest.NewRequest(http.MethodGet, "/miniapp/balance", nil)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		BalanceCredits int64 `json:"balance_credits"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	// New user gets the default starting balance.
	if resp.BalanceCredits != billingservice.DefaultStartingBalance {
		t.Fatalf("expected %d credits, got %d", billingservice.DefaultStartingBalance, resp.BalanceCredits)
	}
}

func TestHandler_ValidSign(t *testing.T) {
	const secret = "my-app-secret"
	routes := newTestHandler(secret).Routes()

	raw := buildSignedParams(777, secret, time.Now().Unix())
	req := httptest.NewRequest(http.MethodGet, "/miniapp/balance", nil)
	req.Header.Set("X-Launch-Params", raw)

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid sign, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_InvalidSign(t *testing.T) {
	const secret = "my-app-secret"
	routes := newTestHandler(secret).Routes()

	// Build params signed with a different secret.
	raw := buildSignedParams(777, "wrong-secret", time.Now().Unix())
	req := httptest.NewRequest(http.MethodGet, "/miniapp/balance", nil)
	req.Header.Set("X-Launch-Params", raw)

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong sign, got %d", w.Code)
	}
}

func TestHandler_ListJobs_Empty(t *testing.T) {
	routes := newTestHandler("").Routes()

	req := httptest.NewRequest(http.MethodGet, "/miniapp/jobs", nil)
	req.Header.Set("X-Launch-Params", devLaunchParams(999))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected empty items, got %d", len(resp.Items))
	}
}

func TestHandler_CreateJob_Idempotency(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "idempotent prompt",
	})

	makeReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Launch-Params", devLaunchParams(777))
		req.Header.Set("X-Idempotency-Key", "client-key-xyz")
		return req
	}

	w1 := httptest.NewRecorder()
	routes.ServeHTTP(w1, makeReq())
	if w1.Code != http.StatusCreated {
		t.Fatalf("first call: expected 201, got %d: %s", w1.Code, w1.Body.String())
	}

	w2 := httptest.NewRecorder()
	routes.ServeHTTP(w2, makeReq())
	if w2.Code != http.StatusCreated {
		t.Fatalf("second call (idempotent): expected 201, got %d: %s", w2.Code, w2.Body.String())
	}

	var r1, r2 map[string]any
	_ = json.Unmarshal(w1.Body.Bytes(), &r1)
	_ = json.Unmarshal(w2.Body.Bytes(), &r2)
	if r1["id"] != r2["id"] {
		t.Fatalf("idempotent requests must return the same job id: %v != %v", r1["id"], r2["id"])
	}
}

func TestHandler_CreateJob_AcceptsSupportedModelID(t *testing.T) {
	userRepo := memory.NewUserRepo()
	jobRepo := memory.NewJobRepo()
	billingRepo := memory.NewBillingRepo()
	outboxRepo := memory.NewOutboxRepo()
	uowMgr := memory.NewUnitOfWork(jobRepo, outboxRepo, billingRepo)
	billing := billingservice.New(billingRepo)
	orch := joborchestrator.New(jobRepo, uowMgr, billing, 0)

	handler := miniappinbound.NewHandler(
		miniappinbound.Config{},
		miniappinbound.Deps{
			Users:        userRepo,
			Jobs:         jobRepo,
			Billing:      billing,
			BillingRepo:  billingRepo,
			Orchestrator: orch,
		},
	)
	routes := handler.Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "model prompt",
		"model_id":  "gpt-4o-mini",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if _, ok := resp["model_id"]; ok {
		t.Fatalf("job response must not expose model_id: %s", w.Body.String())
	}
	idRaw, ok := resp["id"].(string)
	if !ok {
		t.Fatalf("expected job id in response: %s", w.Body.String())
	}
	jobID, err := uuid.Parse(idRaw)
	if err != nil {
		t.Fatalf("invalid job id: %v", err)
	}
	job, err := jobRepo.GetByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("expected stored job: %v", err)
	}
	var params struct {
		Prompt  string `json:"prompt"`
		ModelID string `json:"model_id"`
	}
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatalf("invalid job params: %v", err)
	}
	if params.ModelID != "gpt-4o-mini" {
		t.Fatalf("expected model_id persisted in params, got %q", params.ModelID)
	}
}

func TestHandler_CreateJob_RejectsUnsupportedModelID(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "model prompt",
		"model_id":  "kling",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("invalid error response json: %v", err)
	}
	if errResp["error"] != "unsupported model" {
		t.Fatalf("unexpected error body: %s", w.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/miniapp/jobs", nil)
	listReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	listW := httptest.NewRecorder()
	routes.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list after rejected create: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var listResp struct {
		Items []any `json:"items"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("invalid list response json: %v", err)
	}
	if len(listResp.Items) != 0 {
		t.Fatalf("rejected model must not create a job, got %d jobs", len(listResp.Items))
	}
}

func TestHandler_CreateJob_RateLimitByVerifiedUserID(t *testing.T) {
	limiter := &countingLimiter{burst: 1}
	routes := newTestHandlerWithLimiter("", limiter).Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "limited prompt",
	})
	makeReq := func(vkUserID int64) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Launch-Params", devLaunchParams(vkUserID))
		return req
	}

	w1 := httptest.NewRecorder()
	routes.ServeHTTP(w1, makeReq(777))
	if w1.Code != http.StatusCreated {
		t.Fatalf("first user request: expected 201, got %d: %s", w1.Code, w1.Body.String())
	}

	w2 := httptest.NewRecorder()
	routes.ServeHTTP(w2, makeReq(777))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second same-user request: expected 429, got %d: %s", w2.Code, w2.Body.String())
	}
	if got := w2.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("expected Retry-After=1, got %q", got)
	}
	var errResp map[string]string
	if err := json.Unmarshal(w2.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("invalid 429 response json: %v", err)
	}
	if errResp["error"] != "rate limit exceeded" {
		t.Fatalf("unexpected 429 body: %s", w2.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/miniapp/jobs", nil)
	listReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	listW := httptest.NewRecorder()
	routes.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list after rate limit: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var listResp struct {
		Items []any `json:"items"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("invalid list response json: %v", err)
	}
	if len(listResp.Items) != 1 {
		t.Fatalf("rate-limited request must not create a new job, got %d jobs", len(listResp.Items))
	}

	w3 := httptest.NewRecorder()
	routes.ServeHTTP(w3, makeReq(888))
	if w3.Code != http.StatusCreated {
		t.Fatalf("different user request: expected 201, got %d: %s", w3.Code, w3.Body.String())
	}
	if limiter.counts["miniapp_job:777"] != 2 || limiter.counts["miniapp_job:888"] != 1 {
		t.Fatalf("limiter keys = %#v, want verified vk_user_id counts", limiter.counts)
	}
}
