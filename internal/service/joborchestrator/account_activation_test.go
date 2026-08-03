package joborchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/uow"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

// preparedJobActivator keeps the activation API testable before it exists: the
// test fails as a normal assertion instead of failing to compile while the
// method is still absent.
type preparedJobActivator interface {
	ActivatePreparedAccountJob(context.Context, uuid.UUID, uuid.UUID) (*domain.Job, error)
}

type expiredAccountImageJobRetrier interface {
	RetryExpiredAccountImageJob(context.Context, uuid.UUID, uuid.UUID) (*domain.Job, error)
}

func retryExpiredAccountImageJob(t *testing.T, orch *joborchestrator.Orchestrator, accountID, originalJobID uuid.UUID) (*domain.Job, error) {
	t.Helper()
	retrier, ok := any(orch).(expiredAccountImageJobRetrier)
	if !ok {
		t.Fatal("RetryExpiredAccountImageJob is not implemented")
	}
	return retrier.RetryExpiredAccountImageJob(context.Background(), accountID, originalJobID)
}

func activatePreparedAccountJob(t *testing.T, orch *joborchestrator.Orchestrator, accountID, jobID uuid.UUID) (*domain.Job, error) {
	t.Helper()
	activator, ok := any(orch).(preparedJobActivator)
	if !ok {
		t.Fatal("ActivatePreparedAccountJob is not implemented")
	}
	return activator.ActivatePreparedAccountJob(context.Background(), accountID, jobID)
}

func prepareActivatableAccountJob(t *testing.T, f *fixture, accountID uuid.UUID, key string, estimate int64) *domain.Job {
	t.Helper()
	job, err := f.orch.PrepareAccountJob(context.Background(), joborchestrator.PrepareAccountJobInput{
		AccountID:           accountID,
		Operation:           domain.OperationTextGenerate,
		Modality:            domain.ModalityText,
		IdempotencyKey:      key,
		CorrelationID:       "activation-test",
		CostEstimateCredits: estimate,
	})
	if err != nil {
		t.Fatalf("prepare account job: %v", err)
	}
	return job
}

func prepareExpiredAccountImageJob(t *testing.T, f *fixture, accountID uuid.UUID, key string) *domain.Job {
	t.Helper()
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static pricing catalog: %v", err)
	}
	snapshot, err := catalog.Snapshot(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: pricingcatalog.PublicImageNanoBanana2,
		Quality:      pricingcatalog.ImageQuality1K,
	})
	if err != nil {
		t.Fatalf("image pricing snapshot: %v", err)
	}
	artifactID := seedInputArtifactForAccount(t, f, uuid.Nil, accountID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, "private-artifacts", "retry/input.png")
	params := json.RawMessage(`{"prompt":"night city","provider":"poyo","model_code":"trusted-private-code","image_quality":"1K"}`)
	job, err := f.orch.PrepareAccountJob(context.Background(), joborchestrator.PrepareAccountJobInput{
		AccountID:        accountID,
		Operation:        domain.OperationImageGenerate,
		Modality:         domain.ModalityImage,
		IdempotencyKey:   key,
		CorrelationID:    "retry-original:" + key,
		InputArtifactIDs: []uuid.UUID{artifactID},
		Params:           params,
		PricingSnapshot:  snapshot,
	})
	if err != nil {
		t.Fatalf("prepare original image job: %v", err)
	}
	if err := f.jobs.UpdateStatus(context.Background(), job.ID, domain.JobStatusPrepared, domain.JobStatusExpired, domain.PreparedConfirmationExpiredCode, domain.PreparedConfirmationExpiredMessage); err != nil {
		t.Fatalf("expire original image job: %v", err)
	}
	job.Status = domain.JobStatusExpired
	job.ErrorCode = domain.PreparedConfirmationExpiredCode
	job.ErrorMessage = domain.PreparedConfirmationExpiredMessage
	return job
}

