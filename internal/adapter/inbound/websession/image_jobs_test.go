package websession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/preparedjobexpiry"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/resultservice"
)

func TestWebImageJobPrepareRouteExistsAndFailsClosedWhenUnconfigured(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	req := authenticatedConversationRequest(t, http.MethodPost, "/web/v1/image-jobs/prepare", sessions, uuid.New())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://app.example.test")
	req.Header.Set("X-CSRF-Token", "csrf")
	req.Header.Set("X-Idempotency-Key", uuid.NewString())
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	req.Body = io.NopCloser(strings.NewReader(`{"prompt":"night city","model_id":"nano-banana-2"}`))

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestWebImageJobPrepareUsesServerResolvedPriceAndReturnsSafeDTO(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]string{
		"prompt":        "night city after rain",
		"model_id":      modelcatalog.MiniAppImageNanoBanana2,
		"image_quality": modelcatalog.ImageQuality2K,
	})
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if jobs.prepareCalls != 1 {
		t.Fatalf("prepare calls = %d, want 1", jobs.prepareCalls)
	}
	if jobs.prepareInput.AccountID != accountID || jobs.prepareInput.Operation != domain.OperationImageGenerate || jobs.prepareInput.Modality != domain.ModalityImage {
		t.Fatalf("prepare input = %+v", jobs.prepareInput)
	}
	if jobs.prepareInput.CostEstimateCredits != 60 || jobs.prepareInput.PricingSnapshot.InternalCredits != 60 {
		t.Fatalf("server price = %d / %+v, want 60", jobs.prepareInput.CostEstimateCredits, jobs.prepareInput.PricingSnapshot)
	}
	assertPreparedImageJobParams(t, jobs.prepareInput.Params, "night city after rain", modelcatalog.MiniAppImageNanoBanana2, modelcatalog.ImageQuality2K)
	assertSafeWebImagePreparation(t, rec.Body.Bytes(), jobs.prepared.ID, domain.JobStatusPrepared, 60, 104)
	if bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte("provider")) || bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte("model_code")) {
		t.Fatalf("public response leaked private routing: %s", rec.Body.String())
	}
}

func TestWebImageJobPrepareRejectsAccountScopedRateLimitBeforeDurablePrepare(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	limiter := &imagePrepareLimiterStub{allowed: false}
	h.deps.ImageJobPrepareLimiter = limiter
	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]string{
		"prompt":        "night city after rain",
		"model_id":      modelcatalog.MiniAppImageNanoBanana2,
		"image_quality": modelcatalog.ImageQuality2K,
	})
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Fatalf("Retry-After = %q, want no unverified retry deadline", got)
	}
	if jobs.prepareCalls != 0 {
		t.Fatalf("rate-limited request created %d prepared jobs", jobs.prepareCalls)
	}
	if len(limiter.keys) != 1 || limiter.keys[0] != "account:"+accountID.String() {
		t.Fatalf("rate limit keys = %#v, want exact account scope", limiter.keys)
	}
}

func TestWebImageJobPrepareFailsClosedWhenSharedRateLimiterIsUnavailable(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	h.deps.ImageJobPrepareLimiter = &imagePrepareLimiterStub{allowed: true, err: errors.New("redis unavailable")}
	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]string{
		"prompt":        "night city after rain",
		"model_id":      modelcatalog.MiniAppImageNanoBanana2,
		"image_quality": modelcatalog.ImageQuality2K,
	})
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if jobs.prepareCalls != 0 {
		t.Fatalf("unavailable limiter created %d prepared jobs", jobs.prepareCalls)
	}
}

func TestWebImageJobPrepareMapsPreparedJobCapacityToRateLimit(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobs.prepareErr = domain.ErrPreparedJobLimitExceeded
	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]string{
		"prompt":        "night city after rain",
		"model_id":      modelcatalog.MiniAppImageNanoBanana2,
		"image_quality": modelcatalog.ImageQuality2K,
	})
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestWebImageJobPrepareReplayReturnsStoredJobBeforeCurrentPriceResolution(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	idempotencyKey := uuid.New()
	pricing, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static pricing catalog: %v", err)
	}
	originalSnapshot, err := pricing.Snapshot(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: modelcatalog.MiniAppImageNanoBanana2,
		Quality:      modelcatalog.ImageQuality2K,
	})
	if err != nil {
		t.Fatalf("original snapshot: %v", err)
	}
	existing := &domain.Job{
		ID:              uuid.New(),
		AccountID:       accountID,
		Source:          "web",
		ChannelContext:  &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:      domain.ResultModeAccountHistory,
		OperationType:   domain.OperationImageGenerate,
		Modality:        domain.ModalityImage,
		Status:          domain.JobStatusPrepared,
		IdempotencyKey:  idempotencyKey.String(),
		CorrelationID:   "web-image:" + idempotencyKey.String(),
		CostEstimate:    originalSnapshot.InternalCredits,
		PricingSnapshot: mustMarshalPricingSnapshot(originalSnapshot),
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       "night city after rain",
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-original-model-code",
			Resolution:   modelcatalog.ImageQuality2K,
			Size:         "1:1",
		}),
	}
	reader := &imageJobIdempotencyReaderStub{job: existing}
	h.deps.ImageJobIdempotency = reader
	limiter := &imagePrepareLimiterStub{allowed: true}
	h.deps.ImageJobPrepareLimiter = limiter
	changedPricing := &imagePricingSnapshotStub{snapshot: originalSnapshot}
	changedPricing.snapshot.InternalCredits = originalSnapshot.InternalCredits + 17
	h.deps.ImagePricing = changedPricing

	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]string{
		"prompt":        "night city after rain",
		"model_id":      modelcatalog.MiniAppImageNanoBanana2,
		"image_quality": modelcatalog.ImageQuality2K,
	})
	req.Header.Set("X-Idempotency-Key", idempotencyKey.String())
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if reader.accountID != accountID || reader.key != idempotencyKey.String() || reader.calls != 1 {
		t.Fatalf("idempotency scope = account:%s key:%q calls:%d", reader.accountID, reader.key, reader.calls)
	}
	if changedPricing.calls != 0 {
		t.Fatalf("replay resolved a new price %d times", changedPricing.calls)
	}
	if jobs.prepareCalls != 0 {
		t.Fatalf("replay created another prepared job %d times", jobs.prepareCalls)
	}
	if len(limiter.keys) != 0 {
		t.Fatalf("idempotent replay consumed rate-limit quota: %#v", limiter.keys)
	}
	assertSafeWebImagePreparation(t, rec.Body.Bytes(), existing.ID, domain.JobStatusPrepared, originalSnapshot.InternalCredits, 104)
}

