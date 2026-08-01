package resultreadyreconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/uow"
	"vk-ai-aggregator/internal/service/outboxrelay"
)

func TestReconcileProcessesOneBoundedCandidatePage(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	outbox := memory.NewOutboxRepo()
	for i := range 3 {
		if err := jobs.Create(ctx, canonicalReadyJob("bounded-"+string(rune('a'+i)))); err != nil {
			t.Fatalf("create job %d: %v", i, err)
		}
	}
	candidates := &recordingCandidates{
		ResultReadyCandidateRepository: memory.NewResultReadyCandidateRepository(jobs, outbox),
	}
	readiness := &recordingReadiness{}
	manager := &countingManager{Manager: memory.NewUnitOfWork(jobs, outbox, nil)}

	result, err := New(candidates, readiness, manager).Reconcile(ctx, 2)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result != (Result{
		Candidates: 2,
		Eligible:   2,
		Created:    2,
		HasMore:    true,
	}) {
		t.Fatalf("result = %+v, want one bounded two-row page", result)
	}
	if candidates.calls != 1 || candidates.limit != 2 || candidates.after != nil {
		t.Fatalf("candidate calls/limit/after = %d/%d/%+v, want 1/2/nil", candidates.calls, candidates.limit, candidates.after)
	}
	if len(readiness.calls) != 2 {
		t.Fatalf("readiness calls = %d, want page size 2", len(readiness.calls))
	}
	if manager.calls != 1 {
		t.Fatalf("unit-of-work calls = %d, want one short UOW", manager.calls)
	}
	if events := outbox.Events(); len(events) != 2 {
		t.Fatalf("outbox events = %d, want bounded page size 2", len(events))
	}
}

func TestReconcileRequiresExactAccountReadinessAndBlocksIncompleteResults(t *testing.T) {
	ctx := context.Background()
	jobs := []*domain.Job{
		canonicalReadyJob("missing-artifact"),
		canonicalReadyJob("missing-moderation"),
	}
	missingArtifact := jobs[0].ID
	missingModeration := jobs[1].ID
	readiness := &recordingReadiness{errorsByJob: map[uuid.UUID]error{
		missingArtifact:   domain.ErrNotFound,
		missingModeration: domain.ErrNotFound,
	}}
	outbox := memory.NewOutboxRepo()
	candidatesBefore := reconciliationCounterValue(t, metrics.ResultReadyReconciliationItems, "candidates")
	blockedBefore := reconciliationCounterValue(t, metrics.ResultReadyReconciliationItems, "blocked")
	durationBefore := reconciliationHistogramCount(t, metrics.ResultReadyReconciliationDuration, "success")

	result, err := New(
		&staticCandidates{jobs: jobs},
		readiness,
		staticManager{repos: uow.Repositories{Outbox: outbox}},
	).Reconcile(ctx, 10)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result != (Result{Candidates: 2, Blocked: 2}) {
		t.Fatalf("result = %+v, want two blocked incomplete results", result)
	}
	if len(readiness.calls) != 2 {
		t.Fatalf("readiness calls = %d, want 2", len(readiness.calls))
	}
	for _, call := range readiness.calls {
		var matched bool
		for _, job := range jobs {
			if call.jobID == job.ID {
				matched = true
				if call.accountID != job.AccountID {
					t.Fatalf("readiness account for job %s = %s, want %s", job.ID, call.accountID, job.AccountID)
				}
			}
		}
		if !matched {
			t.Fatalf("readiness called for unexpected job %s", call.jobID)
		}
	}
	if events := outbox.Events(); len(events) != 0 {
		t.Fatalf("incomplete results emitted events: %+v", events)
	}
	if got := reconciliationCounterValue(t, metrics.ResultReadyReconciliationItems, "candidates"); got != candidatesBefore+2 {
		t.Fatalf("candidate metric = %v, want %v", got, candidatesBefore+2)
	}
	if got := reconciliationCounterValue(t, metrics.ResultReadyReconciliationItems, "blocked"); got != blockedBefore+2 {
		t.Fatalf("blocked metric = %v, want %v", got, blockedBefore+2)
	}
	if got := reconciliationHistogramCount(t, metrics.ResultReadyReconciliationDuration, "success"); got != durationBefore+1 {
		t.Fatalf("duration samples = %d, want %d", got, durationBefore+1)
	}
}

func TestReconcileBlocksIncompleteExternalPushResultWithoutEmittingEvent(t *testing.T) {
	ctx := context.Background()
	job := canonicalReadyJob("external-push-missing-artifact")
	job.ResultMode = domain.ResultModeExternalPush
	job.DeliveryTarget = &domain.DeliveryTarget{
		Channel:      domain.ChannelVKBot,
		RecipientRef: "peer:2000000001",
	}
	readiness := &recordingReadiness{errorsByJob: map[uuid.UUID]error{
		job.ID: domain.ErrNotFound,
	}}
	outbox := memory.NewOutboxRepo()

	result, err := New(
		&staticCandidates{jobs: []*domain.Job{job}},
		readiness,
		staticManager{repos: uow.Repositories{Outbox: outbox}},
	).Reconcile(ctx, 1)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result != (Result{Candidates: 1, Blocked: 1}) {
		t.Fatalf("result = %+v, want incomplete external push blocked", result)
	}
	if len(readiness.calls) != 1 {
		t.Fatalf("readiness calls = %d, want 1", len(readiness.calls))
	}
	if call := readiness.calls[0]; call.accountID != job.AccountID || call.jobID != job.ID {
		t.Fatalf("readiness call = %+v, want account/job %s/%s", call, job.AccountID, job.ID)
	}
	if events := outbox.Events(); len(events) != 0 {
		t.Fatalf("incomplete external push emitted events: %+v", events)
	}
}

