package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/resultservice"
	"vk-ai-aggregator/internal/worker"
)

type deliveryHarness struct {
	jobs       *memory.JobRepo
	deliveries *memory.DeliveryRepo
	artifacts  *memory.ArtifactRepo
	moderation *memory.ModerationRepo
	objects    *memory.ObjectStore
	vk         *vkdelivery.MockClient
	uploader   *fakeVKUploader
	billingRpo *memory.BillingRepo
	billing    *billingservice.Service
	worker     *worker.DeliveryWorker
}

func newDeliveryHarness(t *testing.T) *deliveryHarness {
	t.Helper()
	jobs := memory.NewJobRepo()
	deliveries := memory.NewDeliveryRepo()
	artifacts := memory.NewArtifactRepo()
	moderation := memory.NewModerationRepo()
	objects := memory.NewObjectStore()
	vk := vkdelivery.NewMockClient()
	uploader := &fakeVKUploader{}
	billingRpo := memory.NewBillingRepo()
	billing := billingservice.New(billingRpo, billingservice.WithStartingBalance(1000))
	harness := &deliveryHarness{
		jobs:       jobs,
		deliveries: deliveries,
		artifacts:  artifacts,
		moderation: moderation,
		objects:    objects,
		vk:         vk,
		uploader:   uploader,
		billingRpo: billingRpo,
		billing:    billing,
	}
	harness.worker = harness.deliveryWorker(vkdelivery.PublisherDeps{Uploader: uploader}, worker.DeliveryDeps{})
	return harness
}

func (h *deliveryHarness) deliveryWorker(publisherDeps vkdelivery.PublisherDeps, workerDeps worker.DeliveryDeps) *worker.DeliveryWorker {
	publisherDeps.Deliveries = h.deliveries
	publisherDeps.Artifacts = h.artifacts
	publisherDeps.Objects = h.objects
	if publisherDeps.Client == nil {
		publisherDeps.Client = h.vk
	}
	workerDeps.Jobs = h.jobs
	workerDeps.Deliveries = h.deliveries
	workerDeps.Artifacts = h.artifacts
	workerDeps.Publishers = []worker.ExternalPublisher{vkdelivery.NewPublisher(publisherDeps)}
	if workerDeps.Billing == nil {
		workerDeps.Billing = h.billing
	}
	if workerDeps.Readiness == nil {
		workerDeps.Readiness = resultservice.New(h.jobs, h.artifacts, h.moderation)
	}
	return worker.NewDeliveryWorker(workerDeps)
}

type fakeVKUploader struct {
	photoBytes    []byte
	photoFilename string
	videoBytes    []byte
	videoFilename string
	err           error
}

func (u *fakeVKUploader) UploadPhoto(_ context.Context, peerID int64, filename string, data []byte, _ string) (string, error) {
	if u.err != nil {
		return "", u.err
	}
	u.photoBytes = append([]byte(nil), data...)
	u.photoFilename = filename
	return "photo123_456_key", nil
}

func (u *fakeVKUploader) UploadVideo(_ context.Context, peerID int64, filename string, data []byte, _ string) (string, error) {
	if u.err != nil {
		return "", u.err
	}
	u.videoBytes = append([]byte(nil), data...)
	u.videoFilename = filename
	return "video123_456_key", nil
}

type fakeURLSigner struct {
	err error
}

func (s fakeURLSigner) PresignedGetURL(context.Context, string, string, time.Duration) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return "https://signed-delivery.example/artifact", nil
}

type failOnceDeliveryBiller struct {
	inner        worker.DeliveryBiller
	captureCalls int
}

func (b *failOnceDeliveryBiller) CaptureForJob(ctx context.Context, jobID uuid.UUID, amount int64) error {
	b.captureCalls++
	if b.captureCalls == 1 {
		return errors.New("capture unavailable")
	}
	return b.inner.CaptureForJob(ctx, jobID, amount)
}

func (b *failOnceDeliveryBiller) ReleaseForJob(ctx context.Context, jobID uuid.UUID) error {
	return b.inner.ReleaseForJob(ctx, jobID)
}

type completionReadinessCall struct {
	accountID uuid.UUID
	jobID     uuid.UUID
}

type recordingCompletionReadiness struct {
	inner worker.CompletionReadiness
	err   error
	calls []completionReadinessCall
}

func (r *recordingCompletionReadiness) RequireCompletionReady(ctx context.Context, accountID, jobID uuid.UUID) error {
	r.calls = append(r.calls, completionReadinessCall{accountID: accountID, jobID: jobID})
	if r.err != nil {
		return r.err
	}
	return r.inner.RequireCompletionReady(ctx, accountID, jobID)
}

type failFailedDeliveryUpdateRepo struct {
	domain.DeliveryRepository
	err    error
	failed bool
}

func (r *failFailedDeliveryUpdateRepo) Update(ctx context.Context, delivery *domain.Delivery) error {
	if delivery.Status == domain.DeliveryStatusFailed && !r.failed {
		r.failed = true
		return r.err
	}
	return r.DeliveryRepository.Update(ctx, delivery)
}

type failReleaseOnceDeliveryBiller struct {
	inner        worker.DeliveryBiller
	err          error
	releaseCalls int
}

func (b *failReleaseOnceDeliveryBiller) CaptureForJob(ctx context.Context, jobID uuid.UUID, amount int64) error {
	return b.inner.CaptureForJob(ctx, jobID, amount)
}

func (b *failReleaseOnceDeliveryBiller) ReleaseForJob(ctx context.Context, jobID uuid.UUID) error {
	b.releaseCalls++
	if b.releaseCalls == 1 {
		return b.err
	}
	return b.inner.ReleaseForJob(ctx, jobID)
}

type failTerminalStatusOnceJobRepo struct {
	domain.JobRepository
	err      error
	attempts int
}

func (r *failTerminalStatusOnceJobRepo) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	from, to domain.JobStatus,
	errCode, errMessage string,
) error {
	if to == domain.JobStatusFailedTerminal {
		r.attempts++
		if r.attempts == 1 {
			return r.err
		}
	}
	return r.JobRepository.UpdateStatus(ctx, id, from, to, errCode, errMessage)
}

// resultReadyJob creates a user account, reserves credits, stores an output
// artifact and a job in result_ready, returning the job.
func (h *deliveryHarness) resultReadyJob(t *testing.T, mediaType domain.MediaType, body string) *domain.Job {
	return h.resultReadyJobWithCost(t, mediaType, body, 10, nil)
}

func (h *deliveryHarness) resultReadyJobWithCost(t *testing.T, mediaType domain.MediaType, body string, cost int64, pricingSnapshot json.RawMessage) *domain.Job {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	if _, err := h.billing.EnsureAccount(ctx, userID); err != nil {
		t.Fatalf("ensure account: %v", err)
	}

	job := &domain.Job{
		ID:              uuid.New(),
		UserID:          userID,
		AccountID:       userID,
		Source:          "vk",
		VKPeerID:        555,
		ResultMode:      domain.ResultModeExternalPush,
		DeliveryTarget:  &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		OperationType:   domain.OperationImageGenerate,
		Modality:        domain.ModalityImage,
		Status:          domain.JobStatusResultReady,
		IdempotencyKey:  "job:" + uuid.NewString(),
		PricingSnapshot: pricingSnapshot,
		CostReserved:    cost,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if _, err := h.billing.Reserve(ctx, userID, job.ID, cost); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	art := &domain.Artifact{
		ID:             uuid.New(),
		OwnerUserID:    userID,
		OwnerAccountID: userID,
		JobID:          &job.ID,
		Kind:           domain.ArtifactKindOutput,
		MediaType:      mediaType,
		StorageBucket:  "artifacts",
		StorageKey:     "k/" + job.ID.String(),
		SHA256:         uuid.NewString(),
		Status:         domain.ArtifactStatusReady,
	}
	if err := h.artifacts.Create(ctx, art); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	_ = h.objects.Put(ctx, art.StorageBucket, art.StorageKey, []byte(body), "text/plain")
	job.OutputArtifactIDs = []uuid.UUID{art.ID}
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("attach artifact: %v", err)
	}
	artifactID := art.ID
	if err := h.moderation.Create(ctx, &domain.ModerationResult{
		JobID:      job.ID,
		ArtifactID: &artifactID,
		Stage:      domain.ModerationStageOutput,
		Decision:   domain.ModerationAllow,
		Provider:   "test",
	}); err != nil {
		t.Fatalf("create output moderation verdict: %v", err)
	}
	return job
}

