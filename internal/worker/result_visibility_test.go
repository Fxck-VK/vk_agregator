package worker_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/provider/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/moderationservice"
	"vk-ai-aggregator/internal/service/resultservice"
	"vk-ai-aggregator/internal/worker"
)

type recordingModerator struct {
	inputs []moderationservice.Input
}

func (m *recordingModerator) Name() string { return "recording-moderator" }

func (m *recordingModerator) Check(_ context.Context, input moderationservice.Input) (moderationservice.Outcome, error) {
	m.inputs = append(m.inputs, input)
	return moderationservice.Outcome{Decision: domain.ModerationAllow}, nil
}

type multiOutputProvider struct {
	*mock.Provider
}

func (p *multiOutputProvider) Poll(ctx context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	result, err := p.Provider.Poll(ctx, ref)
	if err == nil && result.Status == domain.ProviderTaskSucceeded {
		result.OutputURLs = []string{
			"mock://" + ref.ExternalID + "/first.png",
			"mock://" + ref.ExternalID + "/second.png",
		}
		result.Text = ""
	}
	return result, err
}

type recordingDeliveryBiller struct {
	captures int
	releases int
}

func (b *recordingDeliveryBiller) CaptureForJob(context.Context, uuid.UUID, int64) error {
	b.captures++
	return nil
}

func (b *recordingDeliveryBiller) ReleaseForJob(context.Context, uuid.UUID) error {
	b.releases++
	return nil
}

type failingModerationRepository struct {
	inner *memory.ModerationRepo
	err   error
	calls int
}

type failOnceAtModerationRepository struct {
	inner  *memory.ModerationRepo
	err    error
	failAt int
	calls  int
	failed bool
}

func (r *failOnceAtModerationRepository) Create(ctx context.Context, result *domain.ModerationResult) error {
	r.calls++
	if !r.failed && r.calls == r.failAt {
		r.failed = true
		return r.err
	}
	return r.inner.Create(ctx, result)
}

func (r *failOnceAtModerationRepository) ListByJob(ctx context.Context, jobID uuid.UUID) ([]*domain.ModerationResult, error) {
	return r.inner.ListByJob(ctx, jobID)
}

func (r *failingModerationRepository) Create(ctx context.Context, result *domain.ModerationResult) error {
	r.calls++
	return r.err
}

func (r *failingModerationRepository) ListByJob(ctx context.Context, jobID uuid.UUID) ([]*domain.ModerationResult, error) {
	return r.inner.ListByJob(ctx, jobID)
}