func TestReconcileFailsClosedBeforeReadinessForInvalidCandidates(t *testing.T) {
	valid := canonicalReadyJob("template")
	invalid := []*domain.Job{
		nil,
		func() *domain.Job { job := *valid; job.ID = uuid.Nil; return &job }(),
		func() *domain.Job { job := *valid; job.AccountID = uuid.Nil; return &job }(),
		func() *domain.Job { job := *valid; job.ResultMode = domain.ResultModeLegacyUnknown; return &job }(),
		func() *domain.Job {
			job := *valid
			job.DeliveryTarget = &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "1"}
			return &job
		}(),
		func() *domain.Job {
			job := *valid
			job.ResultMode = domain.ResultModeExternalPush
			return &job
		}(),
		func() *domain.Job {
			job := *valid
			job.OperationType = domain.OperationType("unknown_operation")
			return &job
		}(),
		func() *domain.Job {
			job := *valid
			job.Modality = domain.Modality("unknown_modality")
			return &job
		}(),
	}
	readiness := &recordingReadiness{}
	outbox := memory.NewOutboxRepo()

	result, err := New(
		&staticCandidates{jobs: invalid},
		readiness,
		staticManager{repos: uow.Repositories{Outbox: outbox}},
	).Reconcile(context.Background(), len(invalid))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result != (Result{Candidates: len(invalid), Blocked: len(invalid)}) {
		t.Fatalf("result = %+v, want every invalid candidate blocked", result)
	}
	if len(readiness.calls) != 0 {
		t.Fatalf("readiness called for invalid candidates: %+v", readiness.calls)
	}
	if events := outbox.Events(); len(events) != 0 {
		t.Fatalf("invalid candidates emitted events: %+v", events)
	}
}

func TestReconcileCountsSemanticInsertRaceAsExisting(t *testing.T) {
	job := canonicalReadyJob("race")
	baseOutbox := memory.NewOutboxRepo()
	racingOutbox := &existingInsertOutbox{OutboxRepository: baseOutbox}

	result, err := New(
		&staticCandidates{jobs: []*domain.Job{job}},
		&recordingReadiness{},
		staticManager{repos: uow.Repositories{Outbox: racingOutbox}},
	).Reconcile(context.Background(), 1)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result != (Result{Candidates: 1, Eligible: 1, Existing: 1}) {
		t.Fatalf("result = %+v, want semantic race counted existing", result)
	}
	if racingOutbox.calls != 1 {
		t.Fatalf("AddIfAbsentByID calls = %d, want 1", racingOutbox.calls)
	}
	if events := baseOutbox.Events(); len(events) != 0 {
		t.Fatalf("race path created duplicate events: %+v", events)
	}
}

func TestReconcileCreatesDeterministicResultReadyEventWithoutMutatingJob(t *testing.T) {
	job := canonicalReadyJob("deterministic")
	before := cloneTestJob(job)
	outbox := memory.NewOutboxRepo()

	result, err := New(
		&staticCandidates{jobs: []*domain.Job{job}},
		&recordingReadiness{},
		staticManager{repos: uow.Repositories{Outbox: outbox}},
	).Reconcile(context.Background(), 1)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result != (Result{Candidates: 1, Eligible: 1, Created: 1}) {
		t.Fatalf("result = %+v, want one created event", result)
	}
	if !reflect.DeepEqual(job, before) {
		t.Fatalf("candidate mutated:\nbefore=%+v\nafter=%+v", before, job)
	}
	events := outbox.Events()
	if len(events) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(events))
	}
	event := events[0]
	wantID := uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte("urn:vk-ai-aggregator:outbox:"+outboxrelay.EventJobResultReady+":"+job.ID.String()),
	)
	if event.ID != wantID || event.AggregateType != "job" || event.AggregateID != job.ID ||
		event.EventType != outboxrelay.EventJobResultReady || event.Status != domain.OutboxPending {
		t.Fatalf("event identity/state = %+v, want deterministic pending result-ready", event)
	}
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	wantPayload := map[string]any{
		"job_id":         job.ID.String(),
		"operation":      string(job.OperationType),
		"modality":       string(job.Modality),
		"correlation_id": job.CorrelationID,
	}
	if !reflect.DeepEqual(payload, wantPayload) {
		t.Fatalf("payload = %#v, want %#v", payload, wantPayload)
	}
}