func TestRetryExpiredAccountImageJobCreatesOneQueuedAccountJobAndReplaysIt(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(100))
	accountID := uuid.New()
	original := prepareExpiredAccountImageJob(t, f, accountID, "web:image:retry:queued-original")

	retried, err := retryExpiredAccountImageJob(t, f.orch, accountID, original.ID)
	if err != nil {
		t.Fatalf("retry expired image job: %v", err)
	}
	if retried == nil || retried.ID == original.ID || retried.Status != domain.JobStatusQueued {
		t.Fatalf("retried job = %+v, want a new queued job", retried)
	}
	if retried.AccountID != accountID || retried.UserID != uuid.Nil || retried.Source != "web" || retried.ChannelContext == nil || retried.ChannelContext.Channel != domain.ChannelWeb || retried.ResultMode != domain.ResultModeAccountHistory || retried.DeliveryTarget != nil {
		t.Fatalf("retry ownership/result contract = %+v, want account-native Web history", retried)
	}
	if string(retried.Params) != string(original.Params) || len(retried.InputArtifactIDs) != 1 || retried.InputArtifactIDs[0] != original.InputArtifactIDs[0] || string(retried.PricingSnapshot) != string(original.PricingSnapshot) {
		t.Fatalf("retry did not preserve trusted stored facts: original=%+v retry=%+v", original, retried)
	}
	reservation, err := f.bill.GetReservationByJob(context.Background(), retried.ID)
	if err != nil || reservation.OwnerAccountID != accountID || reservation.Amount != retried.CostEstimate {
		t.Fatalf("retry reservation = %+v, %v", reservation, err)
	}
	if events := f.outbox.Events(); len(events) != 3 || events[0].EventType != "event.job.created" || events[1].EventType != "event.job.created" || events[2].EventType != "event.job.queued" {
		t.Fatalf("retry events = %+v, want original created plus one retry created/queued", events)
	}

	replayed, err := retryExpiredAccountImageJob(t, f.orch, accountID, original.ID)
	if err != nil || replayed == nil || replayed.ID != retried.ID || replayed.Status != domain.JobStatusQueued {
		t.Fatalf("retry replay = %+v, %v; want same queued job", replayed, err)
	}
	if events := f.outbox.Events(); len(events) != 3 {
		t.Fatalf("retry replay wrote duplicate events: %+v", events)
	}
	replayedReservation, err := f.bill.GetReservationByJob(context.Background(), retried.ID)
	if err != nil || replayedReservation.ID != reservation.ID {
		t.Fatalf("retry replay reservation = %+v, %v; want original reservation %+v", replayedReservation, err, reservation)
	}
	storedOriginal, err := f.jobs.GetByID(context.Background(), original.ID)
	if err != nil || storedOriginal.Status != domain.JobStatusExpired || storedOriginal.ErrorCode != domain.PreparedConfirmationExpiredCode || storedOriginal.CostReserved != 0 || storedOriginal.CostCaptured != 0 {
		t.Fatalf("original after retry = %+v, %v; want unchanged expired confirmation without billing", storedOriginal, err)
	}
}