func TestWebImageJobPrepareReplayRejectsDifferentPublicIntentBeforeCurrentPriceResolution(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	idempotencyKey := uuid.New()
	pricing, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static pricing catalog: %v", err)
	}
	snapshot, err := pricing.Snapshot(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: modelcatalog.MiniAppImageNanoBanana2,
		Quality:      modelcatalog.ImageQuality2K,
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	h.deps.ImageJobIdempotency = &imageJobIdempotencyReaderStub{job: &domain.Job{
		ID:              uuid.New(),
		AccountID:       accountID,
		Source:          "web",
		ChannelContext:  &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:      domain.ResultModeAccountHistory,
		OperationType:   domain.OperationImageGenerate,
		Modality:        domain.ModalityImage,
		Status:          domain.JobStatusPrepared,
		IdempotencyKey:  idempotencyKey.String(),
		CorrelationID:   "web-image:" + idempotencyKey.String(),
		CostEstimate:    snapshot.InternalCredits,
		PricingSnapshot: mustMarshalPricingSnapshot(snapshot),
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       "night city after rain",
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-original-model-code",
			Resolution:   modelcatalog.ImageQuality2K,
			Size:         "1:1",
		}),
	}}
	changedPricing := &imagePricingSnapshotStub{snapshot: snapshot}
	changedPricing.snapshot.InternalCredits = snapshot.InternalCredits + 17
	h.deps.ImagePricing = changedPricing

	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]string{
		"prompt":        "different prompt",
		"model_id":      modelcatalog.MiniAppImageNanoBanana2,
		"image_quality": modelcatalog.ImageQuality2K,
	})
	req.Header.Set("X-Idempotency-Key", idempotencyKey.String())
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if changedPricing.calls != 0 {
		t.Fatalf("different replay resolved a new price %d times", changedPricing.calls)
	}
	if jobs.prepareCalls != 0 {
		t.Fatalf("different replay created another prepared job %d times", jobs.prepareCalls)
	}
}

func TestWebImageJobPrepareRejectsExpiredStoredConfirmationBeforeRateLimit(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	idempotencyKey := uuid.New()
	expiredAt := time.Now().Add(-time.Second)
	h.deps.ImageJobIdempotency = &imageJobIdempotencyReaderStub{job: newPreparedWebImageJobForReplay(t, accountID, idempotencyKey, "night city after rain", &expiredAt)}
	limiter := &imagePrepareLimiterStub{allowed: true}
	h.deps.ImageJobPrepareLimiter = limiter
	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]string{
		"prompt":        "night city after rain",
		"model_id":      modelcatalog.MiniAppImageNanoBanana2,
		"image_quality": modelcatalog.ImageQuality2K,
	})
	req.Header.Set("X-Idempotency-Key", idempotencyKey.String())
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if jobs.prepareCalls != 0 {
		t.Fatalf("expired confirmation created %d prepared jobs", jobs.prepareCalls)
	}
	if len(limiter.keys) != 0 {
		t.Fatalf("expired confirmation consumed rate-limit quota: %#v", limiter.keys)
	}
}

func TestWebImageJobPrepareConflictRereadsStoredJobAfterPriceChange(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	idempotencyKey := uuid.New()
	stored := newPreparedWebImageJobForReplay(t, accountID, idempotencyKey, "night city after rain", nil)
	var originalSnapshot pricingcatalog.PricingSnapshot
	if err := json.Unmarshal(stored.PricingSnapshot, &originalSnapshot); err != nil {
		t.Fatalf("decode stored snapshot: %v", err)
	}
	changedPricing := &imagePricingSnapshotStub{snapshot: originalSnapshot}
	changedPricing.snapshot.Floor.Amount *= 2
	changedPricing.snapshot.InternalCreditCap = 0
	changedPricing.snapshot.FloorAmountCap = 0
	changedCredits, err := pricingcatalog.CalculateInternalCredits(
		changedPricing.snapshot.Floor,
		changedPricing.snapshot.UnitConversion,
		changedPricing.snapshot.Multiplier,
		pricingcatalog.SafetyCaps{},
	)
	if err != nil || changedCredits == originalSnapshot.InternalCredits {
		t.Fatalf("derive changed valid price = %d, %v", changedCredits, err)
	}
	changedPricing.snapshot.InternalCredits = changedCredits
	h.deps.ImagePricing = changedPricing
	h.deps.ImageJobIdempotency = &imageJobIdempotencyReaderStub{results: []imageJobIdempotencyLookup{
		{err: domain.ErrNotFound},
		{job: stored},
	}}
	jobs.prepareErr = domain.ErrConflict
	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]string{
		"prompt":        "night city after rain",
		"model_id":      modelcatalog.MiniAppImageNanoBanana2,
		"image_quality": modelcatalog.ImageQuality2K,
	})
	req.Header.Set("X-Idempotency-Key", idempotencyKey.String())
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if jobs.prepareCalls != 1 {
		t.Fatalf("prepare calls = %d, want one racing initial attempt", jobs.prepareCalls)
	}
	if changedPricing.calls != 1 {
		t.Fatalf("current pricing calls = %d, want one initial preparation", changedPricing.calls)
	}
	assertSafeWebImagePreparation(t, rec.Body.Bytes(), stored.ID, domain.JobStatusPrepared, originalSnapshot.InternalCredits, 104)
}

