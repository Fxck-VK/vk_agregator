package vk_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/inbound/vk"
	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	antispamservice "vk-ai-aggregator/internal/service/antispam"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/outboxrelay"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/referralservice"
)

type harness struct {
	handler *vk.Handler
	users   *memory.UserRepo
	cmds    *memory.CommandRepo
	jobs    *memory.JobRepo
	inbound *memory.InboundRepo
	billing *memory.BillingRepo
	payment *memory.PaymentRepo
	refs    *memory.ReferralRepo
	arts    *memory.ArtifactRepo
	objects *memory.ObjectStore
	pub     *queue.MemoryPublisher
	relay   *outboxrelay.Relay
}

func newHarness() *harness {
	return newHarnessWithControl(nil)
}

func newHarnessWithControl(control vkdelivery.ControlClient) *harness {
	return newHarnessWithConfig(control, vk.Config{ConfirmationToken: "conf-token-123", Secret: "s3cr3t"})
}

func newHarnessWithConfig(control vkdelivery.ControlClient, cfg vk.Config) *harness {
	return newHarnessWithDeps(control, cfg, nil, nil)
}

func newHarnessWithConfigAndAntiSpam(control vkdelivery.ControlClient, cfg vk.Config, antiSpam vk.AntiSpam) *harness {
	return newHarnessWithDeps(control, cfg, antiSpam, nil)
}

func newHarnessWithConfigAndDialogState(control vkdelivery.ControlClient, cfg vk.Config, dialogState vk.DialogState) *harness {
	return newHarnessWithDeps(control, cfg, nil, dialogState)
}

func newHarnessWithDeps(control vkdelivery.ControlClient, cfg vk.Config, antiSpam vk.AntiSpam, dialogState vk.DialogState) *harness {
	return newHarnessWithReferenceDownloader(control, cfg, antiSpam, dialogState, nil)
}

func newHarnessWithReferenceDownloader(control vkdelivery.ControlClient, cfg vk.Config, antiSpam vk.AntiSpam, dialogState vk.DialogState, downloader vk.Downloader) *harness {
	return newHarnessWithReferenceDownloaderAndOrchestratorOptions(control, cfg, antiSpam, dialogState, downloader)
}

func newHarnessWithReferenceDownloaderAndOrchestratorOptions(control vkdelivery.ControlClient, cfg vk.Config, antiSpam vk.AntiSpam, dialogState vk.DialogState, downloader vk.Downloader, opts ...joborchestrator.Option) *harness {
	if cfg.ImageModels == nil {
		cfg.ImageModels = defaultTestVKImageModels()
	}
	var profile vkdelivery.UserProfileClient
	if p, ok := control.(vkdelivery.UserProfileClient); ok {
		profile = p
	}
	users := memory.NewUserRepo()
	cmds := memory.NewCommandRepo()
	jobs := memory.NewJobRepo()
	outbox := memory.NewOutboxRepo()
	inbound := memory.NewInboundRepo()
	idem := memory.NewIdempotencyRepo()
	bill := memory.NewBillingRepo()
	arts := memory.NewArtifactRepo()
	objects := memory.NewObjectStore()
	billing := billingservice.New(bill)
	payments := memory.NewPaymentRepo()
	vatCode := int16(1)
	payments.PutProduct(&domain.PaymentProduct{
		Code:           "crystals_99",
		Title:          "NeiroHub 99 crystals",
		Amount:         9900,
		Currency:       domain.CurrencyRUB,
		Credits:        99,
		PriceVersion:   1,
		IsActive:       true,
		VATCode:        &vatCode,
		PaymentSubject: "service",
		PaymentMode:    "full_prepayment",
	})
	payments.PutProduct(&domain.PaymentProduct{
		Code:           "crystals_700",
		Title:          "NeiroHub 700 crystals",
		Amount:         70000,
		Currency:       domain.CurrencyRUB,
		Credits:        700,
		PriceVersion:   1,
		IsActive:       true,
		VATCode:        &vatCode,
		PaymentSubject: "service",
		PaymentMode:    "full_prepayment",
	})
	payment := paymentservice.New(payments, paymentmock.New(), paymentservice.Config{
		ReturnURL: "https://neiirohub.ru/payments/return",
	})
	refs := memory.NewReferralRepo()
	referrals := referralservice.New(refs, billing, referralservice.Config{
		ReferrerSignupRewardCredits: 10,
		RewardOnActivation:          true,
	})
	pub := queue.NewMemoryPublisher()
	uowMgr := memory.NewUnitOfWork(jobs, outbox, bill)
	prices := staticPricingCatalogForVKTest()
	orchOpts := []joborchestrator.Option{
		joborchestrator.WithVideoRouteResolver(testVKVideoRouteResolver()),
		joborchestrator.WithPricingCatalog(prices),
	}
	orchOpts = append(orchOpts, opts...)
	orch := joborchestrator.New(jobs, uowMgr, billing, 0, orchOpts...)
	h := vk.NewHandler(cfg, vk.Deps{
		Idempotency:    idem,
		Inbound:        inbound,
		Users:          users,
		Jobs:           jobs,
		Commands:       cmds,
		Billing:        billing,
		Payment:        payment,
		Referrals:      referrals,
		Orchestrator:   orch,
		PricingCatalog: prices,
		Router:         commandrouter.New(),
		Control:        control,
		Profile:        profile,
		DialogState:    dialogState,
		AntiSpam:       antiSpam,
		Artifacts:      arts,
		Objects:        objects,
		Downloader:     downloader,
	})
	return &harness{handler: h, users: users, cmds: cmds, jobs: jobs, inbound: inbound, billing: bill, payment: payments, refs: refs, arts: arts, objects: objects, pub: pub, relay: outboxrelay.New(uowMgr, pub)}
}

func defaultTestVKImageModels() []productcatalog.ImageModel {
	catalog := productcatalog.New(productcatalog.Config{
		ImageProviderReady: map[domain.ProviderName]bool{
			domain.ProviderAPIMart: true,
			domain.ProviderPoYo:    true,
		},
		EnabledImageModels: map[string]bool{
			modelcatalog.MiniAppImageNanoBanana2:   true,
			modelcatalog.MiniAppImageNanoBananaPro: true,
			modelcatalog.MiniAppImageGPTImage2:     true,
		},
		PricingCatalog: staticPricingCatalogForVKTest(),
	})
	return catalog.ImageModels()
}

func staticPricingCatalogForVKTest() *pricingcatalog.Catalog {
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		panic(err)
	}
	return catalog
}

func vkTestImageCostCredits(t *testing.T, modelID, quality string) int64 {
	t.Helper()
	credits, err := staticPricingCatalogForVKTest().CostEstimateCredits(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: modelID,
		Quality:      quality,
	})
	if err != nil {
		t.Fatalf("image cost estimate %s/%s: %v", modelID, quality, err)
	}
	return credits
}

func vkTestImageDisplayCredits(t *testing.T, modelID, quality string) int64 {
	t.Helper()
	credits, err := staticPricingCatalogForVKTest().DisplayEstimateCredits(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: modelID,
		Quality:      quality,
	})
	if err != nil {
		t.Fatalf("image display estimate %s/%s: %v", modelID, quality, err)
	}
	return credits
}

func vkTestVideoCostCredits(t *testing.T, alias domain.VideoRouteAlias, resolution string, durationSec int) int64 {
	t.Helper()
	credits, err := staticPricingCatalogForVKTest().CostEstimateCredits(pricingcatalog.ProductKey{
		Operation:       domain.OperationVideoGenerate,
		Modality:        domain.ModalityVideo,
		VideoRouteAlias: alias,
		Resolution:      resolution,
		DurationSec:     durationSec,
	})
	if err != nil {
		t.Fatalf("video cost estimate %s/%s/%ds: %v", alias, resolution, durationSec, err)
	}
	return credits
}

func testKlingVideoRoute() productcatalog.VideoRoute {
	return productcatalog.VideoRoute{
		Type:                   productcatalog.TypeVideo,
		Alias:                  string(domain.VideoRouteKlingO3Standard),
		Name:                   "Kling O3 Standard",
		Description:            "Тестовый публичный маршрут.",
		Enabled:                true,
		AllowedDurationsSec:    []int{5, 10},
		AllowedAspectRatios:    []string{"16:9", "9:16", "1:1"},
		DefaultDurationSec:     5,
		DefaultResolution:      pricingcatalog.VideoResolution720p,
		DefaultAspectRatio:     "16:9",
		SupportsReferenceImage: true,
		MaxReferenceImages:     1,
	}
}

func testRunwayVideoRoute() productcatalog.VideoRoute {
	return productcatalog.VideoRoute{
		Type:                   productcatalog.TypeVideo,
		Alias:                  string(domain.VideoRouteRunwayGen4Turbo),
		Name:                   "Runway Gen-4 Turbo",
		Description:            "Тестовый public image-to-video маршрут.",
		Enabled:                true,
		AllowedDurationsSec:    []int{5, 10},
		AllowedAspectRatios:    []string{"16:9", "9:16", "1:1"},
		DefaultDurationSec:     5,
		DefaultResolution:      pricingcatalog.VideoResolution720p,
		DefaultAspectRatio:     "16:9",
		RequiresStartImage:     true,
		SupportsReferenceImage: true,
		MaxReferenceImages:     1,
	}
}

func testVKVideoRouteResolver() joborchestrator.VideoRouteResolver {
	return joborchestrator.VideoRouteResolverFunc(func(_ context.Context, in joborchestrator.VideoRouteCheckInput) (joborchestrator.VideoRouteResolution, error) {
		var params struct {
			VideoRouteAlias string `json:"video_route_alias"`
			DurationSec     int    `json:"duration_sec"`
		}
		if err := json.Unmarshal(in.Params, &params); err != nil {
			return joborchestrator.VideoRouteResolution{}, err
		}
		alias := domain.VideoRouteAlias(strings.TrimSpace(params.VideoRouteAlias))
		if alias == "" {
			return joborchestrator.VideoRouteResolution{}, nil
		}
		duration := params.DurationSec
		if duration == 0 {
			duration = 5
		}
		snapshot := domain.VideoRouteSnapshot{
			Alias:                  alias,
			Provider:               domain.ProviderPoYo,
			ProviderModelID:        "hidden-vk-test-model",
			ModelClass:             "vk_test_video_route",
			DurationSec:            duration,
			Resolution:             "720p",
			AspectRatio:            "16:9",
			ProviderCostCredits:    int64(duration),
			InternalCostCredits:    int64(duration * 2),
			PriceMultiplier:        2,
			MaxProviderCostCredits: 20,
			MaxInternalCostCredits: 40,
		}
		return joborchestrator.VideoRouteResolution{
			Resolved:            true,
			Params:              in.Params,
			Snapshot:            snapshot,
			InternalCostCredits: snapshot.InternalCostCredits,
		}, nil
	})
}

func enabledVideoCommands(commands ...domain.CommandType) vk.MenuFeatureFlags {
	enabled := make(map[domain.CommandType]bool, len(commands))
	for _, command := range commands {
		enabled[command] = true
	}
	return vk.MenuFeatureFlags{EnabledCommands: enabled}
}

// post serves the webhook and then drains the outbox relay, mirroring the
// api+worker split where the relay publishes queued jobs to the worker queue.
func (h *harness) post(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/vk", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	_, _ = h.relay.Drain(context.Background())
	return rec
}