func TestRetryExpiredAccountImageJobConcurrentCallsCreateOneRetryReservationAndQueueEvent(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(100))
	accountID := uuid.New()
	original := prepareExpiredAccountImageJob(t, f, accountID, "web:image:retry:concurrent-original")
	retrier, ok := any(f.orch).(expiredAccountImageJobRetrier)
	if !ok {
		t.Fatal("RetryExpiredAccountImageJob is not implemented")
	}

	type result struct {
		job *domain.Job
		err error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			job, err := retrier.RetryExpiredAccountImageJob(context.Background(), accountID, original.ID)
			results <- result{job: job, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var retryID uuid.UUID
	for result := range results {
		if result.err != nil || result.job == nil || result.job.Status != domain.JobStatusQueued || result.job.ID == original.ID {
			t.Fatalf("concurrent retry = %+v, %v; want queued retry", result.job, result.err)
		}
		if retryID == uuid.Nil {
			retryID = result.job.ID
		} else if result.job.ID != retryID {
			t.Fatalf("concurrent retries created %s and %s", retryID, result.job.ID)
		}
	}
	jobs, err := f.jobs.ListByAccount(context.Background(), accountID, 10, 0)
	if err != nil || len(jobs) != 2 {
		t.Fatalf("account jobs after concurrent retry = %+v, %v; want original plus one retry", jobs, err)
	}
	reservation, err := f.bill.GetReservationByJob(context.Background(), retryID)
	if err != nil || reservation.OwnerAccountID != accountID {
		t.Fatalf("concurrent retry reservation = %+v, %v", reservation, err)
	}
	if events := f.outbox.Events(); len(events) != 3 || events[2].EventType != "event.job.queued" {
		t.Fatalf("concurrent retry events = %+v, want one retry queue event", events)
	}
	storedOriginal, err := f.jobs.GetByID(context.Background(), original.ID)
	if err != nil || storedOriginal.Status != domain.JobStatusExpired || storedOriginal.ErrorCode != domain.PreparedConfirmationExpiredCode || storedOriginal.CostReserved != 0 || storedOriginal.CostCaptured != 0 {
		t.Fatalf("original after concurrent retry = %+v, %v", storedOriginal, err)
	}
}

func TestRetryExpiredAccountImageJobInsufficientCreditsCreatesOneResumableJobWithoutQueue(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(1))
	countingBiller := &retryCountingBiller{Service: f.billing}
	f.orch = joborchestrator.New(
		f.jobs,
		activationTestUnitOfWork{repos: uow.Repositories{Jobs: f.jobs, Outbox: f.outbox, Billing: f.bill}},
		countingBiller,
		0,
		joborchestrator.WithArtifactRepository(f.arts),
	)
	accountID := uuid.New()
	original := prepareExpiredAccountImageJob(t, f, accountID, "web:image:retry:insufficient-original")

	retried, err := retryExpiredAccountImageJob(t, f.orch, accountID, original.ID)
	if !errors.Is(err, domain.ErrInsufficientCredits) || retried == nil || retried.ID == original.ID || retried.Status != domain.JobStatusAwaitingPayment {
		t.Fatalf("insufficient retry = %+v, %v; want new awaiting_payment job", retried, err)
	}
	if _, err := f.bill.GetReservationByJob(context.Background(), retried.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("insufficient retry created reservation: %v", err)
	}
	if events := f.outbox.Events(); len(events) != 2 || events[0].EventType != "event.job.created" || events[1].EventType != "event.job.created" {
		t.Fatalf("insufficient retry events = %+v, want no queue event", events)
	}
	if countingBiller.reserveWithOwnerCalls != 1 || countingBiller.reserveForAccountCalls != 0 {
		t.Fatalf("first insufficient retry reservation attempts = create:%d activate:%d, want one create attempt", countingBiller.reserveWithOwnerCalls, countingBiller.reserveForAccountCalls)
	}

	replayed, replayErr := retryExpiredAccountImageJob(t, f.orch, accountID, original.ID)
	if !errors.Is(replayErr, domain.ErrInsufficientCredits) || replayed == nil || replayed.ID != retried.ID || replayed.Status != domain.JobStatusAwaitingPayment {
		t.Fatalf("insufficient retry replay = %+v, %v", replayed, replayErr)
	}
	if events := f.outbox.Events(); len(events) != 2 {
		t.Fatalf("insufficient retry replay wrote duplicate events: %+v", events)
	}
	if err := f.billing.GrantWithAccount(context.Background(), f.bill, accountID, 100, "retry-top-up:"+original.ID.String(), "retry recovery test"); err != nil {
		t.Fatalf("top up retry account: %v", err)
	}
	queued, queuedErr := retryExpiredAccountImageJob(t, f.orch, accountID, original.ID)
	if queuedErr != nil || queued == nil || queued.ID != retried.ID || queued.Status != domain.JobStatusQueued {
		t.Fatalf("retry after top-up = %+v, %v; want same queued job", queued, queuedErr)
	}
	reservation, err := f.bill.GetReservationByJob(context.Background(), retried.ID)
	if err != nil || reservation.OwnerAccountID != accountID || reservation.Amount != retried.CostEstimate {
		t.Fatalf("retry reservation after top-up = %+v, %v", reservation, err)
	}
	if events := f.outbox.Events(); len(events) != 3 || events[2].EventType != "event.job.queued" {
		t.Fatalf("retry after top-up events = %+v, want exactly one queue event", events)
	}
	queuedReplay, queuedReplayErr := retryExpiredAccountImageJob(t, f.orch, accountID, original.ID)
	if queuedReplayErr != nil || queuedReplay == nil || queuedReplay.ID != retried.ID || queuedReplay.Status != domain.JobStatusQueued {
		t.Fatalf("queued retry replay = %+v, %v", queuedReplay, queuedReplayErr)
	}
	if events := f.outbox.Events(); len(events) != 3 {
		t.Fatalf("queued retry replay wrote duplicate events: %+v", events)
	}
	storedOriginal, err := f.jobs.GetByID(context.Background(), original.ID)
	if err != nil || storedOriginal.Status != domain.JobStatusExpired || storedOriginal.ErrorCode != domain.PreparedConfirmationExpiredCode || storedOriginal.CostReserved != 0 || storedOriginal.CostCaptured != 0 {
		t.Fatalf("original after top-up retry = %+v, %v", storedOriginal, err)
	}
}