func TestWebImageModelsReturnsOnlyServerSafeCatalog(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-models", sessions, uuid.New())
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Items []struct {
			ID                     string   `json:"id"`
			Name                   string   `json:"name"`
			QualityOptions         []string `json:"quality_options"`
			DefaultQuality         string   `json:"default_quality"`
			SupportsReferenceImage bool     `json:"supports_reference_image"`
			MaxReferenceImages     int      `json:"max_reference_images"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != modelcatalog.MiniAppImageNanoBanana2 || response.Items[0].Name != "Nano Banana 2" || response.Items[0].DefaultQuality != modelcatalog.ImageQuality1K {
		t.Fatalf("catalog = %+v", response.Items)
	}
	if response.Items[0].MaxReferenceImages != 4 || !response.Items[0].SupportsReferenceImage {
		t.Fatalf("reference support = %+v", response.Items[0])
	}
	if bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte("provider")) || bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte("model_code")) || bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte("estimate_credits")) {
		t.Fatalf("catalog leaked private routing or mutable price: %s", rec.Body.String())
	}
}

func TestWebImageJobActivateUsesCookiePrincipalAndMapsInsufficientCredits(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobID := uuid.New()
	jobs.activateJob = &domain.Job{
		ID:            jobID,
		AccountID:     accountID,
		Source:        "web",
		OperationType: domain.OperationImageGenerate,
		Modality:      domain.ModalityImage,
		Status:        domain.JobStatusAwaitingPayment,
		CostEstimate:  60,
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       "image prompt",
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-model-code",
		}),
	}
	jobs.activateErr = domain.ErrInsufficientCredits
	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/image-jobs/"+jobID.String()+"/activate", nil)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if jobs.activateCalls != 1 || jobs.activateAccountID != accountID || jobs.activateJobID != jobID {
		t.Fatalf("activate invocation = calls:%d account:%s job:%s", jobs.activateCalls, jobs.activateAccountID, jobs.activateJobID)
	}
	assertSafeWebImageActivation(t, rec.Body.Bytes(), jobID, domain.JobStatusAwaitingPayment)
}

func TestWebImageJobReadUsesExactCookieAccountAndSafeDTO(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobID := uuid.New()
	reader := &imageJobReaderStub{job: &domain.Job{
		ID:            jobID,
		AccountID:     accountID,
		Source:        "web",
		OperationType: domain.OperationImageGenerate,
		Modality:      domain.ModalityImage,
		Status:        domain.JobStatusQueued,
		CostEstimate:  60,
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       "image prompt",
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-model-code",
		}),
	}}
	h.deps.ImageJobReader = reader
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs/"+jobID.String(), sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if reader.accountID != accountID || reader.jobID != jobID {
		t.Fatalf("reader scope = account:%s job:%s", reader.accountID, reader.jobID)
	}
	assertSafeWebImageActivation(t, rec.Body.Bytes(), jobID, domain.JobStatusQueued)
}

func TestWebImageJobReadReconcilesExactDuePreparationBeforeReturningIt(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobID := uuid.New()
	expiresAt := time.Now().Add(-time.Minute)
	reader := &imageJobReaderStub{job: newPreparedWebImageJobForReplay(t, accountID, uuid.New(), "image prompt", &expiresAt)}
	reader.job.ID = jobID
	expiry := &imageJobExpiryReconcilerStub{onReconcileJob: func() {
		reader.job.Status = domain.JobStatusExpired
		reader.job.ErrorCode = domain.PreparedConfirmationExpiredCode
		reader.job.ErrorMessage = domain.PreparedConfirmationExpiredMessage
	}}
	h.deps.ImageJobReader = reader
	h.deps.ImageJobExpiry = expiry
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs/"+jobID.String(), sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if expiry.jobCalls != 1 || expiry.jobAccountID != accountID || expiry.jobID != jobID {
		t.Fatalf("expiry job scope = calls:%d account:%s job:%s", expiry.jobCalls, expiry.jobAccountID, expiry.jobID)
	}
	assertSafeWebImageActivation(t, rec.Body.Bytes(), jobID, domain.JobStatusExpired)
}

func TestWebImageJobReadFailsClosedWhenExpiryReconciliationFails(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobID := uuid.New()
	reader := &imageJobReaderStub{job: newWebImageHistoryJob(t, accountID, jobID, time.Now())}
	h.deps.ImageJobReader = reader
	h.deps.ImageJobExpiry = &imageJobExpiryReconcilerStub{jobErr: errors.New("database unavailable")}
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs/"+jobID.String(), sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if reader.accountID != uuid.Nil || reader.jobID != uuid.Nil {
		t.Fatalf("reader must not expose stale job after expiry error: account:%s job:%s", reader.accountID, reader.jobID)
	}
}

func TestWebImageJobHistoryUsesBoundedWebSourceCursor(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	history := &imageJobHistoryReaderStub{jobs: []*domain.Job{
		{
			ID:            uuid.New(),
			AccountID:     accountID,
			Source:        "web",
			OperationType: domain.OperationImageGenerate,
			Modality:      domain.ModalityImage,
			Status:        domain.JobStatusSucceeded,
			CostEstimate:  60,
			Params: mustMarshalWebImageJobParams(t, webImageJobParams{
				Prompt:       "image prompt",
				ModelID:      modelcatalog.MiniAppImageNanoBanana2,
				ModelName:    "Nano Banana 2",
				ImageQuality: modelcatalog.ImageQuality2K,
				Provider:     domain.ProviderPoYo,
				ModelCode:    "private-model-code",
			}),
		},
	}}
	h.deps.ImageJobHistory = history
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs?limit=10", sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if history.filter.AccountID == nil || *history.filter.AccountID != accountID || history.filter.Source != "web" || history.filter.Operation != domain.OperationImageGenerate || history.filter.Modality != domain.ModalityImage || history.limit != 11 || history.after != nil {
		t.Fatalf("history query = %+v, limit=%d after=%+v", history.filter, history.limit, history.after)
	}
	var response struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
		HasMore bool `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != history.jobs[0].ID || response.HasMore {
		t.Fatalf("history response = %+v", response)
	}
	if bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte("provider")) || bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte("model_code")) {
		t.Fatalf("history leaked private routing: %s", rec.Body.String())
	}
}