func TestGenerationTextPersistsDurableModeratedArtifactBeforeAccountFinalization(t *testing.T) {
	ctx := context.Background()
	moderator := &recordingModerator{}
	h := newHarnessWithProvider(t, mock.New(), func(deps *worker.Deps) {
		deps.Moderator = moderator
	})
	job := h.queueJob(t, domain.OperationTextGenerate, domain.ModalityText, "safe answer")
	job.AccountID = job.UserID
	job.Source = "web"
	job.ResultMode = domain.ResultModeAccountHistory
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("configure account-history job: %v", err)
	}

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("generate: %v", err)
	}

	ready := h.reload(t, job.ID)
	if ready.Status != domain.JobStatusResultReady || len(ready.OutputArtifactIDs) != 1 {
		t.Fatalf("generated job = status %q outputs %v, want result_ready with one text artifact", ready.Status, ready.OutputArtifactIDs)
	}
	artifact, err := h.artRepo.GetByIDForAccount(ctx, ready.AccountID, ready.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("load account-owned text artifact: %v", err)
	}
	if artifact.OwnerAccountID != ready.AccountID ||
		artifact.OwnerUserID != ready.UserID ||
		artifact.JobID == nil ||
		*artifact.JobID != ready.ID ||
		artifact.Kind != domain.ArtifactKindOutput ||
		artifact.MediaType != domain.MediaTypeText ||
		artifact.Status != domain.ArtifactStatusReady {
		t.Fatalf("unexpected durable text artifact: %+v", artifact)
	}
	body, err := h.store.GetObject(ctx, artifact.StorageBucket, artifact.StorageKey)
	if err != nil {
		t.Fatalf("load stored text body: %v", err)
	}
	if !strings.Contains(string(body), "Mock generated text result") || string(body) == "output" {
		t.Fatalf("stored text body = %q, want canonical provider text", string(body))
	}
	verdicts, err := h.modRepo.ListByJob(ctx, ready.ID)
	if err != nil {
		t.Fatalf("list moderation verdicts: %v", err)
	}
	if len(verdicts) != 1 ||
		verdicts[0].ArtifactID == nil ||
		*verdicts[0].ArtifactID != artifact.ID ||
		!verdicts[0].Decision.Allowed() {
		t.Fatalf("text moderation verdicts = %+v, want one matching allow", verdicts)
	}
	if len(moderator.inputs) != 1 || moderator.inputs[0].Text != string(body) {
		t.Fatalf("moderator inputs = %+v, want exact durable text body", moderator.inputs)
	}

	results := resultservice.New(h.jobs, h.artRepo, h.modRepo)
	if err := results.RequireCompletionReady(ctx, ready.AccountID, ready.ID); err != nil {
		t.Fatalf("worker readiness before capture: %v", err)
	}
	if _, err := results.GetResult(ctx, ready.AccountID, ready.ID); err != domain.ErrNotFound {
		t.Fatalf("public result before capture = %v, want exact ErrNotFound", err)
	}

	biller := &recordingDeliveryBiller{}
	finalizer := worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:      h.jobs,
		Readiness: results,
		Billing:   biller,
	})
	if err := finalizer.Process(ctx, deliveryTask(ready)); err != nil {
		t.Fatalf("finalize account history: %v", err)
	}
	if biller.captures != 1 {
		t.Fatalf("capture calls = %d, want 1", biller.captures)
	}
	publicResult, err := results.GetResult(ctx, ready.AccountID, ready.ID)
	if err != nil {
		t.Fatalf("public result after finalization: %v", err)
	}
	if len(publicResult.Artifacts) != 1 || publicResult.Artifacts[0].ID != artifact.ID {
		t.Fatalf("public result artifacts = %+v, want text artifact %s", publicResult.Artifacts, artifact.ID)
	}
}