type retryCountingBiller struct {
	*billingservice.Service
	reserveWithOwnerCalls  int
	reserveForAccountCalls int
}

func (b *retryCountingBiller) ReserveWithOwner(ctx context.Context, repo domain.BillingRepository, userID, accountID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error) {
	b.reserveWithOwnerCalls++
	return b.Service.ReserveWithOwner(ctx, repo, userID, accountID, jobID, amount)
}

func (b *retryCountingBiller) ReserveForAccountWith(ctx context.Context, repo domain.BillingRepository, accountID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error) {
	b.reserveForAccountCalls++
	return b.Service.ReserveForAccountWith(ctx, repo, accountID, jobID, amount)
}

func TestPrepareAccountJobWritesWebAccountHistoryContract(t *testing.T) {
	f := newFixture()
	accountID := uuid.New()

	job := prepareActivatableAccountJob(t, f, accountID, "web:prepare:contract", 25)
	if job.ChannelContext == nil || job.ChannelContext.Channel != domain.ChannelWeb {
		t.Fatalf("channel context = %+v, want web", job.ChannelContext)
	}
	if job.ResultMode != domain.ResultModeAccountHistory || job.DeliveryTarget != nil {
		t.Fatalf("result contract = mode %q target %+v, want account_history without target", job.ResultMode, job.DeliveryTarget)
	}
	if err := job.ValidateResultContract(); err != nil {
		t.Fatalf("prepared web result contract: %v", err)
	}
}

func TestActivatePreparedAccountJobReservesAndQueuesExactlyOnce(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(100))
	accountID := uuid.New()
	prepared := prepareActivatableAccountJob(t, f, accountID, "web:activate:one", 25)

	activated, err := activatePreparedAccountJob(t, f.orch, accountID, prepared.ID)
	if err != nil {
		t.Fatalf("activate prepared job: %v", err)
	}
	if activated.Status != domain.JobStatusQueued || activated.CostReserved != 25 {
		t.Fatalf("activated job = %+v, want queued with 25 reserved", activated)
	}
	if activated.UserID != uuid.Nil || activated.VKPeerID != 0 || activated.CommandID != uuid.Nil {
		t.Fatalf("activation fabricated legacy provenance: %+v", activated)
	}
	if activated.ChannelContext == nil || activated.ChannelContext.Channel != domain.ChannelWeb || activated.ResultMode != domain.ResultModeAccountHistory || activated.DeliveryTarget != nil {
		t.Fatalf("activation changed result contract: %+v", activated)
	}

	reservation, err := f.bill.GetReservationByJob(context.Background(), prepared.ID)
	if err != nil {
		t.Fatalf("reservation by job: %v", err)
	}
	if reservation.OwnerAccountID != accountID || reservation.Amount != 25 {
		t.Fatalf("reservation = %+v, want account-owned 25-credit hold", reservation)
	}
	account, err := f.bill.GetAccountByOwner(context.Background(), accountID, domain.CurrencyCredits)
	if err != nil || account.UserID != uuid.Nil || account.OwnerAccountID != accountID {
		t.Fatalf("native credit account = %+v, %v", account, err)
	}

	replayed, err := activatePreparedAccountJob(t, f.orch, accountID, prepared.ID)
	if err != nil || replayed.Status != domain.JobStatusQueued || replayed.ID != prepared.ID {
		t.Fatalf("activated replay = %+v, %v", replayed, err)
	}
	if events := f.outbox.Events(); len(events) != 2 || events[0].EventType != "event.job.created" || events[1].EventType != "event.job.queued" {
		t.Fatalf("events = %+v, want created then one queued", events)
	}
}