func TestWebImageJobHistoryReconcilesDuePreparationsAndRefreshesVisiblePage(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	expiresAt := time.Now().Add(-time.Minute)
	prepared := newPreparedWebImageJobForReplay(t, accountID, uuid.New(), "image prompt", &expiresAt)
	prepared.CreatedAt = time.Now().Add(-2 * time.Minute)
	prepared.UpdatedAt = prepared.CreatedAt
	expired := *prepared
	expired.Status = domain.JobStatusExpired
	expired.ErrorCode = domain.PreparedConfirmationExpiredCode
	expired.ErrorMessage = domain.PreparedConfirmationExpiredMessage
	history := &imageJobHistoryReaderStub{results: []imageJobHistoryResult{
		{jobs: []*domain.Job{prepared}},
		{jobs: []*domain.Job{&expired}},
	}}
	expiry := &imageJobExpiryReconcilerStub{}
	h.deps.ImageJobHistory = history
	h.deps.ImageJobExpiry = expiry
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs?limit=10", sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if expiry.accountCalls != 1 || expiry.accountID != accountID || expiry.accountLimit != preparedjobexpiry.DefaultAccountReconcileLimit {
		t.Fatalf("expiry account scope = calls:%d account:%s limit:%d", expiry.accountCalls, expiry.accountID, expiry.accountLimit)
	}
	if expiry.jobCalls != 1 || expiry.jobAccountID != accountID || expiry.jobID != prepared.ID {
		t.Fatalf("expiry exact scope = calls:%d account:%s job:%s", expiry.jobCalls, expiry.jobAccountID, expiry.jobID)
	}
	if history.calls != 2 {
		t.Fatalf("history calls = %d, want initial read plus refreshed page", history.calls)
	}
	var response struct {
		Items []struct {
			ID     uuid.UUID        `json:"id"`
			Status domain.JobStatus `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != prepared.ID || response.Items[0].Status != domain.JobStatusExpired {
		t.Fatalf("history response = %+v", response)
	}
}

func TestWebImageJobHistoryFailsClosedWhenAccountExpiryReconciliationFails(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	history := &imageJobHistoryReaderStub{}
	h.deps.ImageJobHistory = history
	h.deps.ImageJobExpiry = &imageJobExpiryReconcilerStub{accountErr: errors.New("database unavailable")}
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs?limit=10", sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if history.calls != 0 {
		t.Fatalf("history must not expose stale jobs after expiry error, calls = %d", history.calls)
	}
}

func TestWebImageJobHistoryUsesOpaqueCursorForSubsequentBoundedPage(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	newestID := uuid.New()
	oldestID := uuid.New()
	newestCreatedAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	history := &imageJobHistoryReaderStub{jobs: []*domain.Job{
		newWebImageHistoryJob(t, accountID, newestID, newestCreatedAt),
		newWebImageHistoryJob(t, accountID, oldestID, newestCreatedAt.Add(-time.Minute)),
	}}
	h.deps.ImageJobHistory = history
	firstRequest := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs?limit=1", sessions, accountID)
	firstRecorder := httptest.NewRecorder()

	h.Routes().ServeHTTP(firstRecorder, firstRequest)

	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", firstRecorder.Code, firstRecorder.Body.String())
	}
	var firstPage struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
		HasMore    bool    `json:"has_more"`
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.Unmarshal(firstRecorder.Body.Bytes(), &firstPage); err != nil {
		t.Fatalf("decode first history page: %v", err)
	}
	if len(firstPage.Items) != 1 || firstPage.Items[0].ID != newestID || !firstPage.HasMore || firstPage.NextCursor == nil || *firstPage.NextCursor == "" {
		t.Fatalf("first history page = %+v", firstPage)
	}
	if strings.Contains(*firstPage.NextCursor, newestID.String()) || strings.Contains(*firstPage.NextCursor, newestCreatedAt.Format(time.RFC3339Nano)) {
		t.Fatalf("next cursor must be opaque, got %q", *firstPage.NextCursor)
	}

	history.jobs = []*domain.Job{newWebImageHistoryJob(t, accountID, oldestID, newestCreatedAt.Add(-time.Minute))}
	secondRequest := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs?limit=1&cursor="+*firstPage.NextCursor, sessions, accountID)
	secondRecorder := httptest.NewRecorder()

	h.Routes().ServeHTTP(secondRecorder, secondRequest)

	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", secondRecorder.Code, secondRecorder.Body.String())
	}
	if history.after == nil || history.after.ID != newestID || !history.after.CreatedAt.Equal(newestCreatedAt) {
		t.Fatalf("history after = %+v, want newest cursor", history.after)
	}
	var secondPage struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
		HasMore    bool    `json:"has_more"`
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &secondPage); err != nil {
		t.Fatalf("decode second history page: %v", err)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].ID != oldestID || secondPage.HasMore || secondPage.NextCursor != nil {
		t.Fatalf("second history page = %+v", secondPage)
	}
}

func TestWebImageJobHistoryRejectsMalformedCursorBeforeRepository(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	history := &imageJobHistoryReaderStub{}
	h.deps.ImageJobHistory = history
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs?cursor=not-a-valid-cursor", sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if history.calls != 0 {
		t.Fatalf("history should not be queried for malformed cursor, calls = %d", history.calls)
	}
}

func TestWebImageJobResultUsesOwnerScopedWebJobAndSafeMetadata(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobID := uuid.New()
	h.deps.ImageJobReader = &imageJobReaderStub{job: &domain.Job{
		ID:            jobID,
		AccountID:     accountID,
		Source:        "web",
		OperationType: domain.OperationImageGenerate,
		Modality:      domain.ModalityImage,
		Status:        domain.JobStatusSucceeded,
		CostEstimate:  60,
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       "image prompt",
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-model-code",
		}),
	}}
	artifactID := uuid.New()
	results := &imageResultReaderStub{result: resultservice.Result{
		ID:        jobID,
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Status:    domain.JobStatusSucceeded,
		Artifacts: []resultservice.ArtifactMetadata{{ID: artifactID, MediaType: domain.MediaTypeImage, MIMEType: "image/png", SizeBytes: 42, Width: 1024, Height: 1024}},
	}}
	h.deps.ImageResults = results
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-jobs/"+jobID.String()+"/result", sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if results.accountID != accountID || results.jobID != jobID {
		t.Fatalf("result scope = account:%s job:%s", results.accountID, results.jobID)
	}
	var response struct {
		JobID     uuid.UUID `json:"job_id"`
		Status    string    `json:"status"`
		Artifacts []struct {
			ID uuid.UUID `json:"id"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if response.JobID != jobID || response.Status != string(domain.JobStatusSucceeded) || len(response.Artifacts) != 1 || response.Artifacts[0].ID != artifactID {
		t.Fatalf("result response = %+v", response)
	}
	for _, forbidden := range []string{"provider", "model_code", "storage_key", "storage_bucket", "url"} {
		if bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte(forbidden)) {
			t.Fatalf("result leaked forbidden data %q: %s", forbidden, rec.Body.String())
		}
	}
}

