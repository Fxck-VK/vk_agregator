package worker_test

import (
	"context"
	"errors"
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
	billing := billingservice.New(billingRpo)
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
	photoBytes []byte
	videoBytes []byte
}

func (u *fakeVKUploader) UploadPhoto(_ context.Context, peerID int64, filename string, data []byte, _ string) (string, error) {
	u.photoBytes = append([]byte(nil), data...)
	return "photo123_456_key", nil
}

func (u *fakeVKUploader) UploadVideo(_ context.Context, peerID int64, filename string, data []byte, _ string) (string, error) {
	u.videoBytes = append([]byte(nil), data...)
	return "video123_456_key", nil
}

// resultReadyJob creates a user account, reserves credits, stores an output
// artifact and a job in result_ready, returning the job.
func (h *deliveryHarness) resultReadyJob(t *testing.T, mediaType domain.MediaType, body string) *domain.Job {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	if _, err := h.billing.EnsureAccount(ctx, userID); err != nil {
		t.Fatalf("ensure account: %v", err)
	}

	job := &domain.Job{
		ID:             uuid.New(),
		UserID:         userID,
		VKPeerID:       555,
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusResultReady,
		IdempotencyKey: "job:" + uuid.NewString(),
		CostReserved:   10,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if _, err := h.billing.Reserve(ctx, userID, job.ID, 10); err != nil {
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
	if len(h.vk.Sent()) != 1 || h.vk.Sent()[0].Type != "photo" {
		t.Fatalf("expected one photo send, got %+v", h.vk.Sent())
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