func TestActivatePreparedAccountJobInsufficientCreditsKeepsResumableAwaitingPayment(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(10))
	accountID := uuid.New()
	prepared := prepareActivatableAccountJob(t, f, accountID, "web:activate:insufficient", 25)

	activated, err := activatePreparedAccountJob(t, f.orch, accountID, prepared.ID)
	if !errors.Is(err, domain.ErrInsufficientCredits) {
		t.Fatalf("activate insufficient = %+v, %v; want ErrInsufficientCredits", activated, err)
	}
	if activated == nil || activated.Status != domain.JobStatusAwaitingPayment || activated.CostReserved != 0 {
		t.Fatalf("insufficient activation = %+v, want awaiting_payment without hold", activated)
	}
	if _, err := f.bill.GetReservationByJob(context.Background(), prepared.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("insufficient activation created reservation: %v", err)
	}
	if events := f.outbox.Events(); len(events) != 1 || events[0].EventType != "event.job.created" {
		t.Fatalf("insufficient activation events = %+v, want created only", events)
	}

	replayed, replayErr := activatePreparedAccountJob(t, f.orch, accountID, prepared.ID)
	if !errors.Is(replayErr, domain.ErrInsufficientCredits) || replayed == nil || replayed.Status != domain.JobStatusAwaitingPayment {
		t.Fatalf("insufficient replay = %+v, %v; want awaiting_payment and ErrInsufficientCredits", replayed, replayErr)
	}
}

func TestActivatePreparedAccountJobHidesForeignJob(t *testing.T) {
	f := newFixture()
	ownerID := uuid.New()
	prepared := prepareActivatableAccountJob(t, f, ownerID, "web:activate:foreign", 25)

	activated, err := activatePreparedAccountJob(t, f.orch, uuid.New(), prepared.ID)
	if !errors.Is(err, domain.ErrNotFound) || activated != nil {
		t.Fatalf("foreign activation = %+v, %v; want nil ErrNotFound", activated, err)
	}
	stored, err := f.jobs.GetByID(context.Background(), prepared.ID)
	if err != nil || stored.Status != domain.JobStatusPrepared || stored.CostReserved != 0 {
		t.Fatalf("foreign activation mutated job = %+v, %v", stored, err)
	}
	if events := f.outbox.Events(); len(events) != 1 || events[0].EventType != "event.job.created" {
		t.Fatalf("foreign activation events = %+v", events)
	}
}

func TestActivatePreparedAccountJobExpiresPastConfirmationDeadline(t *testing.T) {
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithMaxPreparedWebImageJobsPerAccount(2, time.Hour),
	}, billingservice.WithStartingBalance(100))
	accountID := uuid.New()
	prepared, err := f.orch.PrepareAccountJob(context.Background(), joborchestrator.PrepareAccountJobInput{
		AccountID:           accountID,
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "web:activate:expired-confirmation",
		CostEstimateCredits: 25,
	})
	if err != nil || prepared == nil || prepared.ExpiresAt == nil {
		t.Fatalf("prepare expiring image job = %+v, %v", prepared, err)
	}
	expiredAt := time.Now().Add(-time.Second)
	prepared.ExpiresAt = &expiredAt
	if err := f.jobs.Update(context.Background(), prepared); err != nil {
		t.Fatalf("expire prepared job: %v", err)
	}

	activated, err := activatePreparedAccountJob(t, f.orch, accountID, prepared.ID)
	if !errors.Is(err, domain.ErrConflict) || activated != nil {
		t.Fatalf("expired confirmation activation = %+v, %v; want nil ErrConflict", activated, err)
	}
	stored, err := f.jobs.GetByID(context.Background(), prepared.ID)
	if err != nil || stored.Status != domain.JobStatusExpired || stored.CostReserved != 0 || stored.CostCaptured != 0 {
		t.Fatalf("expired confirmation state = %+v, %v", stored, err)
	}
	if _, err := f.bill.GetReservationByJob(context.Background(), prepared.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired confirmation must not reserve credits, got %v", err)
	}
	if events := f.outbox.Events(); len(events) != 1 || events[0].EventType != "event.job.created" {
		t.Fatalf("expired confirmation must not enqueue work: %+v", events)
	}
}