func TestWebImageArtifactStreamsOwnerModeratedOutputThroughWebOrigin(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobID := uuid.New()
	artifactID := uuid.New()
	artifactBytes := []byte("safe generated image bytes")
	h.deps.ImageJobReader = &imageJobReaderStub{job: &domain.Job{
		ID:            jobID,
		AccountID:     accountID,
		Source:        "web",
		OperationType: domain.OperationImageGenerate,
		Modality:      domain.ModalityImage,
		Status:        domain.JobStatusSucceeded,
		CostEstimate:  60,
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       "image prompt",
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-model-code",
		}),
	}}
	h.deps.ImageResults = &imageResultReaderStub{result: resultservice.Result{
		ID:        jobID,
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Status:    domain.JobStatusSucceeded,
		Artifacts: []resultservice.ArtifactMetadata{{ID: artifactID, MediaType: domain.MediaTypeImage, MIMEType: "image/png", SizeBytes: int64(len(artifactBytes))}},
	}}
	h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: &domain.Artifact{
		ID:             artifactID,
		OwnerAccountID: accountID,
		JobID:          &jobID,
		Kind:           domain.ArtifactKindOutput,
		MediaType:      domain.MediaTypeImage,
		MimeType:       "image/png",
		StorageBucket:  "private-artifacts",
		StorageKey:     "web/generated/output.png",
		SizeBytes:      int64(len(artifactBytes)),
		Status:         domain.ArtifactStatusReady,
	}}
	store := &imageArtifactObjectStoreStub{data: artifactBytes}
	h.deps.ImageArtifactURLSigner = store
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-artifacts/"+artifactID.String(), sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type = %q, want image/png", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), artifactBytes) {
		t.Fatalf("artifact body = %q, want %q", rec.Body.Bytes(), artifactBytes)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("artifact response must remain same-origin, location = %q", got)
	}
	if store.bucket != "private-artifacts" || store.key != "web/generated/output.png" {
		t.Fatalf("object store invocation = bucket:%q key:%q", store.bucket, store.key)
	}
}

func TestWebImageArtifactRejectsSignedURLOutsideConfiguredObjectStore(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobID := uuid.New()
	artifactID := uuid.New()
	h.deps.ImageJobReader = &imageJobReaderStub{job: &domain.Job{
		ID:            jobID,
		AccountID:     accountID,
		Source:        "web",
		OperationType: domain.OperationImageGenerate,
		Modality:      domain.ModalityImage,
		Status:        domain.JobStatusSucceeded,
		CostEstimate:  60,
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       "image prompt",
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-model-code",
		}),
	}}
	h.deps.ImageResults = &imageResultReaderStub{result: resultservice.Result{
		ID:        jobID,
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Status:    domain.JobStatusSucceeded,
		Artifacts: []resultservice.ArtifactMetadata{{ID: artifactID, MediaType: domain.MediaTypeImage, MIMEType: "image/png", SizeBytes: 42}},
	}}
	h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: &domain.Artifact{
		ID:             artifactID,
		OwnerAccountID: accountID,
		JobID:          &jobID,
		Kind:           domain.ArtifactKindOutput,
		MediaType:      domain.MediaTypeImage,
		MimeType:       "image/png",
		StorageBucket:  "private-artifacts",
		StorageKey:     "web/generated/output.png",
		SizeBytes:      42,
		Status:         domain.ArtifactStatusReady,
	}}
	signer := &imageArtifactURLSignerStub{url: "https://attacker.example.test/signed-output"}
	h.deps.ImageArtifactURLSigner = signer
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-artifacts/"+artifactID.String(), sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("redirect location = %q, want no redirect", got)
	}
}

