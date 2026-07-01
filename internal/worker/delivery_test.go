package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/worker"
)

type deliveryHarness struct {
	jobs       *memory.JobRepo
	deliveries *memory.DeliveryRepo
	artifacts  *memory.ArtifactRepo
	objects    *memory.ObjectStore
	vk         *vkdelivery.MockClient
	billingRpo *memory.BillingRepo
	billing    *billingservice.Service
	worker     *worker.DeliveryWorker
}

func newDeliveryHarness(t *testing.T) *deliveryHarness {
	t.Helper()
	jobs := memory.NewJobRepo()
	deliveries := memory.NewDeliveryRepo()
	artifacts := memory.NewArtifactRepo()
	objects := memory.NewObjectStore()
	vk := vkdelivery.NewMockClient()
	billingRpo := memory.NewBillingRepo()
	billing := billingservice.New(billingRpo, billingservice.WithStartingBalance(1000))
	dw := worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:       jobs,
		Deliveries: deliveries,
		Artifacts:  artifacts,
		Objects:    objects,
		VK:         vk,
		Billing:    billing,
	})
	return &deliveryHarness{
		jobs:       jobs,
		deliveries: deliveries,
		artifacts:  artifacts,
		objects:    objects,
		vk:         vk,
		billingRpo: billingRpo,
		billing:    billing,
		worker:     dw,
	}
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
		VKPeerID:        555,
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
		ID:            uuid.New(),
		OwnerUserID:   userID,
		JobID:         &job.ID,
		Kind:          domain.ArtifactKindOutput,
		MediaType:     mediaType,
		StorageBucket: "artifacts",
		StorageKey:    "k/" + job.ID.String(),
		SHA256:        uuid.NewString(),
		Status:        domain.ArtifactStatusReady,
	}
	if err := h.artifacts.Create(ctx, art); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	_ = h.objects.Put(ctx, art.StorageBucket, art.StorageKey, []byte(body), "text/plain")
	job.OutputArtifactIDs = []uuid.UUID{art.ID}
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("attach artifact: %v", err)
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
}

func TestDeliveryUploadsRawPhotoArtifactToVK(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{}
	h.worker = worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:       h.jobs,
		Deliveries: h.deliveries,
		Artifacts:  h.artifacts,
		Objects:    h.objects,
		VK:         h.vk,
		VKUploader: uploader,
		Billing:    h.billing,
	})
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

func TestDeliveryNamesRawVideoArtifactFromPrompt(t *testing.T) {
	h := newDeliveryHarness(t)
	uploader := &fakeVKUploader{}
	h.worker = worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:       h.jobs,
		Deliveries: h.deliveries,
		Artifacts:  h.artifacts,
		Objects:    h.objects,
		VK:         h.vk,
		VKUploader: uploader,
		Billing:    h.billing,
	})
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
	h.worker = worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:                   h.jobs,
		Deliveries:             h.deliveries,
		Artifacts:              h.artifacts,
		Objects:                h.objects,
		VK:                     h.vk,
		VKUploader:             uploader,
		Billing:                h.billing,
		RawVideoDeliveryPolicy: "if_probe_passed",
	})
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
	h.worker = worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:                   h.jobs,
		Deliveries:             h.deliveries,
		Artifacts:              h.artifacts,
		Objects:                h.objects,
		VK:                     h.vk,
		VKUploader:             uploader,
		Billing:                h.billing,
		MaxAttempts:            1,
		RawVideoDeliveryPolicy: "never",
	})
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
			h.worker = worker.NewDeliveryWorker(worker.DeliveryDeps{
				Jobs:       h.jobs,
				Deliveries: h.deliveries,
				Artifacts:  h.artifacts,
				Objects:    h.objects,
				VK:         h.vk,
				VKUploader: uploader,
				Billing:    h.billing,
			})
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
	h.worker = worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:       h.jobs,
		Deliveries: h.deliveries,
		Artifacts:  h.artifacts,
		Objects:    h.objects,
		VK:         h.vk,
		VKUploader: uploader,
		Billing:    h.billing,
	})
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
	h.worker = worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:        h.jobs,
		Deliveries:  h.deliveries,
		Artifacts:   h.artifacts,
		Objects:     h.objects,
		VK:          h.vk,
		VKUploader:  uploader,
		Billing:     h.billing,
		MaxAttempts: 2,
	})
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
	h.worker = worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:       h.jobs,
		Deliveries: h.deliveries,
		Artifacts:  h.artifacts,
		Objects:    h.objects,
		VK:         h.vk,
		VKUploader: uploader,
		Billing:    h.billing,
	})
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