func TestConfirmation(t *testing.T) {
	h := newHarness()
	rec := h.post(`{"type":"confirmation","group_id":1,"secret":"s3cr3t"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "conf-token-123" {
		t.Fatalf("body = %q, want confirmation token", got)
	}
}

func TestInvalidSecret(t *testing.T) {
	h := newHarness()
	rec := h.post(`{"type":"confirmation","group_id":1,"secret":"wrong"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestMessageNewCreatesJob(t *testing.T) {
	h := newHarness()
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-100","secret":"s3cr3t",
		"object":{"message":{"from_id":555,"peer_id":555,"text":"/image neon cat"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 555)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}

	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].OperationType != domain.OperationImageGenerate {
		t.Fatalf("operation = %q, want image_generate", jobs[0].OperationType)
	}
	if h.pub.Len() != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", h.pub.Len())
	}
}

func TestMessageNewPersistsRedactedInboundPayload(t *testing.T) {
	h := newHarness()
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-redacted","secret":"s3cr3t",
		"object":{"message":{"from_id":556,"peer_id":556,"text":"SENSITIVE_TEXT_MARKER","payload":"{\"marker\":\"SENSITIVE_PAYLOAD_MARKER\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	inbound, err := h.inbound.GetByIdempotencyKey(context.Background(), "vk_event:1:evt-redacted")
	if err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	payload := string(inbound.Payload)
	for _, forbidden := range []string{"SENSITIVE_TEXT_MARKER", "SENSITIVE_PAYLOAD_MARKER"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("inbound payload contains forbidden raw content marker %q: %s", forbidden, payload)
		}
	}
	for _, want := range []string{`"redacted":true`, `"payload_class":"vk_callback_metadata"`, `"event_type":"message_new"`} {
		if !strings.Contains(payload, want) {
			t.Fatalf("inbound payload = %s, want marker %s", payload, want)
		}
	}
}

func TestAntiSpamDenialSkipsCommandAndJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	antiSpam := &fakeAntiSpam{
		decision: antispamservice.Decision{
			Allowed: false,
			Kind:    antispamservice.DecisionCooldown,
			Message: "\u0421\u043b\u0438\u0448\u043a\u043e\u043c \u043c\u043d\u043e\u0433\u043e \u0441\u043e\u043e\u0431\u0449\u0435\u043d\u0438\u0439. \u041f\u043e\u043f\u0440\u043e\u0431\u0443\u0439\u0442\u0435 \u0447\u0435\u0440\u0435\u0437 30 \u0441\u0435\u043a\u0443\u043d\u0434",
		},
	}
	h := newHarnessWithConfigAndAntiSpam(control, vk.Config{ConfirmationToken: "conf-token-123", Secret: "s3cr3t"}, antiSpam)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-antispam","secret":"s3cr3t",
		"object":{"message":{"from_id":559,"peer_id":559,"text":"/image neon cat"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}
	if len(antiSpam.inputs) != 1 || antiSpam.inputs[0].CommandType != domain.CommandImageGenerate {
		t.Fatalf("unexpected anti-spam inputs: %+v", antiSpam.inputs)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 559)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 0 {
		t.Fatalf("anti-spam denial must not create commands, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("anti-spam denial must not create jobs/tasks, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Слишком много сообщений") {
		t.Fatalf("unexpected anti-spam response: %+v", sent)
	}
}

func TestAntiSpamErrorBlocksGenerationInDegradedMode(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	control := vkdelivery.NewMockClient()
	antiSpam := &fakeAntiSpam{err: errors.New("backend SENSITIVE_BACKEND_MARKER unavailable")}
	h := newHarnessWithConfigAndAntiSpam(control, vk.Config{ConfirmationToken: "conf-token-123", Secret: "s3cr3t"}, antiSpam)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-antispam-degraded-image","secret":"s3cr3t",
		"object":{"message":{"from_id":560,"peer_id":560,"text":"/image SENSITIVE_PROMPT_MARKER"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}
	if len(antiSpam.inputs) != 1 || antiSpam.inputs[0].CommandType != domain.CommandImageGenerate || !antiSpam.inputs[0].CreatesJob {
		t.Fatalf("unexpected anti-spam inputs: %+v", antiSpam.inputs)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 560)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 0 {
		t.Fatalf("degraded anti-spam must not create commands, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("degraded anti-spam must not create jobs/tasks, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	inbound, err := h.inbound.GetByIdempotencyKey(ctx, "vk_event:1:evt-antispam-degraded-image")
	if err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if inbound.Status != domain.InboundProcessed {
		t.Fatalf("inbound status = %s, want processed", inbound.Status)
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Генерации временно приостановлены") {
		t.Fatalf("unexpected degraded response: %+v", sent)
	}

	logOutput := logs.String()
	for _, want := range []string{`"action_class":"generation"`, `"reason":"dependency_error"`, `"operation":"image_generate"`, `"error_code":"internal_error"`} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("degraded log missing %s: %s", want, logOutput)
		}
	}
	for _, forbidden := range []string{"SENSITIVE_BACKEND_MARKER", "SENSITIVE_PROMPT_MARKER", "vk_user_id"} {
		if strings.Contains(logOutput, forbidden) {
			t.Fatalf("degraded log contains forbidden marker %q: %s", forbidden, logOutput)
		}
	}
}

func TestAntiSpamErrorAllowsCheapHelpCommand(t *testing.T) {
	control := vkdelivery.NewMockClient()
	antiSpam := &fakeAntiSpam{err: errors.New("antispam store unavailable")}
	h := newHarnessWithConfigAndAntiSpam(control, vk.Config{ConfirmationToken: "conf-token-123", Secret: "s3cr3t"}, antiSpam)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-antispam-degraded-help","secret":"s3cr3t",
		"object":{"message":{"from_id":561,"peer_id":561,"text":"/help"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}
	if len(antiSpam.inputs) != 1 || antiSpam.inputs[0].CommandType != domain.CommandHelp || antiSpam.inputs[0].CreatesJob {
		t.Fatalf("unexpected anti-spam inputs: %+v", antiSpam.inputs)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 561)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandHelp {
		t.Fatalf("cheap help command should remain available, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("help command must not create jobs/tasks, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	if sent := control.Sent(); len(sent) != 1 {
		t.Fatalf("expected help response, got %+v", sent)
	}
}

func TestAntiSpamDenialCompletesWhenResponseSendFails(t *testing.T) {
	control := vkdelivery.NewMockClient()
	control.FailNext(errors.New("vk response unavailable"))
	antiSpam := &fakeAntiSpam{
		decision: antispamservice.Decision{
			Allowed: false,
			Kind:    antispamservice.DecisionCooldown,
			Message: "Слишком много сообщений. Попробуйте позже",
		},
	}
	h := newHarnessWithConfigAndAntiSpam(control, vk.Config{ConfirmationToken: "conf-token-123", Secret: "s3cr3t"}, antiSpam)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-antispam-denied-send-fails","secret":"s3cr3t",
		"object":{"message":{"from_id":562,"peer_id":562,"text":"/image neon cat"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("anti-spam denial should still acknowledge VK, got %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 562)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 0 || len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("failed anti-spam response must not resume processing, cmds=%+v jobs=%+v tasks=%d", cmds, jobs, h.pub.Len())
	}
	inbound, err := h.inbound.GetByIdempotencyKey(ctx, "vk_event:1:evt-antispam-denied-send-fails")
	if err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if inbound.Status != domain.InboundProcessed {
		t.Fatalf("inbound status = %s, want processed", inbound.Status)
	}
}

func TestMessageNewStickerOutsideGPTDoesNotCreateTextJob(t *testing.T) {
	h := newHarness()
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-sticker","secret":"s3cr3t",
		"object":{"message":{
			"from_id":556,"peer_id":556,"text":"",
			"attachments":[{"type":"sticker","sticker":{"sticker_id":123,"product_id":456,"emoji":"🙂"}}]
		}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 556)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandStart {
		t.Fatalf("unexpected command raw text: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("sticker outside GPT mode must not create a text job, got %+v", jobs)
	}
	if h.pub.Len() != 0 {
		t.Fatalf("expected no enqueued task, got %d", h.pub.Len())
	}
}

func TestMessageNewOutgoingBotMessageIgnored(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-outgoing-video-pending","secret":"s3cr3t",
		"object":{"message":{"from_id":-1,"peer_id":557,"out":1,"text":"\u041d\u0435\u0439\u0440\u043e\u0425\u0430\u0431 \u0433\u043e\u0442\u043e\u0432\u0438\u0442 \u0432\u0438\u0434\u0435\u043e..."}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}
	if sent := control.Sent(); len(sent) != 0 {
		t.Fatalf("outgoing bot message must not send welcome/menu response, got %+v", sent)
	}
	if h.pub.Len() != 0 {
		t.Fatalf("outgoing bot message must not enqueue jobs, got %d", h.pub.Len())
	}
	if _, err := h.users.GetByVKUserID(context.Background(), -1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("outgoing bot message must not create a user, err=%v", err)
	}
}

func TestStartSendsWelcomeMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start","secret":"s3cr3t",
		"object":{"message":{"from_id":557,"peer_id":557,"text":"/start"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 557)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandStart {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("start menu must not create a job, got %d", len(jobs))
	}
	if h.pub.Len() != 0 {
		t.Fatalf("expected no enqueued tasks, got %d", h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("expected persistent keyboard update and welcome message, got %+v", sent)
	}
	if !strings.Contains(sent[0].Keyboard, "Показать меню") || !strings.Contains(sent[0].Keyboard, `"inline":false`) {
		t.Fatalf("unexpected persistent keyboard: %q", sent[0].Keyboard)
	}
	if !strings.Contains(sent[0].Keyboard, `"type":"text"`) || strings.Contains(sent[0].Keyboard, `"type":"callback"`) {
		t.Fatalf("persistent keyboard must stay text-only: %q", sent[0].Keyboard)
	}
	if !strings.Contains(sent[1].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("unexpected text: %q", sent[1].Text)
	}
	if !strings.Contains(sent[1].Keyboard, `"inline":true`) || !strings.Contains(sent[1].Keyboard, "Создать видео") || !strings.Contains(sent[1].Keyboard, "Пополнить баланс") {
		t.Fatalf("unexpected keyboard: %q", sent[1].Keyboard)
	}
	if !strings.Contains(sent[1].Keyboard, `"type":"callback"`) {
		t.Fatalf("inline menu must use callback buttons by default: %q", sent[1].Keyboard)
	}
}

func TestStartWithReferralCodeAppliesAndActivatesSharedReferralNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	ctx := context.Background()
	referrer := &domain.User{
		VKUserID: 900001,
		Role:     domain.RoleUser,
		Status:   domain.StatusActive,
		Locale:   "ru",
		Timezone: "Europe/Moscow",
	}
	if err := h.users.Create(ctx, referrer); err != nil {
		t.Fatalf("create referrer: %v", err)
	}
	if err := h.refs.CreateCode(ctx, &domain.ReferralCode{UserID: referrer.ID, Code: "ABC23456"}); err != nil {
		t.Fatalf("create referral code: %v", err)
	}

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-ref","secret":"s3cr3t",
		"object":{"message":{"from_id":900002,"peer_id":900002,"text":"/start ABC23456"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	referred, err := h.users.GetByVKUserID(ctx, 900002)
	if err != nil {
		t.Fatalf("referred user not created: %v", err)
	}
	referral, err := h.refs.GetReferralByReferredUserID(ctx, referred.ID)
	if err != nil {
		t.Fatalf("referral not created: %v", err)
	}
	if referral.ReferrerUserID != referrer.ID || referral.ReferralCode != "ABC23456" || referral.Source != domain.ReferralSourceVKBot {
		t.Fatalf("unexpected referral: %+v", referral)
	}
	if referral.Status != domain.ReferralStatusRewarded || referral.RewardStatus != domain.ReferralRewardApplied || referral.ActivatedAt == nil || referral.RewardedAt == nil {
		t.Fatalf("unexpected referral status: status=%q reward=%q", referral.Status, referral.RewardStatus)
	}
	referrerAccount, err := h.billing.GetAccountByUser(ctx, referrer.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("referrer account not rewarded: %v", err)
	}
	if referrerAccount.BalanceCached != billingservice.DefaultStartingBalance+10 {
		t.Fatalf("referrer balance = %d, want %d", referrerAccount.BalanceCached, billingservice.DefaultStartingBalance+10)
	}
	jobs, _ := h.jobs.ListByUser(ctx, referred.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("referral start must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestFirstPlainTextDoesNotActivateRegisteredReferral(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	ctx := context.Background()
	referrer := &domain.User{
		VKUserID: 900011,
		Role:     domain.RoleUser,
		Status:   domain.StatusActive,
		Locale:   "ru",
		Timezone: "Europe/Moscow",
	}
	referred := &domain.User{
		VKUserID: 900012,
		Role:     domain.RoleUser,
		Status:   domain.StatusActive,
		Locale:   "ru",
		Timezone: "Europe/Moscow",
	}
	if err := h.users.Create(ctx, referrer); err != nil {
		t.Fatalf("create referrer: %v", err)
	}
	if err := h.users.Create(ctx, referred); err != nil {
		t.Fatalf("create referred: %v", err)
	}
	if err := h.refs.CreateReferral(ctx, &domain.Referral{
		ReferrerUserID: referrer.ID,
		ReferredUserID: referred.ID,
		ReferralCode:   "ABC23456",
		Source:         domain.ReferralSourceVKBot,
		Status:         domain.ReferralStatusRegistered,
		RewardStatus:   domain.ReferralRewardPending,
	}); err != nil {
		t.Fatalf("create referral: %v", err)
	}

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-ref-random-text","secret":"s3cr3t",
		"object":{"message":{"from_id":900012,"peer_id":900012,"text":"random text outside start"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	referral, err := h.refs.GetReferralByReferredUserID(ctx, referred.ID)
	if err != nil {
		t.Fatalf("referral not found: %v", err)
	}
	if referral.Status != domain.ReferralStatusRegistered || referral.RewardStatus != domain.ReferralRewardPending || referral.ActivatedAt != nil || referral.RewardedAt != nil {
		t.Fatalf("random text must not activate referral, got %+v", referral)
	}
	if _, err := h.billing.GetAccountByUser(ctx, referrer.ID, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("referrer balance must not be rewarded by random text, err=%v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, referred.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("random onboarding must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestAccountMenuShowsReferralStatsAndShareLink(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken:                   "conf-token-123",
		Secret:                              "s3cr3t",
		ReferralLinkBase:                    "https://vk.com/write-1",
		ReferralShareBase:                   "https://vk.com/share.php",
		ReferralReferrerSignupRewardCredits: 10,
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-account-ref","secret":"s3cr3t",
		"object":{"message":{"from_id":900003,"peer_id":900003,"text":"Мой аккаунт","payload":"{\"command\":\"account\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected account response, got %+v", sent)
	}
	for _, want := range []string{"Мой аккаунт", "общение с НейроХаб", "Реферальная программа", "Приглашённых: 0", "Зарегистрировано: 0", "Активировано: 0", "Бонус начислен: 0", "https://vk.com/write-1", "Поддержка: @neirohub_help"} {
		if !strings.Contains(sent[0].Text, want) {
			t.Fatalf("expected %q in account text: %q", want, sent[0].Text)
		}
	}
	if strings.Contains(sent[0].Text, "безлимитное") {
		t.Fatalf("account text must not promise unlimited usage: %q", sent[0].Text)
	}
	if !strings.Contains(sent[0].Text, fmt.Sprintf("Баланс: %d ⭐️", billingservice.DefaultStartingBalance)) {
		t.Fatalf("account text must include current balance, got %q", sent[0].Text)
	}
	for _, notWant := range []string{"Осталось генераций", "Выполнено генераций", "@supergptsupportbot"} {
		if strings.Contains(sent[0].Text, notWant) {
			t.Fatalf("account text should not contain %q: %q", notWant, sent[0].Text)
		}
	}
	if strings.Contains(sent[0].Keyboard, `"type":"open_link"`) || strings.Contains(sent[0].Keyboard, "Поделиться") || strings.Contains(sent[0].Keyboard, "vk.com/share.php") {
		t.Fatalf("account keyboard should not include share button, got %q", sent[0].Keyboard)
	}
	if !strings.Contains(sent[0].Keyboard, "Назад") {
		t.Fatalf("account keyboard should keep back button, got %q", sent[0].Keyboard)
	}
	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 900003)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	code, err := h.refs.GetCodeByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("referral code not created: %v", err)
	}
	if !strings.Contains(sent[0].Text, code.Code) {
		t.Fatalf("account text must include user's referral code %q: %q", code.Code, sent[0].Text)
	}
}

func TestFirstStartUsesVKFirstNameOnce(t *testing.T) {
	control := vkdelivery.NewMockClient()
	control.SetUserProfile(vkdelivery.UserProfile{UserID: 582, FirstName: "Сергей", LastName: "Макаров"})
	h := newHarnessWithControl(control)
	first := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-name-first","secret":"s3cr3t",
		"object":{"message":{"from_id":582,"peer_id":582,"text":"Старт"}}
	}`
	if rec := h.post(first); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected first response: %d %q", rec.Code, rec.Body.String())
	}
	sent := control.Sent()
	if len(sent) != 2 || !strings.Contains(sent[1].Text, "Сергей, добро пожаловать в НейроХаб") {
		t.Fatalf("expected personalized first welcome, got %+v", sent)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 582)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if user.VKFirstName != "Сергей" || user.WelcomeNameSentAt.IsZero() {
		t.Fatalf("expected cached profile and welcome marker, got %+v", user)
	}

	second := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-name-second","secret":"s3cr3t",
		"object":{"message":{"from_id":582,"peer_id":582,"text":"Старт"}}
	}`
	if rec := h.post(second); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected second response: %d %q", rec.Code, rec.Body.String())
	}
	sent = control.Sent()
	if len(sent) != 4 {
		t.Fatalf("expected second persistent keyboard and welcome, got %+v", sent)
	}
	if strings.Contains(sent[3].Text, "Сергей, добро пожаловать") || !strings.Contains(sent[3].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("expected regular follow-up welcome, got %q", sent[3].Text)
	}
}

func TestMenuFeatureFlagsHideMainMenuButtons(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuFeatures: vk.MenuFeatureFlags{DisabledCommands: map[domain.CommandType]bool{
			domain.CommandMenuStudents: true,
			domain.CommandTopUp:        true,
		}},
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-flags","secret":"s3cr3t",
		"object":{"message":{"from_id":580,"peer_id":580,"text":"/start"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("expected persistent keyboard update and welcome message, got %+v", sent)
	}
	if strings.Contains(sent[1].Keyboard, "Студентам и школьникам") || strings.Contains(sent[1].Keyboard, "Пополнить баланс") {
		t.Fatalf("disabled main menu buttons should be hidden: %q", sent[1].Keyboard)
	}
	for _, want := range []string{"Создать видео", "Создать фото", "Спросить у НейроХаб", "Мой аккаунт"} {
		if !strings.Contains(sent[1].Keyboard, want) {
			t.Fatalf("expected enabled button %q in keyboard: %q", want, sent[1].Keyboard)
		}
	}
}

func TestTopUpMenuCreatesPaymentIntentAfterProductSelection(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken:           "conf-token-123",
		Secret:                      "s3cr3t",
		TopUpReceiptEmail:           "bot-payments@example.com",
		TopUpPaymentRedirectBaseURL: "https://vk.example.test",
	})

	menuBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":590,"peer_id":590,"text":"topup","payload":"{\"command\":\"top_up\"}"}}
	}`
	if rec := h.post(menuBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}
	if sent := control.Sent(); len(sent) != 1 ||
		!strings.Contains(sent[0].Text, "Выберите пакет для пополнения баланса") ||
		!strings.Contains(sent[0].Text, fmt.Sprintf("Баланс сейчас: %d ⭐️", billingservice.DefaultStartingBalance)) ||
		!strings.Contains(sent[0].Keyboard, "crystals_99") ||
		!strings.Contains(sent[0].Keyboard, "99 ⭐️") {
		t.Fatalf("expected top-up product keyboard, got %+v", sent)
	}

	productBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-product","secret":"s3cr3t",
		"object":{"message":{"from_id":590,"peer_id":590,"text":"99 crystals","payload":"{\"command\":\"top_up\",\"product_code\":\"crystals_99\"}"}}
	}`
	if rec := h.post(productBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected product response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 590)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	intents, err := h.payment.ListIntentsByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("list payment intents: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected one payment intent, got %d", len(intents))
	}
	if intents[0].Credits != 99 || intents[0].ReceiptEmail != "bot-payments@example.com" || !strings.Contains(intents[0].ConfirmationURL, "mock.payments.local") {
		t.Fatalf("unexpected payment intent: %+v", intents[0])
	}
	var metadata struct {
		ReturnURL string `json:"return_url"`
	}
	if err := json.Unmarshal(intents[0].Metadata, &metadata); err != nil {
		t.Fatalf("decode payment metadata: %v", err)
	}
	if metadata.ReturnURL != "https://vk.com/write-1" {
		t.Fatalf("unexpected bot payment return url: %q", metadata.ReturnURL)
	}
	if jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0); len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("top-up flow must not create AI jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	expectedPaymentLink := "https://vk.example.test/payments/vk/" + intents[0].ID.String()
	sent := control.Sent()
	if len(sent) < 2 || !strings.Contains(sent[len(sent)-1].Keyboard, `"type":"open_link"`) ||
		!strings.Contains(sent[len(sent)-1].Keyboard, expectedPaymentLink) ||
		!strings.Contains(sent[len(sent)-1].Text, "Баланс сейчас") ||
		strings.Contains(sent[len(sent)-1].Text, "mock.payments.local") ||
		strings.Contains(sent[len(sent)-1].Keyboard, "mock.payments.local") ||
		strings.Contains(sent[len(sent)-1].Keyboard, "payment_link") {
		t.Fatalf("expected final payment open_link keyboard, got %+v", sent)
	}

	reopenBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-reopen","secret":"s3cr3t",
		"object":{"message":{"from_id":590,"peer_id":590,"text":"topup","payload":"{\"command\":\"top_up\"}"}}
	}`
	if rec := h.post(reopenBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected reopen response: %d %q", rec.Code, rec.Body.String())
	}
	sent = control.Sent()
	last := sent[len(sent)-1]
	if !strings.Contains(last.Keyboard, `"type":"open_link"`) ||
		!strings.Contains(last.Keyboard, expectedPaymentLink) ||
		!strings.Contains(last.Text, "Баланс сейчас") ||
		!strings.Contains(last.Keyboard, "new_payment") ||
		strings.Contains(last.Keyboard, "mock.payments.local") ||
		strings.Contains(last.Text, "mock.payments.local") {
		t.Fatalf("expected pending payment keyboard with continue/new actions, got %+v", sent)
	}
	intents, err = h.payment.ListIntentsByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("list payment intents after reopen: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("reopening top-up must not create another intent, got %d", len(intents))
	}
}

func TestTopUpCatalogKeyboardCompactsManyProducts(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken:           "conf-token-123",
		Secret:                      "s3cr3t",
		TopUpReceiptEmail:           "bot-payments@example.com",
		TopUpPaymentRedirectBaseURL: "https://vk.example.test",
	})
	vatCode := int16(1)
	for _, product := range []domain.PaymentProduct{
		{Code: "crystals_10", Title: "NeiroHub 10 crystals", Amount: 1000, Credits: 10},
		{Code: "crystals_150", Title: "NeiroHub 150 crystals", Amount: 15000, Credits: 150},
		{Code: "crystals_250", Title: "NeiroHub 250 crystals", Amount: 25000, Credits: 250},
		{Code: "crystals_400", Title: "NeiroHub 400 crystals", Amount: 40000, Credits: 400},
	} {
		product.Currency = domain.CurrencyRUB
		product.PriceVersion = 1
		product.IsActive = true
		product.VATCode = &vatCode
		product.PaymentSubject = "service"
		product.PaymentMode = "full_prepayment"
		h.payment.PutProduct(&product)
	}

	menuBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-many-products","secret":"s3cr3t",
		"object":{"message":{"from_id":596,"peer_id":596,"text":"topup","payload":"{\"command\":\"top_up\"}"}}
	}`
	if rec := h.post(menuBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one top-up catalog message, got %+v", sent)
	}
	var keyboard struct {
		Inline  bool            `json:"inline"`
		Buttons [][]interface{} `json:"buttons"`
	}
	if err := json.Unmarshal([]byte(sent[0].Keyboard), &keyboard); err != nil {
		t.Fatalf("decode keyboard: %v; json=%s", err, sent[0].Keyboard)
	}
	if !keyboard.Inline {
		t.Fatalf("top-up keyboard must be inline: %s", sent[0].Keyboard)
	}
	if len(keyboard.Buttons) != 4 {
		t.Fatalf("expected compact 4-row keyboard for 6 products plus back, got %d rows: %s", len(keyboard.Buttons), sent[0].Keyboard)
	}
	for i := 0; i < 3; i++ {
		if len(keyboard.Buttons[i]) != 2 {
			t.Fatalf("expected product row %d to contain two buttons, got %d: %s", i, len(keyboard.Buttons[i]), sent[0].Keyboard)
		}
	}
	if !strings.Contains(sent[0].Keyboard, "crystals_10") ||
		!strings.Contains(sent[0].Keyboard, "crystals_700") ||
		!strings.Contains(sent[0].Keyboard, "Назад") {
		t.Fatalf("expected all products and back button in compact keyboard: %s", sent[0].Keyboard)
	}
}

func TestTopUpPaymentRequiresServerOwnedRedirectBase(t *testing.T) {
	tests := []struct {
		name string
		base string
	}{
		{name: "missing", base: ""},
		{name: "http", base: "http://vk.example.test"},
		{name: "with_query", base: "https://vk.example.test?x=1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			control := vkdelivery.NewMockClient()
			h := newHarnessWithConfig(control, vk.Config{
				ConfirmationToken:           "conf-token-123",
				Secret:                      "s3cr3t",
				TopUpReceiptEmail:           "bot-payments@example.com",
				TopUpPaymentRedirectBaseURL: tt.base,
			})

			body := fmt.Sprintf(`{
				"type":"message_new","group_id":1,"event_id":"evt-topup-invalid-redirect-%s","secret":"s3cr3t",
				"object":{"message":{"from_id":594,"peer_id":594,"text":"99 crystals","payload":"{\"command\":\"top_up\",\"product_code\":\"crystals_99\"}"}}
			}`, tt.name)
			if rec := h.post(body); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
				t.Fatalf("unexpected product response: %d %q", rec.Code, rec.Body.String())
			}

			ctx := context.Background()
			user, err := h.users.GetByVKUserID(ctx, 594)
			if err != nil {
				t.Fatalf("user not created: %v", err)
			}
			intents, err := h.payment.ListIntentsByUser(ctx, user.ID, 10, 0)
			if err != nil {
				t.Fatalf("list payment intents: %v", err)
			}
			if len(intents) != 0 {
				t.Fatalf("expected no payment intents without server-owned redirect base, got %d", len(intents))
			}
			sent := control.Sent()
			if len(sent) == 0 {
				t.Fatal("expected safe unavailable notice")
			}
			last := sent[len(sent)-1]
			if !strings.Contains(last.Text, "Платежи временно недоступны") ||
				strings.Contains(last.Text, "mock.payments.local") ||
				strings.Contains(last.Keyboard, "mock.payments.local") ||
				strings.Contains(last.Keyboard, `"type":"open_link"`) {
				t.Fatalf("expected safe notice without provider link, got %+v", last)
			}
		})
	}
}

func TestTopUpPendingPaymentDoesNotExposeProviderURLWhenRedirectBaseInvalid(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken:           "conf-token-123",
		Secret:                      "s3cr3t",
		TopUpReceiptEmail:           "bot-payments@example.com",
		TopUpPaymentRedirectBaseURL: "http://vk.example.test",
	})

	firstMenuBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-pending-invalid-base-user","secret":"s3cr3t",
		"object":{"message":{"from_id":595,"peer_id":595,"text":"topup","payload":"{\"command\":\"top_up\"}"}}
	}`
	if rec := h.post(firstMenuBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 595)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	metadata := json.RawMessage(`{"source":"vk_bot","return_url":"https://vk.com/write-1","product_code":"crystals_99"}`)
	if err := h.payment.CreateIntent(ctx, &domain.PaymentIntent{
		UserID:            user.ID,
		Status:            domain.PaymentIntentWaitingForUser,
		Amount:            9900,
		Currency:          domain.CurrencyRUB,
		Credits:           99,
		PriceVersion:      1,
		Provider:          domain.PaymentProviderMock,
		ProviderPaymentID: "mock-provider-payment",
		ConfirmationURL:   "https://mock.payments.local/checkout/direct",
		IdempotencyKey:    "vk_payment:1:existing-pending-invalid-base",
		ReceiptEmail:      "bot-payments@example.com",
		Metadata:          metadata,
	}); err != nil {
		t.Fatalf("seed pending intent: %v", err)
	}

	reopenBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-pending-invalid-base-reopen","secret":"s3cr3t",
		"object":{"message":{"from_id":595,"peer_id":595,"text":"topup","payload":"{\"command\":\"top_up\"}"}}
	}`
	if rec := h.post(reopenBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected reopen response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	last := sent[len(sent)-1]
	if !strings.Contains(last.Text, "Платежи временно недоступны") ||
		strings.Contains(last.Text, "mock.payments.local") ||
		strings.Contains(last.Keyboard, "mock.payments.local") ||
		strings.Contains(last.Keyboard, `"type":"open_link"`) {
		t.Fatalf("expected pending intent safe fallback without provider link, got %+v", last)
	}
}

func TestTopUpPaymentMessageTrackingStoresSentMessageID(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken:           "conf-token-123",
		Secret:                      "s3cr3t",
		TopUpReceiptEmail:           "bot-payments@example.com",
		TopUpPaymentRedirectBaseURL: "https://vk.example.test",
		TopUpStatusEditEnabled:      true,
	})

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-track","secret":"s3cr3t",
		"object":{"message":{"from_id":593,"peer_id":593,"text":"99 crystals","payload":"{\"command\":\"top_up\",\"product_code\":\"crystals_99\"}"}}
	}`
	if rec := h.post(body); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected product response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 593)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	intents, err := h.payment.ListIntentsByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("list payment intents: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected one payment intent, got %d", len(intents))
	}
	sent := control.Sent()
	if len(sent) == 0 {
		t.Fatal("expected payment message send")
	}
	var metadata struct {
		Source             string `json:"source"`
		VKPeerID           int64  `json:"vk_peer_id"`
		VKPaymentMessageID int64  `json:"vk_payment_message_id"`
	}
	if err := json.Unmarshal(intents[0].Metadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata.Source != "vk_bot" || metadata.VKPeerID != 593 || metadata.VKPaymentMessageID != sent[len(sent)-1].MessageID {
		t.Fatalf("unexpected tracking metadata: %+v sent=%+v", metadata, sent[len(sent)-1])
	}
}

func TestTopUpStaleCatalogDifferentProductCreatesSelectedIntent(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken:           "conf-token-123",
		Secret:                      "s3cr3t",
		TopUpReceiptEmail:           "bot-payments@example.com",
		TopUpPaymentRedirectBaseURL: "https://vk.example.test",
	})

	firstProductBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-99","secret":"s3cr3t",
		"object":{"message":{"from_id":592,"peer_id":592,"text":"99 crystals","payload":"{\"command\":\"top_up\",\"product_code\":\"crystals_99\"}"}}
	}`
	if rec := h.post(firstProductBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected first product response: %d %q", rec.Code, rec.Body.String())
	}

	staleCatalogProductBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-700-stale","secret":"s3cr3t",
		"object":{"message":{"from_id":592,"peer_id":592,"text":"700 crystals","payload":"{\"command\":\"top_up\",\"product_code\":\"crystals_700\",\"action\":\"new_payment\"}"}}
	}`
	if rec := h.post(staleCatalogProductBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected stale catalog product response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 592)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	intents, err := h.payment.ListIntentsByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("list payment intents: %v", err)
	}
	if len(intents) != 2 {
		t.Fatalf("expected two payment intents for different products, got %d", len(intents))
	}
	byCredits := map[int64]*domain.PaymentIntent{}
	for _, intent := range intents {
		byCredits[intent.Credits] = intent
	}
	if byCredits[99] == nil {
		t.Fatalf("expected original 99-credit intent, got %+v", intents)
	}
	if byCredits[700] == nil || byCredits[700].Amount != 70000 {
		t.Fatalf("expected selected 700-credit intent, got %+v", intents)
	}
	sent := control.Sent()
	if len(sent) < 2 || !strings.Contains(sent[len(sent)-1].Text, "700") || strings.Contains(sent[len(sent)-1].Text, "99") {
		t.Fatalf("expected final payment message for 700 package, got %+v", sent[len(sent)-1])
	}
	if jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0); len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("top-up flow must not create AI jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestTopUpMenuWithoutServerReceiptContactDoesNotCreateIntent(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-topup-no-contact","secret":"s3cr3t",
		"object":{"message":{"from_id":591,"peer_id":591,"text":"99 crystals","payload":"{\"command\":\"top_up\",\"product_code\":\"crystals_99\"}"}}
	}`
	if rec := h.post(body); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 591)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	intents, err := h.payment.ListIntentsByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("list payment intents: %v", err)
	}
	if len(intents) != 0 {
		t.Fatalf("expected no payment intents without server receipt contact, got %d", len(intents))
	}
	sent := control.Sent()
	if len(sent) != 1 || strings.Contains(sent[0].Text, "email") || !strings.Contains(sent[0].Text, "не настроены данные для чека") {
		t.Fatalf("expected safe configuration notice without user contact prompt, got %+v", sent)
	}
}