func TestImageArtifactRedirectPolicyPermitsInsecureHTTPOnlyForExplicitLoopbackDevelopment(t *testing.T) {
	policy, err := NewImageArtifactRedirectPolicy("http://127.0.0.1:9000", false, "path", "development", true)
	if err != nil {
		t.Fatalf("new local development policy: %v", err)
	}
	if !policy.Allows("http://127.0.0.1:9000/private-artifacts/signed-output", "private-artifacts") {
		t.Fatal("explicit local development policy must allow its own HTTP storage origin")
	}

	for _, testCase := range []struct {
		name        string
		endpoint    string
		environment string
		explicit    bool
	}{
		{name: "not explicit", endpoint: "http://127.0.0.1:9000", environment: "development", explicit: false},
		{name: "non-loopback", endpoint: "http://objects.example.test", environment: "development", explicit: true},
		{name: "non-development", endpoint: "http://127.0.0.1:9000", environment: "staging", explicit: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := NewImageArtifactRedirectPolicy(testCase.endpoint, false, "path", testCase.environment, testCase.explicit); err == nil {
				t.Fatal("insecure HTTP policy must fail closed")
			}
		})
	}
}

func TestImageArtifactRedirectPolicyRejectsStorageEndpointPaths(t *testing.T) {
	if _, err := NewImageArtifactRedirectPolicy("https://objects.example.test/private-artifacts", true, "path", "development", false); err == nil {
		t.Fatal("storage endpoint with a path must not become a redirect allowlist")
	}
}

func TestImageArtifactRedirectPolicyVirtualHostedURLMustUseTheStoredBucket(t *testing.T) {
	policy, err := NewImageArtifactRedirectPolicy("https://objects.example.test", true, "virtual-hosted", "development", false)
	if err != nil {
		t.Fatalf("new virtual-hosted policy: %v", err)
	}
	if !policy.Allows("https://private-artifacts.objects.example.test/signed-output", "private-artifacts") {
		t.Fatal("stored virtual-hosted bucket URL must be allowed")
	}
	if policy.Allows("https://another-bucket.objects.example.test/signed-output", "private-artifacts") {
		t.Fatal("another virtual-hosted bucket must not be allowed")
	}
}

func TestWebImageArtifactDoesNotSignOutputOutsideTheModeratedResult(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobID := uuid.New()
	artifactID := uuid.New()
	h.deps.ImageJobReader = &imageJobReaderStub{job: &domain.Job{
		ID:            jobID,
		AccountID:     accountID,
		Source:        "web",
		OperationType: domain.OperationImageGenerate,
		Modality:      domain.ModalityImage,
		Status:        domain.JobStatusSucceeded,
		CostEstimate:  60,
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       "image prompt",
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-model-code",
		}),
	}}
	h.deps.ImageResults = &imageResultReaderStub{result: resultservice.Result{
		ID:        jobID,
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		Status:    domain.JobStatusSucceeded,
		Artifacts: []resultservice.ArtifactMetadata{{ID: uuid.New(), MediaType: domain.MediaTypeImage, MIMEType: "image/png", SizeBytes: 42}},
	}}
	h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: &domain.Artifact{
		ID:             artifactID,
		OwnerAccountID: accountID,
		JobID:          &jobID,
		Kind:           domain.ArtifactKindOutput,
		MediaType:      domain.MediaTypeImage,
		MimeType:       "image/png",
		StorageBucket:  "private-artifacts",
		StorageKey:     "web/generated/not-moderated.png",
		SizeBytes:      42,
		Status:         domain.ArtifactStatusReady,
	}}
	signer := &imageArtifactURLSignerStub{url: "https://objects.example.test/signed-output"}
	h.deps.ImageArtifactURLSigner = signer
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-artifacts/"+artifactID.String(), sessions, accountID)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if signer.bucket != "" || signer.key != "" {
		t.Fatalf("signer must not run for an artifact outside the moderated result: bucket:%q key:%q", signer.bucket, signer.key)
	}
}

type imageJobServiceStub struct {
	prepareCalls int
	prepareInput joborchestrator.PrepareAccountJobInput
	prepared     *domain.Job
	prepareErr   error

	activateCalls     int
	activateAccountID uuid.UUID
	activateJobID     uuid.UUID
	activateJob       *domain.Job
	activateErr       error
}

type imageJobReaderStub struct {
	accountID uuid.UUID
	jobID     uuid.UUID
	job       *domain.Job
	err       error
}

type imageJobIdempotencyReaderStub struct {
	accountID uuid.UUID
	key       string
	calls     int
	job       *domain.Job
	err       error
	results   []imageJobIdempotencyLookup
}

type imageJobIdempotencyLookup struct {
	job *domain.Job
	err error
}

type imagePrepareLimiterStub struct {
	allowed bool
	err     error
	keys    []string
}

func (s *imagePrepareLimiterStub) Allow(_ context.Context, key string) (bool, error) {
	s.keys = append(s.keys, key)
	return s.allowed, s.err
}