func TestGenerationModeratesEveryOutputAndReadinessRejectsMissingOrBlockedVerdicts(t *testing.T) {
	ctx := context.Background()
	moderator := &recordingModerator{}
	h := newHarnessWithProvider(t, &multiOutputProvider{Provider: mock.New()}, func(deps *worker.Deps) {
		deps.Moderator = moderator
	})
	job := h.queueJob(t, domain.OperationImageGenerate, domain.ModalityImage, "two safe images")
	job.AccountID = job.UserID
	job.Source = "web"
	job.ResultMode = domain.ResultModeAccountHistory
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("configure account-history job: %v", err)
	}

	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("generate: %v", err)
	}
	ready := h.reload(t, job.ID)
	if ready.Status != domain.JobStatusResultReady || len(ready.OutputArtifactIDs) != 2 {
		t.Fatalf("generated job = status %q outputs %v, want two result-ready outputs", ready.Status, ready.OutputArtifactIDs)
	}
	verdicts, err := h.modRepo.ListByJob(ctx, ready.ID)
	if err != nil {
		t.Fatalf("list moderation verdicts: %v", err)
	}
	if len(verdicts) != 2 || len(moderator.inputs) != 2 {
		t.Fatalf("verdicts/moderator calls = %d/%d, want 2/2", len(verdicts), len(moderator.inputs))
	}
	seen := map[uuid.UUID]bool{}
	for _, verdict := range verdicts {
		if verdict.ArtifactID == nil || !verdict.Decision.Allowed() {
			t.Fatalf("unsafe or unlinked verdict: %+v", verdict)
		}
		seen[*verdict.ArtifactID] = true
	}
	for _, artifactID := range ready.OutputArtifactIDs {
		if !seen[artifactID] {
			t.Fatalf("output %s has no matching moderation verdict: %+v", artifactID, verdicts)
		}
	}

	partial := memory.NewModerationRepo()
	if err := partial.Create(ctx, verdicts[0]); err != nil {
		t.Fatalf("create partial verdict: %v", err)
	}
	if err := resultservice.New(h.jobs, h.artRepo, partial).RequireCompletionReady(ctx, ready.AccountID, ready.ID); err != domain.ErrNotFound {
		t.Fatalf("readiness with a missing verdict = %v, want exact ErrNotFound", err)
	}

	blockedID := ready.OutputArtifactIDs[1]
	if err := h.modRepo.Create(ctx, &domain.ModerationResult{
		JobID:      ready.ID,
		ArtifactID: &blockedID,
		Stage:      domain.ModerationStageOutput,
		Decision:   domain.ModerationBlock,
		Provider:   "conflicting-test-verdict",
	}); err != nil {
		t.Fatalf("create conflicting block verdict: %v", err)
	}
	if err := resultservice.New(h.jobs, h.artRepo, h.modRepo).RequireCompletionReady(ctx, ready.AccountID, ready.ID); err != domain.ErrNotFound {
		t.Fatalf("readiness with conflicting block verdict = %v, want exact ErrNotFound", err)
	}
}

func TestModerationPersistenceFailureStopsBeforeResultReadyAndDialogSave(t *testing.T) {
	ctx := context.Background()
	textContext := &fakeTextContext{preparedPrompt: "context packet\n"}
	persistErr := errors.New("moderation repository unavailable")
	failingRepo := &failingModerationRepository{
		inner: memory.NewModerationRepo(),
		err:   persistErr,
	}
	h := newHarnessCore(t, mock.New(), textContext, func(deps *worker.Deps) {
		deps.Moderator = moderationservice.NewKeywordModerator()
		deps.ModResults = failingRepo
	})
	job := h.queueJob(t, domain.OperationTextGenerate, domain.ModalityText, "safe answer")
	job.AccountID = job.UserID
	job.Source = "web"
	job.ResultMode = domain.ResultModeAccountHistory
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("configure account-history job: %v", err)
	}

	err := h.gen.Process(ctx, taskFor(job))
	if !errors.Is(err, persistErr) {
		t.Fatalf("generation error = %v, want moderation persistence error", err)
	}
	got := h.reload(t, job.ID)
	if got.Status == domain.JobStatusResultReady || got.Status == domain.JobStatusSucceeded {
		t.Fatalf("job advanced after moderation persistence failure: %+v", got)
	}
	if resultReadyEventCount(h.outbox, job.ID) != 0 {
		t.Fatalf("moderation persistence failure created result-ready event: %+v", h.outbox.Events())
	}
	if textContext.completeCalls != 0 {
		t.Fatalf("moderation persistence failure saved dialog answer: calls=%d", textContext.completeCalls)
	}
	if failingRepo.calls != 1 {
		t.Fatalf("moderation persistence calls = %d, want 1", failingRepo.calls)
	}
}

