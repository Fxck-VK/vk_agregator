package admin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/inbound/admin"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	platformconfig "vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/billingservice"
)

const testAdminToken = "test-admin-token"

func setup(t *testing.T) (*admin.Handler, *memory.JobRepo, *memory.UserRepo, *memory.DeliveryRepo, *billingservice.Service) {
	t.Helper()
	jobs := memory.NewJobRepo()
	users := memory.NewUserRepo()
	deliveries := memory.NewDeliveryRepo()
	billingRepo := memory.NewBillingRepo()
	billing := billingservice.New(billingRepo)
	h := admin.NewHandler(admin.Config{Token: testAdminToken}, admin.Deps{
		Jobs:       jobs,
		Users:      users,
		Deliveries: deliveries,
		Referrals:  memory.NewReferralRepo(),
		Billing:    billingRepo,
	})
	return h, jobs, users, deliveries, billing
}

type fakeMaintenanceReader struct {
	now time.Time
}

func (f fakeMaintenanceReader) RetentionStatus(context.Context, time.Time) (domain.RetentionStatus, error) {
	old := f.now.Add(-48 * time.Hour)
	return domain.RetentionStatus{
		GeneratedAt: f.now,
		Items: []domain.RetentionStatusItem{
			{
				TableName:      "conversation_messages",
				RetentionClass: domain.DataClassUserContent,
				TotalRows:      42,
				ExpiredRows:    3,
				RedactedRows:   7,
				OldestHotAt:    &old,
			},
		},
	}, nil
}

func (f fakeMaintenanceReader) RetentionDryRun(context.Context, time.Time, int) (domain.RetentionDryRun, error) {
	old := f.now.Add(-72 * time.Hour)
	return domain.RetentionDryRun{
		GeneratedAt: f.now,
		Items: []domain.RetentionDryRunItem{
			{
				Action:         "process_expired_rows",
				TableName:      "provider_tasks",
				RetentionClass: domain.DataClassProviderPayload,
				Count:          5,
				Bytes:          0,
				OldestAt:       &old,
			},
		},
	}, nil
}

func (f fakeMaintenanceReader) AnalyticsAggregationStatus(context.Context) (domain.AnalyticsAggregationStatus, error) {
	day := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	return domain.AnalyticsAggregationStatus{
		GeneratedAt: f.now,
		Items: []domain.AnalyticsAggregationStatusItem{
			{TableName: "daily_generation_stats", Status: "ok", Rows: 10, LatestActivityDate: &day, LastUpdatedAt: &f.now},
		},
	}, nil
}

func (f fakeMaintenanceReader) OldestHotRows(context.Context) (domain.OldestHotRowsReport, error) {
	old := f.now.Add(-96 * time.Hour)
	return domain.OldestHotRowsReport{
		GeneratedAt: f.now,
		Items: []domain.OldestHotRow{
			{TableName: "conversation_messages", RetentionClass: domain.DataClassUserContent, Count: 42, OldestAt: &old, AgeSeconds: int64(f.now.Sub(old).Seconds())},
		},
	}, nil
}

func (f fakeMaintenanceReader) OrphanArtifactsCount(context.Context, time.Time) (domain.OrphanArtifactsReport, error) {
	old := f.now.Add(-12 * time.Hour)
	return domain.OrphanArtifactsReport{
		GeneratedAt: f.now,
		Total:       2,
		Bytes:       2048,
		Items: []domain.OrphanArtifactCount{
			{
				ArtifactTier:   domain.ArtifactTierFree,
				LifecycleClass: domain.ArtifactLifecycleProviderOriginal,
				Status:         domain.ArtifactStatusReady,
				MediaType:      domain.MediaTypeImage,
				Count:          2,
				Bytes:          2048,
				OldestAt:       &old,
			},
		},
	}, nil
}