func (h *deliveryHarness) addVideoVariant(t *testing.T, job *domain.Job, variantType domain.VariantType, body string) *domain.ArtifactVariant {
	t.Helper()
	ctx := context.Background()
	art, err := h.artifacts.GetByID(ctx, job.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	variant := &domain.ArtifactVariant{
		ArtifactID:    art.ID,
		VariantType:   variantType,
		StorageBucket: "artifacts",
		StorageKey:    "variants/" + string(variantType) + "/" + art.ID.String() + ".mp4",
		MimeType:      "video/mp4",
		SizeBytes:     int64(len(body)),
		Codec:         "h264",
		Container:     "mp4",
		ProbeStatus:   domain.MediaProbePassed,
	}
	if err := h.artifacts.AddVariant(ctx, variant); err != nil {
		t.Fatalf("add variant: %v", err)
	}
	if err := h.objects.Put(ctx, variant.StorageBucket, variant.StorageKey, []byte(body), variant.MimeType); err != nil {
		t.Fatalf("put variant bytes: %v", err)
	}
	return variant
}

func (h *deliveryHarness) markVideoOriginal(t *testing.T, job *domain.Job, metadata domain.ArtifactMediaMetadata) {
	t.Helper()
	ctx := context.Background()
	art, err := h.artifacts.GetByID(ctx, job.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	art.MimeType = "video/mp4"
	art.ApplyMediaMetadata(metadata)
	if err := h.artifacts.Update(ctx, art); err != nil {
		t.Fatalf("update artifact metadata: %v", err)
	}
}

func (h *deliveryHarness) captureEntryCount(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	ctx := context.Background()
	acc, err := h.billingRpo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	entries, err := h.billingRpo.ListEntries(ctx, acc.ID, 100, 0)
	if err != nil {
		t.Fatalf("list ledger entries: %v", err)
	}
	var captures int
	for _, entry := range entries {
		if entry.Type == domain.LedgerCapture {
			captures++
		}
	}
	return captures
}

func (h *deliveryHarness) releaseEntryCount(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	ctx := context.Background()
	acc, err := h.billingRpo.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	entries, err := h.billingRpo.ListEntries(ctx, acc.ID, 100, 0)
	if err != nil {
		t.Fatalf("list ledger entries: %v", err)
	}
	var releases int
	for _, entry := range entries {
		if entry.Type == domain.LedgerRelease {
			releases++
		}
	}
	return releases
}

func (h *deliveryHarness) balance(t *testing.T, userID uuid.UUID) int64 {
	t.Helper()
	acc, err := h.billingRpo.GetAccountByUser(context.Background(), userID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	return acc.BalanceCached
}

func deliveryTask(job *domain.Job) queue.Task {
	return queue.Task{JobID: job.ID, Operation: job.OperationType, Modality: job.Modality}
}

func TestDeliverySuccessCapturesAndSucceeds(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "")

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", got.Status)
	}
	if got.CostCaptured != 10 {
		t.Fatalf("captured = %d, want 10", got.CostCaptured)
	}
	if len(h.vk.Sent()) != 1 || h.vk.Sent()[0].Type != "message" || h.vk.Sent()[0].Attachment == "" {
		t.Fatalf("expected one photo send, got %+v", h.vk.Sent())
	}
	if !strings.Contains(h.vk.Sent()[0].Keyboard, "Сгенерировать ещё") ||
		!strings.Contains(h.vk.Sent()[0].Keyboard, "Главное меню") ||
		!strings.Contains(h.vk.Sent()[0].Keyboard, string(domain.CommandMenuImageBackToQuality)) ||
		!strings.Contains(h.vk.Sent()[0].Keyboard, string(domain.CommandShowMenu)) {
		t.Fatalf("expected image result action keyboard, got %q", h.vk.Sent()[0].Keyboard)
	}
	// Balance: 1000 start - 10 captured = 990.
	acc, _ := h.billingRpo.GetAccountByUser(ctx, got.UserID, domain.CurrencyCredits)
	if acc.BalanceCached != 990 {
		t.Fatalf("balance = %d, want 990", acc.BalanceCached)
	}
	dels, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(dels) != 1 || dels[0].Status != domain.DeliveryStatusSent {
		t.Fatalf("unexpected deliveries: %+v", dels)
	}
}

func TestDeliveryUsesPricingSnapshotAmountForCapture(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	snapshot := json.RawMessage(`{"internal_credits":15}`)
	job := h.resultReadyJobWithCost(t, domain.MediaTypeImage, "", 15, snapshot)

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusSucceeded || got.CostCaptured != 15 {
		t.Fatalf("expected succeeded job with snapshot capture, got %+v", got)
	}
	if h.balance(t, got.UserID) != 985 {
		t.Fatalf("balance = %d, want 985", h.balance(t, got.UserID))
	}
}

func TestDeliveryMiniAppJobCapturesWithoutVKSend(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeText, "generated answer")
	job.Source = "miniapp"
	job.ResultMode = domain.ResultModeAccountHistory
	job.DeliveryTarget = nil
	job.OperationType = domain.OperationTextGenerate
	job.Modality = domain.ModalityText
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", got.Status)
	}
	if got.CostCaptured != 10 {
		t.Fatalf("captured = %d, want 10", got.CostCaptured)
	}
	if sent := h.vk.Sent(); len(sent) != 0 {
		t.Fatalf("miniapp job must not send VK message, got %+v", sent)
	}
	dels, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(dels) != 0 {
		t.Fatalf("miniapp job must not create VK delivery rows, got %+v", dels)
	}
	if balance := h.balance(t, job.UserID); balance != 990 {
		t.Fatalf("balance = %d, want 990", balance)
	}
	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("repeat process: %v", err)
	}
	if captures := h.captureEntryCount(t, job.UserID); captures != 1 {
		t.Fatalf("capture entries after repeat = %d, want 1", captures)
	}
}

func TestDeliveryWebAccountHistoryCapturesWithoutVKSendOrDelivery(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
	job.Source = "web"
	job.ResultMode = domain.ResultModeAccountHistory
	job.DeliveryTarget = nil
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("repeat process: %v", err)
	}

	got, err := h.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if got.Status != domain.JobStatusSucceeded || got.CostCaptured != 10 {
		t.Fatalf("account-history job = %+v, want one capture and succeeded", got)
	}
	if sent := h.vk.Sent(); len(sent) != 0 {
		t.Fatalf("account-history job must not send VK messages, got %+v", sent)
	}
	deliveries, err := h.deliveries.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 0 {
		t.Fatalf("account-history job must not create delivery rows, got %+v", deliveries)
	}
	if captures := h.captureEntryCount(t, job.UserID); captures != 1 {
		t.Fatalf("capture entries = %d, want 1", captures)
	}
}

func TestDeliveryAccountHistoryCaptureFailureStaysResultReadyAndRetries(t *testing.T) {
	h := newDeliveryHarness(t)
	biller := &failOnceDeliveryBiller{inner: h.billing}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{Uploader: h.uploader}, worker.DeliveryDeps{Billing: biller})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
	job.ResultMode = domain.ResultModeAccountHistory
	job.DeliveryTarget = nil
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err == nil {
		t.Fatal("expected capture error")
	}
	got, err := h.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if got.Status != domain.JobStatusResultReady || got.CostCaptured != 0 {
		t.Fatalf("job after failed capture = %+v, want result_ready without capture", got)
	}
	deliveries, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(deliveries) != 0 || len(h.vk.Sent()) != 0 {
		t.Fatalf("capture failure created external work: deliveries=%+v sends=%+v", deliveries, h.vk.Sent())
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("capture retry: %v", err)
	}
	got, _ = h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusSucceeded || got.CostCaptured != 10 {
		t.Fatalf("job after capture retry = %+v, want succeeded", got)
	}
	if biller.captureCalls != 2 || h.captureEntryCount(t, job.UserID) != 1 {
		t.Fatalf("capture attempts/entries = %d/%d, want 2/1", biller.captureCalls, h.captureEntryCount(t, job.UserID))
	}
}

