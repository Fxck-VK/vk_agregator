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
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	miniappinbound "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/referralservice"
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

func TestVerifyLaunchParams_AllowsSmallFutureClockSkew(t *testing.T) {
	const secret = "test-secret"
	futureTS := time.Now().Add(2 * time.Minute).Unix()
	raw := buildSignedParams(777, secret, futureTS)

	params, err := miniappinbound.VerifyLaunchParams(raw, secret, time.Hour)
	if err != nil {
		t.Fatalf("expected small future skew to pass, got %v", err)
	}
	if got := params.Get("vk_user_id"); got != "777" {
		t.Fatalf("unexpected vk_user_id %q", got)
	}
}

func TestVerifyLaunchParams_RejectsLargeFutureTimestamp(t *testing.T) {
	const secret = "test-secret"
	futureTS := time.Now().Add(10 * time.Minute).Unix()
	raw := buildSignedParams(777, secret, futureTS)

	_, err := miniappinbound.VerifyLaunchParams(raw, secret, time.Hour)
	if !errors.Is(err, miniappinbound.ErrInvalidTimestamp) {
		t.Fatalf("expected ErrInvalidTimestamp, got %v", err)
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
	return newTestFixture(appSecret, limiter).handler
}

type testFixture struct {
	handler          *miniappinbound.Handler
	userRepo         *memory.UserRepo
	jobRepo          *memory.JobRepo
	conversationRepo *memory.ConversationRepo
	artifactRepo     *memory.ArtifactRepo
	moderationRepo   *memory.ModerationRepo
	objects          *memory.ObjectStore
	billingRepo      *memory.BillingRepo
	billing          *billingservice.Service
	paymentRepo      *memory.PaymentRepo
	payment          *paymentservice.Service
	referralRepo     *memory.ReferralRepo
	referrals        *referralservice.Service
}

func newTestFixture(appSecret string, limiter interface{ Allow(string) bool }) *testFixture {
	return newTestFixtureWithConfig(appSecret, limiter, nil)
}

func newTestFixtureWithConfig(appSecret string, limiter interface{ Allow(string) bool }, configure func(*miniappinbound.Config)) *testFixture {
	return newTestFixtureWithConfigAndPaymentProvider(appSecret, limiter, configure, nil)
}

func newTestFixtureWithConfigAndPaymentProvider(appSecret string, limiter interface{ Allow(string) bool }, configure func(*miniappinbound.Config), payProvider domain.PaymentProvider) *testFixture {
	userRepo := memory.NewUserRepo()
	jobRepo := memory.NewJobRepo()
	conversationRepo := memory.NewConversationRepo()
	artifactRepo := memory.NewArtifactRepo()
	moderationRepo := memory.NewModerationRepo()
	objects := memory.NewObjectStore()
	billingRepo := memory.NewBillingRepo()
	paymentRepo := memory.NewPaymentRepo()
	referralRepo := memory.NewReferralRepo()
	outboxRepo := memory.NewOutboxRepo()
	uowMgr := memory.NewUnitOfWork(jobRepo, outboxRepo, billingRepo)

	billing := billingservice.New(billingRepo)
	if payProvider == nil {
		payProvider = paymentmock.New()
	}
	payment := paymentservice.New(paymentRepo, payProvider, paymentservice.Config{
		ReturnURL: "https://neiirohub.ru/payments/return",
	})
	referrals := referralservice.New(referralRepo, billing, referralservice.Config{
		ReferrerSignupRewardCredits: 10,
		RewardOnActivation:          true,
	})
	cfg := miniappinbound.Config{
		AppSecret:                           appSecret,
		LaunchParamsMaxAge:                  time.Hour,
		JobRateLimiter:                      limiter,
		ReferralLinkBase:                    "https://vk.com/write-239332376",
		ReferralReferrerSignupRewardCredits: 10,
		PaymentReturnURL:                    "https://neiirohub.ru/payments/return",
	}
	if configure != nil {
		configure(&cfg)
	}
	orchOptions := []joborchestrator.Option{}
	if cfg.VideoRouteResolver != nil {
		orchOptions = append(orchOptions, joborchestrator.WithVideoRouteResolver(cfg.VideoRouteResolver))
	}
	orch := joborchestrator.New(jobRepo, uowMgr, billing, 0, orchOptions...)
	handler := miniappinbound.NewHandler(
		cfg,
		miniappinbound.Deps{
			Users:         userRepo,
			Jobs:          jobRepo,
			Conversations: conversationRepo,
			Artifacts:     artifactRepo,
			Moderation:    moderationRepo,
			Objects:       objects,
			Billing:       billing,
			BillingRepo:   billingRepo,
			Payment:       payment,
			Referrals:     referrals,
			Orchestrator:  orch,
		},
	)
	return &testFixture{
		handler:          handler,
		userRepo:         userRepo,
		jobRepo:          jobRepo,
		conversationRepo: conversationRepo,
		artifactRepo:     artifactRepo,
		moderationRepo:   moderationRepo,
		objects:          objects,
		billingRepo:      billingRepo,
		billing:          billing,
		paymentRepo:      paymentRepo,
		payment:          payment,
		referralRepo:     referralRepo,
		referrals:        referrals,
	}
}

type recordingPaymentProvider struct {
	createInputs []domain.CreatePaymentInput
}

func (p *recordingPaymentProvider) Code() domain.PaymentProviderCode {
	return domain.PaymentProviderMock
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

func (p *recordingPaymentProvider) CreateRefund(context.Context, domain.CreateRefundInput) (domain.RefundResult, error) {
	return domain.RefundResult{}, domain.ErrNotFound
}

func (p *recordingPaymentProvider) ParseWebhook(context.Context, []byte, http.Header) (domain.WebhookEvent, error) {
	return domain.WebhookEvent{}, domain.ErrNotFound
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

func TestHandler_ClientEvent_DisabledNoops(t *testing.T) {
	fixture := newTestFixture("", nil)
	req := httptest.NewRequest(http.MethodPost, "/miniapp/client-events", bytes.NewReader([]byte(`{"event_type":"api_failure","route":"/miniapp/jobs","status":"500"}`)))
	req.Header.Set("X-VK-User-ID", "777")

	w := httptest.NewRecorder()
	fixture.handler.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ClientEvent_AcceptsSafeTelemetry(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.FrontendTelemetryEnabled = true
		cfg.FrontendTelemetryUserHashSecret = "test-secret"
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/client-events", bytes.NewReader([]byte(`{"event_type":"js_error","screen":"global","error_class":"TypeError"}`)))
	req.Header.Set("X-VK-User-ID", "777")

	w := httptest.NewRecorder()
	fixture.handler.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ClientEvent_RejectsPromptField(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.FrontendTelemetryEnabled = true
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/client-events", bytes.NewReader([]byte(`{"event_type":"api_failure","prompt":"do not collect"}`)))
	req.Header.Set("X-VK-User-ID", "777")

	w := httptest.NewRecorder()
	fixture.handler.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
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

func multipartUploadBody(t *testing.T, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "input.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func pngBytes() []byte {
	return pngSizedBytes(1, 1)
}

func pngSizedBytes(width, height int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func minimalWebPBytes() []byte {
	return []byte{'R', 'I', 'F', 'F', 4, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}
}

func createTestArtifact(t *testing.T, fixture *testFixture, ownerID uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, status domain.ArtifactStatus) *domain.Artifact {
	return createTestArtifactWithDimensions(t, fixture, ownerID, kind, mediaType, status, 0, 0)
}

func createTestArtifactWithDimensions(t *testing.T, fixture *testFixture, ownerID uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, status domain.ArtifactStatus, width, height int) *domain.Artifact {
	t.Helper()
	data := pngBytes()
	if width > 0 && height > 0 {
		data = pngSizedBytes(width, height)
	}
	artifact := &domain.Artifact{
		OwnerUserID:   ownerID,
		Kind:          kind,
		MediaType:     mediaType,
		MimeType:      "image/png",
		StorageBucket: "artifacts",
		StorageKey:    "inputs/" + uuid.NewString() + ".png",
		SHA256:        uuid.NewString(),
		SizeBytes:     int64(len(data)),
		Status:        status,
		Width:         width,
		Height:        height,
	}
	if mediaType != domain.MediaTypeImage {
		artifact.MimeType = "text/plain"
	}
	if err := fixture.artifactRepo.Create(context.Background(), artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	if err := fixture.objects.Put(context.Background(), artifact.StorageBucket, artifact.StorageKey, data, artifact.MimeType); err != nil {
		t.Fatalf("store artifact bytes: %v", err)
	}
	return artifact
}

func enableTestVideoRoute(cfg *miniappinbound.Config, alias domain.VideoRouteAlias) {
	cfg.VideoRoutes = []miniappinbound.VideoRouteDTO{
		{
			Alias:                  string(alias),
			AllowedDurationsSec:    []int{5, 10},
			DefaultDurationSec:     5,
			AllowedResolutions:     []string{"720p"},
			AllowedAspectRatios:    []string{"16:9", "9:16", "1:1"},
			SupportsReferenceImage: true,
			MaxReferenceImages:     1,
		},
	}
	cfg.VideoRouteResolver = joborchestrator.VideoRouteResolverFunc(func(_ context.Context, in joborchestrator.VideoRouteCheckInput) (joborchestrator.VideoRouteResolution, error) {
		var params struct {
			VideoRouteAlias string `json:"video_route_alias"`
			DurationSec     int    `json:"duration_sec"`
			AspectRatio     string `json:"aspect_ratio"`
		}
		if err := json.Unmarshal(in.Params, &params); err != nil {
			return joborchestrator.VideoRouteResolution{}, err
		}
		if params.VideoRouteAlias != string(alias) {
			return joborchestrator.VideoRouteResolution{}, domain.ErrNotFound
		}
		duration := params.DurationSec
		if duration == 0 {
			duration = 5
		}
		aspectRatio := strings.TrimSpace(params.AspectRatio)
		if aspectRatio == "" {
			aspectRatio = "16:9"
		}
		snapshot := domain.VideoRouteSnapshot{
			Alias:                  alias,
			Provider:               domain.ProviderPoYo,
			ProviderModelID:        "hidden-provider-model",
			ModelClass:             "test_video_route",
			DurationSec:            duration,
			Resolution:             "720p",
			AspectRatio:            aspectRatio,
			ProviderCostCredits:    int64(duration),
			InternalCostCredits:    int64(duration * 2),
			PriceMultiplier:        2,
			MaxProviderCostCredits: 10,
			MaxInternalCostCredits: 20,
		}
		return joborchestrator.VideoRouteResolution{
			Resolved:            true,
			Params:              in.Params,
			Snapshot:            snapshot,
			InternalCostCredits: snapshot.InternalCostCredits,
		}, nil
	})
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
	req.Header.Set("X-Idempotency-Key", "create-job-ok")

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
	if resp["prompt"] != "hello world" {
		t.Fatalf("expected prompt in job response, got %#v", resp["prompt"])
	}
}

func TestHandler_BillableEndpointsRequireBoundedIdempotencyKey(t *testing.T) {
	routes := newTestHandler("").Routes()

	tests := []struct {
		name   string
		path   string
		body   any
		header string
	}{
		{
			name: "job missing",
			path: "/miniapp/jobs",
			body: map[string]string{"operation": "text_generate", "prompt": "hello"},
		},
		{
			name:   "job too long",
			path:   "/miniapp/jobs",
			body:   map[string]string{"operation": "text_generate", "prompt": "hello"},
			header: strings.Repeat("a", 129),
		},
		{
			name:   "chat unsafe chars",
			path:   "/miniapp/chat/messages",
			body:   map[string]string{"prompt": "hello", "conversation_id": "thread-a"},
			header: "bad/key?secret",
		},
		{
			name:   "payment too short",
			path:   "/miniapp/payments/intents",
			body:   map[string]string{"product_code": "credits_100", "receipt_email": "user@example.com"},
			header: "short",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Launch-Params", devLaunchParams(777))
			if tc.header != "" {
				req.Header.Set("X-Idempotency-Key", tc.header)
			}

			w := httptest.NewRecorder()
			routes.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandler_ListJobs_ExposesPromptForChatAndImageJobs(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	create := func(operation, prompt, modelID, idem string) {
		t.Helper()
		payload := map[string]string{
			"operation": operation,
			"prompt":    prompt,
		}
		if modelID != "" {
			payload["model_id"] = modelID
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Launch-Params", devLaunchParams(777))
		req.Header.Set("X-Idempotency-Key", idem)
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: expected 201, got %d: %s", operation, w.Code, w.Body.String())
		}
	}

	chatBody, _ := json.Marshal(map[string]string{
		"prompt":          "КОЗЯВКИ",
		"conversation_id": "thread-alpha",
	})
	chatReq := httptest.NewRequest(http.MethodPost, "/miniapp/chat/messages", bytes.NewReader(chatBody))
	chatReq.Header.Set("Content-Type", "application/json")
	chatReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	chatReq.Header.Set("X-Idempotency-Key", "list-jobs-chat-1")
	chatW := httptest.NewRecorder()
	routes.ServeHTTP(chatW, chatReq)
	if chatW.Code != http.StatusCreated {
		t.Fatalf("chat message: expected 201, got %d: %s", chatW.Code, chatW.Body.String())
	}

	secondChatBody, _ := json.Marshal(map[string]string{
		"prompt":          "второе сообщение",
		"conversation_id": "thread-alpha",
	})
	secondChatReq := httptest.NewRequest(http.MethodPost, "/miniapp/chat/messages", bytes.NewReader(secondChatBody))
	secondChatReq.Header.Set("Content-Type", "application/json")
	secondChatReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	secondChatReq.Header.Set("X-Idempotency-Key", "list-jobs-chat-2")
	secondChatW := httptest.NewRecorder()
	routes.ServeHTTP(secondChatW, secondChatReq)
	if secondChatW.Code != http.StatusCreated {
		t.Fatalf("second chat message: expected 201, got %d: %s", secondChatW.Code, secondChatW.Body.String())
	}

	create("image_generate", "Кот в киберпанке", "nano_banana_pro", "list-jobs-image")

	listReq := httptest.NewRequest(http.MethodGet, "/miniapp/jobs", nil)
	listReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	listW := httptest.NewRecorder()
	routes.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list jobs: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}

	var listResp struct {
		Items []struct {
			Operation      string `json:"operation"`
			Prompt         string `json:"prompt"`
			ConversationID string `json:"conversation_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("invalid list response: %v", err)
	}
	var chatPrompt string
	var chatConversationID string
	var imagePrompt string
	for _, item := range listResp.Items {
		switch item.Operation {
		case "text_generate":
			if chatPrompt == "" || item.Prompt == "КОЗЯВКИ" {
				chatPrompt = item.Prompt
				chatConversationID = item.ConversationID
			}
		case "image_generate":
			imagePrompt = item.Prompt
		}
	}
	if chatPrompt != "КОЗЯВКИ" {
		t.Fatalf("chat prompt = %q, want КОЗЯВКИ", chatPrompt)
	}
	if chatConversationID != "thread-alpha" {
		t.Fatalf("conversation_id = %q, want thread-alpha", chatConversationID)
	}
	if imagePrompt != "Кот в киберпанке" {
		t.Fatalf("image prompt = %q, want Кот в киберпанке", imagePrompt)
	}
}

func TestHandler_ChatMessage_CreatesTextJobWithPublicAlias(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]string{
		"prompt":          "hello chat",
		"conversation_id": "chat-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/chat/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "chat-public-alias")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "deepseek") || strings.Contains(strings.ToLower(w.Body.String()), "deepinfra") {
		t.Fatalf("chat response leaked provider/model detail: %s", w.Body.String())
	}
	var resp struct {
		ID        string `json:"id"`
		Operation string `json:"operation"`
		ModelName string `json:"model_name"`
		ModelID   string `json:"model_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.Operation != "text_generate" || resp.ModelName != "ChatGPT" || resp.ModelID != "" {
		t.Fatalf("unexpected chat response: %+v", resp)
	}

	jobID, err := uuid.Parse(resp.ID)
	if err != nil {
		t.Fatalf("invalid job id: %v", err)
	}
	job, err := fixture.jobRepo.GetByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("expected stored job: %v", err)
	}
	var params struct {
		Prompt             string `json:"prompt"`
		ModelID            string `json:"model_id"`
		ModelName          string `json:"model_name"`
		ConversationID     string `json:"conversation_id"`
		ConversationSource string `json:"conversation_source"`
		ExternalThreadID   string `json:"external_thread_id"`
	}
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatalf("invalid job params: %v", err)
	}
	if job.OperationType != domain.OperationTextGenerate || job.Modality != domain.ModalityText {
		t.Fatalf("unexpected job operation/modality: %s/%s", job.OperationType, job.Modality)
	}
	if params.Prompt != "hello chat" || params.ModelID != "chatgpt" || params.ModelName != "ChatGPT" {
		t.Fatalf("unexpected job params: %+v", params)
	}
	if params.ConversationID != "" || params.ConversationSource != "miniapp" || params.ExternalThreadID != "chat-1" {
		t.Fatalf("unexpected conversation params: %+v", params)
	}
	if strings.Contains(strings.ToLower(string(job.Params)), "deepseek") || strings.Contains(strings.ToLower(string(job.Params)), "deepinfra") {
		t.Fatalf("job params leaked provider/model detail: %s", string(job.Params))
	}
}

func TestHandler_ChatMessage_UsesDurableRefsWithoutPromptPrefix(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()

	createChat := func(prompt, threadID, idem string) uuid.UUID {
		body, _ := json.Marshal(map[string]string{
			"prompt":          prompt,
			"conversation_id": threadID,
		})
		req := httptest.NewRequest(http.MethodPost, "/miniapp/chat/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Launch-Params", devLaunchParams(777))
		req.Header.Set("X-Idempotency-Key", idem)
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create chat: expected 201, got %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid response json: %v", err)
		}
		id, err := uuid.Parse(resp.ID)
		if err != nil {
			t.Fatalf("invalid job id: %v", err)
		}
		return id
	}

	firstJobID := createChat("first question", "thread-a", "chat-context-1")
	secondJobID := createChat("follow up", "thread-a", "chat-context-2")
	thirdJobID := createChat("other thread", "thread-b", "chat-context-3")

	firstJob, err := fixture.jobRepo.GetByID(ctx, firstJobID)
	if err != nil {
		t.Fatalf("get first job: %v", err)
	}
	secondJob, err := fixture.jobRepo.GetByID(ctx, secondJobID)
	if err != nil {
		t.Fatalf("get second job: %v", err)
	}
	thirdJob, err := fixture.jobRepo.GetByID(ctx, thirdJobID)
	if err != nil {
		t.Fatalf("get third job: %v", err)
	}

	var firstParams, secondParams, thirdParams struct {
		Prompt             string `json:"prompt"`
		ConversationSource string `json:"conversation_source"`
		ExternalThreadID   string `json:"external_thread_id"`
	}
	if err := json.Unmarshal(firstJob.Params, &firstParams); err != nil {
		t.Fatalf("invalid first job params: %v", err)
	}
	if err := json.Unmarshal(secondJob.Params, &secondParams); err != nil {
		t.Fatalf("invalid second job params: %v", err)
	}
	if err := json.Unmarshal(thirdJob.Params, &thirdParams); err != nil {
		t.Fatalf("invalid third job params: %v", err)
	}

	if firstParams.Prompt != "first question" || secondParams.Prompt != "follow up" || thirdParams.Prompt != "other thread" {
		t.Fatalf("BFF should not prefix prompts with local context: first=%q second=%q third=%q", firstParams.Prompt, secondParams.Prompt, thirdParams.Prompt)
	}
	if firstParams.ConversationSource != "miniapp" || secondParams.ConversationSource != "miniapp" || thirdParams.ConversationSource != "miniapp" {
		t.Fatalf("missing miniapp conversation source: first=%+v second=%+v third=%+v", firstParams, secondParams, thirdParams)
	}
	if firstParams.ExternalThreadID != "thread-a" || secondParams.ExternalThreadID != "thread-a" || thirdParams.ExternalThreadID != "thread-b" {
		t.Fatalf("thread refs not isolated: first=%+v second=%+v third=%+v", firstParams, secondParams, thirdParams)
	}
}

func TestHandler_ChatMessage_InvalidConversationID(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{
		"prompt":          "hello chat",
		"conversation_id": "bad/id",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/chat/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid conversation id") {
		t.Fatalf("expected safe invalid conversation message, got %s", w.Body.String())
	}
}

func TestHandler_ChatMessage_EmptyConversationIDUsesDefaultThread(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]string{
		"prompt": "hello default thread",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/chat/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "chat-default-thread")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	jobID, err := uuid.Parse(resp.ID)
	if err != nil {
		t.Fatalf("invalid job id: %v", err)
	}
	job, err := fixture.jobRepo.GetByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	var params struct {
		Prompt             string `json:"prompt"`
		ConversationSource string `json:"conversation_source"`
		ExternalThreadID   string `json:"external_thread_id"`
	}
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatalf("invalid job params: %v", err)
	}
	if params.Prompt != "hello default thread" || params.ConversationSource != "miniapp" || params.ExternalThreadID != "default" {
		t.Fatalf("unexpected default thread params: %+v", params)
	}
}

func TestHandler_ChatConversations_AuthRequired(t *testing.T) {
	routes := newTestHandler("real-secret").Routes()

	req := httptest.NewRequest(http.MethodGet, "/miniapp/chat/conversations", nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ChatConversations_ListAndMessages(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive, Locale: "ru", Timezone: "UTC"}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	conversation := &domain.Conversation{
		UserID:           user.ID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "thread-a",
		Title:            "Support thread",
	}
	if err := fixture.conversationRepo.CreateConversation(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	userMessage, err := fixture.conversationRepo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversation.ID,
		JobID:          uuid.New(),
		Role:           domain.ConversationRoleUser,
		Text:           "hello history",
		TokenCount:     5,
	})
	if err != nil {
		t.Fatalf("upsert user message: %v", err)
	}
	if _, err := fixture.conversationRepo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversation.ID,
		JobID:          uuid.New(),
		Role:           domain.ConversationRoleAssistant,
		Text:           "history answer",
		TokenCount:     5,
	}); err != nil {
		t.Fatalf("upsert assistant message: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/miniapp/chat/conversations?limit=999", nil)
	listReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	listW := httptest.NewRecorder()
	routes.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list conversations: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var listResp struct {
		Items []struct {
			ID                 string `json:"id"`
			Title              string `json:"title"`
			LastMessagePreview string `json:"last_message_preview"`
			LastMessageRole    string `json:"last_message_role"`
		} `json:"items"`
		Pagination struct {
			Limit int `json:"limit"`
			Count int `json:"count"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("invalid list response: %v", err)
	}
	if listResp.Pagination.Limit != 100 || listResp.Pagination.Count != 1 {
		t.Fatalf("unexpected pagination: %+v", listResp.Pagination)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].ID != "thread-a" || listResp.Items[0].Title != "Support thread" {
		t.Fatalf("unexpected conversations: %+v", listResp.Items)
	}
	if listResp.Items[0].LastMessagePreview != "history answer" || listResp.Items[0].LastMessageRole != "bot" {
		t.Fatalf("unexpected preview: %+v", listResp.Items[0])
	}

	msgReq := httptest.NewRequest(http.MethodGet, "/miniapp/chat/conversations/thread-a/messages?after_seq="+strconv.FormatInt(userMessage.Seq-1, 10), nil)
	msgReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	msgW := httptest.NewRecorder()
	routes.ServeHTTP(msgW, msgReq)
	if msgW.Code != http.StatusOK {
		t.Fatalf("list messages: expected 200, got %d: %s", msgW.Code, msgW.Body.String())
	}
	var msgResp struct {
		Items []struct {
			Seq  int64  `json:"seq"`
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"items"`
	}
	if err := json.Unmarshal(msgW.Body.Bytes(), &msgResp); err != nil {
		t.Fatalf("invalid message response: %v", err)
	}
	if len(msgResp.Items) != 2 || msgResp.Items[0].Role != "user" || msgResp.Items[0].Text != "hello history" || msgResp.Items[1].Role != "bot" {
		t.Fatalf("unexpected messages: %+v", msgResp.Items)
	}
}

func TestHandler_ChatConversations_RejectedOutputNotVisible(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 778, Role: domain.RoleUser, Status: domain.StatusActive, Locale: "ru", Timezone: "UTC"}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	conversation := &domain.Conversation{
		UserID:           user.ID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "blocked-thread",
		Title:            "Blocked thread",
	}
	if err := fixture.conversationRepo.CreateConversation(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	jobID := uuid.New()
	if err := fixture.jobRepo.Create(ctx, &domain.Job{
		ID:             jobID,
		UserID:         user.ID,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusRejected,
		IdempotencyKey: "miniapp-chat:" + uuid.NewString(),
		ErrorCode:      "content_rejected",
		ErrorMessage:   "blocked by output moderation",
	}); err != nil {
		t.Fatalf("create rejected job: %v", err)
	}
	if _, err := fixture.conversationRepo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversation.ID,
		JobID:          jobID,
		Role:           domain.ConversationRoleUser,
		Text:           "safe user question",
		TokenCount:     3,
	}); err != nil {
		t.Fatalf("upsert user message: %v", err)
	}
	if err := fixture.moderationRepo.Create(ctx, &domain.ModerationResult{
		JobID:      jobID,
		Stage:      domain.ModerationStageOutput,
		Decision:   domain.ModerationBlock,
		Categories: []string{"test:block"},
		Provider:   "test",
	}); err != nil {
		t.Fatalf("create moderation result: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/miniapp/chat/conversations", nil)
	listReq.Header.Set("X-Launch-Params", devLaunchParams(778))
	listW := httptest.NewRecorder()
	routes.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list conversations: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	if strings.Contains(listW.Body.String(), "blocked generated answer") {
		t.Fatalf("blocked output leaked in conversation preview: %s", listW.Body.String())
	}
	var listResp struct {
		Items []struct {
			ID                 string `json:"id"`
			LastMessagePreview string `json:"last_message_preview"`
			LastMessageRole    string `json:"last_message_role"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("invalid list response: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].ID != "blocked-thread" {
		t.Fatalf("unexpected conversations: %+v", listResp.Items)
	}
	if listResp.Items[0].LastMessagePreview != "safe user question" || listResp.Items[0].LastMessageRole != "user" {
		t.Fatalf("blocked output should not become preview: %+v", listResp.Items[0])
	}

	msgReq := httptest.NewRequest(http.MethodGet, "/miniapp/chat/conversations/blocked-thread/messages", nil)
	msgReq.Header.Set("X-Launch-Params", devLaunchParams(778))
	msgW := httptest.NewRecorder()
	routes.ServeHTTP(msgW, msgReq)
	if msgW.Code != http.StatusOK {
		t.Fatalf("list messages: expected 200, got %d: %s", msgW.Code, msgW.Body.String())
	}
	if strings.Contains(msgW.Body.String(), "blocked generated answer") {
		t.Fatalf("blocked output leaked in message history: %s", msgW.Body.String())
	}
	var msgResp struct {
		Items []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"items"`
	}
	if err := json.Unmarshal(msgW.Body.Bytes(), &msgResp); err != nil {
		t.Fatalf("invalid message response: %v", err)
	}
	if len(msgResp.Items) != 1 || msgResp.Items[0].Role != "user" || msgResp.Items[0].Text != "safe user question" {
		t.Fatalf("unexpected messages: %+v", msgResp.Items)
	}
}

func TestHandler_ChatConversationMessages_OwnerOnlyAndInvalidID(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()
	owner := &domain.User{VKUserID: 100, Role: domain.RoleUser, Status: domain.StatusActive, Locale: "ru", Timezone: "UTC"}
	other := &domain.User{VKUserID: 200, Role: domain.RoleUser, Status: domain.StatusActive, Locale: "ru", Timezone: "UTC"}
	if err := fixture.userRepo.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := fixture.userRepo.Create(ctx, other); err != nil {
		t.Fatalf("create other: %v", err)
	}
	if err := fixture.conversationRepo.CreateConversation(ctx, &domain.Conversation{
		UserID:           owner.ID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "owner-thread",
	}); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/miniapp/chat/conversations/owner-thread/messages", nil)
	otherReq.Header.Set("X-Launch-Params", devLaunchParams(200))
	otherW := httptest.NewRecorder()
	routes.ServeHTTP(otherW, otherReq)
	if otherW.Code != http.StatusNotFound {
		t.Fatalf("owner-only expected 404, got %d: %s", otherW.Code, otherW.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodGet, "/miniapp/chat/conversations/bad%2Fid/messages", nil)
	invalidReq.Header.Set("X-Launch-Params", devLaunchParams(100))
	invalidW := httptest.NewRecorder()
	routes.ServeHTTP(invalidW, invalidReq)
	if invalidW.Code != http.StatusBadRequest {
		t.Fatalf("invalid id expected 400, got %d: %s", invalidW.Code, invalidW.Body.String())
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
	req.Header.Set("X-Idempotency-Key", "invalid-operation-key")

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
	req.Header.Set("X-Idempotency-Key", "missing-prompt-key")

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

func TestHandler_GetArtifactRejectsForeignOwnerWithoutStorageLeak(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()
	owner := &domain.User{VKUserID: 100, Role: domain.RoleUser, Status: domain.StatusActive}
	requester := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := fixture.userRepo.Create(ctx, requester); err != nil {
		t.Fatalf("create requester: %v", err)
	}
	job := &domain.Job{
		UserID:         owner.ID,
		VKPeerID:       owner.VKUserID,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusSucceeded,
		IdempotencyKey: "artifact-foreign:" + uuid.NewString(),
	}
	if err := fixture.jobRepo.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	artifact := &domain.Artifact{
		OwnerUserID:   owner.ID,
		JobID:         &job.ID,
		Kind:          domain.ArtifactKindOutput,
		MediaType:     domain.MediaTypeText,
		MimeType:      "text/plain",
		StorageBucket: "artifacts",
		StorageKey:    "outputs/private-" + uuid.NewString() + ".txt",
		SHA256:        uuid.NewString(),
		SizeBytes:     int64(len("foreign safe result")),
		Status:        domain.ArtifactStatusReady,
	}
	if err := fixture.artifactRepo.Create(ctx, artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	job.OutputArtifactIDs = []uuid.UUID{artifact.ID}
	if err := fixture.jobRepo.Update(ctx, job); err != nil {
		t.Fatalf("update job outputs: %v", err)
	}
	if err := fixture.objects.Put(ctx, artifact.StorageBucket, artifact.StorageKey, []byte("foreign safe result"), artifact.MimeType); err != nil {
		t.Fatalf("put object: %v", err)
	}
	artID := artifact.ID
	if err := fixture.moderationRepo.Create(ctx, &domain.ModerationResult{
		JobID:      job.ID,
		ArtifactID: &artID,
		Stage:      domain.ModerationStageOutput,
		Decision:   domain.ModerationAllow,
		Provider:   "test",
	}); err != nil {
		t.Fatalf("create moderation result: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/miniapp/artifacts/"+artifact.ID.String(), nil)
	req.Header.Set("X-Launch-Params", devLaunchParams(requester.VKUserID))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, forbidden := range []string{artifact.ID.String(), artifact.StorageBucket, artifact.StorageKey, "private-"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("foreign artifact response leaked %q in %s", forbidden, body)
		}
	}
}

func TestHandler_UploadArtifact_HappyPath(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	body, contentType := multipartUploadBody(t, pngBytes())

	req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "upload-happy")
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "url") || strings.Contains(strings.ToLower(w.Body.String()), "storage") || strings.Contains(strings.ToLower(w.Body.String()), "provider") {
		t.Fatalf("upload response leaked private details: %s", w.Body.String())
	}
	var resp struct {
		ArtifactID uuid.UUID `json:"artifact_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	artifact, err := fixture.artifactRepo.GetByID(context.Background(), resp.ArtifactID)
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	user, err := fixture.userRepo.GetByVKUserID(context.Background(), 777)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if artifact.OwnerUserID != user.ID || artifact.Kind != domain.ArtifactKindInput || artifact.MediaType != domain.MediaTypeImage || artifact.Status != domain.ArtifactStatusReady {
		t.Fatalf("unexpected artifact: %+v", artifact)
	}
	if artifact.Width != 1 || artifact.Height != 1 {
		t.Fatalf("upload did not persist image dimensions: got %dx%d", artifact.Width, artifact.Height)
	}
	if _, err := fixture.objects.GetObject(context.Background(), artifact.StorageBucket, artifact.StorageKey); err != nil {
		t.Fatalf("stored object missing: %v", err)
	}
}

func TestHandler_UploadArtifact_ConcurrentSmallUploadReadinessDrill(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	const requests = 16

	start := make(chan struct{})
	errs := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			body, contentType := multipartUploadBody(t, pngBytes())
			req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
			req.Header.Set("Content-Type", contentType)
			req.Header.Set("X-Launch-Params", devLaunchParams(10_000+int64(i)))
			req.Header.Set("X-Idempotency-Key", fmt.Sprintf("upload-readiness-concurrent-%d", i))
			w := httptest.NewRecorder()
			routes.ServeHTTP(w, req)
			if w.Code != http.StatusCreated {
				errs <- fmt.Errorf("request %d status = %d body = %s", i, w.Code, w.Body.String())
				return
			}
			bodyText := strings.ToLower(w.Body.String())
			for _, forbidden := range []string{"storage", "provider", "launch", "vk_user", "url"} {
				if strings.Contains(bodyText, forbidden) {
					errs <- fmt.Errorf("request %d response leaked %q in %s", i, forbidden, w.Body.String())
					return
				}
			}
			errs <- nil
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := fixture.objects.Len(); got != requests {
		t.Fatalf("stored objects = %d, want %d owner-isolated uploads", got, requests)
	}
}

func TestHandler_UploadArtifact_RejectsWrongMime(t *testing.T) {
	routes := newTestHandler("").Routes()
	body, contentType := multipartUploadBody(t, []byte("not an image"))

	req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), domain.JobErrMediaUploadUnsupported) {
		t.Fatalf("expected safe unsupported upload error, got %s", w.Body.String())
	}
}

func TestHandler_UploadArtifact_InvalidFloodDoesNotStoreObjects(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	const requests = 24

	for i := 0; i < requests; i++ {
		body, contentType := multipartUploadBody(t, []byte("not an image"))
		req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Launch-Params", devLaunchParams(20_000+int64(i)))
		req.Header.Set("X-Idempotency-Key", fmt.Sprintf("upload-readiness-invalid-%d", i))
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("request %d expected 400, got %d: %s", i, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), domain.JobErrMediaUploadUnsupported) {
			t.Fatalf("request %d expected safe unsupported upload error, got %s", i, w.Body.String())
		}
	}
	if got := fixture.objects.Len(); got != 0 {
		t.Fatalf("invalid upload flood stored %d objects, want 0", got)
	}
}

func TestHandler_UploadArtifact_RejectsOversize(t *testing.T) {
	routes := newTestHandler("").Routes()
	oversize := append(pngBytes(), bytes.Repeat([]byte{0}, 21<<20)...)
	body, contentType := multipartUploadBody(t, oversize)

	req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), domain.JobErrMediaUploadTooLarge) {
		t.Fatalf("expected safe too-large upload error, got %s", w.Body.String())
	}
}

func TestHandler_UploadArtifact_DisabledKillSwitch(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.ReferenceUploadsDisabled = true
	})
	routes := fixture.handler.Routes()
	body, contentType := multipartUploadBody(t, pngBytes())

	req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "reference_artifacts_unsupported") {
		t.Fatalf("expected safe disabled upload error, got %s", w.Body.String())
	}
	if got := fixture.objects.Len(); got != 0 {
		t.Fatalf("disabled upload stored %d objects, want 0", got)
	}
}

func TestHandler_UploadArtifact_RejectsWebPWhenDisabled(t *testing.T) {
	routes := newTestHandler("").Routes()
	body, contentType := multipartUploadBody(t, minimalWebPBytes())

	req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), domain.JobErrMediaUploadUnsupported) {
		t.Fatalf("expected safe unsupported WebP error, got %s", w.Body.String())
	}
}

func TestHandler_UploadArtifact_RejectsImagePixelLimit(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.MaxUploadImagePixels = 1
	})
	routes := fixture.handler.Routes()
	body, contentType := multipartUploadBody(t, pngSizedBytes(2, 1))

	req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), domain.JobErrMediaUploadTooLarge) {
		t.Fatalf("expected safe pixel-limit error, got %s", w.Body.String())
	}
	if got := fixture.objects.Len(); got != 0 {
		t.Fatalf("pixel rejected upload stored %d objects, want 0", got)
	}
}

func TestHandler_UploadArtifact_DuplicateReferenceBurstReusesArtifact(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	const requests = 8
	var first uuid.UUID

	for i := 0; i < requests; i++ {
		body, contentType := multipartUploadBody(t, pngBytes())
		req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Launch-Params", devLaunchParams(30_000))
		req.Header.Set("X-Idempotency-Key", fmt.Sprintf("upload-readiness-duplicate-%d", i))
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("request %d expected 201, got %d: %s", i, w.Code, w.Body.String())
		}
		var resp struct {
			ArtifactID uuid.UUID `json:"artifact_id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("request %d invalid response json: %v", i, err)
		}
		if resp.ArtifactID == uuid.Nil {
			t.Fatalf("request %d returned nil artifact id", i)
		}
		if i == 0 {
			first = resp.ArtifactID
			continue
		}
		if resp.ArtifactID != first {
			t.Fatalf("request %d got artifact %s, want reused %s", i, resp.ArtifactID, first)
		}
	}
	if got := fixture.objects.Len(); got != 1 {
		t.Fatalf("duplicate upload burst stored %d objects, want 1", got)
	}
}

func TestHandler_UploadArtifact_AuthRequired(t *testing.T) {
	routes := newTestHandler("real-secret").Routes()
	body, contentType := multipartUploadBody(t, pngBytes())

	req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadArtifact_IdempotentReplaySameArtifact(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	makeReq := func() *http.Request {
		body, contentType := multipartUploadBody(t, pngBytes())
		req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Launch-Params", devLaunchParams(777))
		req.Header.Set("X-Idempotency-Key", "upload-replay")
		return req
	}

	w1 := httptest.NewRecorder()
	routes.ServeHTTP(w1, makeReq())
	if w1.Code != http.StatusCreated {
		t.Fatalf("first upload expected 201, got %d: %s", w1.Code, w1.Body.String())
	}
	w2 := httptest.NewRecorder()
	routes.ServeHTTP(w2, makeReq())
	if w2.Code != http.StatusCreated {
		t.Fatalf("second upload expected 201, got %d: %s", w2.Code, w2.Body.String())
	}
	var r1, r2 struct {
		ArtifactID uuid.UUID `json:"artifact_id"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &r1)
	_ = json.Unmarshal(w2.Body.Bytes(), &r2)
	if r1.ArtifactID == uuid.Nil || r1.ArtifactID != r2.ArtifactID {
		t.Fatalf("expected same artifact id on replay, got %s and %s", r1.ArtifactID, r2.ArtifactID)
	}
}

func TestHandler_UploadArtifact_RateLimitByVerifiedUserID(t *testing.T) {
	limiter := &countingLimiter{burst: 1}
	routes := newTestHandlerWithLimiter("", limiter).Routes()

	makeReq := func(vkUserID int64) *http.Request {
		body, contentType := multipartUploadBody(t, pngBytes())
		req := httptest.NewRequest(http.MethodPost, "/miniapp/artifacts", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Launch-Params", devLaunchParams(vkUserID))
		return req
	}

	w1 := httptest.NewRecorder()
	routes.ServeHTTP(w1, makeReq(777))
	if w1.Code != http.StatusCreated {
		t.Fatalf("first upload expected 201, got %d: %s", w1.Code, w1.Body.String())
	}
	w2 := httptest.NewRecorder()
	routes.ServeHTTP(w2, makeReq(777))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second upload expected 429, got %d: %s", w2.Code, w2.Body.String())
	}
	w3 := httptest.NewRecorder()
	routes.ServeHTTP(w3, makeReq(888))
	if w3.Code != http.StatusCreated {
		t.Fatalf("different user upload expected 201, got %d: %s", w3.Code, w3.Body.String())
	}
	if limiter.counts["miniapp_artifact:777"] != 2 || limiter.counts["miniapp_artifact:888"] != 1 {
		t.Fatalf("limiter keys = %#v, want upload keys by verified vk_user_id", limiter.counts)
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

func TestHandler_GetReferralReturnsStableCodeAndBotStyleLink(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	req := httptest.NewRequest(http.MethodGet, "/miniapp/referral", nil)
	req.Header.Set("X-Launch-Params", devLaunchParams(778))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp miniappinbound.ReferralDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if resp.Code == "" || resp.InviteURL == "" || resp.InvitedCount != 0 ||
		resp.RegisteredCount != 0 || resp.ActivatedCount != 0 || resp.RewardedCount != 0 {
		t.Fatalf("unexpected referral dto: %+v", resp)
	}
	if !strings.HasPrefix(resp.InviteURL, "https://vk.com/write-239332376?") ||
		!strings.Contains(resp.InviteURL, "ref="+url.QueryEscape(resp.Code)) {
		t.Fatalf("invite url must use bot-style ref link, got %+v", resp)
	}
	if resp.ReferrerSignupRewardCredits != 10 || resp.ReferredSignupRewardCredits != 0 {
		t.Fatalf("unexpected reward copy: %+v", resp)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/miniapp/referral", nil)
	secondReq.Header.Set("X-Launch-Params", devLaunchParams(778))
	second := httptest.NewRecorder()
	routes.ServeHTTP(second, secondReq)
	if second.Code != http.StatusOK {
		t.Fatalf("expected second 200, got %d: %s", second.Code, second.Body.String())
	}
	var secondResp miniappinbound.ReferralDTO
	if err := json.Unmarshal(second.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("invalid second response: %v", err)
	}
	if secondResp.Code != resp.Code || secondResp.InviteURL != resp.InviteURL {
		t.Fatalf("referral code/link must be stable, first=%+v second=%+v", resp, secondResp)
	}
}

func TestHandler_AcceptReferralUsesMiniAppSourceAndLedgerNoJob(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()
	referrer := &domain.User{
		VKUserID: 900101,
		Role:     domain.RoleUser,
		Status:   domain.StatusActive,
		Locale:   "ru",
		Timezone: "Europe/Moscow",
	}
	if err := fixture.userRepo.Create(ctx, referrer); err != nil {
		t.Fatalf("create referrer: %v", err)
	}
	if err := fixture.referralRepo.CreateCode(ctx, &domain.ReferralCode{UserID: referrer.ID, Code: "MNN2345A"}); err != nil {
		t.Fatalf("create referral code: %v", err)
	}

	body := strings.NewReader(`{"code":"MNN2345A"}`)
	req := httptest.NewRequest(http.MethodPost, "/miniapp/referral/accept", body)
	req.Header.Set("X-Launch-Params", devLaunchParams(900102))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp miniappinbound.ApplyReferralDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Applied || resp.AlreadyApplied || resp.InvalidCode || resp.SelfReferral {
		t.Fatalf("unexpected apply response: %+v", resp)
	}
	referred, err := fixture.userRepo.GetByVKUserID(ctx, 900102)
	if err != nil {
		t.Fatalf("referred user not created: %v", err)
	}
	referral, err := fixture.referralRepo.GetReferralByReferredUserID(ctx, referred.ID)
	if err != nil {
		t.Fatalf("referral not created: %v", err)
	}
	if referral.ReferrerUserID != referrer.ID ||
		referral.ReferralCode != "MNN2345A" ||
		referral.Source != domain.ReferralSourceVKMiniApp ||
		referral.Status != domain.ReferralStatusRewarded ||
		referral.RewardStatus != domain.ReferralRewardApplied ||
		referral.ActivatedAt == nil ||
		referral.RewardedAt == nil {
		t.Fatalf("unexpected referral: %+v", referral)
	}
	referrerAccount, err := fixture.billingRepo.GetAccountByUser(ctx, referrer.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("referrer account not rewarded: %v", err)
	}
	if referrerAccount.BalanceCached != billingservice.DefaultStartingBalance+10 {
		t.Fatalf("referrer balance = %d, want %d", referrerAccount.BalanceCached, billingservice.DefaultStartingBalance+10)
	}
	jobs, _ := fixture.jobRepo.ListByUser(ctx, referred.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("referral accept must not create jobs, got %+v", jobs)
	}

	replay := httptest.NewRecorder()
	replayReq := httptest.NewRequest(http.MethodPost, "/miniapp/referral/accept", strings.NewReader(`{"code":"MNN2345A"}`))
	replayReq.Header.Set("X-Launch-Params", devLaunchParams(900102))
	routes.ServeHTTP(replay, replayReq)
	if replay.Code != http.StatusOK {
		t.Fatalf("expected replay 200, got %d: %s", replay.Code, replay.Body.String())
	}
	var replayResp miniappinbound.ApplyReferralDTO
	if err := json.Unmarshal(replay.Body.Bytes(), &replayResp); err != nil {
		t.Fatalf("invalid replay response: %v", err)
	}
	if !replayResp.AlreadyApplied || replayResp.Applied {
		t.Fatalf("expected idempotent already-applied response, got %+v", replayResp)
	}
	referrerAccount, err = fixture.billingRepo.GetAccountByUser(ctx, referrer.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get referrer account after replay: %v", err)
	}
	if referrerAccount.BalanceCached != billingservice.DefaultStartingBalance+10 {
		t.Fatalf("referrer balance after replay = %d, want %d", referrerAccount.BalanceCached, billingservice.DefaultStartingBalance+10)
	}
	count, err := fixture.referralRepo.CountByReferrer(ctx, referrer.ID)
	if err != nil {
		t.Fatalf("count referrals: %v", err)
	}
	if count != 1 {
		t.Fatalf("referral count = %d, want 1", count)
	}
	entries, err := fixture.billingRepo.ListEntries(ctx, referrerAccount.ID, 10, 0)
	if err != nil {
		t.Fatalf("list billing entries: %v", err)
	}
	rewardEntries := 0
	for _, entry := range entries {
		if entry.Reason == "referral signup reward" {
			rewardEntries++
		}
	}
	if rewardEntries != 1 {
		t.Fatalf("reward entries after replay = %d, want 1", rewardEntries)
	}
	statsReq := httptest.NewRequest(http.MethodGet, "/miniapp/referral", nil)
	statsReq.Header.Set("X-Launch-Params", devLaunchParams(900101))
	statsResp := httptest.NewRecorder()
	routes.ServeHTTP(statsResp, statsReq)
	if statsResp.Code != http.StatusOK {
		t.Fatalf("expected stats 200, got %d: %s", statsResp.Code, statsResp.Body.String())
	}
	var statsDTO miniappinbound.ReferralDTO
	if err := json.Unmarshal(statsResp.Body.Bytes(), &statsDTO); err != nil {
		t.Fatalf("invalid stats response: %v", err)
	}
	if statsDTO.InvitedCount != 1 || statsDTO.RegisteredCount != 0 || statsDTO.ActivatedCount != 0 || statsDTO.RewardedCount != 1 {
		t.Fatalf("unexpected referral funnel stats: %+v", statsDTO)
	}
}

func TestHandler_AcceptReferralRejectsInvalidAndSelfReferral(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	invalidReq := httptest.NewRequest(http.MethodPost, "/miniapp/referral/accept", strings.NewReader(`{"code":"bad code!"}`))
	invalidReq.Header.Set("X-Launch-Params", devLaunchParams(900201))
	invalid := httptest.NewRecorder()
	routes.ServeHTTP(invalid, invalidReq)
	if invalid.Code != http.StatusOK {
		t.Fatalf("expected invalid 200, got %d: %s", invalid.Code, invalid.Body.String())
	}
	var invalidResp miniappinbound.ApplyReferralDTO
	if err := json.Unmarshal(invalid.Body.Bytes(), &invalidResp); err != nil {
		t.Fatalf("invalid response body: %v", err)
	}
	if !invalidResp.InvalidCode || invalidResp.Applied {
		t.Fatalf("expected invalid-code no-op, got %+v", invalidResp)
	}

	selfStats := httptest.NewRecorder()
	selfStatsReq := httptest.NewRequest(http.MethodGet, "/miniapp/referral", nil)
	selfStatsReq.Header.Set("X-Launch-Params", devLaunchParams(900202))
	routes.ServeHTTP(selfStats, selfStatsReq)
	if selfStats.Code != http.StatusOK {
		t.Fatalf("expected stats 200, got %d: %s", selfStats.Code, selfStats.Body.String())
	}
	var stats miniappinbound.ReferralDTO
	if err := json.Unmarshal(selfStats.Body.Bytes(), &stats); err != nil {
		t.Fatalf("invalid stats response: %v", err)
	}
	selfReq := httptest.NewRequest(http.MethodPost, "/miniapp/referral/accept", strings.NewReader(fmt.Sprintf(`{"code":%q}`, stats.Code)))
	selfReq.Header.Set("X-Launch-Params", devLaunchParams(900202))
	self := httptest.NewRecorder()
	routes.ServeHTTP(self, selfReq)
	if self.Code != http.StatusOK {
		t.Fatalf("expected self 200, got %d: %s", self.Code, self.Body.String())
	}
	var selfResp miniappinbound.ApplyReferralDTO
	if err := json.Unmarshal(self.Body.Bytes(), &selfResp); err != nil {
		t.Fatalf("invalid self response: %v", err)
	}
	if !selfResp.SelfReferral || selfResp.Applied {
		t.Fatalf("expected self-referral no-op, got %+v", selfResp)
	}
}

func TestHandler_ListPaymentProductsReturnsActiveCatalog(t *testing.T) {
	fixture := newTestFixture("", nil)
	fixture.paymentRepo.PutProduct(&domain.PaymentProduct{
		Code:         "credits_500",
		Title:        "500 credits",
		Amount:       45000,
		Currency:     domain.CurrencyRUB,
		Credits:      500,
		PriceVersion: 1,
		IsActive:     true,
	})
	fixture.paymentRepo.PutProduct(&domain.PaymentProduct{
		Code:         "credits_100",
		Title:        "100 credits",
		Amount:       9900,
		Currency:     domain.CurrencyRUB,
		Credits:      100,
		PriceVersion: 1,
		IsActive:     true,
	})
	fixture.paymentRepo.PutProduct(&domain.PaymentProduct{
		Code:         "hidden",
		Title:        "Hidden",
		Amount:       1,
		Currency:     domain.CurrencyRUB,
		Credits:      1,
		PriceVersion: 1,
		IsActive:     false,
	})

	req := httptest.NewRequest(http.MethodGet, "/miniapp/payment-products", nil)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	rec := httptest.NewRecorder()
	fixture.handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Items []miniappinbound.PaymentProductDTO `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode products response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 active products, got %+v", resp.Items)
	}
	if resp.Items[0].Code != "credits_100" || resp.Items[1].Code != "credits_500" {
		t.Fatalf("unexpected product ordering: %+v", resp.Items)
	}
}

func TestHandler_CreatePaymentIntent_IdempotentAndSafeDTO(t *testing.T) {
	fixture := newTestFixture("", nil)
	product := &domain.PaymentProduct{
		Code:         "credits_100",
		Title:        "100 credits",
		Amount:       9900,
		Currency:     domain.CurrencyRUB,
		Credits:      100,
		PriceVersion: 1,
		IsActive:     true,
	}
	fixture.paymentRepo.PutProduct(product)
	routes := fixture.handler.Routes()

	body := []byte(`{"product_code":"credits_100","receipt_email":"user@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/miniapp/payments/intents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "pay-client-1")
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "provider_payment_id") || strings.Contains(w.Body.String(), "user_id") {
		t.Fatalf("miniapp payment dto leaked operator fields: %s", w.Body.String())
	}
	var first miniappinbound.PaymentIntentDTO
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode payment dto: %v", err)
	}
	if first.Status != string(domain.PaymentIntentWaitingForUser) || first.ConfirmationURL == "" || first.Credits != 100 {
		t.Fatalf("unexpected payment dto: %+v", first)
	}

	replay := httptest.NewRequest(http.MethodPost, "/miniapp/payments/intents", bytes.NewReader(body))
	replay.Header.Set("Content-Type", "application/json")
	replay.Header.Set("X-Launch-Params", devLaunchParams(777))
	replay.Header.Set("X-Idempotency-Key", "pay-client-1")
	replayRec := httptest.NewRecorder()
	routes.ServeHTTP(replayRec, replay)
	if replayRec.Code != http.StatusOK {
		t.Fatalf("expected replay 200, got %d: %s", replayRec.Code, replayRec.Body.String())
	}
	var second miniappinbound.PaymentIntentDTO
	if err := json.Unmarshal(replayRec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode replay dto: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("replay created a different intent: %s != %s", second.ID, first.ID)
	}

	another := httptest.NewRequest(http.MethodPost, "/miniapp/payments/intents", bytes.NewReader(body))
	another.Header.Set("Content-Type", "application/json")
	another.Header.Set("X-Launch-Params", devLaunchParams(777))
	another.Header.Set("X-Idempotency-Key", "pay-client-2")
	anotherRec := httptest.NewRecorder()
	routes.ServeHTTP(anotherRec, another)
	if anotherRec.Code != http.StatusOK {
		t.Fatalf("expected active pending reuse 200, got %d: %s", anotherRec.Code, anotherRec.Body.String())
	}
	var reused miniappinbound.PaymentIntentDTO
	if err := json.Unmarshal(anotherRec.Body.Bytes(), &reused); err != nil {
		t.Fatalf("decode active reuse dto: %v", err)
	}
	if reused.ID != first.ID || !reused.ReusedActivePayment || reused.Notice == "" {
		t.Fatalf("expected active payment reuse, got %+v", reused)
	}

	forceBody := []byte(`{"product_code":"credits_100","receipt_email":"user@example.com","force_new":true}`)
	forcedReq := httptest.NewRequest(http.MethodPost, "/miniapp/payments/intents", bytes.NewReader(forceBody))
	forcedReq.Header.Set("Content-Type", "application/json")
	forcedReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	forcedReq.Header.Set("X-Idempotency-Key", "pay-client-3")
	forcedRec := httptest.NewRecorder()
	routes.ServeHTTP(forcedRec, forcedReq)
	if forcedRec.Code != http.StatusCreated {
		t.Fatalf("expected force_new 201, got %d: %s", forcedRec.Code, forcedRec.Body.String())
	}
	var forced miniappinbound.PaymentIntentDTO
	if err := json.Unmarshal(forcedRec.Body.Bytes(), &forced); err != nil {
		t.Fatalf("decode forced dto: %v", err)
	}
	if forced.ID == first.ID || forced.ReusedActivePayment {
		t.Fatalf("expected new forced payment intent, got %+v first=%+v", forced, first)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/miniapp/payments", nil)
	listReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	listRec := httptest.NewRecorder()
	routes.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listResp struct {
		Items []miniappinbound.PaymentIntentDTO `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Items) != 2 || listResp.Items[0].ID != forced.ID || listResp.Items[1].ID != first.ID {
		t.Fatalf("unexpected payment list: %+v", listResp.Items)
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/miniapp/payments/"+first.ID.String(), nil)
	otherReq.Header.Set("X-Launch-Params", devLaunchParams(888))
	otherRec := httptest.NewRecorder()
	routes.ServeHTTP(otherRec, otherReq)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("expected other user 404, got %d: %s", otherRec.Code, otherRec.Body.String())
	}
}

func TestHandler_CancelPaymentIntentFeatureFlagAndOwnerChecks(t *testing.T) {
	disabled := newTestFixture("", nil)
	disabledReq := httptest.NewRequest(http.MethodPost, "/miniapp/payments/"+uuid.NewString()+"/cancel", nil)
	disabledReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	disabledRec := httptest.NewRecorder()
	disabled.handler.Routes().ServeHTTP(disabledRec, disabledReq)
	if disabledRec.Code != http.StatusNotFound {
		t.Fatalf("expected disabled cancel 404, got %d: %s", disabledRec.Code, disabledRec.Body.String())
	}

	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.PaymentCancelEnabled = true
	})
	fixture.paymentRepo.PutProduct(&domain.PaymentProduct{
		Code:         "credits_100",
		Title:        "100 credits",
		Amount:       9900,
		Currency:     domain.CurrencyRUB,
		Credits:      100,
		PriceVersion: 1,
		IsActive:     true,
	})
	routes := fixture.handler.Routes()

	body := []byte(`{"product_code":"credits_100","receipt_email":"user@example.com"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/miniapp/payments/intents", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	createReq.Header.Set("X-Idempotency-Key", "pay-cancel-1")
	createRec := httptest.NewRecorder()
	routes.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created miniappinbound.PaymentIntentDTO
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	otherReq := httptest.NewRequest(http.MethodPost, "/miniapp/payments/"+created.ID.String()+"/cancel", nil)
	otherReq.Header.Set("X-Launch-Params", devLaunchParams(888))
	otherRec := httptest.NewRecorder()
	routes.ServeHTTP(otherRec, otherReq)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("expected foreign cancel 404, got %d: %s", otherRec.Code, otherRec.Body.String())
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/miniapp/payments/"+created.ID.String()+"/cancel", nil)
	cancelReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	cancelRec := httptest.NewRecorder()
	routes.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected cancel 200, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}
	var canceled miniappinbound.PaymentIntentDTO
	if err := json.Unmarshal(cancelRec.Body.Bytes(), &canceled); err != nil {
		t.Fatalf("decode canceled: %v", err)
	}
	if canceled.ID != created.ID || canceled.Status != string(domain.PaymentIntentCanceled) {
		t.Fatalf("unexpected canceled dto: %+v", canceled)
	}
}

func TestHandler_CreatePaymentIntentIgnoresClientReturnURL(t *testing.T) {
	provider := &recordingPaymentProvider{}
	fixture := newTestFixtureWithConfigAndPaymentProvider("", nil, nil, provider)
	fixture.paymentRepo.PutProduct(&domain.PaymentProduct{
		Code:         "credits_100",
		Title:        "100 credits",
		Amount:       9900,
		Currency:     domain.CurrencyRUB,
		Credits:      100,
		PriceVersion: 1,
		IsActive:     true,
	})

	body := []byte(`{"product_code":"credits_100","receipt_email":"user@example.com","return_url":"https://attacker.example/return"}`)
	req := httptest.NewRequest(http.MethodPost, "/miniapp/payments/intents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "pay-client-return-url")
	rec := httptest.NewRecorder()
	fixture.handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(provider.createInputs) != 1 {
		t.Fatalf("expected one provider call, got %d", len(provider.createInputs))
	}
	if got := provider.createInputs[0].ReturnURL; got != "https://neiirohub.ru/payments/return" {
		t.Fatalf("client return_url was not ignored, got %q", got)
	}
}

func TestHandler_CreatePaymentIntentRequiresClientIdempotencyKey(t *testing.T) {
	fixture := newTestFixture("", nil)
	fixture.paymentRepo.PutProduct(&domain.PaymentProduct{
		Code: "credits_100", Title: "100 credits", Amount: 9900,
		Currency: domain.CurrencyRUB, Credits: 100, PriceVersion: 1, IsActive: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/payments/intents", bytes.NewReader([]byte(`{"product_code":"credits_100","receipt_email":"user@example.com"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	rec := httptest.NewRecorder()
	fixture.handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without idempotency key, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Estimate_OKNoJobNoReservation(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()

	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	acc, err := fixture.billing.EnsureAccount(ctx, user.ID)
	if err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	beforeEntries, err := fixture.billingRepo.ListEntries(ctx, acc.ID, 100, 0)
	if err != nil {
		t.Fatalf("list entries before: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"operation": "image_generate",
		"prompt":    "estimate prompt",
		"model_id":  "sdxl",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Operation      string `json:"operation"`
		ModelID        string `json:"model_id"`
		CostEstimate   int64  `json:"cost_estimate"`
		BalanceCredits int64  `json:"balance_credits"`
		EnoughCredits  bool   `json:"enough_credits"`
		Provider       string `json:"provider"`
		Prompt         string `json:"prompt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.Operation != "image_generate" || resp.ModelID != "sdxl" {
		t.Fatalf("unexpected operation/model response: %+v", resp)
	}
	if resp.CostEstimate != 10 {
		t.Fatalf("cost_estimate = %d, want 10", resp.CostEstimate)
	}
	if resp.BalanceCredits != billingservice.DefaultStartingBalance || !resp.EnoughCredits {
		t.Fatalf("balance/enough = %d/%v, want %d/true", resp.BalanceCredits, resp.EnoughCredits, billingservice.DefaultStartingBalance)
	}
	if resp.Provider != "" || resp.Prompt != "" {
		t.Fatalf("estimate response leaked provider/prompt fields: %s", w.Body.String())
	}

	jobs, err := fixture.jobRepo.ListByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("list jobs after estimate: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("estimate must not create jobs, got %d", len(jobs))
	}
	afterEntries, err := fixture.billingRepo.ListEntries(ctx, acc.ID, 100, 0)
	if err != nil {
		t.Fatalf("list entries after: %v", err)
	}
	if len(afterEntries) != len(beforeEntries) {
		t.Fatalf("estimate must not create reservations or ledger entries, before=%d after=%d", len(beforeEntries), len(afterEntries))
	}
}

func TestHandler_Estimate_RejectsUnsupportedModelID(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "estimate prompt",
		"model_id":  "kling",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
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
}

func TestHandler_Estimate_TextUsesPublicModelName(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "estimate prompt",
		"model_id":  "deepseek-v4-flash",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "deepseek") || strings.Contains(strings.ToLower(w.Body.String()), "deepinfra") {
		t.Fatalf("estimate response leaked provider/model detail: %s", w.Body.String())
	}
	var resp struct {
		ModelName string `json:"model_name"`
		ModelID   string `json:"model_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.ModelName != "ChatGPT" || resp.ModelID != "" {
		t.Fatalf("unexpected model response: %+v", resp)
	}
}

func TestHandler_Estimate_ImageAliasUsesPublicModelOnly(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "image_generate",
		"prompt":    "estimate image",
		"model_id":  "nano_banana_pro",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"deepinfra", "bytedance", "seedream", "apimart", "gemini-3-pro-image-preview", "model_code", "provider"} {
		if strings.Contains(lower, private) {
			t.Fatalf("estimate response leaked provider/model internals %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		ModelID   string `json:"model_id"`
		ModelName string `json:"model_name"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.ModelID != "nano_banana_pro" || resp.ModelName != "Nano Banana Pro" {
		t.Fatalf("unexpected model response: %+v", resp)
	}
}

func TestHandler_Estimate_NanoBanana2UsesServerOwnedCost(t *testing.T) {
	routes := newTestHandler("").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "image_generate",
		"prompt":    "estimate image",
		"model_id":  "nano_banana_2",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"poyo", "nano-banana-2-new", "model_code", "provider", "price"} {
		if strings.Contains(lower, private) {
			t.Fatalf("estimate response leaked private detail %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		ModelID      string `json:"model_id"`
		ModelName    string `json:"model_name"`
		CostEstimate int64  `json:"cost_estimate"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.ModelID != "nano_banana_2" || resp.ModelName != "Nano Banana 2" || resp.CostEstimate != 10 {
		t.Fatalf("unexpected model estimate response: %+v", resp)
	}
}

func TestHandler_ListImageModels_PublicAliasesOnly(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.ImageModels = []miniappinbound.ImageModelDTO{{
			ID:                     "nano_banana_2",
			Name:                   "Nano Banana 2",
			SupportsReferenceImage: true,
			MaxReferenceImages:     4,
		}}
	})
	routes := fixture.handler.Routes()

	req := httptest.NewRequest(http.MethodGet, "/miniapp/image-models", nil)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"poyo", "nano-banana-2-new", "model_code", "provider", "cost", "price"} {
		if strings.Contains(lower, private) {
			t.Fatalf("image model response leaked %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		Items []miniappinbound.ImageModelDTO `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].ID != "nano_banana_2" || resp.Items[0].Name != "Nano Banana 2" {
		t.Fatalf("unexpected image models: %+v", resp.Items)
	}
	if !resp.Items[0].SupportsReferenceImage || resp.Items[0].MaxReferenceImages != 4 {
		t.Fatalf("missing public image constraints: %+v", resp.Items[0])
	}
}

func TestHandler_ListVideoRoutes_PublicAliasesOnly(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		enableTestVideoRoute(cfg, domain.VideoRouteKlingO3Standard)
	})
	routes := fixture.handler.Routes()

	req := httptest.NewRequest(http.MethodGet, "/miniapp/video-routes", nil)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"provider", "model_code", "hidden-provider-model", "cost", "price"} {
		if strings.Contains(lower, private) {
			t.Fatalf("video route response leaked %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		Items []miniappinbound.VideoRouteDTO `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Alias != string(domain.VideoRouteKlingO3Standard) {
		t.Fatalf("unexpected video routes: %+v", resp.Items)
	}
	if resp.Items[0].MaxReferenceImages != 1 || len(resp.Items[0].AllowedDurationsSec) != 2 {
		t.Fatalf("missing public route constraints: %+v", resp.Items[0])
	}
}

func TestHandler_Estimate_ReferenceArtifactsFailClosedAfterValidation(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	artifact := createTestArtifact(t, fixture, user.ID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady)

	body, _ := json.Marshal(map[string]any{
		"operation":              "image_generate",
		"prompt":                 "estimate with reference",
		"model_id":               "nano_banana_pro",
		"reference_artifact_ids": []string{artifact.ID.String()},
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "reference_artifacts_unsupported") {
		t.Fatalf("expected reference unsupported error, got %s", w.Body.String())
	}
}

func TestHandler_Estimate_ReferenceArtifactsPassWhenEnabled(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.ImageReferenceEnabled = true
	})
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	artifact := createTestArtifact(t, fixture, user.ID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady)

	body, _ := json.Marshal(map[string]any{
		"operation":              "image_generate",
		"prompt":                 "estimate with enabled reference",
		"model_id":               "nano_banana_pro",
		"reference_artifact_ids": []string{artifact.ID.String()},
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"deepinfra", "bytedance", "seedream", "apimart", "gemini-3-pro-image-preview", "model_code", "provider"} {
		if strings.Contains(lower, private) {
			t.Fatalf("estimate response leaked provider/model internals %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		Operation      string `json:"operation"`
		ModelID        string `json:"model_id"`
		ModelName      string `json:"model_name"`
		CostEstimate   int64  `json:"cost_estimate"`
		BalanceCredits int64  `json:"balance_credits"`
		EnoughCredits  bool   `json:"enough_credits"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.Operation != "image_generate" || resp.ModelID != "nano_banana_pro" || resp.ModelName != "Nano Banana Pro" {
		t.Fatalf("unexpected estimate response: %+v", resp)
	}
	if resp.CostEstimate <= 0 || resp.BalanceCredits <= 0 || !resp.EnoughCredits {
		t.Fatalf("unexpected estimate billing fields: %+v", resp)
	}
}

func TestHandler_Estimate_ReferenceArtifactsRejectTooMany(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.ImageReferenceEnabled = true
	})
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	ids := make([]string, 0, 15)
	for i := 0; i < 15; i++ {
		artifact := createTestArtifact(t, fixture, user.ID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady)
		ids = append(ids, artifact.ID.String())
	}

	body, _ := json.Marshal(map[string]any{
		"operation":              "image_generate",
		"prompt":                 "estimate with too many references",
		"model_id":               "nano_banana_pro",
		"reference_artifact_ids": ids,
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "too many reference artifacts") {
		t.Fatalf("expected too-many-references error, got %s", w.Body.String())
	}
}

func TestHandler_Estimate_VideoRouteUsesResolvedCost(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		enableTestVideoRoute(cfg, domain.VideoRouteKlingO3Standard)
	})
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]any{
		"operation":         "video_generate",
		"prompt":            "estimate video",
		"video_route_alias": string(domain.VideoRouteKlingO3Standard),
		"duration_sec":      10,
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"provider", "model_code", "hidden-provider-model"} {
		if strings.Contains(lower, private) {
			t.Fatalf("estimate response leaked private detail %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		Operation       string `json:"operation"`
		ModelID         string `json:"model_id"`
		VideoRouteAlias string `json:"video_route_alias"`
		CostEstimate    int64  `json:"cost_estimate"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.Operation != "video_generate" || resp.ModelID != "" || resp.VideoRouteAlias != string(domain.VideoRouteKlingO3Standard) {
		t.Fatalf("unexpected estimate identity fields: %+v", resp)
	}
	if resp.CostEstimate != 20 {
		t.Fatalf("cost estimate = %d, want 20", resp.CostEstimate)
	}
}

func TestHandler_Estimate_UnauthorizedNoParams(t *testing.T) {
	routes := newTestHandler("real-secret").Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "estimate prompt",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Estimate_RateLimitByVerifiedUserID(t *testing.T) {
	limiter := &countingLimiter{burst: 1}
	routes := newTestHandlerWithLimiter("", limiter).Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "text_generate",
		"prompt":    "estimate prompt",
	})
	makeReq := func(vkUserID int64) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Launch-Params", devLaunchParams(vkUserID))
		return req
	}

	w1 := httptest.NewRecorder()
	routes.ServeHTTP(w1, makeReq(777))
	if w1.Code != http.StatusOK {
		t.Fatalf("first user request: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	w2 := httptest.NewRecorder()
	routes.ServeHTTP(w2, makeReq(777))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second same-user request: expected 429, got %d: %s", w2.Code, w2.Body.String())
	}

	w3 := httptest.NewRecorder()
	routes.ServeHTTP(w3, makeReq(888))
	if w3.Code != http.StatusOK {
		t.Fatalf("different user request: expected 200, got %d: %s", w3.Code, w3.Body.String())
	}
	if limiter.counts["miniapp_estimate:777"] != 2 || limiter.counts["miniapp_estimate:888"] != 1 {
		t.Fatalf("limiter keys = %#v, want estimate keys by verified vk_user_id", limiter.counts)
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

func TestHandler_CreateJob_NormalizesLegacyTextModelID(t *testing.T) {
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
		"model_id":  "deepseek-v4-flash",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "legacy-model-id")

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
	if strings.Contains(strings.ToLower(w.Body.String()), "deepseek") || strings.Contains(strings.ToLower(w.Body.String()), "deepinfra") {
		t.Fatalf("job response leaked provider/model detail: %s", w.Body.String())
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
	if params.ModelID != "chatgpt" {
		t.Fatalf("expected public model alias persisted in params, got %q", params.ModelID)
	}
	if strings.Contains(strings.ToLower(string(job.Params)), "deepseek") || strings.Contains(strings.ToLower(string(job.Params)), "deepinfra") {
		t.Fatalf("job params leaked provider/model detail: %s", string(job.Params))
	}
}

func TestHandler_CreateJob_ImageAliasPersistsProviderModelCodePrivately(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "image_generate",
		"prompt":    "image prompt",
		"model_id":  "nano_banana_pro",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "image-model-alias")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"deepinfra", "bytedance", "seedream", "apimart", "gemini-3-pro-image-preview", "model_code", "provider"} {
		if strings.Contains(lower, private) {
			t.Fatalf("job response leaked provider/model internals %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		ID        string `json:"id"`
		Operation string `json:"operation"`
		ModelID   string `json:"model_id"`
		ModelName string `json:"model_name"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.Operation != "image_generate" || resp.ModelID != "nano_banana_pro" || resp.ModelName != "Nano Banana Pro" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	jobID, err := uuid.Parse(resp.ID)
	if err != nil {
		t.Fatalf("invalid job id: %v", err)
	}
	job, err := fixture.jobRepo.GetByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	var params struct {
		ModelID   string `json:"model_id"`
		ModelName string `json:"model_name"`
		Provider  string `json:"provider"`
		ModelCode string `json:"model_code"`
	}
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatalf("invalid params: %v", err)
	}
	if params.ModelID != "nano_banana_pro" || params.ModelName != "Nano Banana Pro" || params.Provider != "apimart" || params.ModelCode != "gemini-3-pro-image-preview" {
		t.Fatalf("unexpected stored params: %+v", params)
	}
}

func TestHandler_CreateJob_GPTImage2PersistsAPIMartSnapshotPrivately(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "image_generate",
		"prompt":    "image prompt",
		"model_id":  "gpt_image_2",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "image-model-gpt-image-2")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"apimart", "gpt-image-2", "model_code", "provider"} {
		if strings.Contains(lower, private) {
			t.Fatalf("job response leaked provider/model internals %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		ID           string `json:"id"`
		Operation    string `json:"operation"`
		ModelID      string `json:"model_id"`
		ModelName    string `json:"model_name"`
		CostEstimate int64  `json:"cost_estimate"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.Operation != "image_generate" || resp.ModelID != "gpt_image_2" || resp.ModelName != "GPT Image 2" || resp.CostEstimate != 20 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	jobID, err := uuid.Parse(resp.ID)
	if err != nil {
		t.Fatalf("invalid job id: %v", err)
	}
	job, err := fixture.jobRepo.GetByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	var params struct {
		ModelID   string `json:"model_id"`
		ModelName string `json:"model_name"`
		Provider  string `json:"provider"`
		ModelCode string `json:"model_code"`
	}
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatalf("invalid params: %v", err)
	}
	if params.ModelID != "gpt_image_2" || params.ModelName != "GPT Image 2" || params.Provider != "apimart" || params.ModelCode != "gpt-image-2" {
		t.Fatalf("unexpected stored params: %+v", params)
	}
}

func TestHandler_CreateJob_NanoBanana2PersistsPoYoSnapshotPrivately(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "image_generate",
		"prompt":    "image prompt",
		"model_id":  "nano_banana_2",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "image-model-nano-banana-2")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"poyo", "nano-banana-2-new", "model_code", "provider"} {
		if strings.Contains(lower, private) {
			t.Fatalf("job response leaked private detail %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		ID           string `json:"id"`
		Operation    string `json:"operation"`
		ModelID      string `json:"model_id"`
		ModelName    string `json:"model_name"`
		CostEstimate int64  `json:"cost_estimate"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.Operation != "image_generate" || resp.ModelID != "nano_banana_2" || resp.ModelName != "Nano Banana 2" || resp.CostEstimate != 10 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	jobID, err := uuid.Parse(resp.ID)
	if err != nil {
		t.Fatalf("invalid job id: %v", err)
	}
	job, err := fixture.jobRepo.GetByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.CostEstimate != 10 || job.CostReserved != 10 {
		t.Fatalf("cost estimate/reserved = %d/%d, want 10/10", job.CostEstimate, job.CostReserved)
	}
	var params struct {
		ModelID   string `json:"model_id"`
		ModelName string `json:"model_name"`
		Provider  string `json:"provider"`
		ModelCode string `json:"model_code"`
	}
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatalf("invalid params: %v", err)
	}
	if params.ModelID != "nano_banana_2" || params.ModelName != "Nano Banana 2" || params.Provider != "poyo" || params.ModelCode != "nano-banana-2-new" {
		t.Fatalf("unexpected stored params: %+v", params)
	}
}

func TestHandler_CreateJob_VideoRejectsLegacyModelID(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]string{
		"operation": "video_generate",
		"prompt":    "snow over neon city",
		"model_id":  "kling",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "video-model-alias")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	jobs, err := fixture.jobRepo.List(context.Background(), domain.JobFilter{}, 10, 0)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("legacy video model id must not create a job, got %d", len(jobs))
	}
}

func TestHandler_CreateJob_VideoRejectsUnknownRouteAlias(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		enableTestVideoRoute(cfg, domain.VideoRouteKlingO3Standard)
	})
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]string{
		"operation":         "video_generate",
		"prompt":            "snow over neon city",
		"video_route_alias": "video_unknown",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "video-unknown-route")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	jobs, err := fixture.jobRepo.List(context.Background(), domain.JobFilter{}, 10, 0)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("unknown video route must not create a job, got %d", len(jobs))
	}
}

func TestHandler_CreateJob_VideoDurationValidatedAndPersisted(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		enableTestVideoRoute(cfg, domain.VideoRouteKlingO3Standard)
	})
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]any{
		"operation":         "video_generate",
		"prompt":            "snow over neon city",
		"video_route_alias": string(domain.VideoRouteKlingO3Standard),
		"duration_sec":      10,
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "video-duration-10")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	jobID, err := uuid.Parse(resp.ID)
	if err != nil {
		t.Fatalf("invalid job id: %v", err)
	}
	job, err := fixture.jobRepo.GetByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	var params struct {
		ModelID         string `json:"model_id"`
		Provider        string `json:"provider"`
		ModelCode       string `json:"model_code"`
		VideoRouteAlias string `json:"video_route_alias"`
		DurationSec     int    `json:"duration_sec"`
	}
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatalf("invalid params: %v", err)
	}
	if params.VideoRouteAlias != string(domain.VideoRouteKlingO3Standard) || params.DurationSec != 10 {
		t.Fatalf("unexpected video route params: %+v", params)
	}
	if params.ModelID != "" || params.Provider != "" || params.ModelCode != "" {
		t.Fatalf("miniapp video params must not persist legacy provider selection: %+v", params)
	}
}

func TestHandler_CreateJob_VideoDurationRejectsInvalidValue(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		enableTestVideoRoute(cfg, domain.VideoRouteKlingO3Standard)
	})
	routes := fixture.handler.Routes()

	body, _ := json.Marshal(map[string]any{
		"operation":         "video_generate",
		"prompt":            "snow over neon city",
		"video_route_alias": string(domain.VideoRouteKlingO3Standard),
		"duration_sec":      7,
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CreateJob_VideoReferenceArtifactPassesRouteValidation(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		enableTestVideoRoute(cfg, domain.VideoRouteKlingO3Standard)
	})
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	artifact := createTestArtifact(t, fixture, user.ID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady)

	body, _ := json.Marshal(map[string]any{
		"operation":              "video_generate",
		"prompt":                 "video with reference",
		"video_route_alias":      string(domain.VideoRouteKlingO3Standard),
		"reference_artifact_ids": []string{artifact.ID.String()},
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "video-reference-enabled")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID              uuid.UUID `json:"id"`
		ModelID         string    `json:"model_id"`
		VideoRouteAlias string    `json:"video_route_alias"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.ModelID != "" || resp.VideoRouteAlias != string(domain.VideoRouteKlingO3Standard) {
		t.Fatalf("unexpected response identity fields: %+v", resp)
	}
	job, err := fixture.jobRepo.GetByID(ctx, resp.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if len(job.InputArtifactIDs) != 1 || job.InputArtifactIDs[0] != artifact.ID {
		t.Fatalf("unexpected input artifacts: %+v", job.InputArtifactIDs)
	}
}

func TestHandler_CreateJob_VideoDerivesAspectRatioFromReferenceArtifact(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		enableTestVideoRoute(cfg, domain.VideoRouteRunwayGen4Turbo)
	})
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	artifact := createTestArtifactWithDimensions(t, fixture, user.ID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, 720, 1280)

	body, _ := json.Marshal(map[string]any{
		"operation":              "video_generate",
		"prompt":                 "video with vertical reference",
		"video_route_alias":      string(domain.VideoRouteRunwayGen4Turbo),
		"reference_artifact_ids": []string{artifact.ID.String()},
		"duration_sec":           5,
		"aspect_ratio":           "16:9",
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "video-reference-vertical-aspect")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	job, err := fixture.jobRepo.GetByID(ctx, resp.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	var params struct {
		VideoRouteAlias      string      `json:"video_route_alias"`
		AspectRatio          string      `json:"aspect_ratio"`
		ReferenceArtifactIDs []uuid.UUID `json:"reference_artifact_ids"`
	}
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatalf("invalid params: %v", err)
	}
	if params.VideoRouteAlias != string(domain.VideoRouteRunwayGen4Turbo) {
		t.Fatalf("unexpected video route alias: %+v", params)
	}
	if params.AspectRatio != "9:16" {
		t.Fatalf("backend must derive vertical aspect ratio from artifact metadata, got %q", params.AspectRatio)
	}
	if len(params.ReferenceArtifactIDs) != 1 || params.ReferenceArtifactIDs[0] != artifact.ID {
		t.Fatalf("unexpected stored reference params: %+v", params.ReferenceArtifactIDs)
	}
}

func TestHandler_CreateJob_ReferenceArtifactsValidateOwnershipAndFailClosed(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	artifact := createTestArtifact(t, fixture, user.ID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady)

	body, _ := json.Marshal(map[string]any{
		"operation":              "image_generate",
		"prompt":                 "image with reference",
		"model_id":               "nano_banana_pro",
		"reference_artifact_ids": []string{artifact.ID.String()},
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "reference-fail-closed")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "reference_artifacts_unsupported") {
		t.Fatalf("expected reference unsupported error, got %s", w.Body.String())
	}
	jobs, err := fixture.jobRepo.ListByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("reference fail-closed must not create a billable job, got %d", len(jobs))
	}
}

func TestHandler_CreateJob_ReferenceArtifactsPassWhenEnabled(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.ImageReferenceEnabled = true
	})
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	artifact := createTestArtifact(t, fixture, user.ID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady)

	body, _ := json.Marshal(map[string]any{
		"operation":              "image_generate",
		"prompt":                 "image with enabled reference",
		"model_id":               "nano_banana_pro",
		"reference_artifact_ids": []string{artifact.ID.String()},
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "reference-enabled")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	lower := strings.ToLower(w.Body.String())
	for _, private := range []string{"deepinfra", "bytedance", "seedream", "apimart", "gemini-3-pro-image-preview", "model_code", "provider"} {
		if strings.Contains(lower, private) {
			t.Fatalf("job response leaked provider/model internals %q: %s", private, w.Body.String())
		}
	}
	var resp struct {
		ID        string `json:"id"`
		Operation string `json:"operation"`
		ModelID   string `json:"model_id"`
		ModelName string `json:"model_name"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.Operation != "image_generate" || resp.ModelID != "nano_banana_pro" || resp.ModelName != "Nano Banana Pro" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	jobID, err := uuid.Parse(resp.ID)
	if err != nil {
		t.Fatalf("invalid job id: %v", err)
	}
	job, err := fixture.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if len(job.InputArtifactIDs) != 1 || job.InputArtifactIDs[0] != artifact.ID {
		t.Fatalf("unexpected job input artifacts: %+v", job.InputArtifactIDs)
	}
	var params struct {
		ModelID              string      `json:"model_id"`
		ModelName            string      `json:"model_name"`
		Provider             string      `json:"provider"`
		ModelCode            string      `json:"model_code"`
		ReferenceArtifactIDs []uuid.UUID `json:"reference_artifact_ids"`
	}
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatalf("invalid params: %v", err)
	}
	if params.ModelID != "nano_banana_pro" || params.ModelName != "Nano Banana Pro" || params.Provider != "apimart" || params.ModelCode != "gemini-3-pro-image-preview" {
		t.Fatalf("unexpected stored model params: %+v", params)
	}
	if len(params.ReferenceArtifactIDs) != 1 || params.ReferenceArtifactIDs[0] != artifact.ID {
		t.Fatalf("unexpected stored reference params: %+v", params.ReferenceArtifactIDs)
	}
}

func TestHandler_CreateJob_ReferenceArtifactsRejectTooMany(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		cfg.ImageReferenceEnabled = true
	})
	routes := fixture.handler.Routes()
	ctx := context.Background()
	user := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	ids := make([]string, 0, 15)
	for i := 0; i < 15; i++ {
		artifact := createTestArtifact(t, fixture, user.ID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady)
		ids = append(ids, artifact.ID.String())
	}

	body, _ := json.Marshal(map[string]any{
		"operation":              "image_generate",
		"prompt":                 "image with too many references",
		"model_id":               "nano_banana_pro",
		"reference_artifact_ids": ids,
	})
	req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	req.Header.Set("X-Idempotency-Key", "reference-too-many")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "too many reference artifacts") {
		t.Fatalf("expected too-many-references error, got %s", w.Body.String())
	}
	jobs, err := fixture.jobRepo.ListByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("too many references must not create a job, got %d", len(jobs))
	}
}

func TestHandler_CreateJob_ReferenceArtifactsRejectForeignAndNonImage(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()
	owner := &domain.User{VKUserID: 100, Role: domain.RoleUser, Status: domain.StatusActive}
	requester := &domain.User{VKUserID: 777, Role: domain.RoleUser, Status: domain.StatusActive}
	if err := fixture.userRepo.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := fixture.userRepo.Create(ctx, requester); err != nil {
		t.Fatalf("create requester: %v", err)
	}
	foreign := createTestArtifact(t, fixture, owner.ID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady)
	nonImage := createTestArtifact(t, fixture, requester.ID, domain.ArtifactKindInput, domain.MediaTypeText, domain.ArtifactStatusReady)

	tests := []struct {
		name     string
		id       uuid.UUID
		wantCode int
	}{
		{name: "foreign", id: foreign.ID, wantCode: http.StatusNotFound},
		{name: "non-image", id: nonImage.ID, wantCode: http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{
				"operation":              "image_generate",
				"prompt":                 "image with reference",
				"model_id":               "nano_banana_pro",
				"reference_artifact_ids": []string{tc.id.String()},
			})
			req := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Launch-Params", devLaunchParams(777))
			req.Header.Set("X-Idempotency-Key", "reference-"+tc.name)
			w := httptest.NewRecorder()
			routes.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandler_CreateImageJobDoesNotCreateChatConversationMessages(t *testing.T) {
	fixture := newTestFixture("", nil)
	routes := fixture.handler.Routes()
	ctx := context.Background()

	body, _ := json.Marshal(map[string]string{
		"operation": "image_generate",
		"prompt":    "image only",
		"model_id":  "nano_banana_pro",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	createReq.Header.Set("X-Idempotency-Key", "image-create-not-chat")
	createW := httptest.NewRecorder()
	routes.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create image expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	var createResp struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("invalid create response: %v", err)
	}
	user, err := fixture.userRepo.GetByVKUserID(ctx, 777)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	conversation := &domain.Conversation{
		UserID:           user.ID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "thread-a",
		Title:            "Chat thread",
	}
	if err := fixture.conversationRepo.CreateConversation(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	textJobID := uuid.New()
	if _, err := fixture.conversationRepo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversation.ID,
		JobID:          textJobID,
		Role:           domain.ConversationRoleUser,
		Text:           "chat prompt",
		TokenCount:     2,
	}); err != nil {
		t.Fatalf("create message: %v", err)
	}

	msgReq := httptest.NewRequest(http.MethodGet, "/miniapp/chat/conversations/thread-a/messages", nil)
	msgReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	msgW := httptest.NewRecorder()
	routes.ServeHTTP(msgW, msgReq)
	if msgW.Code != http.StatusOK {
		t.Fatalf("messages expected 200, got %d: %s", msgW.Code, msgW.Body.String())
	}
	var msgResp struct {
		Items []struct {
			JobID uuid.UUID `json:"job_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(msgW.Body.Bytes(), &msgResp); err != nil {
		t.Fatalf("invalid message response: %v", err)
	}
	for _, item := range msgResp.Items {
		if item.JobID == createResp.ID {
			t.Fatalf("image job leaked into chat messages: %+v", msgResp.Items)
		}
	}

	jobsReq := httptest.NewRequest(http.MethodGet, "/miniapp/jobs", nil)
	jobsReq.Header.Set("X-Launch-Params", devLaunchParams(777))
	jobsW := httptest.NewRecorder()
	routes.ServeHTTP(jobsW, jobsReq)
	if jobsW.Code != http.StatusOK {
		t.Fatalf("jobs expected 200, got %d: %s", jobsW.Code, jobsW.Body.String())
	}
	var jobsResp struct {
		Items []struct {
			ID        uuid.UUID `json:"id"`
			Operation string    `json:"operation"`
		} `json:"items"`
	}
	if err := json.Unmarshal(jobsW.Body.Bytes(), &jobsResp); err != nil {
		t.Fatalf("invalid jobs response: %v", err)
	}
	var foundImage bool
	for _, item := range jobsResp.Items {
		if item.ID == createResp.ID && item.Operation == "image_generate" {
			foundImage = true
		}
	}
	if !foundImage {
		t.Fatalf("image job not listable through /miniapp/jobs: %+v", jobsResp.Items)
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
		req.Header.Set("X-Idempotency-Key", "rate-limit-key")
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