func TestDisabledMenuPayloadFallsBackToCurrentMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuFeatures: vk.MenuFeatureFlags{DisabledCommands: map[domain.CommandType]bool{
			domain.CommandMenuStudents: true,
		}},
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-students-disabled","secret":"s3cr3t",
		"object":{"message":{"from_id":581,"peer_id":581,"text":"🎁 Студентам и школьникам","payload":"{\"command\":\"menu.students\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 581)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandShowMenu {
		t.Fatalf("disabled payload should be recorded as show_menu fallback, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("disabled payload must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("expected current welcome menu fallback, got %+v", sent)
	}
	if strings.Contains(sent[0].Keyboard, "Студентам и школьникам") {
		t.Fatalf("disabled students button should stay hidden in fallback menu: %q", sent[0].Keyboard)
	}
}

func TestTextMenuButtonModeKeepsInlineButtonsAsText(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuButtonMode:    "text",
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-text-mode","secret":"s3cr3t",
		"object":{"message":{"from_id":572,"peer_id":572,"text":"/start"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("expected persistent keyboard update and welcome message, got %+v", sent)
	}
	if strings.Contains(sent[1].Keyboard, `"type":"callback"`) || !strings.Contains(sent[1].Keyboard, `"type":"text"`) {
		t.Fatalf("inline menu should use legacy text buttons in text mode: %q", sent[1].Keyboard)
	}
}

func TestShowMenuSendsWelcomeWithoutResettingPersistentKeyboard(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-show-menu-start","secret":"s3cr3t",
		"object":{"message":{"from_id":559,"peer_id":559,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-show-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":559,"peer_id":559,"text":"Показать меню","payload":"{\"command\":\"show_menu\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 559)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandShowMenu) {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("show menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 3 {
		t.Fatalf("expected persistent keyboard, start menu, and fresh show-menu message, got %+v", sent)
	}
	if !strings.Contains(sent[2].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("unexpected text: %q", sent[2].Text)
	}
	if !strings.Contains(sent[2].Keyboard, `"inline":true`) || strings.Contains(sent[2].Keyboard, "Показать меню") {
		t.Fatalf("unexpected keyboard: %q", sent[2].Keyboard)
	}
	if edits := control.Edits(); len(edits) != 0 {
		t.Fatalf("lower show-menu button must send a fresh menu instead of editing, got edits %+v", edits)
	}
}

func TestTypedMenuRepairPhraseRestoresPersistentKeyboardAndMenu(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-repair-start","secret":"s3cr3t",
		"object":{"message":{"from_id":571,"peer_id":571,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-repair-typed","secret":"s3cr3t",
		"object":{"message":{"from_id":571,"peer_id":571,"text":"нет меню"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 571)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandShowMenu) {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("typed menu repair must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 4 {
		t.Fatalf("expected start keyboard/menu plus repair keyboard/menu, got %+v", sent)
	}
	if !strings.Contains(sent[2].Keyboard, "Показать меню") || strings.Contains(sent[2].Keyboard, `"inline":true`) {
		t.Fatalf("typed menu repair should restore lower keyboard, got %+v", sent[2])
	}
	if !strings.Contains(sent[3].Text, "Добро пожаловать в НейроХаб") || !strings.Contains(sent[3].Keyboard, `"inline":true`) {
		t.Fatalf("typed menu repair should send inline welcome menu, got %+v", sent[3])
	}
}

func TestMenuButtonEditsActiveMenuMessage(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-edit-start","secret":"s3cr3t",
		"object":{"message":{"from_id":570,"peer_id":570,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	initial := control.Sent()
	if len(initial) != 2 {
		t.Fatalf("expected persistent keyboard and active menu, got %+v", initial)
	}
	activeID := initial[1].MessageID

	video := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-edit-video","secret":"s3cr3t",
        "object":{"message":{"from_id":570,"peer_id":570,"text":"\ud83c\udfac \u0421\u043e\u0437\u0434\u0430\u0442\u044c \u0432\u0438\u0434\u0435\u043e","payload":"{\"command\":\"menu.video\"}"}}
	}`
	if rec := h.post(video); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected video response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("menu button should edit the active menu instead of sending a new one, got %+v", sent)
	}
	edits := control.Edits()
	if len(edits) != 1 || edits[0].MessageID != activeID {
		t.Fatalf("expected one edit of active menu %d, got %+v", activeID, edits)
	}
	if sent[1].Text != "Выбери режим видео:" {
		t.Fatalf("active menu was not updated to video picker: %+v", sent[1])
	}
}

func TestCallbackMenuEventEditsActiveMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-callback-start","secret":"s3cr3t",
		"object":{"message":{"from_id":573,"peer_id":573,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	activeID := control.Sent()[1].MessageID

	callback := `{
		"type":"message_event","group_id":1,"event_id":"evt-callback-video","secret":"s3cr3t",
		"object":{"user_id":573,"peer_id":573,"event_id":"vk-button-event-1","payload":{"command":"menu.video"}}
	}`
	rec := h.post(callback)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected callback response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 573)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandMenuVideo) {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("callback menu event must not create a job, got %d", len(jobs))
	}
	if len(control.Sent()) != 2 {
		t.Fatalf("callback should edit active menu instead of sending a new message, got %+v", control.Sent())
	}
	edits := control.Edits()
	if len(edits) != 1 || edits[0].MessageID != activeID || !strings.Contains(edits[0].Text, "Выбери режим видео") {
		t.Fatalf("expected active menu edit to video picker, got %+v", edits)
	}
	answers := control.EventAnswers()
	if len(answers) != 1 || answers[0].EventID != "vk-button-event-1" || answers[0].UserID != 573 || answers[0].PeerID != 573 {
		t.Fatalf("expected callback event answer, got %+v", answers)
	}
}

func TestPlainMessageKeepsPreviousMenuAndLowerShowMenuSendsFreshMenu(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-clear-start","secret":"s3cr3t",
		"object":{"message":{"from_id":571,"peer_id":571,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}

	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-clear-text","secret":"s3cr3t",
        "object":{"message":{"from_id":571,"peer_id":571,"text":"\u041f\u0440\u0438\u0434\u0443\u043c\u0430\u0439 \u0438\u0434\u0435\u0438 \u0434\u043b\u044f \u0432\u0438\u0434\u0435\u043e"}}
	}`
	if rec := h.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}
	afterPlain := control.Sent()
	if len(afterPlain) != 3 {
		t.Fatalf("plain text should send a choose-mode hint, got %+v", afterPlain)
	}
	if afterPlain[2].Text != "Выберите режим в меню выше или нажмите на кнопку показать меню" || !strings.Contains(afterPlain[2].Keyboard, "Показать меню") || strings.Contains(afterPlain[2].Keyboard, `"inline":true`) {
		t.Fatalf("unexpected choose-mode hint: %+v", afterPlain[2])
	}

	menu := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-clear-show","secret":"s3cr3t",
        "object":{"message":{"from_id":571,"peer_id":571,"text":"\u041f\u043e\u043a\u0430\u0437\u0430\u0442\u044c \u043c\u0435\u043d\u044e","payload":"{\"command\":\"show_menu\"}"}}
	}`
	if rec := h.post(menu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 4 {
		t.Fatalf("lower show-menu should send a fresh menu below the text hint, got %+v", sent)
	}
	if sent[2].Text != "Выберите режим в меню выше или нажмите на кнопку показать меню" || !strings.Contains(sent[2].Keyboard, "Показать меню") || strings.Contains(sent[2].Keyboard, `"inline":true`) {
		t.Fatalf("choose-mode hint should keep lower keyboard after lower show-menu, got %+v", sent[2])
	}
	if !strings.Contains(sent[3].Text, "Добро пожаловать в НейроХаб") || !strings.Contains(sent[3].Keyboard, `"inline":true`) {
		t.Fatalf("expected fresh welcome menu after lower show-menu, got %+v", sent[3])
	}
	if edits := control.Edits(); len(edits) != 0 {
		t.Fatalf("lower show-menu must not edit an older menu, got edits %+v", edits)
	}
}

func TestVideoMenuButtonSendsModelPickerNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":560,"peer_id":560,"text":"🎬 Создать видео","payload":"{\"command\":\"menu.video\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 560)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuVideo {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("video menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one model picker message, got %+v", sent)
	}
	if sent[0].Text != "Выбери режим видео:" {
		t.Fatalf("unexpected text: %q", sent[0].Text)
	}
	if !strings.Contains(sent[0].Keyboard, "⬅️ Назад") || !strings.Contains(sent[0].Keyboard, string(domain.CommandShowMenu)) {
		t.Fatalf("expected back button in keyboard: %q", sent[0].Keyboard)
	}
	for _, hidden := range []string{
		"PrunaAI",
		"Runway Gen-4 Turbo",
		"Kling O3 Standard",
		"Seedance 2.0 Fast",
		"Hailuo 2.3 Standard",
		"Hailuo 2.3 Fast",
	} {
		if strings.Contains(sent[0].Keyboard, hidden) {
			t.Fatalf("expected video route button %q to be hidden: %q", hidden, sent[0].Keyboard)
		}
	}
}

func TestVideoMenuButtonUsesProductCatalogRoutes(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		VideoRoutes: []productcatalog.VideoRoute{
			testKlingVideoRoute(),
			testRunwayVideoRoute(),
			{
				Type:                productcatalog.TypeVideo,
				Alias:               string(domain.VideoRouteSeedance20Fast),
				Name:                "Seedance 2.0 Fast",
				Enabled:             false,
				AllowedDurationsSec: []int{5, 10},
				DefaultDurationSec:  5,
			},
		},
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-menu-catalog","secret":"s3cr3t",
		"object":{"message":{"from_id":5601,"peer_id":5601,"text":"🎬 Создать видео","payload":"{\"command\":\"menu.video\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one model picker message, got %+v", sent)
	}
	keyboard := sent[0].Keyboard
	for _, want := range []string{
		"Kling O3 Standard",
		"Runway Gen-4 Turbo",
		string(domain.CommandMenuVideoRouteSelect),
		string(domain.VideoRouteKlingO3Standard),
		string(domain.VideoRouteRunwayGen4Turbo),
	} {
		if !strings.Contains(keyboard, want) {
			t.Fatalf("catalog video keyboard missing %q: %q", want, keyboard)
		}
	}
	for _, forbidden := range []string{
		"Seedance 2.0 Fast",
		"provider",
		"provider_model_id",
		"model_code",
		"api_key",
		"authorization",
		"auth_header",
		"bearer",
		"resolved_video_route",
		"resolved_snapshot",
		"raw_provider_payload",
		"private_url",
		"prompt",
		"provider_cost_credits",
		"price_multiplier",
		"max_internal_cost_credits",
		"price",
		"cost",
		string(domain.CommandMenuVideoSora2Start),
		string(domain.CommandMenuVideoKling21Start),
		string(domain.CommandMenuVideoSeedance1Lite),
		string(domain.CommandMenuVideoHailuo02Standard),
		string(domain.CommandMenuVideoHailuo02Fast),
	} {
		if strings.Contains(keyboard, forbidden) {
			t.Fatalf("catalog video keyboard leaked/kept forbidden value %q: %q", forbidden, keyboard)
		}
	}
}

func TestVideoRouteButtonEnablesPlainTextVideoJobs(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		VideoRoutes:       []productcatalog.VideoRoute{testKlingVideoRoute()},
	})
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-route-on","secret":"s3cr3t",
		"object":{"message":{"from_id":5622,"peer_id":5622,"text":"Kling O3 Standard","payload":"{\"command\":\"menu.video.route.select\",\"video_route_alias\":\"video_kling_o3_standard\"}"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected route response: %d %q", rec.Code, rec.Body.String())
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-route-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5622,"peer_id":5622,"text":"cinematic neon city at night, rain reflections, slow drone movement"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5622)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandMenuVideoRouteSelect, domain.CommandVideoGenerate) {
		t.Fatalf("unexpected command types: %+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	expectedCost := vkTestVideoCostCredits(t, domain.VideoRouteKlingO3Standard, pricingcatalog.VideoResolution720p, 5)
	if len(jobs) != 1 ||
		jobs[0].OperationType != domain.OperationVideoGenerate ||
		jobs[0].Modality != domain.ModalityVideo ||
		jobs[0].CostEstimate != expectedCost ||
		jobs[0].CostReserved != expectedCost ||
		h.pub.Len() != 1 {
		t.Fatalf("video route mode should create one video job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 || !strings.Contains(sent[0].Text, "Выберите длительность видео") || sent[1].Text != "НейроХаб готовит видео..." {
		t.Fatalf("unexpected video route mode responses: %+v", sent)
	}
	if strings.Contains(sent[0].Keyboard, "provider") || strings.Contains(sent[0].Keyboard, "model_code") || strings.Contains(sent[0].Keyboard, "resolved_video_route") {
		t.Fatalf("video duration keyboard must not expose provider internals: %q", sent[0].Keyboard)
	}
	var params struct {
		Prompt                 string `json:"prompt"`
		ModelID                string `json:"model_id"`
		ModelName              string `json:"model_name"`
		VideoRouteAlias        string `json:"video_route_alias"`
		Provider               string `json:"provider"`
		ModelCode              string `json:"model_code"`
		DurationSec            int    `json:"duration_sec"`
		VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
	}
	if err := json.Unmarshal(jobs[0].Params, &params); err != nil {
		t.Fatalf("decode job params: %v", err)
	}
	if params.Prompt != "cinematic neon city at night, rain reflections, slow drone movement" ||
		params.ModelID != "" ||
		params.ModelName != "Kling O3 Standard" ||
		params.VideoRouteAlias != string(domain.VideoRouteKlingO3Standard) ||
		params.Provider != "" ||
		params.ModelCode != "" ||
		params.DurationSec != 5 ||
		params.VKPlaceholderMessageID != sent[1].MessageID {
		t.Fatalf("unexpected video route job params: %+v, pending=%+v", params, sent[1])
	}
}

func TestVideoRouteActiveJobLimitShowsSpecificMessage(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithReferenceDownloaderAndOrchestratorOptions(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		VideoRoutes:       []productcatalog.VideoRoute{testKlingVideoRoute()},
	}, nil, nil, nil, joborchestrator.WithMaxActiveVideoJobsPerUser(1))
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-active-limit-route","secret":"s3cr3t",
		"object":{"message":{"from_id":5625,"peer_id":5625,"text":"Kling O3 Standard","payload":"{\"command\":\"menu.video.route.select\",\"video_route_alias\":\"video_kling_o3_standard\"}"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected route response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5625)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if err := h.jobs.Create(ctx, &domain.Job{
		ID:             uuid.New(),
		UserID:         user.ID,
		Source:         "vk_bot",
		VKPeerID:       5625,
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusDelivering,
		IdempotencyKey: "test:active-video-limit",
		CorrelationID:  "test-active-video-limit",
		CostEstimate:   150,
		CostReserved:   150,
	}); err != nil {
		t.Fatalf("seed active job: %v", err)
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-active-limit-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5625,"peer_id":5625,"text":"cinematic city lights, slow camera movement"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	edits := control.Edits()
	if len(edits) == 0 {
		t.Fatalf("expected pending message edit, sent=%+v", control.Sent())
	}
	last := edits[len(edits)-1]
	if last.Text != "У вас уже есть видео в обработке\nДождитесь результата или попробуйте позже" {
		t.Fatalf("unexpected active-limit message: %+v", last)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || h.pub.Len() != 0 {
		t.Fatalf("active limit must not create or publish a second job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestVideoBackTextButtonReturnsToVideoMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuButtonMode:    "text",
		VideoRoutes:       []productcatalog.VideoRoute{testKlingVideoRoute()},
	})
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-back-text-route","secret":"s3cr3t",
		"object":{"message":{"from_id":5624,"peer_id":5624,"text":"Kling O3 Standard","payload":"{\"command\":\"menu.video.route.select\",\"video_route_alias\":\"video_kling_o3_standard\"}"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected route response: %d %q", rec.Code, rec.Body.String())
	}
	back := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-back-text","secret":"s3cr3t",
		"object":{"message":{"from_id":5624,"peer_id":5624,"text":"Назад к видео"}}
	}`
	if rec := h.post(back); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected back response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5624)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("back text button must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) < 2 || !strings.Contains(sent[len(sent)-1].Keyboard, "Kling O3 Standard") {
		t.Fatalf("expected video menu after text back button, got %+v", sent)
	}
	if strings.Contains(sent[len(sent)-1].Keyboard, `"type":"callback"`) ||
		!strings.Contains(sent[len(sent)-1].Keyboard, `"type":"text"`) {
		t.Fatalf("text button mode should keep text buttons: %q", sent[len(sent)-1].Keyboard)
	}
}

func TestVideoRouteDurationButtonSetsJobDuration(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		VideoRoutes:       []productcatalog.VideoRoute{testKlingVideoRoute()},
	})
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-route-duration","secret":"s3cr3t",
		"object":{"message":{"from_id":5623,"peer_id":5623,"text":"10 сек","payload":"{\"command\":\"menu.video.duration.select\",\"video_route_alias\":\"video_kling_o3_standard\",\"duration_sec\":10}"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected route response: %d %q", rec.Code, rec.Body.String())
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-route-duration-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5623,"peer_id":5623,"text":"cinematic city lights, slow camera movement"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5623)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	expectedCost := vkTestVideoCostCredits(t, domain.VideoRouteKlingO3Standard, pricingcatalog.VideoResolution720p, 10)
	if len(jobs) != 1 || jobs[0].CostEstimate != expectedCost || jobs[0].CostReserved != expectedCost {
		t.Fatalf("duration-specific video job was not estimated from 10s route, jobs=%+v", jobs)
	}
	var params struct {
		VideoRouteAlias string `json:"video_route_alias"`
		DurationSec     int    `json:"duration_sec"`
	}
	if err := json.Unmarshal(jobs[0].Params, &params); err != nil {
		t.Fatalf("decode job params: %v", err)
	}
	if params.VideoRouteAlias != string(domain.VideoRouteKlingO3Standard) || params.DurationSec != 10 {
		t.Fatalf("unexpected duration params: %+v", params)
	}
}

func TestVideoRouteWithPhotoAttachmentCreatesReferenceArtifactJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	downloader := &fakeReferenceDownloader{data: pngSizedBytes(20, 40)}
	h := newHarnessWithReferenceDownloader(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuFeatures: enabledVideoCommands(
			domain.CommandMenuVideoSora2,
			domain.CommandMenuVideoSora2Start,
			domain.CommandMenuVideoSora2Examples,
		),
	}, nil, nil, downloader)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-runway-start","secret":"s3cr3t",
		"object":{"message":{"from_id":5630,"peer_id":5630,"text":"Runway Gen-4 Turbo","payload":"{\"command\":\"menu.video.sora_v2.start\"}"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected route response: %d %q", rec.Code, rec.Body.String())
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-runway-photo","secret":"s3cr3t",
		"object":{"message":{
			"from_id":5630,
			"peer_id":5630,
			"text":"animate this portrait with subtle camera movement",
			"attachments":[{
				"type":"photo",
				"photo":{"sizes":[
					{"type":"s","url":"https://vk.example/small.png","width":10,"height":10},
					{"type":"x","url":"https://vk.example/large.png","width":20,"height":40}
				]}
			}]
		}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5630)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || len(jobs[0].InputArtifactIDs) != 1 || h.pub.Len() != 1 {
		t.Fatalf("expected one referenced video job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	if len(downloader.urls) != 1 || downloader.urls[0] != "https://vk.example/large.png" {
		t.Fatalf("unexpected downloaded urls: %+v", downloader.urls)
	}
	artifact, err := h.arts.GetByID(ctx, jobs[0].InputArtifactIDs[0])
	if err != nil {
		t.Fatalf("reference artifact missing: %v", err)
	}
	if artifact.OwnerUserID != user.ID || artifact.Kind != domain.ArtifactKindInput || artifact.MediaType != domain.MediaTypeImage || artifact.MimeType != "image/png" || artifact.Width != 20 || artifact.Height != 40 {
		t.Fatalf("unexpected artifact: %+v", artifact)
	}
	if _, err := h.objects.GetObject(ctx, artifact.StorageBucket, artifact.StorageKey); err != nil {
		t.Fatalf("reference object missing: %v", err)
	}
	var params struct {
		ModelID              string      `json:"model_id"`
		VideoRouteAlias      string      `json:"video_route_alias"`
		Provider             string      `json:"provider"`
		ModelCode            string      `json:"model_code"`
		AspectRatio          string      `json:"aspect_ratio"`
		ReferenceArtifactIDs []uuid.UUID `json:"reference_artifact_ids"`
	}
	if err := json.Unmarshal(jobs[0].Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if params.ModelID != "" || params.Provider != "" || params.ModelCode != "" ||
		params.VideoRouteAlias != string(domain.VideoRouteRunwayGen4Turbo) ||
		params.AspectRatio != "9:16" ||
		len(params.ReferenceArtifactIDs) != 1 || params.ReferenceArtifactIDs[0] != artifact.ID {
		t.Fatalf("unexpected video params: %+v", params)
	}
}

func TestImageRequiredVideoRouteWithoutPhotoDoesNotCreateJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuFeatures: enabledVideoCommands(
			domain.CommandMenuVideoHailuo02,
			domain.CommandMenuVideoHailuo02Fast,
		),
	})
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-hailuo-fast-start","secret":"s3cr3t",
		"object":{"message":{"from_id":5631,"peer_id":5631,"text":"Hailuo 2.3 Fast","payload":"{\"command\":\"menu.video.hailuo_02.fast\"}"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected route response: %d %q", rec.Code, rec.Body.String())
	}
	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-hailuo-fast-no-photo","secret":"s3cr3t",
		"object":{"message":{"from_id":5631,"peer_id":5631,"text":"make it move fast"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5631)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("missing required photo must not create job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 || sent[1].Text == "НейроХаб готовит видео..." {
		t.Fatalf("expected reference notice instead of pending job message, got %+v", sent)
	}
}

func TestPhotoMenuButtonSendsInstructionNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":562,"peer_id":562,"text":"🖼️ Создать фото","payload":"{\"command\":\"menu.image\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 562)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuImage {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("photo menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one photo instruction message, got %+v", sent)
	}
	for _, want := range []string{
		"Генерация фото по тексту",
		"⬅️ Назад",
	} {
		if !strings.Contains(sent[0].Text+sent[0].Keyboard, want) {
			t.Fatalf("expected %q in photo response: text=%q keyboard=%q", want, sent[0].Text, sent[0].Keyboard)
		}
	}
	if strings.Contains(sent[0].Text, "100 бесплатных попыток") {
		t.Fatalf("photo instruction must not promise free daily attempts: %q", sent[0].Text)
	}
	if !strings.Contains(sent[0].Keyboard, "Nano Banana 2") {
		t.Fatalf("expected Nano Banana 2 button in photo keyboard: %q", sent[0].Keyboard)
	}
	if !strings.Contains(sent[0].Keyboard, "menu.image.select") || !strings.Contains(sent[0].Keyboard, "model_id") {
		t.Fatalf("expected catalog-driven public model payloads in photo keyboard: %q", sent[0].Keyboard)
	}
	for _, private := range []string{
		"model_code",
		"provider",
		"provider_model_id",
		"api_key",
		"authorization",
		"auth_header",
		"bearer",
		"resolved_snapshot",
		"raw_provider_payload",
		"private_url",
		"prompt",
		"provider_cost_credits",
		"price_multiplier",
		"max_internal_cost_credits",
		"price",
		"cost",
		"nano-banana-2",
		"gemini-3-pro-image-preview",
		"gpt-image-2",
	} {
		if strings.Contains(sent[0].Keyboard, private) {
			t.Fatalf("photo keyboard leaked private field %q: %q", private, sent[0].Keyboard)
		}
	}
	for _, legacyPayload := range []string{
		string(domain.CommandMenuImageText),
		string(domain.CommandMenuImageNanoBanana2),
		string(domain.CommandMenuImageDeepInfraSeedream),
		string(domain.CommandMenuImageDeepInfraSDXL),
		string(domain.CommandMenuImageGPTImage2),
		string(domain.CommandMenuImageReference),
	} {
		if strings.Contains(sent[0].Keyboard, legacyPayload) {
			t.Fatalf("primary photo menu must use generic catalog payloads, found legacy %q in %q", legacyPayload, sent[0].Keyboard)
		}
	}
	if !strings.Contains(sent[0].Keyboard, "Nano Banana Pro") {
		t.Fatalf("expected Nano Banana Pro button in photo keyboard: %q", sent[0].Keyboard)
	}
	if !strings.Contains(sent[0].Keyboard, "GPT Image 2") {
		t.Fatalf("expected GPT Image 2 button in photo keyboard: %q", sent[0].Keyboard)
	}
	for _, hidden := range []string{"ByteDance Seedream 4.5", "Stability AI SDXL Turbo"} {
		if strings.Contains(sent[0].Keyboard, hidden) {
			t.Fatalf("unpriced image model %q must be hidden: %q", hidden, sent[0].Keyboard)
		}
	}
	if strings.Contains(sent[0].Keyboard, "Фото по тексту") {
		t.Fatalf("photo text button should be hidden because photo menu already enables text-to-image mode: keyboard=%q", sent[0].Keyboard)
	}
	if strings.Contains(sent[0].Keyboard, "Фото с референсом") {
		t.Fatalf("photo reference button should be hidden: keyboard=%q", sent[0].Keyboard)
	}
}

func TestPhotoNanoBananaProQualityFlowCreatesImageJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	menu := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-pro-on","secret":"s3cr3t",
		"object":{"message":{"from_id":5631,"peer_id":5631,"text":"Nano Banana Pro","payload":"{\"command\":\"menu.image.text\"}"}}
	}`
	if rec := h.post(menu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}
	initial := control.Sent()
	if len(initial) != 1 || !strings.Contains(initial[0].Text, "Выберите качество генерации") {
		t.Fatalf("expected Nano Banana Pro quality picker, got %+v", initial)
	}
	display1K := vkTestImageDisplayCredits(t, modelcatalog.MiniAppImageNanoBananaPro, modelcatalog.ImageQuality1K)
	display2K := vkTestImageDisplayCredits(t, modelcatalog.MiniAppImageNanoBananaPro, modelcatalog.ImageQuality2K)
	display4K := vkTestImageDisplayCredits(t, modelcatalog.MiniAppImageNanoBananaPro, modelcatalog.ImageQuality4K)
	for _, want := range []string{
		fmt.Sprintf("1K \u00b7 %d", display1K),
		fmt.Sprintf("2K \u00b7 %d", display2K),
		fmt.Sprintf("4K \u00b7 %d", display4K),
		"⬅️ Назад к фото",
	} {
		if !strings.Contains(initial[0].Keyboard, want) {
			t.Fatalf("expected %q in Nano Banana Pro quality keyboard: %q", want, initial[0].Keyboard)
		}
	}
	quality := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-pro-quality","secret":"s3cr3t",
		"object":{"message":{"from_id":5631,"peer_id":5631,"text":"4K","payload":"{\"command\":\"menu.image.quality.select\",\"model_id\":\"nano_banana_pro\",\"image_quality\":\"4K\"}"}}
	}`
	if rec := h.post(quality); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected quality response: %d %q", rec.Code, rec.Body.String())
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-text-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5631,"peer_id":5631,"text":"кот в очках на пляже"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5631)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandMenuImageText, domain.CommandMenuImageQualitySelect, domain.CommandImageGenerate) {
		t.Fatalf("unexpected command types: %+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	expectedCost := vkTestImageCostCredits(t, modelcatalog.MiniAppImageNanoBananaPro, modelcatalog.ImageQuality4K)
	if len(jobs) != 1 ||
		jobs[0].OperationType != domain.OperationImageGenerate ||
		jobs[0].Modality != domain.ModalityImage ||
		jobs[0].CostEstimate != expectedCost ||
		jobs[0].CostReserved != expectedCost ||
		h.pub.Len() != 1 {
		t.Fatalf("photo text mode should create one image job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 ||
		!strings.Contains(sent[0].Text, "Nano Banana Pro \u00b7 4K") ||
		!strings.Contains(sent[0].Text, fmt.Sprintf("\u0426\u0435\u043d\u0430: %d", expectedCost)) ||
		sent[1].Text != "НейроХаб рисует..." {
		t.Fatalf("unexpected photo mode responses: %+v", sent)
	}
	if strings.Contains(sent[0].Text, "Google") || strings.Contains(sent[0].Text, "Ожидание") {
		t.Fatalf("photo prompt text must not expose provider wait details: %q", sent[0].Text)
	}
	if strings.HasSuffix(sent[0].Text, ".") {
		t.Fatalf("photo prompt text must not end with dot: %q", sent[0].Text)
	}
	for _, want := range []string{"⬅️ Назад к качеству", "⬅️ Назад к фото"} {
		if !strings.Contains(sent[0].Keyboard, want) {
			t.Fatalf("expected %q in prompt keyboard: %q", want, sent[0].Keyboard)
		}
	}
	for _, mojibake := range []string{"в¬", "РќР°", "РјРѕ"} {
		if strings.Contains(sent[0].Keyboard, mojibake) {
			t.Fatalf("prompt keyboard contains mojibake %q: %q", mojibake, sent[0].Keyboard)
		}
	}
	var params struct {
		Prompt                 string `json:"prompt"`
		ModelID                string `json:"model_id"`
		ModelName              string `json:"model_name"`
		Size                   string `json:"size"`
		Resolution             string `json:"resolution"`
		ImageQuality           string `json:"image_quality"`
		VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
	}
	if err := json.Unmarshal(jobs[0].Params, &params); err != nil {
		t.Fatalf("decode job params: %v", err)
	}
	if params.Prompt != "кот в очках на пляже" ||
		params.ModelID != "nano_banana_pro" ||
		params.ModelName != "Nano Banana Pro" ||
		params.Size != "1:1" ||
		params.Resolution != "4K" ||
		params.ImageQuality != "4K" ||
		params.VKPlaceholderMessageID != sent[1].MessageID {
		t.Fatalf("unexpected image job params: %+v, pending=%+v", params, sent[1])
	}
}

func TestPhotoInsufficientBalanceShowsTopUpKeyboard(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	menu := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-insufficient-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":5640,"peer_id":5640,"text":"GPT Image 2","payload":"{\"command\":\"menu.image.select\",\"model_id\":\"gpt_image_2\"}"}}
	}`
	if rec := h.post(menu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}
	quality := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-insufficient-quality","secret":"s3cr3t",
		"object":{"message":{"from_id":5640,"peer_id":5640,"text":"4K","payload":"{\"command\":\"menu.image.quality.select\",\"model_id\":\"gpt_image_2\",\"image_quality\":\"4K\"}"}}
	}`
	if rec := h.post(quality); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected quality response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5640)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	account, err := h.billing.GetAccountByUser(ctx, user.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("billing account not created: %v", err)
	}
	if account.BalanceCached > 0 {
		if err := h.billing.AppendEntry(ctx, &domain.LedgerEntry{
			AccountID:      account.ID,
			Type:           domain.LedgerAdjustment,
			Amount:         -account.BalanceCached,
			Status:         domain.LedgerStatusCommitted,
			IdempotencyKey: "test:drain-balance:" + user.ID.String(),
			Reason:         "test insufficient balance",
		}); err != nil {
			t.Fatalf("drain balance: %v", err)
		}
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-insufficient-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5640,"peer_id":5640,"text":"editorial perfume bottle"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) < 2 {
		t.Fatalf("expected prompt and insufficient balance messages, got %+v", sent)
	}
	last := sent[len(sent)-1]
	if last.Text != "Недостаточный баланс" ||
		!strings.Contains(last.Keyboard, "Пополнить") ||
		!strings.Contains(last.Keyboard, "Назад") ||
		!strings.Contains(last.Keyboard, string(domain.CommandTopUp)) ||
		!strings.Contains(last.Keyboard, string(domain.CommandShowMenu)) {
		t.Fatalf("unexpected insufficient balance message: %+v", last)
	}
	if strings.HasSuffix(last.Text, ".") {
		t.Fatalf("insufficient balance text must not end with dot: %q", last.Text)
	}
	if h.pub.Len() != 0 {
		t.Fatalf("insufficient balance job must not be published, tasks=%d", h.pub.Len())
	}
}

func TestPhotoBackTextButtonReturnsToPhotoMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuButtonMode:    "text",
	})
	menu := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-back-text-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":5641,"peer_id":5641,"text":"GPT Image 2","payload":"{\"command\":\"menu.image.select\",\"model_id\":\"gpt_image_2\"}"}}
	}`
	if rec := h.post(menu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}
	quality := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-back-text-quality","secret":"s3cr3t",
		"object":{"message":{"from_id":5641,"peer_id":5641,"text":"4K","payload":"{\"command\":\"menu.image.quality.select\",\"model_id\":\"gpt_image_2\",\"image_quality\":\"4K\"}"}}
	}`
	if rec := h.post(quality); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected quality response: %d %q", rec.Code, rec.Body.String())
	}
	back := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-back-text","secret":"s3cr3t",
		"object":{"message":{"from_id":5641,"peer_id":5641,"text":"Назад к фото"}}
	}`
	if rec := h.post(back); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected back response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5641)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("back text button must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) < 2 || !strings.Contains(sent[len(sent)-1].Keyboard, "GPT Image 2") {
		t.Fatalf("expected photo menu after text back button, got %+v", sent)
	}
	if strings.Contains(sent[len(sent)-1].Keyboard, `"type":"callback"`) ||
		!strings.Contains(sent[len(sent)-1].Keyboard, `"type":"text"`) {
		t.Fatalf("text button mode should keep text buttons: %q", sent[len(sent)-1].Keyboard)
	}
}

func TestPhotoGPTImage2QualityFlowCreatesAPIMartImageJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	menu := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-gpt-image-2-on","secret":"s3cr3t",
		"object":{"message":{"from_id":5635,"peer_id":5635,"text":"GPT Image 2","payload":"{\"command\":\"menu.image.select\",\"model_id\":\"gpt_image_2\"}"}}
	}`
	if rec := h.post(menu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}
	initial := control.Sent()
	display4K := vkTestImageDisplayCredits(t, modelcatalog.MiniAppImageGPTImage2, modelcatalog.ImageQuality4K)
	if len(initial) != 1 ||
		!strings.Contains(initial[0].Text, "Выберите качество генерации") ||
		!strings.Contains(initial[0].Keyboard, fmt.Sprintf("4K \u00b7 %d", display4K)) {
		t.Fatalf("expected GPT Image 2 quality picker, got %+v", initial)
	}

	quality := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-gpt-image-2-quality","secret":"s3cr3t",
		"object":{"message":{"from_id":5635,"peer_id":5635,"text":"4K","payload":"{\"command\":\"menu.image.quality.select\",\"model_id\":\"gpt_image_2\",\"image_quality\":\"4K\"}"}}
	}`
	if rec := h.post(quality); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected quality response: %d %q", rec.Code, rec.Body.String())
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-gpt-image-2-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5635,"peer_id":5635,"text":"editorial perfume bottle"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5635)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	expectedCost := vkTestImageCostCredits(t, modelcatalog.MiniAppImageGPTImage2, modelcatalog.ImageQuality4K)
	if len(jobs) != 1 || jobs[0].CostEstimate != expectedCost || jobs[0].CostReserved != expectedCost {
		t.Fatalf("GPT Image 2 should create one reserved image job, jobs=%+v", jobs)
	}
	var params struct {
		ModelID      string `json:"model_id"`
		ModelName    string `json:"model_name"`
		Provider     string `json:"provider"`
		ModelCode    string `json:"model_code"`
		Size         string `json:"size"`
		Resolution   string `json:"resolution"`
		ImageQuality string `json:"image_quality"`
	}
	if err := json.Unmarshal(jobs[0].Params, &params); err != nil {
		t.Fatalf("decode job params: %v", err)
	}
	if params.ModelID != "gpt_image_2" ||
		params.ModelName != "GPT Image 2" ||
		params.Provider != "apimart" ||
		params.ModelCode != "gpt-image-2" ||
		params.Size != "1:1" ||
		params.Resolution != "4K" ||
		params.ImageQuality != "4K" {
		t.Fatalf("unexpected GPT Image 2 job params: %+v", params)
	}
}

func TestPhotoNanoBanana2ModeCreatesPoYoImageJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	menu := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-nano-2-on","secret":"s3cr3t",
		"object":{"message":{"from_id":5632,"peer_id":5632,"text":"Nano Banana 2","payload":"{\"command\":\"menu.image.select\",\"model_id\":\"nano_banana_2\"}"}}
	}`
	if rec := h.post(menu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}

	quality := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-nano-2-quality","secret":"s3cr3t",
		"object":{"message":{"from_id":5632,"peer_id":5632,"text":"4K","payload":"{\"command\":\"menu.image.quality.select\",\"model_id\":\"nano_banana_2\",\"image_quality\":\"4K\"}"}}
	}`
	if rec := h.post(quality); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected quality response: %d %q", rec.Code, rec.Body.String())
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-nano-2-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5632,"peer_id":5632,"text":"cinematic robot portrait"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5632)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandMenuImageSelect, domain.CommandMenuImageQualitySelect, domain.CommandImageGenerate) {
		t.Fatalf("unexpected command types: %+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	expectedCost := vkTestImageCostCredits(t, modelcatalog.MiniAppImageNanoBanana2, modelcatalog.ImageQuality4K)
	if len(jobs) != 1 ||
		jobs[0].OperationType != domain.OperationImageGenerate ||
		jobs[0].Modality != domain.ModalityImage ||
		jobs[0].CostEstimate != expectedCost ||
		jobs[0].CostReserved != expectedCost {
		t.Fatalf("nano banana 2 should create one reserved image job, jobs=%+v", jobs)
	}
	var params struct {
		Prompt       string `json:"prompt"`
		ModelID      string `json:"model_id"`
		ModelName    string `json:"model_name"`
		Provider     string `json:"provider"`
		ModelCode    string `json:"model_code"`
		Size         string `json:"size"`
		Resolution   string `json:"resolution"`
		ImageQuality string `json:"image_quality"`
	}
	if err := json.Unmarshal(jobs[0].Params, &params); err != nil {
		t.Fatalf("decode job params: %v", err)
	}
	if params.Prompt != "cinematic robot portrait" ||
		params.ModelID != "nano_banana_2" ||
		params.ModelName != "Nano Banana 2" ||
		params.Provider != "poyo" ||
		params.ModelCode != "nano-banana-2" ||
		params.Size != "1:1" ||
		params.Resolution != "4K" ||
		params.ImageQuality != "4K" {
		t.Fatalf("unexpected nano banana 2 job params: %+v", params)
	}
	sent := control.Sent()
	if len(sent) != 2 ||
		!strings.Contains(sent[0].Text, "Nano Banana 2") ||
		!strings.Contains(sent[0].Text, fmt.Sprintf("\u0426\u0435\u043d\u0430: %d", expectedCost)) ||
		sent[1].Text != "НейроХаб рисует..." {
		t.Fatalf("unexpected nano banana 2 responses: %+v", sent)
	}
	for _, legacyPayload := range []string{
		string(domain.CommandMenuImageQuality1K),
		string(domain.CommandMenuImageQuality2K),
		string(domain.CommandMenuImageQuality4K),
	} {
		if strings.Contains(sent[0].Keyboard, legacyPayload) {
			t.Fatalf("quality picker must use generic catalog payloads, found legacy %q in %q", legacyPayload, sent[0].Keyboard)
		}
	}
}

func TestPhotoNanoBanana2PricingCatalogParityForDefaultQuality(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	expectedDisplay := vkTestImageDisplayCredits(t, modelcatalog.MiniAppImageNanoBanana2, modelcatalog.ImageQuality1K)
	expectedCost := vkTestImageCostCredits(t, modelcatalog.MiniAppImageNanoBanana2, modelcatalog.ImageQuality1K)

	menu := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-nano-2-parity-on","secret":"s3cr3t",
		"object":{"message":{"from_id":5636,"peer_id":5636,"text":"Nano Banana 2","payload":"{\"command\":\"menu.image.select\",\"model_id\":\"nano_banana_2\"}"}}
	}`
	if rec := h.post(menu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Keyboard, fmt.Sprintf("1K \u00b7 %d", expectedDisplay)) {
		t.Fatalf("VK quality display did not use pricing catalog: %+v", sent)
	}

	quality := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-nano-2-parity-quality","secret":"s3cr3t",
		"object":{"message":{"from_id":5636,"peer_id":5636,"text":"1K","payload":"{\"command\":\"menu.image.quality.select\",\"model_id\":\"nano_banana_2\",\"image_quality\":\"1K\"}"}}
	}`
	if rec := h.post(quality); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected quality response: %d %q", rec.Code, rec.Body.String())
	}
	sent = control.Sent()
	if len(sent) != 1 ||
		!strings.Contains(sent[0].Text, "Nano Banana 2 \u00b7 1K") ||
		!strings.Contains(sent[0].Text, fmt.Sprintf("\u0426\u0435\u043d\u0430: %d", expectedDisplay)) {
		t.Fatalf("VK prompt display did not use pricing catalog: %+v", sent)
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-nano-2-parity-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5636,"peer_id":5636,"text":"editorial glass object on white background"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	user, err := h.users.GetByVKUserID(context.Background(), 5636)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(context.Background(), user.ID, 10, 0)
	if len(jobs) != 1 ||
		jobs[0].CostEstimate != expectedCost ||
		jobs[0].CostReserved != expectedCost {
		t.Fatalf("VK job did not use pricing catalog cost %d: %+v", expectedCost, jobs)
	}
	if credits, ok := jobs[0].PricingSnapshotCredits(); !ok || credits != expectedCost {
		t.Fatalf("VK job pricing snapshot credits = %d/%v, want %d/true; snapshot=%s", credits, ok, expectedCost, string(jobs[0].PricingSnapshot))
	}
	lowerSnapshot := strings.ToLower(string(jobs[0].PricingSnapshot))
	for _, private := range []string{"prompt", "model_code", "private_url", "nano-banana-2"} {
		if strings.Contains(lowerSnapshot, private) {
			t.Fatalf("pricing snapshot leaked private field %q: %s", private, string(jobs[0].PricingSnapshot))
		}
	}
}

func TestPhotoDeepInfraSeedreamFailsClosedWithoutPricingTariff(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	menu := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-seedream-on","secret":"s3cr3t",
		"object":{"message":{"from_id":5633,"peer_id":5633,"text":"ByteDance Seedream 4.5","payload":"{\"command\":\"menu.image.deepinfra_seedream_4_5\"}"}}
	}`
	if rec := h.post(menu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}
	sent := control.Sent()
	if len(sent) != 1 || strings.Contains(sent[0].Keyboard, "ByteDance Seedream 4.5") {
		t.Fatalf("unpriced Seedream callback must return safe public photo catalog, got %+v", sent)
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-seedream-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5633,"peer_id":5633,"text":"safe editorial product image"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5633)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if hasCommandTypes(cmds, domain.CommandImageGenerate) {
		t.Fatalf("unexpected command types: %+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("unpriced Seedream must not create jobs/tasks, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestDisabledPhotoModeButtonFallsBackToMainMenu(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuFeatures: vk.MenuFeatureFlags{DisabledCommands: map[domain.CommandType]bool{
			domain.CommandMenuImageText: true,
		}},
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-mode","secret":"s3cr3t",
		"object":{"message":{"from_id":563,"peer_id":563,"text":"▶️ Фото по тексту","payload":"{\"command\":\"menu.image.text\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 563)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandShowMenu {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("disabled photo mode selection must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Добро пожаловать в НейроХаб") || strings.Contains(sent[0].Keyboard, "Фото по тексту") {
		t.Fatalf("disabled photo mode should fall back to main menu without photo text button: %+v", sent)
	}
}

func TestDisabledCatalogPhotoModelPayloadFallsBackToMainMenu(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		ImageModels: []productcatalog.ImageModel{
			{Type: "image", ID: modelcatalog.MiniAppImageNanoBanana2, Name: "Nano Banana 2", Enabled: false},
		},
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-disabled-catalog-model","secret":"s3cr3t",
		"object":{"message":{"from_id":5634,"peer_id":5634,"text":"Nano Banana 2","payload":"{\"command\":\"menu.image.select\",\"model_id\":\"nano_banana_2\"}"}}
	}`
	if rec := h.post(body); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5634)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandShowMenu {
		t.Fatalf("disabled public model should fall back to menu, commands=%+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("disabled public model must not create job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestGPTMenuButtonSendsActivePromptNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":564,"peer_id":564,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 564)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuText {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("gpt menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "НейроХаб активен") || !strings.Contains(sent[0].Keyboard, "⬅️ Назад") {
		t.Fatalf("unexpected gpt response: %+v", sent)
	}
}

func TestFirstPlainTextStartsOnboardingNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-first-plain-start","secret":"s3cr3t",
		"object":{"message":{"from_id":5740,"peer_id":5740,"text":"привет"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5740)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandStart {
		t.Fatalf("first plain text should be recorded as onboarding start, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("first onboarding must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("expected persistent keyboard and welcome, got %+v", sent)
	}
	if !strings.Contains(sent[0].Keyboard, "Показать меню") || !strings.Contains(sent[1].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("unexpected onboarding response: %+v", sent)
	}
	if user.WelcomeNameSentAt.IsZero() {
		t.Fatalf("first onboarding should mark welcome as sent: %+v", user)
	}
}

func TestPlainTextOutsideModeRepliesWithChooseModeNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-reply-start","secret":"s3cr3t",
		"object":{"message":{"from_id":574,"peer_id":574,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-reply","secret":"s3cr3t",
		"object":{"message":{"from_id":574,"peer_id":574,"text":"придумай идею"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 574)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandUnknown) {
		t.Fatalf("plain text outside mode should be recorded as unknown, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("plain text outside mode must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 3 {
		t.Fatalf("expected one choose-mode response, got %+v", sent)
	}
	if sent[2].Text != "Выберите режим в меню выше или нажмите на кнопку показать меню" || !strings.Contains(sent[2].Keyboard, "Показать меню") || strings.Contains(sent[2].Keyboard, `"inline":true`) {
		t.Fatalf("unexpected choose-mode response: %+v", sent)
	}
}

func TestPlainTextOutsideModeCanBeSilent(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		UnroutedTextMode:  "silent",
	})
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-silent-start","secret":"s3cr3t",
		"object":{"message":{"from_id":575,"peer_id":575,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-silent","secret":"s3cr3t",
		"object":{"message":{"from_id":575,"peer_id":575,"text":"придумай идею"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 575)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandUnknown) {
		t.Fatalf("plain text outside mode should be recorded as unknown, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("plain text outside mode must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	if sent := control.Sent(); len(sent) != 2 {
		t.Fatalf("silent mode should not send a choose-mode response, got %+v", sent)
	}
}

func TestPlainTextCanKeepLegacyGPTMode(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		UnroutedTextMode:  "gpt",
	})
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-gpt-start","secret":"s3cr3t",
		"object":{"message":{"from_id":576,"peer_id":576,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-gpt","secret":"s3cr3t",
		"object":{"message":{"from_id":576,"peer_id":576,"text":"придумай идею"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 576)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandTextAsk) {
		t.Fatalf("legacy gpt mode should record text.ask, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationTextGenerate || h.pub.Len() != 1 {
		t.Fatalf("legacy gpt mode should create one text job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	if jobs[0].CostEstimate != 0 || jobs[0].CostReserved != 0 {
		t.Fatalf("text job cost/reserved = %d/%d, want 0/0", jobs[0].CostEstimate, jobs[0].CostReserved)
	}
}

func TestGPTMenuButtonEnablesPlainTextJobs(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-mode-on","secret":"s3cr3t",
		"object":{"message":{"from_id":577,"peer_id":577,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := h.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}

	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-mode-text","secret":"s3cr3t",
		"object":{"message":{"from_id":577,"peer_id":577,"text":"придумай идею"}}
	}`
	if rec := h.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 577)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandMenuText, domain.CommandTextAsk) {
		t.Fatalf("unexpected command types: %+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationTextGenerate || h.pub.Len() != 1 {
		t.Fatalf("gpt mode should create one text job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 || !strings.Contains(sent[0].Text, "НейроХаб активен") || sent[1].Text != "НейроХаб думает..." {
		t.Fatalf("unexpected control responses: %+v", sent)
	}
	var params struct {
		Prompt                 string `json:"prompt"`
		VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
	}
	if err := json.Unmarshal(jobs[0].Params, &params); err != nil {
		t.Fatalf("decode job params: %v", err)
	}
	if params.Prompt == "" || params.VKPlaceholderMessageID != sent[1].MessageID {
		t.Fatalf("unexpected job params: %+v, pending message=%+v", params, sent[1])
	}
}

func TestPersistedGPTModeSurvivesHandlerRestart(t *testing.T) {
	dialogState := newFakeDialogState()
	firstControl := vkdelivery.NewMockClient()
	first := newHarnessWithConfigAndDialogState(firstControl, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
	}, dialogState)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-persist-on","secret":"s3cr3t",
        "object":{"message":{"from_id":590,"peer_id":590,"text":"\ud83d\udcac \u0421\u043f\u0440\u043e\u0441\u0438\u0442\u044c \u0443 \u041d\u0435\u0439\u0440\u043e\u0425\u0430\u0431","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := first.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}
	if mode, ok := dialogState.modes[590]; !ok || mode != "gpt" {
		t.Fatalf("dialog mode was not persisted: %#v", dialogState.modes)
	}

	// New handler/harness simulates an API process restart: its in-memory mode
	// map is empty, but Redis-backed dialog state is still available.
	secondControl := vkdelivery.NewMockClient()
	second := newHarnessWithConfigAndDialogState(secondControl, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
	}, dialogState)
	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-persist-text","secret":"s3cr3t",
		"object":{"message":{"from_id":590,"peer_id":590,"text":"РїСЂРёРґСѓРјР°Р№ РёРґРµСЋ"}}
	}`
	if rec := second.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := second.users.GetByVKUserID(ctx, 590)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := second.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandTextAsk {
		t.Fatalf("persisted mode should route text to GPT, got %+v", cmds)
	}
	jobs, _ := second.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationTextGenerate || second.pub.Len() != 1 {
		t.Fatalf("persisted mode should create one text job, jobs=%+v tasks=%d", jobs, second.pub.Len())
	}
	sent := secondControl.Sent()
	if len(sent) != 1 || sent[0].Text != "НейроХаб думает..." {
		t.Fatalf("unexpected persisted-mode response: %+v", sent)
	}
}

func TestStaleBackCallbackClearsPersistedGPTModeAfterRestart(t *testing.T) {
	dialogState := newFakeDialogState()
	dialogState.modes[591] = "gpt"
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfigAndDialogState(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
	}, dialogState)
	back := `{
		"type":"message_event","group_id":1,"event_id":"evt-gpt-persist-back","secret":"s3cr3t",
		"object":{"user_id":591,"peer_id":591,"event_id":"evt-gpt-persist-back-inner","payload":{"command":"show_menu"}}
	}`
	if rec := h.post(back); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected back response: %d %q", rec.Code, rec.Body.String())
	}
	if _, ok := dialogState.modes[591]; ok {
		t.Fatalf("stale back callback should clear persisted mode: %#v", dialogState.modes)
	}
	if len(control.EventAnswers()) != 1 {
		t.Fatalf("callback should be acknowledged, got %+v", control.EventAnswers())
	}
	if sent := control.Sent(); len(sent) != 0 {
		t.Fatalf("stale back callback should not send a fresh menu, got %+v", sent)
	}
}

func TestStaleCallbackShowMenuAfterGPTTextDoesNotSendMenu(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-stale-show-on","secret":"s3cr3t",
		"object":{"message":{"from_id":580,"peer_id":580,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := h.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}

	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-stale-show-text","secret":"s3cr3t",
		"object":{"message":{"from_id":580,"peer_id":580,"text":"кто такой сантиз?"}}
	}`
	if rec := h.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}

	stale := `{
		"type":"message_event","group_id":1,"event_id":"evt-gpt-stale-show-callback","secret":"s3cr3t",
		"object":{"user_id":580,"peer_id":580,"event_id":"vk-button-event-stale-show","payload":{"command":"show_menu"}}
	}`
	if rec := h.post(stale); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected stale callback response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 580)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandMenuText, domain.CommandTextAsk, domain.CommandShowMenu) {
		t.Fatalf("unexpected command types: %+v", commandTypes(cmds))
	}
	sent := control.Sent()
	if len(sent) != 2 || !strings.Contains(sent[0].Text, "НейроХаб активен") || sent[1].Text != "НейроХаб думает..." {
		t.Fatalf("stale show_menu callback must not send a fresh menu, got %+v", sent)
	}
	answers := control.EventAnswers()
	if len(answers) != 1 || answers[0].EventID != "vk-button-event-stale-show" {
		t.Fatalf("expected stale callback acknowledgement, got %+v", answers)
	}
}