func TestReconcileReturnsOperationalReadinessErrorWithoutWriting(t *testing.T) {
	job := canonicalReadyJob("readiness-error")
	boom := errors.New("readiness storage unavailable")
	outbox := memory.NewOutboxRepo()

	_, err := New(
		&staticCandidates{jobs: []*domain.Job{job}},
		&recordingReadiness{errorsByJob: map[uuid.UUID]error{job.ID: boom}},
		staticManager{repos: uow.Repositories{Outbox: outbox}},
	).Reconcile(context.Background(), 1)
	if !errors.Is(err, boom) {
		t.Fatalf("reconcile error = %v, want readiness error", err)
	}
	if events := outbox.Events(); len(events) != 0 {
		t.Fatalf("operational readiness failure emitted events: %+v", events)
	}
}

func TestResultExportsExactCountOnlyPageFields(t *testing.T) {
	typ := reflect.TypeOf(Result{})
	want := []struct {
		jsonName string
		kind     reflect.Kind
	}{
		{jsonName: "candidates", kind: reflect.Int},
		{jsonName: "eligible", kind: reflect.Int},
		{jsonName: "existing", kind: reflect.Int},
		{jsonName: "created", kind: reflect.Int},
		{jsonName: "blocked", kind: reflect.Int},
		{jsonName: "has_more", kind: reflect.Bool},
	}
	if typ.NumField() != len(want) {
		t.Fatalf("Result fields = %d, want %d exact count/page fields", typ.NumField(), len(want))
	}
	for i, expected := range want {
		field := typ.Field(i)
		if field.Type.Kind() != expected.kind || field.Tag.Get("json") != expected.jsonName {
			t.Fatalf("Result field %d = %s %s %q, want %s json:%q",
				i, field.Name, field.Type, field.Tag.Get("json"), expected.kind, expected.jsonName)
		}
	}
}

func canonicalReadyJob(key string) *domain.Job {
	accountID := uuid.New()
	return &domain.Job{
		ID:                uuid.New(),
		UserID:            accountID,
		AccountID:         accountID,
		Source:            "web",
		ResultMode:        domain.ResultModeAccountHistory,
		OperationType:     domain.OperationTextGenerate,
		Modality:          domain.ModalityText,
		Status:            domain.JobStatusResultReady,
		IdempotencyKey:    key + "-" + uuid.NewString(),
		CorrelationID:     "correlation-" + key,
		OutputArtifactIDs: []uuid.UUID{uuid.New()},
		Params:            json.RawMessage(`{"private_prompt":"must-not-leak"}`),
	}
}

func cloneTestJob(job *domain.Job) *domain.Job {
	copy := *job
	copy.OutputArtifactIDs = append([]uuid.UUID(nil), job.OutputArtifactIDs...)
	copy.Params = append([]byte(nil), job.Params...)
	if job.DeliveryTarget != nil {
		target := *job.DeliveryTarget
		copy.DeliveryTarget = &target
	}
	return &copy
}

func reconciliationCounterValue(t *testing.T, counter *prometheus.CounterVec, labels ...string) float64 {
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

func reconciliationHistogramCount(t *testing.T, histogram *prometheus.HistogramVec, labels ...string) uint64 {
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

type staticCandidates struct {
	jobs    []*domain.Job
	hasMore bool
	err     error
}

func (r *staticCandidates) ListMissingCanonicalResultReady(
	_ context.Context,
	_ int,
	_ *domain.JobCursor,
) ([]*domain.Job, bool, error) {
	return r.jobs, r.hasMore, r.err
}

type recordingCandidates struct {
	domain.ResultReadyCandidateRepository
	calls int
	limit int
	after *domain.JobCursor
}

func (r *recordingCandidates) ListMissingCanonicalResultReady(
	ctx context.Context,
	limit int,
	after *domain.JobCursor,
) ([]*domain.Job, bool, error) {
	r.calls++
	r.limit = limit
	r.after = after
	return r.ResultReadyCandidateRepository.ListMissingCanonicalResultReady(ctx, limit, after)
}

type readinessCall struct {
	accountID uuid.UUID
	jobID     uuid.UUID
}

type recordingReadiness struct {
	calls       []readinessCall
	errorsByJob map[uuid.UUID]error
}

func (r *recordingReadiness) RequireCompletionReady(
	_ context.Context,
	accountID, jobID uuid.UUID,
) error {
	r.calls = append(r.calls, readinessCall{accountID: accountID, jobID: jobID})
	return r.errorsByJob[jobID]
}

type staticManager struct {
	repos uow.Repositories
}

func (m staticManager) Within(ctx context.Context, fn func(context.Context, uow.Repositories) error) error {
	return fn(ctx, m.repos)
}

type countingManager struct {
	uow.Manager
	calls int
}

func (m *countingManager) Within(ctx context.Context, fn func(context.Context, uow.Repositories) error) error {
	m.calls++
	return m.Manager.Within(ctx, fn)
}

type existingInsertOutbox struct {
	domain.OutboxRepository
	calls int
}

func (r *existingInsertOutbox) AddIfAbsentByID(_ context.Context, _ *domain.OutboxEvent) (bool, error) {
	r.calls++
	return false, nil
}