func TestDeliveryAccountHistoryRequiresOwnedReadyMediaArtifact(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*deliveryHarness, *domain.Job)
	}{
		{
			name: "missing output link",
			mutate: func(_ *deliveryHarness, job *domain.Job) {
				job.OutputArtifactIDs = nil
			},
		},
		{
			name: "foreign artifact",
			mutate: func(h *deliveryHarness, job *domain.Job) {
				artifact, err := h.artifacts.GetByID(context.Background(), job.OutputArtifactIDs[0])
				if err != nil {
					t.Fatalf("get artifact: %v", err)
				}
				artifact.OwnerAccountID = uuid.New()
				if err := h.artifacts.Update(context.Background(), artifact); err != nil {
					t.Fatalf("make artifact foreign: %v", err)
				}
			},
		},
		{
			name: "non-ready artifact",
			mutate: func(h *deliveryHarness, job *domain.Job) {
				artifact, err := h.artifacts.GetByID(context.Background(), job.OutputArtifactIDs[0])
				if err != nil {
					t.Fatalf("get artifact: %v", err)
				}
				artifact.Status = domain.ArtifactStatusStored
				if err := h.artifacts.Update(context.Background(), artifact); err != nil {
					t.Fatalf("mark artifact non-ready: %v", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newDeliveryHarness(t)
			ctx := context.Background()
			job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
			job.ResultMode = domain.ResultModeAccountHistory
			job.DeliveryTarget = nil
			test.mutate(h, job)
			if err := h.jobs.Update(ctx, job); err != nil {
				t.Fatalf("update job: %v", err)
			}

			if err := h.worker.Process(ctx, deliveryTask(job)); err == nil {
				t.Fatal("expected unsafe account-history result to fail closed")
			}
			got, _ := h.jobs.GetByID(ctx, job.ID)
			deliveries, _ := h.deliveries.ListByJob(ctx, job.ID)
			if got.Status != domain.JobStatusResultReady ||
				got.CostCaptured != 0 ||
				len(deliveries) != 0 ||
				len(h.vk.Sent()) != 0 ||
				h.captureEntryCount(t, job.UserID) != 0 {
				t.Fatalf("unsafe result mutated finalization: job=%+v deliveries=%+v sends=%+v", got, deliveries, h.vk.Sent())
			}
		})
	}
}

func TestDeliveryAccountHistoryRejectsTextWithoutArtifact(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeText, "generated answer")
	job.ResultMode = domain.ResultModeAccountHistory
	job.DeliveryTarget = nil
	job.OperationType = domain.OperationTextGenerate
	job.Modality = domain.ModalityText
	job.OutputArtifactIDs = nil
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err == nil {
		t.Fatal("expected text result without durable artifact to fail closed")
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusResultReady || got.CostCaptured != 0 || len(h.vk.Sent()) != 0 || h.captureEntryCount(t, job.UserID) != 0 {
		t.Fatalf("text account-history finalization = %+v sends=%+v", got, h.vk.Sent())
	}
}

func TestDeliveryExternalResultReadinessFailsClosedBeforeSideEffects(t *testing.T) {
	tests := []struct {
		name          string
		prepare       func(*deliveryHarness) (*domain.Job, worker.CompletionReadiness)
		readinessCall int
		metricReason  string
	}{
		{
			name: "missing account owner",
			prepare: func(h *deliveryHarness) (*domain.Job, worker.CompletionReadiness) {
				ctx := context.Background()
				job := &domain.Job{
					ID:             uuid.New(),
					ResultMode:     domain.ResultModeExternalPush,
					DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
					OperationType:  domain.OperationTextGenerate,
					Modality:       domain.ModalityText,
					Status:         domain.JobStatusResultReady,
					IdempotencyKey: "job:" + uuid.NewString(),
				}
				if err := h.jobs.Create(ctx, job); err != nil {
					t.Fatalf("create accountless job: %v", err)
				}
				artifact := &domain.Artifact{
					ID:            uuid.New(),
					JobID:         &job.ID,
					Kind:          domain.ArtifactKindOutput,
					MediaType:     domain.MediaTypeText,
					StorageBucket: "artifacts",
					StorageKey:    "k/" + job.ID.String(),
					SHA256:        uuid.NewString(),
					Status:        domain.ArtifactStatusReady,
				}
				if err := h.artifacts.Create(ctx, artifact); err != nil {
					t.Fatalf("create accountless artifact: %v", err)
				}
				job.OutputArtifactIDs = []uuid.UUID{artifact.ID}
				if err := h.jobs.Update(ctx, job); err != nil {
					t.Fatalf("attach accountless artifact: %v", err)
				}
				artifactID := artifact.ID
				if err := h.moderation.Create(ctx, &domain.ModerationResult{
					JobID:      job.ID,
					ArtifactID: &artifactID,
					Stage:      domain.ModerationStageOutput,
					Decision:   domain.ModerationAllow,
					Provider:   "test",
				}); err != nil {
					t.Fatalf("moderate accountless artifact: %v", err)
				}
				return job, &recordingCompletionReadiness{
					inner: resultservice.New(h.jobs, h.artifacts, h.moderation),
				}
			},
			metricReason: "missing_owner",
		},
		{
			name: "missing output",
			prepare: func(h *deliveryHarness) (*domain.Job, worker.CompletionReadiness) {
				ctx := context.Background()
				job := h.resultReadyJob(t, domain.MediaTypeText, "generated answer")
				job.OperationType = domain.OperationTextGenerate
				job.Modality = domain.ModalityText
				job.OutputArtifactIDs = nil
				if err := h.jobs.Update(ctx, job); err != nil {
					t.Fatalf("remove output link: %v", err)
				}
				return job, &recordingCompletionReadiness{
					inner: resultservice.New(h.jobs, h.artifacts, h.moderation),
				}
			},
			readinessCall: 1,
			metricReason:  "incomplete",
		},
		{
			name: "missing output moderation",
			prepare: func(h *deliveryHarness) (*domain.Job, worker.CompletionReadiness) {
				job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
				return job, &recordingCompletionReadiness{
					inner: resultservice.New(h.jobs, h.artifacts, memory.NewModerationRepo()),
				}
			},
			readinessCall: 1,
			metricReason:  "incomplete",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newDeliveryHarness(t)
			ctx := context.Background()
			job, readiness := test.prepare(h)
			readinessFailuresBefore := deliveryCounterValue(
				t,
				metrics.FinalizationReadinessFailures,
				"external_push",
				test.metricReason,
			)
			h.worker = h.deliveryWorker(
				vkdelivery.PublisherDeps{Uploader: h.uploader},
				worker.DeliveryDeps{Readiness: readiness},
			)

			err := h.worker.Process(ctx, deliveryTask(job))
			if err == nil {
				t.Fatal("Process succeeded without durable result")
			}
			got, reloadErr := h.jobs.GetByID(ctx, job.ID)
			if reloadErr != nil {
				t.Fatalf("reload job: %v", reloadErr)
			}
			if got.Status != domain.JobStatusResultReady || got.CostCaptured != 0 {
				t.Fatalf("job = %+v, want unchanged result_ready and no capture", got)
			}
			deliveries, listErr := h.deliveries.ListByJob(ctx, job.ID)
			if listErr != nil {
				t.Fatalf("list deliveries: %v", listErr)
			}
			if len(h.vk.Sent()) != 0 || len(deliveries) != 0 {
				t.Fatal("unsafe external result created delivery side effects")
			}
			recorder := readiness.(*recordingCompletionReadiness)
			if len(recorder.calls) != test.readinessCall {
				t.Fatalf("readiness calls = %+v, want %d", recorder.calls, test.readinessCall)
			}
			if got := deliveryCounterValue(
				t,
				metrics.FinalizationReadinessFailures,
				"external_push",
				test.metricReason,
			); got != readinessFailuresBefore+1 {
				t.Fatalf("readiness failure metric = %v, want %v", got, readinessFailuresBefore+1)
			}
		})
	}
}

func TestDeliveryExternalResultReadinessAllowsOnePublishAndCapture(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
	readiness := &recordingCompletionReadiness{
		inner: resultservice.New(h.jobs, h.artifacts, h.moderation),
	}
	h.worker = h.deliveryWorker(
		vkdelivery.PublisherDeps{Uploader: h.uploader},
		worker.DeliveryDeps{Readiness: readiness},
	)
	captureLatencyBefore := deliveryHistogramCount(
		t,
		metrics.ResultFinalizationCaptureDuration,
		"external_push",
	)

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process completion-ready external result: %v", err)
	}
	got, err := h.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	deliveries, err := h.deliveries.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(readiness.calls) != 1 ||
		readiness.calls[0].accountID != job.AccountID ||
		readiness.calls[0].jobID != job.ID {
		t.Fatalf("readiness calls = %+v, want exact account/job pair", readiness.calls)
	}
	if got.Status != domain.JobStatusSucceeded ||
		got.CostCaptured != 10 ||
		len(h.vk.Sent()) != 1 ||
		len(deliveries) != 1 ||
		h.captureEntryCount(t, job.UserID) != 1 {
		t.Fatalf("completed external finalization: job=%+v sends=%+v deliveries=%+v", got, h.vk.Sent(), deliveries)
	}
	if got := deliveryHistogramCount(
		t,
		metrics.ResultFinalizationCaptureDuration,
		"external_push",
	); got != captureLatencyBefore+1 {
		t.Fatalf("capture latency samples = %d, want %d", got, captureLatencyBefore+1)
	}
}

func TestDeliveryExternalFailureNoticeSkipsReadinessAndCapture(t *testing.T) {
	h := newDeliveryHarness(t)
	readiness := &recordingCompletionReadiness{err: errors.New("readiness must not be called")}
	biller := &failOnceDeliveryBiller{inner: h.billing}
	h.worker = h.deliveryWorker(
		vkdelivery.PublisherDeps{Uploader: h.uploader},
		worker.DeliveryDeps{Readiness: readiness, Billing: biller},
	)
	ctx := context.Background()
	job := &domain.Job{
		ID:             uuid.New(),
		AccountID:      uuid.New(),
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusFailedTerminal,
		IdempotencyKey: "job:" + uuid.NewString(),
		CostReserved:   10,
		ErrorCode:      string(domain.ProviderErrInternal),
		ErrorMessage:   "provider failed",
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create terminal job: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process terminal failure notice: %v", err)
	}
	got, err := h.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	deliveries, err := h.deliveries.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(readiness.calls) != 0 || biller.captureCalls != 0 {
		t.Fatalf("failure notice called readiness/capture: readiness=%+v capture=%d", readiness.calls, biller.captureCalls)
	}
	if got.Status != domain.JobStatusFailedTerminal ||
		got.CostCaptured != 0 ||
		len(h.vk.Sent()) != 1 ||
		len(deliveries) != 1 {
		t.Fatalf("failure notice finalization: job=%+v sends=%+v deliveries=%+v", got, h.vk.Sent(), deliveries)
	}
}