func do(t *testing.T, h *admin.Handler, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Admin-Token", testAdminToken)
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
	h := admin.NewHandler(admin.Config{Token: testAdminToken}, admin.Deps{
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

func TestOperatorRetentionEndpointsExposeOnlySafeAggregates(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	h := admin.NewHandler(admin.Config{Token: testAdminToken}, admin.Deps{
		Jobs:        memory.NewJobRepo(),
		Users:       memory.NewUserRepo(),
		Deliveries:  memory.NewDeliveryRepo(),
		Referrals:   memory.NewReferralRepo(),
		Maintenance: fakeMaintenanceReader{now: now},
	})
	for _, path := range []string{
		"/admin/retention/operator/status",
		"/admin/retention/operator/dry-run",
		"/admin/analytics/operator/status",
		"/admin/data/operator/hot-rows",
		"/admin/artifacts/operator/orphans",
	} {
		rec, _ := do(t, h, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, forbidden := range []string{
			"raw-prompt",
			"provider-secret",
			"vk1.a.",
			"storage_bucket",
			"storage_key",
			"owner_user_id",
		} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("%s leaked forbidden value %q: %s", path, forbidden, body)
			}
		}
	}
}

func TestOperatorJobsListDetailAndQueueSafeDTO(t *testing.T) {
	h, jobs, _, deliveries, billing := setup(t)
	ctx := context.Background()
	userID := uuid.New()
	inputArtifactID := uuid.New()
	outputArtifactID := uuid.New()
	job := &domain.Job{
		ID:                uuid.New(),
		UserID:            userID,
		VKPeerID:          2000000042,
		OperationType:     domain.OperationVideoGenerate,
		Modality:          domain.ModalityVideo,
		Status:            domain.JobStatusFailedRetryable,
		CorrelationID:     "idem:vk:private-correlation",
		InputArtifactIDs:  []uuid.UUID{inputArtifactID},
		OutputArtifactIDs: []uuid.UUID{outputArtifactID},
		CostEstimate:      50,
		CostReserved:      50,
		ErrorCode:         "provider_timeout",
		ErrorMessage:      "raw provider timeout https://private.example/output.mp4 prompt text",
		IdempotencyKey:    "idem:job:secret",
	}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if _, err := billing.Reserve(ctx, userID, job.ID, 50); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := deliveries.Create(ctx, &domain.Delivery{
		ID:             uuid.New(),
		JobID:          job.ID,
		UserID:         userID,
		VKPeerID:       2000000042,
		ArtifactID:     &outputArtifactID,
		Type:           domain.DeliveryTypeVideo,
		Status:         domain.DeliveryStatusRetrying,
		VKRandomID:     999,
		Attachment:     "video123_456",
		Text:           "private prompt text",
		AttemptNo:      2,
		IdempotencyKey: "delivery:secret",
		ErrorCode:      "vk_rate_limited",
		ErrorMessage:   "raw vk error with token",
	}); err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	createdFrom := url.QueryEscape(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	rec, _ := do(t, h, "/admin/jobs/operator?status=failed_retryable&kind=video_generate&error_class=provider_timeout&created_from="+createdFrom)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list admin.OperatorJobsDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode operator jobs: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected one filtered job, got %+v", list.Items)
	}
	item := list.Items[0]
	if !strings.HasPrefix(item.DisplayID, "job_") || !strings.HasPrefix(item.CorrelationRef, "corr_") {
		t.Fatalf("expected safe display refs, got %+v", item)
	}
	if item.LookupID == "" || item.ErrorClass != "provider_timeout" || item.InputCount != 1 || item.OutputCount != 1 {
		t.Fatalf("unexpected operator job item: %+v", item)
	}
	assertNoOperatorJobLeak(t, rec.Body.String(), userID, inputArtifactID, outputArtifactID)

	rec, _ = do(t, h, "/admin/jobs/"+job.ID.String()+"/operator")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected detail 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var detail admin.OperatorJobDetailDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode operator detail: %v", err)
	}
	if detail.Reservation == nil || detail.Reservation.Status != string(domain.ReservationReserved) || detail.Reservation.Amount != 50 {
		t.Fatalf("expected safe reservation summary, got %+v", detail.Reservation)
	}
	if len(detail.Artifacts.InputRefs) != 1 || len(detail.Artifacts.OutputRefs) != 1 ||
		strings.Contains(detail.Artifacts.InputRefs[0], inputArtifactID.String()) ||
		strings.Contains(detail.Artifacts.OutputRefs[0], outputArtifactID.String()) {
		t.Fatalf("expected safe artifact refs, got %+v", detail.Artifacts)
	}
	if detail.Delivery.Attempts != 1 || detail.Delivery.RetryCount != 1 || detail.Delivery.LastErrorClass != "vk_rate_limited" {
		t.Fatalf("unexpected delivery summary: %+v", detail.Delivery)
	}
	if !containsString(detail.AllowedNext, string(domain.JobStatusQueued)) {
		t.Fatalf("expected allowed next statuses, got %+v", detail.AllowedNext)
	}
	assertNoOperatorJobLeak(t, rec.Body.String(), userID, inputArtifactID, outputArtifactID)

	rec, _ = do(t, h, "/admin/jobs/queue")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected queue 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var queue admin.OperatorQueueSummaryDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &queue); err != nil {
		t.Fatalf("decode queue: %v", err)
	}
	if queue.RetryCount != 1 || queue.DLQ.Status != "not_wired" || queue.ProviderCircuit.Status != "not_wired" {
		t.Fatalf("unexpected queue summary: %+v", queue)
	}
	assertNoOperatorJobLeak(t, rec.Body.String(), userID, inputArtifactID, outputArtifactID)
}