func TestModerationRetryReusesPartiallyPersistedAllowedVerdicts(t *testing.T) {
	ctx := context.Background()
	moderator := &recordingModerator{}
	repository := &failOnceAtModerationRepository{
		inner:  memory.NewModerationRepo(),
		err:    errors.New("second moderation insert unavailable"),
		failAt: 2,
	}
	h := newHarnessCore(t, &multiOutputProvider{Provider: mock.New()}, nil, func(deps *worker.Deps) {
		deps.Moderator = moderator
		deps.ModResults = repository
	})
	job := h.queueJob(t, domain.OperationImageGenerate, domain.ModalityImage, "two safe images")
	job.AccountID = job.UserID
	job.Source = "web"
	job.ResultMode = domain.ResultModeAccountHistory
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("configure account-history job: %v", err)
	}

	if err := h.gen.Process(ctx, taskFor(job)); !errors.Is(err, repository.err) {
		t.Fatalf("first generation error = %v, want second moderation insert failure", err)
	}
	partiallyModerated := h.reload(t, job.ID)
	if partiallyModerated.Status == domain.JobStatusResultReady || resultReadyEventCount(h.outbox, job.ID) != 0 {
		t.Fatalf("partial moderation advanced job: %+v events=%+v", partiallyModerated, h.outbox.Events())
	}
	firstVerdicts, err := repository.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list partial verdicts: %v", err)
	}
	if len(firstVerdicts) != 1 {
		t.Fatalf("partial verdict count = %d, want 1", len(firstVerdicts))
	}

	if err := h.gen.Process(ctx, taskFor(partiallyModerated)); err != nil {
		t.Fatalf("retry generation: %v", err)
	}
	ready := h.reload(t, job.ID)
	verdicts, err := repository.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list completed verdicts: %v", err)
	}
	if ready.Status != domain.JobStatusResultReady ||
		len(verdicts) != 2 ||
		repository.calls != 3 ||
		resultReadyEventCount(h.outbox, job.ID) != 1 {
		t.Fatalf("retry result: job=%+v verdicts=%+v create_calls=%d events=%+v", ready, verdicts, repository.calls, h.outbox.Events())
	}
}

func TestTextRecoveryModeratesStoredBodyWhenProviderTextIsUnavailable(t *testing.T) {
	ctx := context.Background()
	moderator := &recordingModerator{}
	h := newHarnessWithProvider(t, mock.New(), func(deps *worker.Deps) {
		deps.Moderator = moderator
	})
	job := h.queueJob(t, domain.OperationTextGenerate, domain.ModalityText, "safe answer")
	job.AccountID = job.UserID
	job.Source = "web"
	job.ResultMode = domain.ResultModeAccountHistory
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("configure account-history job: %v", err)
	}
	if err := h.jobs.UpdateStatus(ctx, job.ID, domain.JobStatusQueued, domain.JobStatusProviderPending, "", ""); err != nil {
		t.Fatalf("set provider pending: %v", err)
	}
	job.Status = domain.JobStatusProviderPending

	const storedAnswer = "durable recovered answer"
	artifact := &domain.Artifact{
		ID:             uuid.New(),
		OwnerUserID:    job.UserID,
		OwnerAccountID: job.AccountID,
		JobID:          &job.ID,
		Kind:           domain.ArtifactKindOutput,
		MediaType:      domain.MediaTypeText,
		MimeType:       "text/plain; charset=utf-8",
		StorageBucket:  "artifacts",
		StorageKey:     "outputs/" + uuid.NewString() + ".txt",
		SHA256:         uuid.NewString(),
		SizeBytes:      int64(len(storedAnswer)),
		Status:         domain.ArtifactStatusReady,
	}
	if err := h.store.Put(ctx, artifact.StorageBucket, artifact.StorageKey, []byte(storedAnswer), artifact.MimeType); err != nil {
		t.Fatalf("store recovered text: %v", err)
	}
	if err := h.artRepo.Create(ctx, artifact); err != nil {
		t.Fatalf("create recovered text artifact: %v", err)
	}
	job.OutputArtifactIDs = []uuid.UUID{artifact.ID}
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("link recovered text artifact: %v", err)
	}
	taskResult := domain.DurableProviderTaskResultJSON(domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded})
	if err := h.tasks.Create(ctx, &domain.ProviderTask{
		JobID:          job.ID,
		Provider:       domain.ProviderMock,
		ExternalID:     "recovered-text-task",
		Status:         domain.ProviderTaskSucceeded,
		Result:         taskResult,
		IdempotencyKey: "provider_submit:" + job.ID.String() + ":1",
	}); err != nil {
		t.Fatalf("create recovered provider task: %v", err)
	}

	if err := h.poll.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("recover provider result: %v", err)
	}
	ready := h.reload(t, job.ID)
	verdicts, err := h.modRepo.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list recovered moderation verdicts: %v", err)
	}
	if ready.Status != domain.JobStatusResultReady ||
		len(moderator.inputs) != 1 ||
		moderator.inputs[0].Text != storedAnswer ||
		len(verdicts) != 1 ||
		verdicts[0].ArtifactID == nil ||
		*verdicts[0].ArtifactID != artifact.ID {
		t.Fatalf("text recovery result: job=%+v inputs=%+v verdicts=%+v", ready, moderator.inputs, verdicts)
	}
}