func TestDeliveryExternalAccountlessDeliveringReplayFailsBeforeSideEffects(t *testing.T) {
	h := newDeliveryHarness(t)
	readiness := &recordingCompletionReadiness{err: errors.New("readiness must not be called")}
	biller := &failOnceDeliveryBiller{inner: h.billing}
	h.worker = h.deliveryWorker(
		vkdelivery.PublisherDeps{Uploader: h.uploader},
		worker.DeliveryDeps{Readiness: readiness, Billing: biller},
	)
	ctx := context.Background()
	job := &domain.Job{
		ID:             uuid.New(),
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusDelivering,
		IdempotencyKey: "job:" + uuid.NewString(),
		CostReserved:   10,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create accountless delivering job: %v", err)
	}
	artifact := &domain.Artifact{
		ID:            uuid.New(),
		JobID:         &job.ID,
		Kind:          domain.ArtifactKindOutput,
		MediaType:     domain.MediaTypeText,
		StorageBucket: "artifacts",
		StorageKey:    "k/" + job.ID.String(),
		SHA256:        uuid.NewString(),
		Status:        domain.ArtifactStatusReady,
	}
	if err := h.artifacts.Create(ctx, artifact); err != nil {
		t.Fatalf("create accountless output: %v", err)
	}
	job.OutputArtifactIDs = []uuid.UUID{artifact.ID}
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("attach accountless output: %v", err)
	}
	key := "delivery:" + job.ID.String()
	existing := &domain.Delivery{
		JobID:          job.ID,
		VKPeerID:       555,
		Type:           domain.DeliveryTypeMessage,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     vkdelivery.DeterministicRandomID(key),
		IdempotencyKey: key,
		AttemptNo:      1,
		Text:           "must not send",
	}
	if err := h.deliveries.Create(ctx, existing); err != nil {
		t.Fatalf("create accountless delivery replay: %v", err)
	}

	err := h.worker.Process(ctx, deliveryTask(job))
	if !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("process error = %v, want invalid result contract", err)
	}
	gotJob, _ := h.jobs.GetByID(ctx, job.ID)
	gotDelivery, _ := h.deliveries.GetByID(ctx, existing.ID)
	if len(readiness.calls) != 0 ||
		biller.captureCalls != 0 ||
		len(h.vk.Sent()) != 0 ||
		gotJob.Status != domain.JobStatusDelivering ||
		gotJob.CostCaptured != 0 ||
		gotDelivery.AccountID != uuid.Nil ||
		gotDelivery.Target != nil ||
		gotDelivery.Status != domain.DeliveryStatusPending ||
		gotDelivery.AttemptNo != 1 {
		t.Fatalf(
			"accountless delivering replay consumed side effects: job=%+v delivery=%+v readiness=%+v capture=%d sends=%+v",
			gotJob,
			gotDelivery,
			readiness.calls,
			biller.captureCalls,
			h.vk.Sent(),
		)
	}
}

func TestDeliveryExternalAccountlessTerminalFailureFailsBeforeNoticeSideEffects(t *testing.T) {
	h := newDeliveryHarness(t)
	readiness := &recordingCompletionReadiness{err: errors.New("readiness must not be called")}
	biller := &failOnceDeliveryBiller{inner: h.billing}
	h.worker = h.deliveryWorker(
		vkdelivery.PublisherDeps{Uploader: h.uploader},
		worker.DeliveryDeps{Readiness: readiness, Billing: biller},
	)
	ctx := context.Background()
	job := &domain.Job{
		ID:             uuid.New(),
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusFailedTerminal,
		IdempotencyKey: "job:" + uuid.NewString(),
		CostReserved:   10,
		ErrorCode:      string(domain.ProviderErrInternal),
		ErrorMessage:   "provider failed",
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create accountless terminal job: %v", err)
	}

	err := h.worker.Process(ctx, deliveryTask(job))
	if !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("process error = %v, want invalid result contract", err)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	deliveries, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(readiness.calls) != 0 ||
		biller.captureCalls != 0 ||
		len(h.vk.Sent()) != 0 ||
		len(deliveries) != 0 ||
		got.Status != domain.JobStatusFailedTerminal ||
		got.CostCaptured != 0 {
		t.Fatalf(
			"accountless terminal notice consumed side effects: job=%+v deliveries=%+v readiness=%+v capture=%d sends=%+v",
			got,
			deliveries,
			readiness.calls,
			biller.captureCalls,
			h.vk.Sent(),
		)
	}
}

func TestDeliveryExternalDeliveringReadinessRejectsPendingAndRetryingBeforePublishOrReconciliation(t *testing.T) {
	for _, status := range []domain.DeliveryStatus{
		domain.DeliveryStatusPending,
		domain.DeliveryStatusRetrying,
	} {
		t.Run(string(status), func(t *testing.T) {
			h := newDeliveryHarness(t)
			ctx := context.Background()
			job := h.resultReadyJob(t, domain.MediaTypeText, "generated answer")
			if err := h.jobs.UpdateStatus(
				ctx,
				job.ID,
				domain.JobStatusResultReady,
				domain.JobStatusDelivering,
				"",
				"",
			); err != nil {
				t.Fatalf("mark job delivering: %v", err)
			}
			readinessErr := errors.New("durable result unavailable")
			readiness := &recordingCompletionReadiness{err: readinessErr}
			biller := &failOnceDeliveryBiller{inner: h.billing}
			h.worker = h.deliveryWorker(
				vkdelivery.PublisherDeps{Uploader: h.uploader},
				worker.DeliveryDeps{Readiness: readiness, Billing: biller},
			)
			key := "delivery:" + job.ID.String()
			existing := &domain.Delivery{
				JobID:          job.ID,
				UserID:         job.UserID,
				VKPeerID:       555,
				Type:           domain.DeliveryTypeMessage,
				Status:         status,
				VKRandomID:     vkdelivery.DeterministicRandomID(key),
				IdempotencyKey: key,
				AttemptNo:      2,
				Text:           "must not send",
			}
			if err := h.deliveries.Create(ctx, existing); err != nil {
				t.Fatalf("create delivery replay: %v", err)
			}

			err := h.worker.Process(ctx, deliveryTask(job))
			if !errors.Is(err, readinessErr) {
				t.Fatalf("process error = %v, want readiness rejection", err)
			}
			gotJob, _ := h.jobs.GetByID(ctx, job.ID)
			gotDelivery, _ := h.deliveries.GetByID(ctx, existing.ID)
			if len(readiness.calls) != 1 ||
				readiness.calls[0].accountID != job.AccountID ||
				readiness.calls[0].jobID != job.ID ||
				biller.captureCalls != 0 ||
				len(h.vk.Sent()) != 0 ||
				gotJob.Status != domain.JobStatusDelivering ||
				gotJob.CostCaptured != 0 ||
				gotDelivery.AccountID != uuid.Nil ||
				gotDelivery.Target != nil ||
				gotDelivery.Status != status ||
				gotDelivery.AttemptNo != 2 {
				t.Fatalf(
					"rejected delivering replay consumed side effects: job=%+v delivery=%+v readiness=%+v capture=%d sends=%+v",
					gotJob,
					gotDelivery,
					readiness.calls,
					biller.captureCalls,
					h.vk.Sent(),
				)
			}
		})
	}
}

func TestDeliveryExternalDeliveringReadinessRejectsSentBeforeCapture(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
	if err := h.jobs.UpdateStatus(
		ctx,
		job.ID,
		domain.JobStatusResultReady,
		domain.JobStatusDelivering,
		"",
		"",
	); err != nil {
		t.Fatalf("mark job delivering: %v", err)
	}
	readinessErr := errors.New("durable result unavailable")
	readiness := &recordingCompletionReadiness{err: readinessErr}
	biller := &failOnceDeliveryBiller{inner: h.billing}
	h.worker = h.deliveryWorker(
		vkdelivery.PublisherDeps{Uploader: h.uploader},
		worker.DeliveryDeps{Readiness: readiness, Billing: biller},
	)
	key := "delivery:" + job.ID.String()
	existing := &domain.Delivery{
		JobID:          job.ID,
		UserID:         job.UserID,
		AccountID:      job.AccountID,
		VKPeerID:       555,
		Target:         &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		Type:           domain.DeliveryTypePhoto,
		Status:         domain.DeliveryStatusSent,
		VKRandomID:     vkdelivery.DeterministicRandomID(key),
		IdempotencyKey: key,
		AttemptNo:      1,
	}
	if err := h.deliveries.Create(ctx, existing); err != nil {
		t.Fatalf("create sent delivery replay: %v", err)
	}

	err := h.worker.Process(ctx, deliveryTask(job))
	if !errors.Is(err, readinessErr) {
		t.Fatalf("process error = %v, want readiness rejection", err)
	}
	gotJob, _ := h.jobs.GetByID(ctx, job.ID)
	gotDelivery, _ := h.deliveries.GetByID(ctx, existing.ID)
	if len(readiness.calls) != 1 ||
		readiness.calls[0].accountID != job.AccountID ||
		readiness.calls[0].jobID != job.ID ||
		biller.captureCalls != 0 ||
		h.captureEntryCount(t, job.UserID) != 0 ||
		len(h.vk.Sent()) != 0 ||
		gotJob.Status != domain.JobStatusDelivering ||
		gotJob.CostCaptured != 0 ||
		gotDelivery.Status != domain.DeliveryStatusSent ||
		gotDelivery.AttemptNo != 1 {
		t.Fatalf(
			"rejected sent replay consumed capture: job=%+v delivery=%+v readiness=%+v capture=%d sends=%+v",
			gotJob,
			gotDelivery,
			readiness.calls,
			biller.captureCalls,
			h.vk.Sent(),
		)
	}
}

func TestDeliveryExternalPushCaptureRetryDoesNotRepublish(t *testing.T) {
	h := newDeliveryHarness(t)
	biller := &failOnceDeliveryBiller{inner: h.billing}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{Uploader: h.uploader}, worker.DeliveryDeps{Billing: biller})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")

	if err := h.worker.Process(ctx, deliveryTask(job)); err == nil {
		t.Fatal("expected capture error after successful publication")
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusDelivering || got.CostCaptured != 0 {
		t.Fatalf("job after failed capture = %+v, want delivering without capture", got)
	}
	deliveries, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(deliveries) != 1 || deliveries[0].Status != domain.DeliveryStatusSent || len(h.vk.Sent()) != 1 {
		t.Fatalf("publication was not persisted exactly once: deliveries=%+v sends=%+v", deliveries, h.vk.Sent())
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("capture retry: %v", err)
	}
	got, _ = h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusSucceeded || got.CostCaptured != 10 {
		t.Fatalf("job after capture retry = %+v, want succeeded", got)
	}
	if len(h.vk.Sent()) != 1 || h.captureEntryCount(t, job.UserID) != 1 {
		t.Fatalf("capture retry duplicated side effects: sends=%d captures=%d", len(h.vk.Sent()), h.captureEntryCount(t, job.UserID))
	}
}