func TestProviderMediaAndConfigOperatorDTOsAreSafe(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	cfg := platformconfig.Config{
		Env:                                  "production",
		PaymentProvider:                      "yookassa",
		PaymentWebhookRequireHTTPS:           true,
		Provider:                             "deepinfra",
		ProviderChain:                        []string{"deepinfra", "openai"},
		ImageProvider:                        "deepinfra",
		VideoProvider:                        "deepinfra",
		DeepInfraVideoModel:                  "raw-provider-model-id",
		OpenAIVideoModel:                     "raw-openai-video-model",
		MediaPipelineEnabled:                 true,
		MediaVideoProbePolicy:                platformconfig.MediaVideoProbePolicyProbeRequired,
		MediaVideoTranscodePolicy:            platformconfig.MediaVideoTranscodePolicyNever,
		MediaDeliverRawProviderVideo:         platformconfig.MediaDeliverRawProviderVideoIfProbePassed,
		MediaReferenceUploadsEnabled:         true,
		MediaReferenceWebPEnabled:            false,
		MediaMaxImageUploadBytes:             20 << 20,
		MediaMaxImagePixels:                  4096 * 4096,
		MediaMaxVideoSizeBytes:               256 << 20,
		MediaMaxVideoDurationSec:             60,
		MediaMaxConcurrentUploads:            8,
		MediaMaxConcurrentProbes:             2,
		MediaMaxConcurrentTranscodes:         0,
		MediaMaxPendingVariants:              16,
		MediaQueueDegradeThreshold:           1000,
		MediaProviderMaxAttemptsPerJob:       1,
		MediaProviderFallbackBudget:          0,
		MediaProviderQualityGuardEnabled:     true,
		MediaProviderQualityDegradedFailures: 3,
		MediaProviderQualityDisabledFailures: 5,
		APIMartAPIKey:                        "raw-apimart-api-key",
		APIMartBaseURL:                       "https://private.example/apimart",
		APIMartProviderEnabled:               true,
		PoYoProviderEnabled:                  true,
		FeatureVideoRouterEnabled:            true,
		FeatureVideoRouteHailuo23FastEnabled: true,
		FeatureVideoRouteRunwayGen45Enabled:  true,
	}
	h := admin.NewHandler(admin.Config{Token: testAdminToken, Runtime: admin.NewRuntimeSnapshot(cfg)}, admin.Deps{
		Jobs:       jobs,
		Users:      memory.NewUserRepo(),
		Deliveries: memory.NewDeliveryRepo(),
		Referrals:  memory.NewReferralRepo(),
	})
	userID := uuid.New()
	for _, job := range []*domain.Job{
		{
			ID:             uuid.New(),
			UserID:         userID,
			OperationType:  domain.OperationVideoGenerate,
			Modality:       domain.ModalityVideo,
			Status:         domain.JobStatusProviderFailed,
			ErrorCode:      string(domain.ProviderErrRateLimited),
			ErrorMessage:   "raw provider payload https://private.example/video prompt text",
			CostReserved:   50,
			IdempotencyKey: "idem:secret:provider",
		},
		{
			ID:             uuid.New(),
			UserID:         userID,
			OperationType:  domain.OperationVideoGenerate,
			Modality:       domain.ModalityVideo,
			Status:         domain.JobStatusFailedTerminal,
			ErrorCode:      domain.JobErrMediaProviderOutputInvalid,
			ErrorMessage:   "raw ffprobe path C:/private/video.mp4",
			CostReserved:   50,
			IdempotencyKey: "idem:secret:invalid",
		},
		{
			ID:             uuid.New(),
			UserID:         userID,
			OperationType:  domain.OperationImageEdit,
			Modality:       domain.ModalityImage,
			Status:         domain.JobStatusRejected,
			ErrorCode:      domain.JobErrMediaUploadTooLarge,
			ErrorMessage:   "raw image name private.png",
			IdempotencyKey: "idem:secret:upload",
		},
		{
			ID:             uuid.New(),
			UserID:         userID,
			OperationType:  domain.OperationVideoGenerate,
			Modality:       domain.ModalityVideo,
			Status:         domain.JobStatusDelivering,
			CostReserved:   50,
			CostCaptured:   0,
			IdempotencyKey: "idem:secret:gap",
		},
	} {
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatalf("create job: %v", err)
		}
	}

	for _, path := range []string{
		"/admin/providers/operator",
		"/admin/media-safety/operator",
		"/admin/config-health/operator",
	} {
		rec, _ := do(t, h, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
		raw := rec.Body.String()
		for _, forbidden := range []string{
			"raw-provider-model-id",
			"raw-openai-video-model",
			"raw-apimart-api-key",
			"provider_model_id",
			"MiniMax-Hailuo-2.3",
			"kling-o3/standard",
			"seedance-2-fast",
			"runway-gen-4.5",
			"https://private.example",
			"private.example",
			"prompt text",
			"idem:secret",
			"raw provider payload",
			"raw ffprobe path",
			"private.png",
			userID.String(),
		} {
			if strings.Contains(raw, forbidden) {
				t.Fatalf("%s leaked forbidden field/value %q: %s", path, forbidden, raw)
			}
		}
	}

	rec, _ := do(t, h, "/admin/providers/operator")
	var providers admin.OperatorProviderControlRoomDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &providers); err != nil {
		t.Fatalf("decode providers: %v", err)
	}
	if len(providers.Providers) == 0 || providers.Fallback.Status != "ok" {
		t.Fatalf("unexpected provider control room: %+v", providers)
	}
	if len(providers.VideoRoutes) == 0 {
		t.Fatalf("expected safe video route state rows, got none")
	}
	hailuoFast := findVideoRouteDTO(providers.VideoRoutes, string(domain.VideoRouteHailuo23Fast))
	if hailuoFast == nil || hailuoFast.Status != "ok" || hailuoFast.Reason != "ready" ||
		hailuoFast.ProviderClass != "apimart" || hailuoFast.ModelClass != "hailuo_2_3_fast" {
		t.Fatalf("unexpected hailuo route state: %+v", hailuoFast)
	}
	hailuoStandard := findVideoRouteDTO(providers.VideoRoutes, string(domain.VideoRouteHailuo23Standard))
	if hailuoStandard == nil || hailuoStandard.Status != "not_wired" || hailuoStandard.Reason != "route_flag_off" {
		t.Fatalf("unexpected disabled route state: %+v", hailuoStandard)
	}
	runwayGen45 := findVideoRouteDTO(providers.VideoRoutes, string(domain.VideoRouteRunwayGen45))
	if runwayGen45 == nil || runwayGen45.Status != "warning" || runwayGen45.Reason != "provider_unconfigured" ||
		runwayGen45.ProviderClass != "poyo" {
		t.Fatalf("unexpected poyo route state: %+v", runwayGen45)
	}
	if providers.ProviderWaste.Value == "0" || providers.DeliveryCaptureGap.Value == "0" {
		t.Fatalf("expected provider waste and capture gap signals, got %+v", providers)
	}

	rec, _ = do(t, h, "/admin/config-health/operator")
	var configHealth admin.OperatorConfigHealthDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &configHealth); err != nil {
		t.Fatalf("decode config health: %v", err)
	}
	if findVideoRouteDTO(configHealth.VideoRoutes, string(domain.VideoRouteHailuo23Fast)) == nil {
		t.Fatalf("expected route state in config health: %+v", configHealth.VideoRoutes)
	}

	rec, _ = do(t, h, "/admin/media-safety/operator")
	var media admin.OperatorMediaSafetyDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &media); err != nil {
		t.Fatalf("decode media safety: %v", err)
	}
	if media.Policy.ProbePolicy != platformconfig.MediaVideoProbePolicyProbeRequired ||
		media.Policy.TranscodePolicy != platformconfig.MediaVideoTranscodePolicyNever ||
		len(media.Uploads) != 3 {
		t.Fatalf("unexpected media safety DTO: %+v", media)
	}
}