func (s *imageJobIdempotencyReaderStub) GetByIdempotencyKeyForAccount(_ context.Context, accountID uuid.UUID, key string) (*domain.Job, error) {
	s.accountID = accountID
	s.key = key
	s.calls++
	if len(s.results) > 0 {
		result := s.results[0]
		s.results = s.results[1:]
		if result.err != nil {
			return nil, result.err
		}
		if result.job == nil {
			return nil, domain.ErrNotFound
		}
		return result.job, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.job == nil {
		return nil, domain.ErrNotFound
	}
	return s.job, nil
}

type imagePricingSnapshotStub struct {
	snapshot pricingcatalog.PricingSnapshot
	calls    int
	err      error
}

func (s *imagePricingSnapshotStub) Snapshot(pricingcatalog.ProductKey) (pricingcatalog.PricingSnapshot, error) {
	s.calls++
	if s.err != nil {
		return pricingcatalog.PricingSnapshot{}, s.err
	}
	return s.snapshot, nil
}

type imageJobHistoryReaderStub struct {
	filter  domain.JobFilter
	limit   int
	after   *domain.JobCursor
	jobs    []*domain.Job
	results []imageJobHistoryResult
	err     error
	calls   int
}

type imageJobHistoryResult struct {
	jobs []*domain.Job
	err  error
}

func (s *imageJobHistoryReaderStub) ListCursor(_ context.Context, filter domain.JobFilter, limit int, after *domain.JobCursor) ([]*domain.Job, error) {
	s.calls++
	s.filter = filter
	s.limit = limit
	s.after = after
	if len(s.results) > 0 {
		result := s.results[0]
		s.results = s.results[1:]
		return result.jobs, result.err
	}
	return s.jobs, s.err
}

type imageJobExpiryReconcilerStub struct {
	accountCalls int
	accountID    uuid.UUID
	accountLimit int
	accountErr   error

	jobCalls       int
	jobAccountID   uuid.UUID
	jobID          uuid.UUID
	jobChanged     bool
	jobErr         error
	onReconcileJob func()
}

func (s *imageJobExpiryReconcilerStub) ReconcileAccount(_ context.Context, accountID uuid.UUID, limit int) (preparedjobexpiry.Result, error) {
	s.accountCalls++
	s.accountID = accountID
	s.accountLimit = limit
	return preparedjobexpiry.Result{}, s.accountErr
}

func (s *imageJobExpiryReconcilerStub) ReconcileJob(_ context.Context, accountID, jobID uuid.UUID) (bool, error) {
	s.jobCalls++
	s.jobAccountID = accountID
	s.jobID = jobID
	if s.onReconcileJob != nil {
		s.onReconcileJob()
	}
	return s.jobChanged, s.jobErr
}

func newWebImageHistoryJob(t *testing.T, accountID, jobID uuid.UUID, createdAt time.Time) *domain.Job {
	t.Helper()
	return &domain.Job{
		ID:            jobID,
		AccountID:     accountID,
		Source:        "web",
		OperationType: domain.OperationImageGenerate,
		Modality:      domain.ModalityImage,
		Status:        domain.JobStatusSucceeded,
		CostEstimate:  60,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       "image prompt",
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-model-code",
		}),
	}
}

type imageResultReaderStub struct {
	accountID uuid.UUID
	jobID     uuid.UUID
	result    resultservice.Result
	err       error
}

func (s *imageResultReaderStub) GetResult(_ context.Context, accountID, jobID uuid.UUID) (resultservice.Result, error) {
	s.accountID = accountID
	s.jobID = jobID
	return s.result, s.err
}

type imageArtifactReaderStub struct {
	accountID  uuid.UUID
	artifactID uuid.UUID
	artifact   *domain.Artifact
	err        error
}

func (s *imageArtifactReaderStub) GetByIDForAccount(_ context.Context, accountID, artifactID uuid.UUID) (*domain.Artifact, error) {
	s.accountID = accountID
	s.artifactID = artifactID
	return s.artifact, s.err
}

type imageArtifactURLSignerStub struct {
	bucket string
	key    string
	expiry time.Duration
	url    string
	err    error
}

func (s *imageArtifactURLSignerStub) PresignedGetURL(_ context.Context, bucket, key string, expiry time.Duration) (string, error) {
	s.bucket = bucket
	s.key = key
	s.expiry = expiry
	return s.url, s.err
}

type imageArtifactObjectStoreStub struct {
	bucket string
	key    string
	data   []byte
	err    error
}

func (s *imageArtifactObjectStoreStub) PresignedGetURL(_ context.Context, bucket, key string, expiry time.Duration) (string, error) {
	_ = bucket
	_ = key
	_ = expiry
	return "", errors.New("object store should stream the artifact instead of presigning it")
}

func (s *imageArtifactObjectStoreStub) GetObject(_ context.Context, bucket, key string) ([]byte, error) {
	s.bucket = bucket
	s.key = key
	return s.data, s.err
}

func (s *imageJobReaderStub) GetByIDForAccount(_ context.Context, accountID, jobID uuid.UUID) (*domain.Job, error) {
	s.accountID = accountID
	s.jobID = jobID
	return s.job, s.err
}

func (s *imageJobServiceStub) PrepareAccountJob(_ context.Context, input joborchestrator.PrepareAccountJobInput) (*domain.Job, error) {
	s.prepareCalls++
	s.prepareInput = input
	if s.prepareErr != nil {
		return nil, s.prepareErr
	}
	if s.prepared == nil {
		s.prepared = &domain.Job{
			ID:              uuid.New(),
			AccountID:       input.AccountID,
			Source:          "web",
			OperationType:   input.Operation,
			Modality:        input.Modality,
			Status:          domain.JobStatusPrepared,
			Params:          input.Params,
			CostEstimate:    input.CostEstimateCredits,
			PricingSnapshot: mustMarshalPricingSnapshot(input.PricingSnapshot),
		}
	}
	return s.prepared, nil
}

func (s *imageJobServiceStub) ActivatePreparedAccountJob(_ context.Context, accountID, jobID uuid.UUID) (*domain.Job, error) {
	s.activateCalls++
	s.activateAccountID = accountID
	s.activateJobID = jobID
	return s.activateJob, s.activateErr
}

type imageBalanceStub struct{ balance int64 }

func (s imageBalanceStub) BalanceForEstimate(context.Context, uuid.UUID) (int64, error) {
	return s.balance, nil
}