func TestDeliveryInvalidAndLegacyContractsFailWithoutMutation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.Job)
	}{
		{
			name: "malformed VK target",
			mutate: func(job *domain.Job) {
				job.DeliveryTarget.RecipientRef = "not-a-peer"
			},
		},
		{
			name: "legacy unknown",
			mutate: func(job *domain.Job) {
				job.ResultMode = domain.ResultModeLegacyUnknown
				job.DeliveryTarget = nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newDeliveryHarness(t)
			ctx := context.Background()
			job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
			test.mutate(job)
			if err := h.jobs.Update(ctx, job); err != nil {
				t.Fatalf("update job: %v", err)
			}

			if err := h.worker.Process(ctx, deliveryTask(job)); err == nil {
				t.Fatal("expected fail-closed finalization error")
			}
			got, _ := h.jobs.GetByID(ctx, job.ID)
			deliveries, _ := h.deliveries.ListByJob(ctx, job.ID)
			if got.Status != domain.JobStatusResultReady ||
				got.CostCaptured != 0 ||
				len(deliveries) != 0 ||
				len(h.vk.Sent()) != 0 ||
				h.captureEntryCount(t, job.UserID) != 0 ||
				h.releaseEntryCount(t, job.UserID) != 0 {
				t.Fatalf("invalid contract mutated finalization state: job=%+v deliveries=%+v sends=%+v", got, deliveries, h.vk.Sent())
			}
		})
	}
}

func TestDeliveryAccountHistoryTerminalFailureDoesNotPublishNotice(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
	job.ResultMode = domain.ResultModeAccountHistory
	job.DeliveryTarget = nil
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	if err := h.jobs.UpdateStatus(ctx, job.ID, domain.JobStatusResultReady, domain.JobStatusFailedTerminal, string(domain.ProviderErrInternal), "provider failed"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process account-history failure: %v", err)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	deliveries, _ := h.deliveries.ListByJob(ctx, job.ID)
	if got.Status != domain.JobStatusFailedTerminal ||
		got.CostCaptured != 0 ||
		len(deliveries) != 0 ||
		len(h.vk.Sent()) != 0 ||
		h.captureEntryCount(t, job.UserID) != 0 ||
		h.releaseEntryCount(t, job.UserID) != 0 {
		t.Fatalf("account-history failure produced notice side effects: job=%+v deliveries=%+v sends=%+v", got, deliveries, h.vk.Sent())
	}
}

func TestDeliveryLegacyUnknownTerminalFailureFailsWithoutNotice(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
	job.ResultMode = domain.ResultModeLegacyUnknown
	job.DeliveryTarget = nil
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	if err := h.jobs.UpdateStatus(ctx, job.ID, domain.JobStatusResultReady, domain.JobStatusFailedTerminal, string(domain.ProviderErrInternal), "provider failed"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("process error = %v, want invalid result contract", err)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	deliveries, _ := h.deliveries.ListByJob(ctx, job.ID)
	if got.Status != domain.JobStatusFailedTerminal ||
		got.CostCaptured != 0 ||
		len(deliveries) != 0 ||
		len(h.vk.Sent()) != 0 ||
		h.captureEntryCount(t, job.UserID) != 0 ||
		h.releaseEntryCount(t, job.UserID) != 0 {
		t.Fatalf("legacy terminal failure produced side effects: job=%+v deliveries=%+v sends=%+v", got, deliveries, h.vk.Sent())
	}
}

func TestDeliveryReplayTargetMismatchDoesNotPublishOrCapture(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
	key := "delivery:" + job.ID.String()
	delivery := &domain.Delivery{
		JobID:          job.ID,
		UserID:         job.UserID,
		AccountID:      job.AccountID,
		VKPeerID:       999,
		Target:         &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "999"},
		Type:           domain.DeliveryTypePhoto,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     vkdelivery.DeterministicRandomID(key),
		IdempotencyKey: key,
		AttemptNo:      1,
	}
	if err := h.deliveries.Create(ctx, delivery); err != nil {
		t.Fatalf("create mismatched delivery: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err == nil {
		t.Fatal("expected replay target mismatch error")
	}
	gotJob, _ := h.jobs.GetByID(ctx, job.ID)
	gotDelivery, _ := h.deliveries.GetByID(ctx, delivery.ID)
	if len(h.vk.Sent()) != 0 ||
		h.captureEntryCount(t, job.UserID) != 0 ||
		gotJob.Status != domain.JobStatusResultReady ||
		gotDelivery.Status != domain.DeliveryStatusPending ||
		gotDelivery.AttemptNo != 1 {
		t.Fatalf("mismatched replay consumed side effects: job=%+v delivery=%+v sends=%+v", gotJob, gotDelivery, h.vk.Sent())
	}
}

func TestDeliveryUploadsRawPhotoArtifactToVK(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{Uploader: uploader}, worker.DeliveryDeps{})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "raw png bytes")

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if string(uploader.photoBytes) != "raw png bytes" {
		t.Fatalf("uploaded bytes = %q", string(uploader.photoBytes))
	}
	sent := h.vk.Sent()
	if len(sent) != 1 || sent[0].Attachment != "photo123_456_key" {
		t.Fatalf("expected uploaded VK attachment send, got %+v", sent)
	}
}

func TestDeliveryFailsClosedWithoutVKAttachmentOrSignedURL(t *testing.T) {
	h := newDeliveryHarness(t)
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{}, worker.DeliveryDeps{MaxAttempts: 1})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "raw png bytes")

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("terminal delivery failure should be acknowledged: %v", err)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusFailedTerminal || got.CostCaptured != 0 {
		t.Fatalf("unsafe media fallback must fail closed without capture: status=%q captured=%d", got.Status, got.CostCaptured)
	}
	if sent := h.vk.Sent(); len(sent) != 0 {
		t.Fatalf("unsafe media fallback sent %d message(s)", len(sent))
	}
	if h.captureEntryCount(t, job.UserID) != 0 || h.releaseEntryCount(t, job.UserID) != 1 {
		t.Fatalf("unsafe media fallback must release reservation without capture")
	}
}

func TestDeliveryUsesSignedURLWhenExplicitlyEnabled(t *testing.T) {
	h := newDeliveryHarness(t)
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{
		SignedURLs: true,
		Signer:     fakeURLSigner{},
	}, worker.DeliveryDeps{})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "raw png bytes")

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusSucceeded || got.CostCaptured != 10 {
		t.Fatalf("signed delivery did not complete safely: status=%q captured=%d", got.Status, got.CostCaptured)
	}
	sent := h.vk.Sent()
	if len(sent) != 1 || sent[0].Attachment == "" || !strings.HasPrefix(sent[0].Attachment, "https://signed-delivery.example/") {
		t.Fatalf("expected one signed delivery attachment")
	}
}

func TestDeliveryNamesRawVideoArtifactFromPrompt(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{Uploader: uploader}, worker.DeliveryDeps{})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "raw mp4 bytes")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	params, _ := json.Marshal(struct {
		Prompt string `json:"prompt"`
	}{
		Prompt: "кот в очках едет на жирафе по городу",
	})
	job.Params = params
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if string(uploader.videoBytes) != "raw mp4 bytes" {
		t.Fatalf("uploaded bytes = %q", string(uploader.videoBytes))
	}
	if uploader.videoFilename != "кот в очках едет на жираф.mp4" {
		t.Fatalf("video filename = %q", uploader.videoFilename)
	}
}

func TestDeliveryAllowsProbePassedRawVideoOriginalByPolicy(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{
		Uploader:               uploader,
		RawVideoDeliveryPolicy: "if_probe_passed",
	}, worker.DeliveryDeps{})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "safe original mp4")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	h.markVideoOriginal(t, job, domain.ArtifactMediaMetadata{
		Width:       1280,
		Height:      720,
		DurationMS:  5000,
		Codec:       "h264",
		Container:   "mp4",
		BitrateBPS:  2400000,
		ProbeStatus: domain.MediaProbePassed,
	})

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if string(uploader.videoBytes) != "safe original mp4" {
		t.Fatalf("uploaded bytes = %q", string(uploader.videoBytes))
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusSucceeded || got.CostCaptured != 10 {
		t.Fatalf("expected succeeded job with capture after safe delivery, got %+v", got)
	}
	if h.captureEntryCount(t, job.UserID) != 1 {
		t.Fatalf("expected exactly one capture ledger entry")
	}
}

func TestDeliveryRejectsRawVideoOriginalWhenPolicyNever(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{
		Uploader:               uploader,
		RawVideoDeliveryPolicy: "never",
	}, worker.DeliveryDeps{MaxAttempts: 1})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "safe original mp4")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	h.markVideoOriginal(t, job, domain.ArtifactMediaMetadata{
		Width:       1280,
		Height:      720,
		DurationMS:  5000,
		Codec:       "h264",
		Container:   "mp4",
		ProbeStatus: domain.MediaProbePassed,
	})

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("terminal delivery failure should be acknowledged: %v", err)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusFailedTerminal || got.CostCaptured != 0 {
		t.Fatalf("raw original without policy must fail without capture, got %+v", got)
	}
	if len(uploader.videoBytes) != 0 {
		t.Fatalf("uploader received raw video bytes despite policy=never")
	}
	if h.captureEntryCount(t, job.UserID) != 0 {
		t.Fatalf("raw original rejection must not capture credits")
	}
}