func findVideoRouteDTO(items []admin.OperatorVideoRouteDTO, alias string) *admin.OperatorVideoRouteDTO {
	for i := range items {
		if items[i].Alias == alias {
			return &items[i]
		}
	}
	return nil
}

func TestUsersReferralsAndAuditOperatorDTOsAreSafe(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	users := memory.NewUserRepo()
	payments := memory.NewPaymentRepo()
	refs := memory.NewReferralRepo()
	audits := memory.NewOperatorAuditRepo()
	adminToken := "stage7-admin-token"
	h := admin.NewHandler(admin.Config{Token: adminToken}, admin.Deps{
		Jobs:       jobs,
		Users:      users,
		Deliveries: memory.NewDeliveryRepo(),
		Audits:     audits,
		Referrals:  refs,
		Payment:    payments,
	})
	user := &domain.User{
		VKUserID:    777777,
		Role:        domain.RoleUser,
		Status:      domain.StatusActive,
		Locale:      "ru",
		Timezone:    "Europe/Moscow",
		RiskLevel:   45,
		VKFirstName: "RawName",
		VKLastName:  "RawLast",
	}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	for _, job := range []*domain.Job{
		{
			ID:             uuid.New(),
			UserID:         user.ID,
			OperationType:  domain.OperationTextGenerate,
			Modality:       domain.ModalityText,
			Status:         domain.JobStatusSucceeded,
			CostReserved:   1,
			CostCaptured:   1,
			ErrorMessage:   "sensitive_generation_input should not render",
			IdempotencyKey: "idem-one",
		},
		{
			ID:             uuid.New(),
			UserID:         user.ID,
			OperationType:  domain.OperationImageGenerate,
			Modality:       domain.ModalityImage,
			Status:         domain.JobStatusFailedTerminal,
			ErrorCode:      "provider_timeout",
			ErrorMessage:   "private_storage_locator should not render",
			IdempotencyKey: "idem-two",
		},
	} {
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatalf("create job: %v", err)
		}
	}
	if err := payments.CreateIntent(ctx, &domain.PaymentIntent{
		ID:             uuid.New(),
		UserID:         user.ID,
		Status:         domain.PaymentIntentSucceeded,
		Amount:         100,
		Currency:       domain.CurrencyRUB,
		Credits:        100,
		Provider:       domain.PaymentProviderMock,
		IdempotencyKey: "stage7-payment-key",
	}); err != nil {
		t.Fatalf("create intent: %v", err)
	}
	if err := refs.CreateCode(ctx, &domain.ReferralCode{UserID: user.ID, Code: "SAFE7777"}); err != nil {
		t.Fatalf("create code: %v", err)
	}
	for i := 0; i < 2; i++ {
		referral := &domain.Referral{
			ReferrerUserID: user.ID,
			ReferredUserID: uuid.New(),
			ReferralCode:   "SAFE7777",
			Source:         domain.ReferralSourceVKBot,
			Status:         domain.ReferralStatusRegistered,
			RewardStatus:   domain.ReferralRewardPending,
		}
		if err := refs.CreateReferral(ctx, referral); err != nil {
			t.Fatalf("create referral: %v", err)
		}
	}

	userReq := httptest.NewRequest(http.MethodGet, "/admin/users/operator?user_id="+user.ID.String(), nil)
	userReq.Header.Set("X-Admin-Token", adminToken)
	userReq.Header.Set("X-Request-ID", "private-request-id")
	userRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(userRec, userReq)
	if userRec.Code != http.StatusOK {
		t.Fatalf("expected users 200, got %d: %s", userRec.Code, userRec.Body.String())
	}
	var userDTO admin.OperatorUsersDTO
	if err := json.Unmarshal(userRec.Body.Bytes(), &userDTO); err != nil {
		t.Fatalf("decode users dto: %v", err)
	}
	if userDTO.User == nil || userDTO.User.UserRef == "" || userDTO.User.RiskClass != "medium" || userDTO.Payment.Succeeded != 1 {
		t.Fatalf("unexpected users dto: %+v", userDTO)
	}
	assertNoOperatorStage7Leak(t, userRec.Body.String(), user.ID)

	refReq := httptest.NewRequest(http.MethodGet, "/admin/referrals/operator?code=SAFE7777&min_registered=2&min_total=10", nil)
	refReq.Header.Set("X-Admin-Token", adminToken)
	refRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(refRec, refReq)
	if refRec.Code != http.StatusOK {
		t.Fatalf("expected referrals 200, got %d: %s", refRec.Code, refRec.Body.String())
	}
	var refDTO admin.OperatorReferralsDTO
	if err := json.Unmarshal(refRec.Body.Bytes(), &refDTO); err != nil {
		t.Fatalf("decode referrals dto: %v", err)
	}
	if refDTO.CodeStats == nil || refDTO.CodeStats.RegisteredCount != 2 || refDTO.Distribution.RegisteredCount != 2 || len(refDTO.Suspicious) != 1 {
		t.Fatalf("unexpected referrals dto: %+v", refDTO)
	}
	assertNoOperatorStage7Leak(t, refRec.Body.String(), user.ID)

	auditReq := httptest.NewRequest(http.MethodGet, "/admin/audit/operator", nil)
	auditReq.Header.Set("X-Admin-Token", adminToken)
	auditRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(auditRec, auditReq)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("expected audit 200, got %d: %s", auditRec.Code, auditRec.Body.String())
	}
	var auditDTO admin.OperatorAuditLogDTO
	if err := json.Unmarshal(auditRec.Body.Bytes(), &auditDTO); err != nil {
		t.Fatalf("decode audit dto: %v", err)
	}
	if len(auditDTO.Items) < 2 {
		t.Fatalf("expected at least two audit entries, got %+v", auditDTO.Items)
	}
	assertNoOperatorStage7Leak(t, auditRec.Body.String(), user.ID)
}