func TestActivatePreparedAccountJobCommitsExpiredConfirmationBeforeReturningConflict(t *testing.T) {
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithMaxPreparedWebImageJobsPerAccount(2, time.Hour),
	})
	accountID := uuid.New()
	prepared, err := f.orch.PrepareAccountJob(context.Background(), joborchestrator.PrepareAccountJobInput{
		AccountID:           accountID,
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "web:activate:commit-expired-confirmation",
		CostEstimateCredits: 25,
	})
	if err != nil || prepared == nil || prepared.ExpiresAt == nil {
		t.Fatalf("prepare expiring image job = %+v, %v", prepared, err)
	}
	past := time.Now().Add(-time.Second)
	prepared.ExpiresAt = &past
	if err := f.jobs.Update(context.Background(), prepared); err != nil {
		t.Fatalf("expire prepared job: %v", err)
	}
	transaction := &recordingActivationUnitOfWork{repos: uow.Repositories{Jobs: f.jobs, Outbox: f.outbox, Billing: f.bill}}
	orch := joborchestrator.New(
		f.jobs,
		transaction,
		f.billing,
		0,
		joborchestrator.WithMaxPreparedWebImageJobsPerAccount(2, time.Hour),
	)

	activated, err := activatePreparedAccountJob(t, orch, accountID, prepared.ID)
	if !errors.Is(err, domain.ErrConflict) || activated != nil {
		t.Fatalf("expired confirmation activation = %+v, %v; want nil ErrConflict", activated, err)
	}
	if transaction.callbackErr != nil {
		t.Fatalf("expiry transition must commit before returning conflict, callback error = %v", transaction.callbackErr)
	}
	stored, err := f.jobs.GetByID(context.Background(), prepared.ID)
	if err != nil || stored.Status != domain.JobStatusExpired {
		t.Fatalf("expired confirmation state = %+v, %v", stored, err)
	}
}

func TestActivatePreparedAccountJobClearsConfirmationDeadline(t *testing.T) {
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithMaxPreparedWebImageJobsPerAccount(2, time.Hour),
	}, billingservice.WithStartingBalance(100))
	accountID := uuid.New()
	prepared, err := f.orch.PrepareAccountJob(context.Background(), joborchestrator.PrepareAccountJobInput{
		AccountID:           accountID,
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "web:activate:clear-confirmation-deadline",
		CostEstimateCredits: 25,
	})
	if err != nil || prepared == nil || prepared.ExpiresAt == nil {
		t.Fatalf("prepare expiring image job = %+v, %v", prepared, err)
	}

	activated, err := activatePreparedAccountJob(t, f.orch, accountID, prepared.ID)
	if err != nil || activated == nil || activated.Status != domain.JobStatusQueued {
		t.Fatalf("activate prepared job = %+v, %v; want queued", activated, err)
	}
	stored, err := f.jobs.GetByID(context.Background(), prepared.ID)
	if err != nil || stored.ExpiresAt != nil {
		t.Fatalf("activated job confirmation deadline = %v, %v; want nil", stored.ExpiresAt, err)
	}
}

func TestActivatePreparedAccountJobRechecksCapacityFromStoredFacts(t *testing.T) {
	var capacityErr error
	f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
		joborchestrator.WithCapacityGuard(joborchestrator.CapacityGuardFunc(func(context.Context, joborchestrator.CapacityCheckInput) error {
			return capacityErr
		})),
	})
	accountID := uuid.New()
	prepared := prepareActivatableAccountJob(t, f, accountID, "web:activate:capacity", 25)
	capacityErr = domain.ErrCapacityDegraded

	activated, err := activatePreparedAccountJob(t, f.orch, accountID, prepared.ID)
	if !errors.Is(err, domain.ErrCapacityDegraded) || activated != nil {
		t.Fatalf("capacity activation = %+v, %v; want nil ErrCapacityDegraded", activated, err)
	}
	stored, err := f.jobs.GetByID(context.Background(), prepared.ID)
	if err != nil || stored.Status != domain.JobStatusPrepared || stored.CostReserved != 0 {
		t.Fatalf("capacity failure mutated job = %+v, %v", stored, err)
	}
}