func TestDeliveryUploadsVKReadyVideoVariantWhenPresent(t *testing.T) {
	for _, variantType := range []domain.VariantType{domain.VariantVKDoc, domain.VariantVKVideo} {
		t.Run(string(variantType), func(t *testing.T) {
			h := newDeliveryHarness(t)
			uploader := &fakeVKUploader{}
			h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{Uploader: uploader}, worker.DeliveryDeps{})
			ctx := context.Background()
			job := h.resultReadyJob(t, domain.MediaTypeVideo, "raw provider video")
			job.OperationType = domain.OperationVideoGenerate
			job.Modality = domain.ModalityVideo
			if err := h.jobs.Update(ctx, job); err != nil {
				t.Fatalf("update job: %v", err)
			}
			h.addVideoVariant(t, job, variantType, "vk-ready mp4 bytes")

			if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
				t.Fatalf("process: %v", err)
			}
			if string(uploader.videoBytes) != "vk-ready mp4 bytes" {
				t.Fatalf("uploaded bytes = %q", string(uploader.videoBytes))
			}
			got, _ := h.jobs.GetByID(ctx, job.ID)
			if got.Status != domain.JobStatusSucceeded || got.CostCaptured != 10 {
				t.Fatalf("expected succeeded job with captured credits, got %+v", got)
			}
			if h.captureEntryCount(t, job.UserID) != 1 {
				t.Fatalf("expected exactly one capture ledger entry")
			}
			sent := h.vk.Sent()
			if len(sent) != 1 || sent[0].Attachment != "video123_456_key" {
				t.Fatalf("expected uploaded VK video attachment send, got %+v", sent)
			}
		})
	}
}

func TestDeliveryPrefersVKDocVariantWhenBothVideoVariantsAreReady(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{Uploader: uploader}, worker.DeliveryDeps{})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "raw provider video")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	h.addVideoVariant(t, job, domain.VariantVKVideo, "vk-video bytes")
	h.addVideoVariant(t, job, domain.VariantVKDoc, "vk-doc bytes")

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	if string(uploader.videoBytes) != "vk-doc bytes" {
		t.Fatalf("uploaded bytes = %q, want vk-doc variant", string(uploader.videoBytes))
	}
}

func TestDeliveryMediaUploadFailureUsesRetryBudget(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{err: errors.New("vk video.save denied")}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{Uploader: uploader}, worker.DeliveryDeps{MaxAttempts: 2})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "raw mp4 bytes")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	_ = h.jobs.Update(ctx, job)
	h.addVideoVariant(t, job, domain.VariantVKVideo, "vk-ready mp4 bytes")

	if err := h.worker.Process(ctx, deliveryTask(job)); err == nil {
		t.Fatalf("expected upload error so the task stays pending for retry")
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.CostCaptured != 0 || h.captureEntryCount(t, job.UserID) != 0 {
		t.Fatalf("failed upload must not capture credits, job=%+v", got)
	}
	dels, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(dels) != 1 || dels[0].Status != domain.DeliveryStatusRetrying || dels[0].AttemptNo != 2 {
		t.Fatalf("expected persisted retrying delivery after upload failure, got %+v", dels)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("terminal retry should be acknowledged after DLQ routing: %v", err)
	}
	got, _ = h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusFailedTerminal || got.ErrorCode != domain.JobErrMediaDeliveryFailed {
		t.Fatalf("expected terminal delivery failure, got %+v", got)
	}
	if got.CostCaptured != 0 || h.captureEntryCount(t, job.UserID) != 0 || h.releaseEntryCount(t, job.UserID) != 1 {
		t.Fatalf("terminal delivery failure must release without capture, job=%+v", got)
	}
	if got.ErrorMessage != "media delivery failed; credits were not charged" {
		t.Fatalf("unsafe delivery error message: %q", got.ErrorMessage)
	}
	if balance := h.balance(t, job.UserID); balance != 1000 {
		t.Fatalf("balance after delivery failure = %d, want reservation released to 1000", balance)
	}
}

func TestDeliveryExhaustedMediaIsNotRepublishedAsFailureNotice(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{err: errors.New("vk video.save denied")}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{Uploader: uploader}, worker.DeliveryDeps{MaxAttempts: 1})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "raw mp4 bytes")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	h.addVideoVariant(t, job, domain.VariantVKVideo, "vk-ready mp4 bytes")

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("exhausted delivery should be acknowledged: %v", err)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status after exhaustion = %q, want failed_terminal", got.Status)
	}

	uploader.err = nil
	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("terminal redelivery: %v", err)
	}
	deliveries, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(deliveries) != 1 || deliveries[0].Status != domain.DeliveryStatusFailed {
		t.Fatalf("exhausted delivery = %+v, want one terminally failed row", deliveries)
	}
	if len(h.vk.Sent()) != 0 ||
		h.captureEntryCount(t, job.UserID) != 0 ||
		h.releaseEntryCount(t, job.UserID) != 1 {
		t.Fatalf("terminal redelivery published or billed original media: sends=%+v captures=%d releases=%d",
			h.vk.Sent(), h.captureEntryCount(t, job.UserID), h.releaseEntryCount(t, job.UserID))
	}
}

func TestDeliveryExhaustionRequiresFailedStatusPersistenceBeforeBookkeeping(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	persistErr := errors.New("delivery update unavailable")
	deliveries := &failFailedDeliveryUpdateRepo{
		DeliveryRepository: h.deliveries,
		err:                persistErr,
	}
	uploader := &fakeVKUploader{err: errors.New("vk video.save denied")}
	publisher := vkdelivery.NewPublisher(vkdelivery.PublisherDeps{
		Deliveries: deliveries,
		Artifacts:  h.artifacts,
		Objects:    h.objects,
		Client:     h.vk,
		Uploader:   uploader,
	})
	streams := newFakeStreams()
	deliveryWorker := worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:        h.jobs,
		Deliveries:  deliveries,
		Artifacts:   h.artifacts,
		Publishers:  []worker.ExternalPublisher{publisher},
		Billing:     h.billing,
		Readiness:   resultservice.New(h.jobs, h.artifacts, h.moderation),
		Streams:     streams,
		MaxAttempts: 1,
	})
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "raw mp4 bytes")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	h.addVideoVariant(t, job, domain.VariantVKVideo, "vk-ready mp4 bytes")

	err := deliveryWorker.Process(ctx, deliveryTask(job))
	if !errors.Is(err, persistErr) {
		t.Fatalf("process error = %v, want failed-delivery persistence error", err)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusDelivering {
		t.Fatalf("status after persistence failure = %q, want delivering", got.Status)
	}
	stored, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(stored) != 1 || stored[0].Status == domain.DeliveryStatusFailed {
		t.Fatalf("failed marker unexpectedly durable after update error: %+v", stored)
	}
	if len(streams.byStream[redisqueue.StreamDLQ]) != 0 ||
		h.releaseEntryCount(t, job.UserID) != 0 ||
		h.captureEntryCount(t, job.UserID) != 0 ||
		len(h.vk.Sent()) != 0 {
		t.Fatalf("persistence failure performed terminal bookkeeping: dlq=%d releases=%d captures=%d sends=%d",
			len(streams.byStream[redisqueue.StreamDLQ]),
			h.releaseEntryCount(t, job.UserID),
			h.captureEntryCount(t, job.UserID),
			len(h.vk.Sent()))
	}
}

func TestDeliveryReleaseFailureResumesFailedBookkeepingWithoutRepublish(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	uploader := &fakeVKUploader{err: errors.New("vk video.save denied")}
	releaseErr := errors.New("release unavailable")
	biller := &failReleaseOnceDeliveryBiller{inner: h.billing, err: releaseErr}
	streams := newFakeStreams()
	publisher := vkdelivery.NewPublisher(vkdelivery.PublisherDeps{
		Deliveries: h.deliveries,
		Artifacts:  h.artifacts,
		Objects:    h.objects,
		Client:     h.vk,
		Uploader:   uploader,
	})
	deliveryWorker := worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:        h.jobs,
		Deliveries:  h.deliveries,
		Artifacts:   h.artifacts,
		Publishers:  []worker.ExternalPublisher{publisher},
		Billing:     biller,
		Readiness:   resultservice.New(h.jobs, h.artifacts, h.moderation),
		Streams:     streams,
		MaxAttempts: 1,
	})
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "raw mp4 bytes")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	h.addVideoVariant(t, job, domain.VariantVKVideo, "vk-ready mp4 bytes")

	if err := deliveryWorker.Process(ctx, deliveryTask(job)); !errors.Is(err, releaseErr) {
		t.Fatalf("first process error = %v, want release error", err)
	}
	stored, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(stored) != 1 || stored[0].Status != domain.DeliveryStatusFailed {
		t.Fatalf("delivery after exhaustion = %+v, want durable failed", stored)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusDelivering {
		t.Fatalf("status after release failure = %q, want delivering", got.Status)
	}
	if biller.releaseCalls != 1 ||
		len(streams.byStream[redisqueue.StreamDLQ]) != 1 ||
		len(h.vk.Sent()) != 0 {
		t.Fatalf("first attempt bookkeeping: release calls=%d dlq=%d sends=%d",
			biller.releaseCalls, len(streams.byStream[redisqueue.StreamDLQ]), len(h.vk.Sent()))
	}

	uploader.err = nil
	if err := deliveryWorker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("bookkeeping retry: %v", err)
	}
	got, _ = h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status after bookkeeping retry = %q, want failed_terminal", got.Status)
	}
	if biller.releaseCalls != 2 ||
		h.releaseEntryCount(t, job.UserID) != 1 ||
		h.captureEntryCount(t, job.UserID) != 0 ||
		len(streams.byStream[redisqueue.StreamDLQ]) != 1 ||
		len(h.vk.Sent()) != 0 {
		t.Fatalf("retry repeated delivery side effects: release calls=%d entries=%d captures=%d dlq=%d sends=%d",
			biller.releaseCalls,
			h.releaseEntryCount(t, job.UserID),
			h.captureEntryCount(t, job.UserID),
			len(streams.byStream[redisqueue.StreamDLQ]),
			len(h.vk.Sent()))
	}

	if err := deliveryWorker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("terminal redelivery: %v", err)
	}
	if biller.releaseCalls != 2 ||
		h.releaseEntryCount(t, job.UserID) != 1 ||
		len(streams.byStream[redisqueue.StreamDLQ]) != 1 ||
		len(h.vk.Sent()) != 0 {
		t.Fatalf("terminal redelivery repeated bookkeeping: release calls=%d entries=%d dlq=%d sends=%d",
			biller.releaseCalls,
			h.releaseEntryCount(t, job.UserID),
			len(streams.byStream[redisqueue.StreamDLQ]),
			len(h.vk.Sent()))
	}
}