func TestExpiredActiveMenuCallbackFallsBackAsStale(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken:      "conf-token-123",
		Secret:                 "s3cr3t",
		LocalUIStateTTL:        time.Nanosecond,
		LocalUIStateMaxEntries: 4,
	})
	videoMenu := `{
		"type":"message_new","group_id":1,"event_id":"evt-expired-active-menu-open","secret":"s3cr3t",
		"object":{"message":{"from_id":592,"peer_id":592,"text":"🎬 Создать видео","payload":"{\"command\":\"menu.video\"}"}}
	}`
	if rec := h.post(videoMenu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}
	if sent := control.Sent(); len(sent) != 1 {
		t.Fatalf("expected initial menu send, got %+v", sent)
	}
	time.Sleep(time.Millisecond)

	back := `{
		"type":"message_event","group_id":1,"event_id":"evt-expired-active-menu-back","secret":"s3cr3t",
		"object":{"user_id":592,"peer_id":592,"event_id":"vk-button-event-expired-active-menu","payload":{"command":"show_menu"}}
	}`
	if rec := h.post(back); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected stale callback response: %d %q", rec.Code, rec.Body.String())
	}
	if sent := control.Sent(); len(sent) != 1 {
		t.Fatalf("expired active menu callback should not send a fresh menu, got %+v", sent)
	}
	if edits := control.Edits(); len(edits) != 0 {
		t.Fatalf("expired active menu callback should not edit old menu, got %+v", edits)
	}
	answers := control.EventAnswers()
	if len(answers) != 1 || answers[0].EventID != "vk-button-event-expired-active-menu" {
		t.Fatalf("expected callback acknowledgement, got %+v", answers)
	}
}