func TestActivatePreparedAccountJobConcurrentReplayCreatesOneHoldAndQueueEvent(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(100))
	accountID := uuid.New()
	prepared := prepareActivatableAccountJob(t, f, accountID, "web:activate:concurrent", 25)
	activator, ok := any(f.orch).(preparedJobActivator)
	if !ok {
		t.Fatal("ActivatePreparedAccountJob is not implemented")
	}

	start := make(chan struct{})
	jobs := make(chan *domain.Job, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			job, err := activator.ActivatePreparedAccountJob(context.Background(), accountID, prepared.ID)
			jobs <- job
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(jobs)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent activation: %v", err)
		}
	}
	for job := range jobs {
		if job == nil || job.ID != prepared.ID || job.Status != domain.JobStatusQueued {
			t.Fatalf("concurrent activation result = %+v, want queued job %s", job, prepared.ID)
		}
	}
	if events := f.outbox.Events(); len(events) != 2 || events[1].EventType != "event.job.queued" {
		t.Fatalf("concurrent activation events = %+v, want one queue event", events)
	}
	if reservation, err := f.bill.GetReservationByJob(context.Background(), prepared.ID); err != nil || reservation.Amount != 25 || reservation.OwnerAccountID != accountID {
		t.Fatalf("concurrent activation reservation = %+v, %v", reservation, err)
	}
}

func TestActivatePreparedAccountJobEnforcesVideoCapacityAcrossIndependentOrchestrators(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(100))
	accountID := uuid.New()
	first := prepareVideoActivatableAccountJob(t, f, accountID, "web:activate:video-capacity:first", 25)
	second := prepareVideoActivatableAccountJob(t, f, accountID, "web:activate:video-capacity:second", 25)

	barrier := &activationCapacityBarrier{
		JobRepository: f.jobs,
		accountID:     accountID,
		operation:     domain.OperationVideoGenerate,
		secondArrival: make(chan struct{}),
	}
	tx := activationTestUnitOfWork{repos: uow.Repositories{Jobs: barrier, Outbox: f.outbox, Billing: f.bill}}
	newOrchestrator := func() *joborchestrator.Orchestrator {
		return joborchestrator.New(
			f.jobs,
			tx,
			f.billing,
			0,
			joborchestrator.WithMaxActiveVideoJobsPerUser(1),
		)
	}
	orchA, orchB := newOrchestrator(), newOrchestrator()

	type result struct {
		job *domain.Job
		err error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i, jobID := range []uuid.UUID{first.ID, second.ID} {
		wg.Add(1)
		go func(orch *joborchestrator.Orchestrator, jobID uuid.UUID) {
			defer wg.Done()
			<-start
			job, err := orch.ActivatePreparedAccountJob(context.Background(), accountID, jobID)
			results <- result{job: job, err: err}
		}([]*joborchestrator.Orchestrator{orchA, orchB}[i], jobID)
	}
	close(start)
	wg.Wait()
	close(results)

	var queued, limited int
	for result := range results {
		switch {
		case result.err == nil && result.job != nil && result.job.Status == domain.JobStatusQueued:
			queued++
		case errors.Is(result.err, domain.ErrActiveJobLimitExceeded):
			limited++
		default:
			t.Fatalf("activation result = %+v, %v", result.job, result.err)
		}
	}
	if queued != 1 || limited != 1 {
		t.Fatalf("queued/limited activations = %d/%d, want 1/1", queued, limited)
	}
	if events := f.outbox.Events(); len(events) != 3 || events[2].EventType != "event.job.queued" {
		t.Fatalf("events = %+v, want two created events and one queued event", events)
	}
}

func TestActivatePreparedAccountJobIgnoresLegacyUserCapacityForAnotherAccount(t *testing.T) {
	f := newFixtureWithOrchestratorOptions(
		[]joborchestrator.Option{joborchestrator.WithMaxActiveVideoJobsPerUser(1)},
		billingservice.WithStartingBalance(100),
	)
	accountID := uuid.New()
	legacy := &domain.Job{
		ID:             uuid.New(),
		UserID:         accountID,
		AccountID:      uuid.New(),
		Source:         "legacy",
		ResultMode:     domain.ResultModeLegacyUnknown,
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "legacy-user-must-not-consume-account-capacity",
	}
	if err := f.jobs.Create(context.Background(), legacy); err != nil {
		t.Fatalf("create legacy active job: %v", err)
	}
	prepared := prepareVideoActivatableAccountJob(t, f, accountID, "web:activate:ignore-legacy-user", 25)

	activated, err := activatePreparedAccountJob(t, f.orch, accountID, prepared.ID)
	if err != nil {
		t.Fatalf("activate account-native job behind legacy user row: %v", err)
	}
	if activated == nil || activated.Status != domain.JobStatusQueued {
		t.Fatalf("activated job = %+v, want queued", activated)
	}
}