func TestDeliveryTerminalStatusFailureResumesFailedBookkeepingWithoutRepublish(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	uploader := &fakeVKUploader{err: errors.New("vk video.save denied")}
	statusErr := errors.New("job status update unavailable")
	jobs := &failTerminalStatusOnceJobRepo{JobRepository: h.jobs, err: statusErr}
	streams := newFakeStreams()
	publisher := vkdelivery.NewPublisher(vkdelivery.PublisherDeps{
		Deliveries: h.deliveries,
		Artifacts:  h.artifacts,
		Objects:    h.objects,
		Client:     h.vk,
		Uploader:   uploader,
	})
	deliveryWorker := worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:        jobs,
		Deliveries:  h.deliveries,
		Artifacts:   h.artifacts,
		Publishers:  []worker.ExternalPublisher{publisher},
		Billing:     h.billing,
		Readiness:   resultservice.New(h.jobs, h.artifacts, h.moderation),
		Streams:     streams,
		MaxAttempts: 1,
	})
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "raw mp4 bytes")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	h.addVideoVariant(t, job, domain.VariantVKVideo, "vk-ready mp4 bytes")

	if err := deliveryWorker.Process(ctx, deliveryTask(job)); !errors.Is(err, statusErr) {
		t.Fatalf("first process error = %v, want terminal-status error", err)
	}
	stored, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(stored) != 1 || stored[0].Status != domain.DeliveryStatusFailed {
		t.Fatalf("delivery after exhaustion = %+v, want durable failed", stored)
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusDelivering {
		t.Fatalf("status after terminal transition failure = %q, want delivering", got.Status)
	}
	if h.releaseEntryCount(t, job.UserID) != 1 ||
		len(streams.byStream[redisqueue.StreamDLQ]) != 1 ||
		len(h.vk.Sent()) != 0 {
		t.Fatalf("first attempt bookkeeping: releases=%d dlq=%d sends=%d",
			h.releaseEntryCount(t, job.UserID),
			len(streams.byStream[redisqueue.StreamDLQ]),
			len(h.vk.Sent()))
	}

	uploader.err = nil
	if err := deliveryWorker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("bookkeeping retry: %v", err)
	}
	got, _ = h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusFailedTerminal {
		t.Fatalf("status after bookkeeping retry = %q, want failed_terminal", got.Status)
	}
	if jobs.attempts != 2 ||
		h.releaseEntryCount(t, job.UserID) != 1 ||
		h.captureEntryCount(t, job.UserID) != 0 ||
		len(streams.byStream[redisqueue.StreamDLQ]) != 1 ||
		len(h.vk.Sent()) != 0 {
		t.Fatalf("retry repeated delivery side effects: status attempts=%d releases=%d captures=%d dlq=%d sends=%d",
			jobs.attempts,
			h.releaseEntryCount(t, job.UserID),
			h.captureEntryCount(t, job.UserID),
			len(streams.byStream[redisqueue.StreamDLQ]),
			len(h.vk.Sent()))
	}
}

func TestDeliveryIdempotentNoDuplicateSendOrCharge(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "")

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process 1: %v", err)
	}
	// Reset job to delivering to simulate a re-delivery after a crash before ack.
	_ = h.jobs.UpdateStatus(ctx, job.ID, domain.JobStatusSucceeded, domain.JobStatusDelivering, "", "")
	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process 2: %v", err)
	}

	if n := len(h.vk.Sent()); n != 1 {
		t.Fatalf("expected exactly one send across redeliveries, got %d", n)
	}
	acc, _ := h.billingRpo.GetAccountByUser(ctx, job.UserID, domain.CurrencyCredits)
	if acc.BalanceCached != 990 {
		t.Fatalf("balance = %d, want 990 (no double charge)", acc.BalanceCached)
	}
	dels, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(dels) != 1 {
		t.Fatalf("expected one delivery row, got %d", len(dels))
	}
}

func TestDeliveryVideoVariantIdempotentNoDuplicateSendOrCharge(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{}
	h.worker = h.deliveryWorker(vkdelivery.PublisherDeps{Uploader: uploader}, worker.DeliveryDeps{})
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeVideo, "raw provider video")
	job.OperationType = domain.OperationVideoGenerate
	job.Modality = domain.ModalityVideo
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	h.addVideoVariant(t, job, domain.VariantVKVideo, "vk-ready mp4 bytes")

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process 1: %v", err)
	}
	_ = h.jobs.UpdateStatus(ctx, job.ID, domain.JobStatusSucceeded, domain.JobStatusDelivering, "", "")
	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process 2: %v", err)
	}

	if n := len(h.vk.Sent()); n != 1 {
		t.Fatalf("expected exactly one send across redeliveries, got %d", n)
	}
	if string(uploader.videoBytes) != "vk-ready mp4 bytes" {
		t.Fatalf("uploaded bytes = %q", string(uploader.videoBytes))
	}
	if h.captureEntryCount(t, job.UserID) != 1 {
		t.Fatalf("expected exactly one capture ledger entry")
	}
	acc, _ := h.billingRpo.GetAccountByUser(ctx, job.UserID, domain.CurrencyCredits)
	if acc.BalanceCached != 990 {
		t.Fatalf("balance = %d, want 990 (no double charge)", acc.BalanceCached)
	}
	dels, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(dels) != 1 || dels[0].VKRandomID == 0 || dels[0].Status != domain.DeliveryStatusSent {
		t.Fatalf("expected one sent delivery with deterministic random id, got %+v", dels)
	}
}

func TestDeliveryTextSendsBody(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeText, "generated answer")
	job.Modality = domain.ModalityText
	_ = h.jobs.Update(ctx, job)

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	sent := h.vk.Sent()
	if len(sent) != 1 || sent[0].Type != "text" || sent[0].Text != "generated answer" {
		t.Fatalf("unexpected text send: %+v", sent)
	}
}

func TestDeliveryTextFormatsMarkdownForVK(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	body := "Привет!\n\n**1. Уход за кожей и телом**\n*   Очищение, тонизирование, увлажнение.\n* Защита от солнца (SPF).\n\n### Итог\n`Главное — регулярность.`"
	job := h.resultReadyJob(t, domain.MediaTypeText, body)
	job.Modality = domain.ModalityText
	_ = h.jobs.Update(ctx, job)

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	sent := h.vk.Sent()
	if len(sent) != 1 || sent[0].Type != "text" {
		t.Fatalf("unexpected text send: %+v", sent)
	}
	want := "Привет!\n\n1. Уход за кожей и телом\n• Очищение, тонизирование, увлажнение.\n• Защита от солнца (SPF).\n\nИтог\nГлавное — регулярность."
	if sent[0].Text != want {
		t.Fatalf("formatted text = %q, want %q", sent[0].Text, want)
	}
}

func TestDeliveryTextEditsGPTPlaceholder(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	pending, err := h.vk.SendMessage(ctx, 555, 9001, vkdelivery.Message{Text: "НейроХаб думает..."})
	if err != nil {
		t.Fatalf("send pending: %v", err)
	}
	job := h.resultReadyJob(t, domain.MediaTypeText, "generated answer")
	job.OperationType = domain.OperationTextGenerate
	job.Modality = domain.ModalityText
	params, _ := json.Marshal(struct {
		Prompt                 string `json:"prompt"`
		VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
	}{
		Prompt:                 "привет",
		VKPlaceholderMessageID: pending.MessageID,
	})
	job.Params = params
	_ = h.jobs.Update(ctx, job)

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	sent := h.vk.Sent()
	if len(sent) != 1 || sent[0].Type != "message" || sent[0].Text != "generated answer" {
		t.Fatalf("expected placeholder edit without a new send, got %+v", sent)
	}
	edits := h.vk.Edits()
	if len(edits) != 1 || edits[0].MessageID != pending.MessageID || edits[0].Text != "generated answer" {
		t.Fatalf("unexpected edits: %+v", edits)
	}
	dels, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(dels) != 1 || dels[0].VKMessageID == nil || *dels[0].VKMessageID != pending.MessageID {
		t.Fatalf("delivery should keep the edited VK message id, got %+v", dels)
	}
}