func TestOtherMenuButtonClearsGPTMode(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-mode-clear-on","secret":"s3cr3t",
		"object":{"message":{"from_id":578,"peer_id":578,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := h.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}
	video := `{
		"type":"message_event","group_id":1,"event_id":"evt-gpt-mode-clear-video","secret":"s3cr3t",
		"object":{"user_id":578,"peer_id":578,"event_id":"vk-button-event-clear","payload":{"command":"menu.video"}}
	}`
	if rec := h.post(video); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected video response: %d %q", rec.Code, rec.Body.String())
	}
	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-mode-clear-text","secret":"s3cr3t",
		"object":{"message":{"from_id":578,"peer_id":578,"text":"придумай идею"}}
	}`
	if rec := h.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 578)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandMenuText, domain.CommandMenuVideo, domain.CommandUnknown) {
		t.Fatalf("unexpected command types after gpt mode clear: %+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("plain text after mode clear must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 || !strings.Contains(sent[1].Text, "Выберите режим") {
		t.Fatalf("expected choose-mode response after mode clear, got %+v", sent)
	}
	if !strings.Contains(sent[1].Keyboard, "Показать меню") || strings.Contains(sent[1].Keyboard, `"inline":true`) {
		t.Fatalf("choose-mode response after mode clear must restore only lower keyboard: %+v", sent[1])
	}
	if answers := control.EventAnswers(); len(answers) != 1 || answers[0].EventID != "vk-button-event-clear" {
		t.Fatalf("expected callback event answer, got %+v", answers)
	}
}

func TestStickerInGPTModeCreatesTextJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-sticker-gpt-on","secret":"s3cr3t",
		"object":{"message":{"from_id":579,"peer_id":579,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := h.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}
	sticker := `{
		"type":"message_new","group_id":1,"event_id":"evt-sticker-gpt-text","secret":"s3cr3t",
		"object":{"message":{
			"from_id":579,"peer_id":579,"text":"",
			"attachments":[{"type":"sticker","sticker":{"sticker_id":123,"product_id":456,"emoji":"🙂"}}]
		}}
	}`
	if rec := h.post(sticker); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected sticker response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 579)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	textCmd, ok := commandByType(cmds, domain.CommandTextAsk)
	if !hasCommandTypes(cmds, domain.CommandMenuText, domain.CommandTextAsk) || !ok || !strings.Contains(textCmd.RawText, "sticker_id=123") {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationTextGenerate || h.pub.Len() != 1 {
		t.Fatalf("sticker in gpt mode should create one text job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestStudentsMenuButtonSendsStudyMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-students-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":565,"peer_id":565,"text":"🎁 Студентам и школьникам","payload":"{\"command\":\"menu.students\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 565)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuStudents {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("students menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one students menu response, got %+v", sent)
	}
	for _, want := range []string{
		"Данные нейронные сети помогут вам во время учебы",
		"Решальник задач",
		"Генерация презентаций (скоро)",
		"Создание рефератов (скоро)",
		"Ответы на вопросы",
		"⬅️ Назад",
	} {
		if !strings.Contains(sent[0].Text+sent[0].Keyboard, want) {
			t.Fatalf("expected %q in students response: text=%q keyboard=%q", want, sent[0].Text, sent[0].Keyboard)
		}
	}
}

func TestStudentsScenarioButtonIsControlCommandNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-students-solver","secret":"s3cr3t",
		"object":{"message":{"from_id":566,"peer_id":566,"text":"Решальник задач","payload":"{\"command\":\"menu.students.solver\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 566)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuStudentSolver {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("students scenario must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Решальник задач активен") || !strings.Contains(sent[0].Keyboard, "Ответы на вопросы") {
		t.Fatalf("unexpected students scenario response: %+v", sent)
	}
}

func TestDisabledVideoModelPayloadFallsBackToCurrentMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-model","secret":"s3cr3t",
		"object":{"message":{"from_id":561,"peer_id":561,"text":"Sora 2 — видео текст+фото","payload":"{\"command\":\"menu.video.sora_2\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 561)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandShowMenu {
		t.Fatalf("disabled video model payload should be recorded as show_menu fallback, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("disabled video model payload must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("expected current welcome menu fallback, got %+v", sent)
	}
	for _, hidden := range []string{"Sora 2 — видео текст+фото", "menu.video.sora_2.start", "menu.video.sora_2.examples"} {
		if strings.Contains(sent[0].Keyboard, hidden) {
			t.Fatalf("disabled video nested button should stay hidden: %q", sent[0].Keyboard)
		}
	}
}

func TestDisabledVideoStartPayloadDoesNotEnablePlainTextVideoJobs(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-start-on","secret":"s3cr3t",
		"object":{"message":{"from_id":5621,"peer_id":5621,"text":"😀 Начать генерацию","payload":"{\"command\":\"menu.video.sora_2.start\"}"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}

	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5621,"peer_id":5621,"text":"cinematic neon city at night, rain reflections, slow drone movement"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5621)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if hasCommandTypes(cmds, domain.CommandMenuVideoSora2Start, domain.CommandVideoGenerate) {
		t.Fatalf("disabled video start payload must not create video commands, got %+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("disabled video start payload must not create video jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 || strings.Contains(sent[0].Text, "Runway Gen-4 Turbo активен") || sent[1].Text == "НейроХаб готовит видео..." {
		t.Fatalf("disabled video start payload must fall back without pending video message, got %+v", sent)
	}
}

func TestPersistedVideoModeSurvivesHandlerRestart(t *testing.T) {
	dialogState := newFakeDialogState()
	dialogState.modes[5931] = "video:kling_o3_standard"

	secondControl := vkdelivery.NewMockClient()
	second := newHarnessWithConfigAndDialogState(secondControl, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuFeatures: enabledVideoCommands(
			domain.CommandMenuVideoKling21,
			domain.CommandMenuVideoKling21Start,
			domain.CommandMenuVideoKling21Examples,
		),
	}, dialogState)
	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-persist-prompt","secret":"s3cr3t",
		"object":{"message":{"from_id":5931,"peer_id":5931,"text":"cinematic ocean waves at sunset"}}
	}`
	if rec := second.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected persisted prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := second.users.GetByVKUserID(ctx, 5931)
	if err != nil {
		t.Fatalf("user not created after restart: %v", err)
	}
	jobs, _ := second.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationVideoGenerate || jobs[0].Modality != domain.ModalityVideo || second.pub.Len() != 1 {
		t.Fatalf("persisted video mode should create one video job, jobs=%+v tasks=%d", jobs, second.pub.Len())
	}
	var params struct {
		VideoRouteAlias string `json:"video_route_alias"`
		Provider        string `json:"provider"`
		ModelCode       string `json:"model_code"`
	}
	if err := json.Unmarshal(jobs[0].Params, &params); err != nil {
		t.Fatalf("decode job params: %v", err)
	}
	if params.VideoRouteAlias != string(domain.VideoRouteKlingO3Standard) || params.Provider != "" || params.ModelCode != "" {
		t.Fatalf("persisted video mode should use public route alias only, got %+v", params)
	}
}