func TestActivatePreparedAccountJobLocksAccountBeforeReadingPreparedJob(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(100))
	accountID := uuid.New()
	prepared := prepareActivatableAccountJob(t, f, accountID, "web:activate:lock-before-read", 25)

	jobs := &lockBeforeReadJobRepo{JobRepository: f.jobs}
	tx := activationTestUnitOfWork{repos: uow.Repositories{Jobs: jobs, Outbox: f.outbox, Billing: f.bill}}
	orch := joborchestrator.New(f.jobs, tx, f.billing, 0)

	activated, err := orch.ActivatePreparedAccountJob(context.Background(), accountID, prepared.ID)
	if err != nil {
		t.Fatalf("activate prepared job: %v", err)
	}
	if activated == nil || activated.Status != domain.JobStatusQueued || !jobs.locked {
		t.Fatalf("activation = %+v, locked=%t; want queued after account lock", activated, jobs.locked)
	}
}

func prepareVideoActivatableAccountJob(t *testing.T, f *fixture, accountID uuid.UUID, key string, estimate int64) *domain.Job {
	t.Helper()
	job, err := f.orch.PrepareAccountJob(context.Background(), joborchestrator.PrepareAccountJobInput{
		AccountID:           accountID,
		Operation:           domain.OperationVideoGenerate,
		Modality:            domain.ModalityVideo,
		IdempotencyKey:      key,
		CorrelationID:       "activation-video-capacity-test",
		CostEstimateCredits: estimate,
	})
	if err != nil {
		t.Fatalf("prepare video account job: %v", err)
	}
	return job
}

type activationTestUnitOfWork struct {
	repos uow.Repositories
}

func (u activationTestUnitOfWork) Within(ctx context.Context, fn func(context.Context, uow.Repositories) error) error {
	return fn(ctx, u.repos)
}

type recordingActivationUnitOfWork struct {
	repos       uow.Repositories
	callbackErr error
}

func (u *recordingActivationUnitOfWork) Within(ctx context.Context, fn func(context.Context, uow.Repositories) error) error {
	u.callbackErr = fn(ctx, u.repos)
	return u.callbackErr
}

type activationCapacityBarrier struct {
	domain.JobRepository

	accountID     uuid.UUID
	operation     domain.OperationType
	mu            sync.Mutex
	arrivals      int
	secondArrival chan struct{}
}

type lockBeforeReadJobRepo struct {
	domain.JobRepository

	locked bool
}

func (r *lockBeforeReadJobRepo) LockAccountForCapacity(ctx context.Context, accountID uuid.UUID) error {
	if err := r.JobRepository.LockAccountForCapacity(ctx, accountID); err != nil {
		return err
	}
	r.locked = true
	return nil
}

func (r *lockBeforeReadJobRepo) GetByIDForAccount(ctx context.Context, accountID, jobID uuid.UUID) (*domain.Job, error) {
	if !r.locked {
		return nil, errors.New("prepared job read before account lock")
	}
	return r.JobRepository.GetByIDForAccount(ctx, accountID, jobID)
}

func (b *activationCapacityBarrier) CountActiveByAccountOperation(ctx context.Context, accountID uuid.UUID, operation domain.OperationType) (int, error) {
	count, err := b.JobRepository.CountActiveByAccountOperation(ctx, accountID, operation)
	if err != nil || accountID != b.accountID || operation != b.operation {
		return count, err
	}
	b.mu.Lock()
	b.arrivals++
	if b.arrivals == 2 {
		close(b.secondArrival)
	}
	b.mu.Unlock()
	select {
	case <-b.secondArrival:
	case <-time.After(100 * time.Millisecond):
	}
	return count, nil
}