func TestAccountHistoryFinalizerWaitsForReadinessThenCapturesExactlyOnce(t *testing.T) {
	ctx := context.Background()
	h := newDeliveryHarness(t)
	job := h.resultReadyJob(t, domain.MediaTypeImage, "generated image")
	job.AccountID = job.UserID
	job.Source = "web"
	job.ResultMode = domain.ResultModeAccountHistory
	job.DeliveryTarget = nil
	artifact, err := h.artifacts.GetByID(ctx, job.OutputArtifactIDs[0])
	if err != nil {
		t.Fatalf("load output artifact: %v", err)
	}
	artifact.OwnerAccountID = job.AccountID
	if err := h.artifacts.Update(ctx, artifact); err != nil {
		t.Fatalf("set canonical artifact owner: %v", err)
	}
	if err := h.jobs.Update(ctx, job); err != nil {
		t.Fatalf("configure account-history job: %v", err)
	}

	moderation := memory.NewModerationRepo()
	readiness := resultservice.New(h.jobs, h.artifacts, moderation)
	h.worker = h.deliveryWorker(vkPublisherDepsForTest(h), worker.DeliveryDeps{Readiness: readiness})

	if err := h.worker.Process(ctx, deliveryTask(job)); err == nil {
		t.Fatal("expected finalizer to wait for missing readiness data")
	}
	got, err := h.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload waiting job: %v", err)
	}
	deliveries, err := h.deliveries.ListByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if got.Status != domain.JobStatusResultReady ||
		got.CostCaptured != 0 ||
		len(deliveries) != 0 ||
		len(h.vk.Sent()) != 0 ||
		h.captureEntryCount(t, job.UserID) != 0 {
		t.Fatalf("unready finalization mutated state: job=%+v deliveries=%+v sends=%+v", got, deliveries, h.vk.Sent())
	}

	artifactID := artifact.ID
	if err := moderation.Create(ctx, &domain.ModerationResult{
		JobID:      job.ID,
		ArtifactID: &artifactID,
		Stage:      domain.ModerationStageOutput,
		Decision:   domain.ModerationAllow,
		Provider:   "test",
	}); err != nil {
		t.Fatalf("supply missing moderation verdict: %v", err)
	}
	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("finalize after readiness supplied: %v", err)
	}
	if err := h.worker.Process(ctx, deliveryTask(job)); err != nil {
		t.Fatalf("repeat finalized task: %v", err)
	}
	got, err = h.jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("reload finalized job: %v", err)
	}
	if got.Status != domain.JobStatusSucceeded || got.CostCaptured != 10 {
		t.Fatalf("finalized job = %+v, want succeeded with one capture", got)
	}
	if h.captureEntryCount(t, job.UserID) != 1 || len(h.vk.Sent()) != 0 {
		t.Fatalf("finalizer side effects: captures=%d sends=%+v", h.captureEntryCount(t, job.UserID), h.vk.Sent())
	}
}

func vkPublisherDepsForTest(h *deliveryHarness) vkdelivery.PublisherDeps {
	return vkdelivery.PublisherDeps{Uploader: h.uploader}
}