func TestUnsupportedPersistedVideoModeDoesNotCreateJob(t *testing.T) {
	dialogState := newFakeDialogState()
	dialogState.modes[5932] = "video:kling_v2_1"

	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfigAndDialogState(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
	}, dialogState)
	prompt := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-persist-unsupported","secret":"s3cr3t",
		"object":{"message":{"from_id":5932,"peer_id":5932,"text":"cinematic ocean waves at sunset"}}
	}`
	if rec := h.post(prompt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected persisted prompt response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5932)
	if err != nil {
		t.Fatalf("user not created after restart: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("unsupported persisted video mode must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestDisabledNestedVideoPayloadFallsBackToCurrentMenu(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-nested-flag","secret":"s3cr3t",
		"object":{"message":{"from_id":582,"peer_id":582,"text":"Sora 2 — видео текст+фото","payload":"{\"command\":\"menu.video.sora_2\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one fallback response, got %+v", sent)
	}
	if !strings.Contains(sent[0].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("expected current welcome menu fallback, got %+v", sent)
	}
	for _, hidden := range []string{"Начать генерацию", "Примеры", "menu.video.sora_2.start", "menu.video.sora_2.examples"} {
		if strings.Contains(sent[0].Keyboard, hidden) {
			t.Fatalf("disabled nested video button should stay hidden: %q", sent[0].Keyboard)
		}
	}
}

func TestVideoNestedButtonsAreControlCommandsNoJob(t *testing.T) {
	tests := []struct {
		name     string
		eventID  string
		text     string
		command  domain.CommandType
		wantText string
		wantKeys []string
	}{
		{
			name:     "runway examples",
			eventID:  "evt-video-runway-examples",
			text:     "ℹ️ Примеры",
			command:  domain.CommandMenuVideoSora2Examples,
			wantText: "Runway Gen-4 Turbo",
			wantKeys: []string{"⬅️ Назад", "menu.video.sora_2"},
		},
		{
			name:     "runway start",
			eventID:  "evt-video-runway-start",
			text:     "😀 Начать генерацию",
			command:  domain.CommandMenuVideoSora2Start,
			wantText: "Runway Gen-4 Turbo",
			wantKeys: []string{"⬅️ Назад", "menu.video.sora_2"},
		},
		{
			name:     "seedance picker",
			eventID:  "evt-video-seedance",
			text:     "Seedance 1 — видео по тексту",
			command:  domain.CommandMenuVideoSeedance1,
			wantText: "Seedance",
			wantKeys: []string{"Seedance 1 Lite", "Seedance 1 Pro", "⬅️ Назад"},
		},
		{
			name:     "seedance lite",
			eventID:  "evt-video-seedance-lite",
			text:     "Seedance 1 Lite",
			command:  domain.CommandMenuVideoSeedance1Lite,
			wantText: "Seedance 1 Lite активен",
			wantKeys: []string{"⬅️ Назад", "menu.video.seedance_1"},
		},
		{
			name:     "hailuo picker",
			eventID:  "evt-video-hailuo",
			text:     "Hailuo v0.2 — видео текст+фото",
			command:  domain.CommandMenuVideoHailuo02,
			wantText: "Hailuo 02",
			wantKeys: []string{"Hailuo v0.2 Обычный", "Hailuo v0.2 Fast", "⬅️ Назад"},
		},
		{
			name:     "hailuo fast",
			eventID:  "evt-video-hailuo-fast",
			text:     "Hailuo v0.2 Fast",
			command:  domain.CommandMenuVideoHailuo02Fast,
			wantText: "Hailuo v0.2 Fast активен",
			wantKeys: []string{"⬅️ Назад", "menu.video.hailuo_v0_2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			control := vkdelivery.NewMockClient()
			h := newHarnessWithControl(control)
			body := fmt.Sprintf(`{
				"type":"message_new","group_id":1,"event_id":%q,"secret":"s3cr3t",
				"object":{"message":{"from_id":561,"peer_id":561,"text":%q,"payload":"{\"command\":\"%s\"}"}}
			}`, tt.eventID, tt.text, tt.command)
			rec := h.post(body)
			if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
				t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
			}

			ctx := context.Background()
			user, err := h.users.GetByVKUserID(ctx, 561)
			if err != nil {
				t.Fatalf("user not created: %v", err)
			}
			cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
			if len(cmds) != 1 || cmds[0].Type != domain.CommandShowMenu {
				t.Fatalf("disabled nested video payload should be recorded as show_menu fallback, got %+v", cmds)
			}
			jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
			if len(jobs) != 0 {
				t.Fatalf("nested video control must not create a job, got %d", len(jobs))
			}
			sent := control.Sent()
			if len(sent) != 1 || !strings.Contains(sent[0].Text, "Добро пожаловать в НейроХаб") {
				t.Fatalf("expected current welcome menu fallback, got %+v", sent)
			}
			for _, hidden := range []string{"Начать генерацию", "Примеры", "Seedance 1 Lite", "Seedance 1 Pro", "Hailuo v0.2 Обычный", "Hailuo v0.2 Fast"} {
				if strings.Contains(sent[0].Keyboard, hidden) {
					t.Fatalf("disabled nested video button should stay hidden: %q", sent[0].Keyboard)
				}
			}
		})
	}
}

func TestStartFallsBackWhenVKKeyboardDisabled(t *testing.T) {
	control := &keyboardFailControl{}
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-fallback","secret":"s3cr3t",
		"object":{"message":{"from_id":558,"peer_id":558,"text":"/start"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}
	if len(control.sent) != 3 {
		t.Fatalf("expected keyboard attempt and fallback send, got %+v", control.sent)
	}
	if control.sent[0].Keyboard == nil {
		t.Fatalf("first send must include persistent keyboard")
	}
	if control.sent[1].Keyboard == nil {
		t.Fatalf("second send must include inline keyboard")
	}
	if control.sent[2].Keyboard != nil {
		t.Fatalf("fallback send must omit keyboard")
	}
}

func TestMenuEditKeyboardFailureSendsNewMessageWithKeyboard(t *testing.T) {
	control := &editKeyboardFailControl{}
	h := newHarnessWithControl(control)
	startBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-edit-keyboard-start","secret":"s3cr3t",
		"object":{"message":{"from_id":559,"peer_id":559,"text":"/start"}}
	}`
	if rec := h.post(startBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	initialSends := len(control.sent)
	if initialSends == 0 {
		t.Fatalf("expected start to send menu messages")
	}

	topUpBody := `{
		"type":"message_new","group_id":1,"event_id":"evt-edit-keyboard-topup","secret":"s3cr3t",
		"object":{"message":{"from_id":559,"peer_id":559,"text":"topup","payload":"{\"command\":\"top_up\"}"}}
	}`
	if rec := h.post(topUpBody); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected top-up response: %d %q", rec.Code, rec.Body.String())
	}
	if len(control.edited) != 1 || control.edited[0].Keyboard == nil {
		t.Fatalf("expected one failed edit attempt with keyboard, got %+v", control.edited)
	}
	if len(control.sent) != initialSends+1 {
		t.Fatalf("expected a new control message after failed edit, sent=%+v edited=%+v", control.sent, control.edited)
	}
	got := control.sent[len(control.sent)-1]
	keyboardJSON, _ := json.Marshal(got.Keyboard)
	if got.Keyboard == nil || !strings.Contains(string(keyboardJSON), "crystals_99") {
		t.Fatalf("new top-up message must keep product keyboard, got %+v", got)
	}
}