func TestDeliveryTextSplitsLongGPTPlaceholderAnswer(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	pending, err := h.vk.SendMessage(ctx, 555, 9001, vkdelivery.Message{Text: "НейроХаб думает..."})
	if err != nil {
		t.Fatalf("send pending: %v", err)
	}
	longAnswer := strings.Repeat("answer ", 700)
	job := h.resultReadyJob(t, domain.MediaTypeText, longAnswer)
	job.OperationType = domain.OperationTextGenerate
	job.Modality = domain.ModalityText
	params, _ := json.Marshal(struct {
		Prompt                 string `json:"prompt"`
		VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
	}{
		Prompt:                 "long",
		VKPlaceholderMessageID: pending.MessageID,
	})
	job.Params = params
	_ = h.jobs.Update(ctx, job)

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}
	edits := h.vk.Edits()
	if len(edits) != 1 || edits[0].MessageID != pending.MessageID {
		t.Fatalf("expected one placeholder edit, got %+v", edits)
	}
	sent := h.vk.Sent()
	if len(sent) < 2 || sent[0].MessageID != pending.MessageID {
		t.Fatalf("expected edited placeholder plus follow-up text chunks, got %+v", sent)
	}
	for i, msg := range sent {
		if len([]rune(msg.Text)) > 3500 {
			t.Fatalf("chunk %d is too long: %d", i, len([]rune(msg.Text)))
		}
		if i > 0 && msg.Type != "text" {
			t.Fatalf("follow-up chunk %d should be text, got %+v", i, msg)
		}
		if !strings.Contains(msg.Text, "answer") {
			t.Fatalf("unexpected split content in chunk %d: %+v", i, msg)
		}
	}
}

func TestDeliverySendsImageProviderFailureNoticeWithoutCapture(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	userID := uuid.New()
	if _, err := h.billing.EnsureAccount(ctx, userID); err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	pending, err := h.vk.SendMessage(ctx, 555, 9001, vkdelivery.Message{Text: "НейроХаб рисует..."})
	if err != nil {
		t.Fatalf("send pending: %v", err)
	}
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         userID,
		VKPeerID:       555,
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusFailedTerminal,
		IdempotencyKey: "job:" + uuid.NewString(),
		CostReserved:   10,
		ErrorCode:      string(domain.ProviderErrInternal),
		ErrorMessage:   "provider failed",
	}
	params, _ := json.Marshal(struct {
		Prompt                 string `json:"prompt"`
		VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
	}{
		Prompt:                 "кот",
		VKPlaceholderMessageID: pending.MessageID,
	})
	job.Params = params
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, err := h.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if got.Status != domain.JobStatusFailedTerminal || got.CostCaptured != 0 {
		t.Fatalf("failure notice must not mark success or capture credits: %+v", got)
	}
	edits := h.vk.Edits()
	if len(edits) != 1 || edits[0].MessageID != pending.MessageID || !strings.Contains(edits[0].Text, "⭐️ не списаны") {
		t.Fatalf("unexpected failure notice edit: %+v", edits)
	}
	if !strings.Contains(edits[0].Text, "Генерация") || strings.Contains(edits[0].Text, "Медиаобработка") || strings.Contains(edits[0].Text, "provider") {
		t.Fatalf("provider failure notice should be safe and specific: %q", edits[0].Text)
	}
	dels, _ := h.deliveries.ListByJob(ctx, job.ID)
	if len(dels) != 1 || dels[0].Status != domain.DeliveryStatusSent || dels[0].VKMessageID == nil || *dels[0].VKMessageID != pending.MessageID {
		t.Fatalf("failure delivery should be persisted as sent edit: %+v", dels)
	}
}

func TestDeliverySendsVideoMediaFailureNoticeWithoutCapture(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	userID := uuid.New()
	if _, err := h.billing.EnsureAccount(ctx, userID); err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	pending, err := h.vk.SendMessage(ctx, 556, 9002, vkdelivery.Message{Text: "НейроХаб готовит видео..."})
	if err != nil {
		t.Fatalf("send pending: %v", err)
	}
	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         userID,
		VKPeerID:       556,
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "556"},
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusFailedTerminal,
		IdempotencyKey: "job:" + uuid.NewString(),
		CostReserved:   10,
		ErrorCode:      domain.JobErrMediaProviderOutputInvalid,
		ErrorMessage:   "generated media failed safety checks; credits were not charged",
	}
	params, _ := json.Marshal(struct {
		Prompt                 string `json:"prompt"`
		VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
	}{
		Prompt:                 "unsafe video",
		VKPlaceholderMessageID: pending.MessageID,
	})
	job.Params = params
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, err := h.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if got.Status != domain.JobStatusFailedTerminal || got.CostCaptured != 0 {
		t.Fatalf("failure notice must not mark success or capture credits: %+v", got)
	}
	edits := h.vk.Edits()
	if len(edits) != 1 || edits[0].MessageID != pending.MessageID || !strings.Contains(edits[0].Text, "⭐️ не списаны") {
		t.Fatalf("unexpected video failure notice edit: %+v", edits)
	}
	if strings.Contains(edits[0].Text, "unsafe video") || strings.Contains(edits[0].Text, "provider") {
		t.Fatalf("video failure notice leaked unsafe details: %q", edits[0].Text)
	}
}

func TestDeliveryUsesSpecificModelAndInvalidRequestFailureNotices(t *testing.T) {
	cases := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "model unavailable",
			code:     domain.JobErrModelUnavailable,
			expected: "Выбранная модель сейчас недоступна. ⭐️ не списаны. Попробуйте другую модель.",
		},
		{
			name:     "invalid request",
			code:     string(domain.ProviderErrInvalidRequest),
			expected: "Модель не приняла запрос. ⭐️ не списаны. Попробуйте другую модель или измените описание; возможны ограничения по содержанию.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newDeliveryHarness(t)
			ctx := context.Background()
			userID := uuid.New()
			if _, err := h.billing.EnsureAccount(ctx, userID); err != nil {
				t.Fatalf("ensure account: %v", err)
			}
			pending, err := h.vk.SendMessage(ctx, 557, 9003, vkdelivery.Message{Text: "pending"})
			if err != nil {
				t.Fatalf("send pending: %v", err)
			}
			job := &domain.Job{
				ID:             uuid.New(),
				UserID:         userID,
				VKPeerID:       557,
				ResultMode:     domain.ResultModeExternalPush,
				DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "557"},
				OperationType:  domain.OperationImageGenerate,
				Modality:       domain.ModalityImage,
				Status:         domain.JobStatusFailedTerminal,
				IdempotencyKey: "job:" + uuid.NewString(),
				CostReserved:   10,
				ErrorCode:      tc.code,
				ErrorMessage:   "raw provider private-model-v9 failure",
			}
			params, _ := json.Marshal(struct {
				Prompt                 string `json:"prompt"`
				VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
			}{
				Prompt:                 "raw unsafe prompt must not leak",
				VKPlaceholderMessageID: pending.MessageID,
			})
			job.Params = params
			if err := h.jobs.Create(ctx, job); err != nil {
				t.Fatalf("create job: %v", err)
			}

			if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
				t.Fatalf("process: %v", err)
			}
			got, err := h.jobs.GetByID(ctx, job.ID)
			if err != nil {
				t.Fatalf("reload job: %v", err)
			}
			if got.Status != domain.JobStatusFailedTerminal || got.CostCaptured != 0 {
				t.Fatalf("failure notice must not mark success or capture credits: %+v", got)
			}
			edits := h.vk.Edits()
			if len(edits) != 1 || edits[0].MessageID != pending.MessageID {
				t.Fatalf("unexpected failure notice edits: %+v", edits)
			}
			if edits[0].Text != tc.expected {
				t.Fatalf("notice = %q, want %q", edits[0].Text, tc.expected)
			}
			for _, forbidden := range []string{"private-model-v9", "raw provider", "raw unsafe prompt"} {
				if strings.Contains(edits[0].Text, forbidden) {
					t.Fatalf("notice leaked %q: %q", forbidden, edits[0].Text)
				}
			}
		})
	}
}

func TestDeliverySendFailureRetries(t *testing.T) {
	h := newDeliveryHarness(t)
	ctx := context.Background()
	job := h.resultReadyJob(t, domain.MediaTypeImage, "")
	h.vk.FailNext(errors.New("vk down"))

	if err := h.worker.Process(ctx, deliveryTask(job)); err == nil {
		t.Fatalf("expected error so the task stays pending for retry")
	}
	got, _ := h.jobs.GetByID(ctx, job.ID)
	if got.Status == domain.JobStatusSucceeded {
		t.Fatalf("job should not be succeeded after send failure")
	}

	// Retry succeeds.
	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("retry: %v", err)
	}
	got, _ = h.jobs.GetByID(ctx, job.ID)
	if got.Status != domain.JobStatusSucceeded {
		t.Fatalf("status = %q, want succeeded after retry", got.Status)
	}
}

func deliveryCounterValue(t *testing.T, counter *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	metricCounter, err := counter.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	var metric dto.Metric
	if err := metricCounter.Write(&metric); err != nil {
		t.Fatalf("counter.Write() error = %v", err)
	}
	return metric.GetCounter().GetValue()
}

func deliveryHistogramCount(t *testing.T, histogram *prometheus.HistogramVec, labels ...string) uint64 {
	t.Helper()
	observer, err := histogram.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	metricWriter, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatal("histogram observer does not implement prometheus.Metric")
	}
	var metric dto.Metric
	if err := metricWriter.Write(&metric); err != nil {
		t.Fatalf("histogram.Write() error = %v", err)
	}
	return metric.GetHistogram().GetSampleCount()
}