func assertNoOperatorStage7Leak(t *testing.T, raw string, userID uuid.UUID) {
	t.Helper()
	for _, forbidden := range []string{
		"vk_user_id",
		"VKUserID",
		"777777",
		"RawName",
		"RawLast",
		"Europe/Moscow",
		"sensitive_generation_input",
		"private_storage_locator",
		"idempotency",
		"idem-one",
		"idem-two",
		"stage7-admin-token",
		"private-request-id",
		"stage7-payment-key",
		userID.String(),
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("operator Stage 7 DTO leaked forbidden field/value %q: %s", forbidden, raw)
		}
	}
}

func assertNoOperatorJobLeak(t *testing.T, raw string, userID, inputArtifactID, outputArtifactID uuid.UUID) {
	t.Helper()
	for _, forbidden := range []string{
		"user_id",
		"vk_peer_id",
		"error_message",
		"idempotency",
		"params",
		"attachment",
		"private-correlation",
		"private.example",
		"prompt text",
		"token",
		userID.String(),
		inputArtifactID.String(),
		outputArtifactID.String(),
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("operator job DTO leaked forbidden field/value %q: %s", forbidden, raw)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
	failClosed := admin.NewHandler(admin.Config{}, admin.Deps{Jobs: jobs, Users: memory.NewUserRepo(), Deliveries: memory.NewDeliveryRepo()})
	req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	rec := httptest.NewRecorder()
	failClosed.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when token is not configured, got %d", rec.Code)
	}

	h := admin.NewHandler(admin.Config{Token: "secret"}, admin.Deps{Jobs: jobs, Users: memory.NewUserRepo(), Deliveries: memory.NewDeliveryRepo()})

	req = httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	rec = httptest.NewRecorder()
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
	h := admin.NewHandler(admin.Config{Token: testAdminToken}, admin.Deps{
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