func TestMessageNewDuplicateIsDeduped(t *testing.T) {
	h := newHarness()
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-dup","secret":"s3cr3t",
		"object":{"message":{"from_id":777,"peer_id":777,"text":"/video sunrise"}}
	}`
	if rec := h.post(body); rec.Code != http.StatusOK {
		t.Fatalf("first delivery status = %d", rec.Code)
	}
	if rec := h.post(body); rec.Code != http.StatusOK {
		t.Fatalf("second delivery status = %d", rec.Code)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 777)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 {
		t.Fatalf("expected exactly 1 job after duplicate delivery, got %d", len(jobs))
	}
	if h.pub.Len() != 1 {
		t.Fatalf("expected exactly 1 enqueued task, got %d", h.pub.Len())
	}
}

func TestMessageNewFallbackEventIDUsesConversationMessageID(t *testing.T) {
	h := newHarness()
	body := func(conversationMessageID int64) string {
		return fmt.Sprintf(`{
		"type":"message_new","group_id":1,"secret":"s3cr3t",
		"object":{"message":{"from_id":777,"peer_id":777,"conversation_message_id":%d,"text":"/video sunrise"}}
	}`, conversationMessageID)
	}
	if rec := h.post(body(101)); rec.Code != http.StatusOK {
		t.Fatalf("first delivery status = %d body=%q", rec.Code, rec.Body.String())
	}
	if rec := h.post(body(102)); rec.Code != http.StatusOK {
		t.Fatalf("second delivery status = %d body=%q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	if _, err := h.inbound.GetByIdempotencyKey(ctx, "vk_event:1:conversation_message:777:777:101"); err != nil {
		t.Fatalf("first synthetic event id was not based on conversation_message_id: %v", err)
	}
	if _, err := h.inbound.GetByIdempotencyKey(ctx, "vk_event:1:conversation_message:777:777:102"); err != nil {
		t.Fatalf("second synthetic event id was not based on conversation_message_id: %v", err)
	}
	user, err := h.users.GetByVKUserID(ctx, 777)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 2 || h.pub.Len() != 2 {
		t.Fatalf("same text with different conversation_message_id should create two jobs, jobs=%d tasks=%d", len(jobs), h.pub.Len())
	}
}

func TestMessageNewReusesExistingInboundOnRetry(t *testing.T) {
	h := newHarness()
	ctx := context.Background()
	if err := h.inbound.Create(ctx, &domain.InboundEvent{
		Source:         "vk",
		EventType:      "message_new",
		GroupID:        1,
		VKEventID:      "evt-inbound-retry",
		PeerID:         778,
		VKUserID:       778,
		Payload:        json.RawMessage(`{}`),
		Status:         domain.InboundReceived,
		IdempotencyKey: "vk_event:1:evt-inbound-retry",
	}); err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-inbound-retry","secret":"s3cr3t",
		"object":{"message":{"from_id":778,"peer_id":778,"text":"/image retry cat"}}
	}`
	if rec := h.post(body); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("retry delivery status = %d body=%q", rec.Code, rec.Body.String())
	}

	inbound, err := h.inbound.GetByIdempotencyKey(ctx, "vk_event:1:evt-inbound-retry")
	if err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if inbound.Status != domain.InboundProcessed {
		t.Fatalf("inbound status = %q, want processed", inbound.Status)
	}
	user, err := h.users.GetByVKUserID(ctx, 778)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationImageGenerate || h.pub.Len() != 1 {
		t.Fatalf("retry delivery should create one image job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

type keyboardFailControl struct {
	sent []vkdelivery.Message
}

func (c *keyboardFailControl) SendMessage(_ context.Context, _ int64, _ int64, msg vkdelivery.Message) (vkdelivery.SendResult, error) {
	c.sent = append(c.sent, msg)
	if msg.Keyboard != nil {
		return vkdelivery.SendResult{}, &vkdelivery.APIError{Code: 912, Message: "Chat bot feature"}
	}
	return vkdelivery.SendResult{MessageID: int64(len(c.sent)), PeerID: 558}, nil
}

func (c *keyboardFailControl) EditMessage(_ context.Context, _ int64, _ int64, msg vkdelivery.Message) (vkdelivery.SendResult, error) {
	c.sent = append(c.sent, msg)
	if msg.Keyboard != nil {
		return vkdelivery.SendResult{}, &vkdelivery.APIError{Code: 912, Message: "Chat bot feature"}
	}
	return vkdelivery.SendResult{MessageID: int64(len(c.sent)), PeerID: 558}, nil
}

func (c *keyboardFailControl) AnswerMessageEvent(_ context.Context, _ string, _, _ int64) error {
	return nil
}

type editKeyboardFailControl struct {
	sent   []vkdelivery.Message
	edited []vkdelivery.Message
	nextID int64
}

func (c *editKeyboardFailControl) SendMessage(_ context.Context, peerID, _ int64, msg vkdelivery.Message) (vkdelivery.SendResult, error) {
	c.sent = append(c.sent, msg)
	c.nextID++
	return vkdelivery.SendResult{MessageID: c.nextID, PeerID: peerID}, nil
}

func (c *editKeyboardFailControl) EditMessage(_ context.Context, peerID, messageID int64, msg vkdelivery.Message) (vkdelivery.SendResult, error) {
	c.edited = append(c.edited, msg)
	if msg.Keyboard != nil {
		return vkdelivery.SendResult{}, &vkdelivery.APIError{Code: 912, Message: "Chat bot feature"}
	}
	return vkdelivery.SendResult{MessageID: messageID, PeerID: peerID}, nil
}

func (c *editKeyboardFailControl) AnswerMessageEvent(_ context.Context, _ string, _, _ int64) error {
	return nil
}

func commandTypes(cmds []*domain.Command) []domain.CommandType {
	types := make([]domain.CommandType, 0, len(cmds))
	for _, cmd := range cmds {
		types = append(types, cmd.Type)
	}
	return types
}

func hasCommandTypes(cmds []*domain.Command, want ...domain.CommandType) bool {
	if len(cmds) != len(want) {
		return false
	}
	counts := map[domain.CommandType]int{}
	for _, cmd := range cmds {
		counts[cmd.Type]++
	}
	for _, t := range want {
		if counts[t] == 0 {
			return false
		}
		counts[t]--
	}
	return true
}

func commandByType(cmds []*domain.Command, t domain.CommandType) (*domain.Command, bool) {
	for _, cmd := range cmds {
		if cmd.Type == t {
			return cmd, true
		}
	}
	return nil, false
}

type fakeReferenceDownloader struct {
	data        []byte
	contentType string
	err         error
	urls        []string
}

func (f *fakeReferenceDownloader) Download(_ context.Context, url string) ([]byte, string, error) {
	f.urls = append(f.urls, url)
	if f.err != nil {
		return nil, "", f.err
	}
	data := append([]byte(nil), f.data...)
	contentType := f.contentType
	if contentType == "" {
		contentType = "image/png"
	}
	return data, contentType, nil
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

type fakeAntiSpam struct {
	decision antispamservice.Decision
	err      error
	inputs   []antispamservice.CheckInput
}

func (f *fakeAntiSpam) Check(_ context.Context, input antispamservice.CheckInput) (antispamservice.Decision, error) {
	f.inputs = append(f.inputs, input)
	return f.decision, f.err
}

type fakeDialogState struct {
	modes map[int64]string
}

func newFakeDialogState() *fakeDialogState {
	return &fakeDialogState{modes: map[int64]string{}}
}

func (f *fakeDialogState) Get(_ context.Context, peerID int64) (string, bool, error) {
	mode, ok := f.modes[peerID]
	return mode, ok, nil
}

func (f *fakeDialogState) Set(_ context.Context, peerID int64, mode string) error {
	f.modes[peerID] = mode
	return nil
}

func (f *fakeDialogState) Clear(_ context.Context, peerID int64) error {
	delete(f.modes, peerID)
	return nil
}

func TestMessageNewControlCommandNoJob(t *testing.T) {
	h := newHarness()
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-bal","secret":"s3cr3t",
		"object":{"message":{"from_id":888,"peer_id":888,"text":"/balance"}}
	}`
	if rec := h.post(body); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 888)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("control command must not create a job, got %d", len(jobs))
	}
	if h.pub.Len() != 0 {
		t.Fatalf("expected no enqueued tasks, got %d", h.pub.Len())
	}
}