func newImageJobTestHandler(t *testing.T) (*Handler, *imageJobServiceStub, *sessionStub) {
	t.Helper()
	h, _, sessions := newTestHandler(t)
	redirectPolicy, err := NewImageArtifactRedirectPolicy("https://objects.example.test", false, "path", "development", false)
	if err != nil {
		t.Fatalf("new image artifact redirect policy: %v", err)
	}
	h.cfg.ImageArtifactRedirectPolicy = redirectPolicy
	pricing, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("new pricing catalog: %v", err)
	}
	jobs := &imageJobServiceStub{}
	h.cfg.ImageModels = []imagegeneration.PublicModel{{
		ID:                     modelcatalog.MiniAppImageNanoBanana2,
		Name:                   "Nano Banana 2",
		Enabled:                true,
		Ready:                  true,
		QualityOptions:         []string{modelcatalog.ImageQuality1K, modelcatalog.ImageQuality2K, modelcatalog.ImageQuality4K},
		DefaultQuality:         modelcatalog.ImageQuality1K,
		SupportsReferenceImage: true,
		MaxReferenceImages:     4,
	}}
	h.deps.ImageJobs = jobs
	h.deps.ImageBalance = imageBalanceStub{balance: 104}
	h.deps.ImagePricing = pricing
	h.deps.ImageJobIdempotency = &imageJobIdempotencyReaderStub{}
	h.deps.ImageJobPrepareLimiter = &imagePrepareLimiterStub{allowed: true}
	h.deps.ImageJobExpiry = &imageJobExpiryReconcilerStub{}
	return h, jobs, sessions
}

func newPreparedWebImageJobForReplay(t *testing.T, accountID, idempotencyKey uuid.UUID, prompt string, expiresAt *time.Time) *domain.Job {
	t.Helper()
	pricing, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static pricing catalog: %v", err)
	}
	snapshot, err := pricing.Snapshot(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: modelcatalog.MiniAppImageNanoBanana2,
		Quality:      modelcatalog.ImageQuality2K,
	})
	if err != nil {
		t.Fatalf("pricing snapshot: %v", err)
	}
	return &domain.Job{
		ID:              uuid.New(),
		AccountID:       accountID,
		Source:          "web",
		ChannelContext:  &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:      domain.ResultModeAccountHistory,
		OperationType:   domain.OperationImageGenerate,
		Modality:        domain.ModalityImage,
		Status:          domain.JobStatusPrepared,
		IdempotencyKey:  idempotencyKey.String(),
		CorrelationID:   "web-image:" + idempotencyKey.String(),
		CostEstimate:    snapshot.InternalCredits,
		PricingSnapshot: mustMarshalPricingSnapshot(snapshot),
		ExpiresAt:       expiresAt,
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{
			Prompt:       prompt,
			ModelID:      modelcatalog.MiniAppImageNanoBanana2,
			ModelName:    "Nano Banana 2",
			ImageQuality: modelcatalog.ImageQuality2K,
			Provider:     domain.ProviderPoYo,
			ModelCode:    "private-original-model-code",
			Resolution:   modelcatalog.ImageQuality2K,
			Size:         "1:1",
		}),
	}
}

func safeImageMutationRequest(t *testing.T, sessions *sessionStub, accountID uuid.UUID, method, path string, body map[string]string) *http.Request {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal image request: %v", err)
	}
	req := authenticatedConversationRequest(t, method, path, sessions, accountID)
	req.Body = io.NopCloser(bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://app.example.test")
	req.Header.Set("X-CSRF-Token", "csrf")
	req.Header.Set("X-Idempotency-Key", uuid.NewString())
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	return req
}

func mustMarshalPricingSnapshot(snapshot pricingcatalog.PricingSnapshot) json.RawMessage {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		panic(err)
	}
	return payload
}

func mustMarshalWebImageJobParams(t *testing.T, params webImageJobParams) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal image job params: %v", err)
	}
	return payload
}

func assertPreparedImageJobParams(t *testing.T, raw json.RawMessage, prompt, modelID, quality string) {
	t.Helper()
	var params map[string]json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatalf("decode prepared params: %v", err)
	}
	for field, want := range map[string]string{
		"prompt":        prompt,
		"model_id":      modelID,
		"image_quality": quality,
	} {
		var got string
		if err := json.Unmarshal(params[field], &got); err != nil || got != want {
			t.Fatalf("prepared param %s = %q (%v), want %q", field, got, err, want)
		}
	}
	if params["provider"] == nil || params["model_code"] == nil {
		t.Fatalf("trusted worker routing missing from prepared params: %s", raw)
	}
}

func assertSafeWebImagePreparation(t *testing.T, body []byte, expectedID uuid.UUID, expectedStatus domain.JobStatus, expectedCost, expectedBalance int64) {
	t.Helper()
	var response struct {
		Job struct {
			ID           uuid.UUID        `json:"id"`
			Status       domain.JobStatus `json:"status"`
			CostEstimate int64            `json:"cost_estimate"`
		} `json:"job"`
		Balance   int64 `json:"balance"`
		CanAfford bool  `json:"can_afford"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Job.ID != expectedID || response.Job.Status != expectedStatus || response.Job.CostEstimate != expectedCost {
		t.Fatalf("public job = %+v, want id=%s status=%s cost=%d", response.Job, expectedID, expectedStatus, expectedCost)
	}
	if response.Balance != expectedBalance || !response.CanAfford {
		t.Fatalf("public balance = %d / can_afford=%t, want %d / true", response.Balance, response.CanAfford, expectedBalance)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("decode response fields: %v", err)
	}
	for _, forbidden := range []string{"account_id", "user_id", "provider", "model_code", "params", "pricing_snapshot"} {
		if _, ok := fields[forbidden]; ok {
			t.Fatalf("response exposed forbidden field %q: %s", forbidden, body)
		}
	}
}

func assertSafeWebImageActivation(t *testing.T, body []byte, expectedID uuid.UUID, expectedStatus domain.JobStatus) {
	t.Helper()
	var response struct {
		Job struct {
			ID     uuid.UUID        `json:"id"`
			Status domain.JobStatus `json:"status"`
		} `json:"job"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Job.ID != expectedID || response.Job.Status != expectedStatus {
		t.Fatalf("public job = %+v, want id=%s status=%s", response.Job, expectedID, expectedStatus)
	}
	if bytes.Contains(bytes.ToLower(body), []byte("provider")) || bytes.Contains(bytes.ToLower(body), []byte("model_code")) {
		t.Fatalf("public response leaked private routing: %s", body)
	}
}
