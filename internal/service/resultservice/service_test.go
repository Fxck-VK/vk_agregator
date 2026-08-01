package resultservice

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestListHistoryIsAccountScopedPagedAndSafe(t *testing.T) {
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	owner := uuid.New()
	foreign := uuid.New()

	older := createHistoryJob(t, ctx, jobs, owner, domain.JobStatusReceived)
	time.Sleep(time.Millisecond)
	_ = createHistoryJob(t, ctx, jobs, foreign, domain.JobStatusSucceeded)
	time.Sleep(time.Millisecond)
	middle := createHistoryJob(t, ctx, jobs, owner, domain.JobStatusSucceeded)
	time.Sleep(time.Millisecond)
	_ = createHistoryJob(t, ctx, jobs, owner, domain.JobStatusQueued)

	service := New(jobs, memory.NewArtifactRepo(), memory.NewModerationRepo())
	got, err := service.ListHistory(ctx, owner, 2, 1)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("history length = %d, want 2", len(got))
	}

	wantJobs := []*domain.Job{middle, older}
	for i, want := range wantJobs {
		assertHistoryItem(t, got[i], want)
	}
	assertExactFields(t, HistoryItem{}, []string{
		"ID", "Operation", "Modality", "Status", "CreatedAt", "UpdatedAt",
	})

	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal history: %v", err)
	}
	assertOmits(t, string(payload),
		older.OutputArtifactIDs[0].String(),
		middle.OutputArtifactIDs[0].String(),
		"output_artifact",
		"private_provider",
		"private_model",
		"private_params",
		"private_pricing",
		"private_correlation",
		"private_idempotency",
		"private_raw_error",
	)
}

func TestListHistoryForwardsPagingAndPreservesRepositoryOrder(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	first := &domain.Job{
		ID:            uuid.New(),
		OperationType: domain.OperationVideoGenerate,
		Modality:      domain.ModalityVideo,
		Status:        domain.JobStatusQueued,
		CreatedAt:     time.Unix(1, 0),
		UpdatedAt:     time.Unix(2, 0),
	}
	second := &domain.Job{
		ID:            uuid.New(),
		OperationType: domain.OperationImageEdit,
		Modality:      domain.ModalityImage,
		Status:        domain.JobStatusSucceeded,
		CreatedAt:     time.Unix(100, 0),
		UpdatedAt:     time.Unix(101, 0),
	}
	repo := &recordingJobRepository{listResult: []*domain.Job{first, second}}

	got, err := New(repo, nil, nil).ListHistory(ctx, accountID, -7, -3)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if repo.listCalls != 1 || repo.listAccountID != accountID || repo.listLimit != -7 || repo.listOffset != -3 {
		t.Fatalf("ListByAccount calls=%d account=%s limit=%d offset=%d", repo.listCalls, repo.listAccountID, repo.listLimit, repo.listOffset)
	}
	if repo.legacyGetCalls != 0 || repo.scopedGetCalls != 0 {
		t.Fatalf("history used point reads: legacy=%d scoped=%d", repo.legacyGetCalls, repo.scopedGetCalls)
	}
	if len(got) != 2 || got[0].ID != first.ID || got[1].ID != second.ID {
		t.Fatalf("history order = %+v, want [%s %s]", got, first.ID, second.ID)
	}
}

func TestListHistoryRejectsNilAccountAndHandlesRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	repo := &recordingJobRepository{}
	service := New(repo, nil, nil)

	got, err := service.ListHistory(ctx, uuid.Nil, 10, 0)
	if err != domain.ErrNotFound {
		t.Fatalf("nil account error = %v, want exact ErrNotFound", err)
	}
	if got != nil {
		t.Fatalf("nil account result = %+v, want nil", got)
	}
	if repo.listCalls != 0 {
		t.Fatalf("nil account performed %d repository reads", repo.listCalls)
	}

	repo.listErr = domain.ErrNotFound
	got, err = service.ListHistory(ctx, uuid.New(), 10, 0)
	if err != domain.ErrNotFound || got != nil {
		t.Fatalf("repository not found = %+v, %v; want nil, exact ErrNotFound", got, err)
	}

	boom := errors.New("history repository unavailable")
	repo.listErr = boom
	got, err = service.ListHistory(ctx, uuid.New(), 10, 0)
	if err != boom || got != nil {
		t.Fatalf("unexpected repository error = %+v, %v; want nil, original error", got, err)
	}
}

func TestGetResultReturnsOnlySafeMetadataForEveryAllowedArtifact(t *testing.T) {
	ctx := context.Background()
	owner, job, artifacts, jobs, artifactRepo, moderationRepo := createValidResultFixture(t, ctx)

	recordingJobs := &recordingJobRepository{JobRepository: jobs}
	recordingArtifacts := &recordingArtifactRepository{ArtifactRepository: artifactRepo}
	recordingModeration := &recordingModerationRepository{
		ModerationResultRepository: moderationRepo,
		appendNil:                  true,
	}

	got, err := New(recordingJobs, recordingArtifacts, recordingModeration).GetResult(ctx, owner, job.ID)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if recordingJobs.scopedGetCalls != 1 || recordingJobs.legacyGetCalls != 0 {
		t.Fatalf("job reads: scoped=%d legacy=%d", recordingJobs.scopedGetCalls, recordingJobs.legacyGetCalls)
	}
	if !reflect.DeepEqual(recordingArtifacts.requestedIDs, job.OutputArtifactIDs) {
		t.Fatalf("artifact read order = %v, want %v", recordingArtifacts.requestedIDs, job.OutputArtifactIDs)
	}
	if recordingArtifacts.legacyGetCalls != 0 {
		t.Fatalf("legacy artifact reads = %d, want 0", recordingArtifacts.legacyGetCalls)
	}
	if recordingModeration.listCalls != 1 || recordingModeration.lastJobID != job.ID {
		t.Fatalf("moderation reads=%d job=%s, want once for %s", recordingModeration.listCalls, recordingModeration.lastJobID, job.ID)
	}

	if got.ID != job.ID ||
		got.Operation != job.OperationType ||
		got.Modality != job.Modality ||
		got.Status != job.Status ||
		!got.CreatedAt.Equal(job.CreatedAt) ||
		!got.UpdatedAt.Equal(job.UpdatedAt) {
		t.Fatalf("unexpected result projection: %+v", got)
	}
	if len(got.Artifacts) != 2 {
		t.Fatalf("artifact metadata length = %d, want 2", len(got.Artifacts))
	}
	for i, want := range artifacts {
		gotArtifact := got.Artifacts[i]
		if gotArtifact.ID != want.ID ||
			gotArtifact.MediaType != want.MediaType ||
			gotArtifact.MIMEType != want.MimeType ||
			gotArtifact.SizeBytes != want.SizeBytes ||
			gotArtifact.Width != want.Width ||
			gotArtifact.Height != want.Height ||
			gotArtifact.DurationMS != want.DurationMS {
			t.Fatalf("artifact %d projection = %+v, want safe fields from %+v", i, gotArtifact, want)
		}
	}

	assertExactFields(t, Result{}, []string{
		"ID", "Operation", "Modality", "Status", "CreatedAt", "UpdatedAt", "Artifacts",
	})
	assertExactFields(t, ArtifactMetadata{}, []string{
		"ID", "MediaType", "MIMEType", "SizeBytes", "Width", "Height", "DurationMS",
	})

	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	assertOmits(t, string(payload),
		"private_provider",
		"private_model",
		"private_params",
		"private_pricing",
		"private_correlation",
		"private_idempotency",
		"private_raw_error",
		"private_bucket",
		"private_storage_key",
		"private_url",
		"private_sha256",
		"private_lifecycle",
		"private_codec",
		"private_container",
		"private_moderation_provider",
		"probe_status",
		"bitrate",
	)
}

func TestRequireCompletionReadySharesStrictValidationButPrecedesPublicResult(t *testing.T) {
	ctx := context.Background()
	owner, job, _, jobs, artifacts, moderation := createValidResultFixture(t, ctx)
	if err := jobs.UpdateStatus(ctx, job.ID, domain.JobStatusSucceeded, domain.JobStatusResultReady, "", ""); err != nil {
		t.Fatalf("set result_ready: %v", err)
	}
	job.Status = domain.JobStatusResultReady
	service := New(jobs, artifacts, moderation)

	if err := service.RequireCompletionReady(ctx, owner, job.ID); err != nil {
		t.Fatalf("RequireCompletionReady for complete result_ready job: %v", err)
	}
	if got, err := service.GetResult(ctx, owner, job.ID); err != domain.ErrNotFound || !reflect.DeepEqual(got, Result{}) {
		t.Fatalf("GetResult before succeeded = %+v, %v; want zero, exact ErrNotFound", got, err)
	}

	if err := jobs.UpdateStatus(ctx, job.ID, domain.JobStatusResultReady, domain.JobStatusDelivering, "", ""); err != nil {
		t.Fatalf("set delivering: %v", err)
	}
	job.Status = domain.JobStatusDelivering
	if err := service.RequireCompletionReady(ctx, owner, job.ID); err != nil {
		t.Fatalf("RequireCompletionReady for complete delivering job: %v", err)
	}
	if got, err := service.GetResult(ctx, owner, job.ID); err != domain.ErrNotFound || !reflect.DeepEqual(got, Result{}) {
		t.Fatalf("GetResult while delivering = %+v, %v; want zero, exact ErrNotFound", got, err)
	}

	if err := jobs.UpdateStatus(ctx, job.ID, domain.JobStatusDelivering, domain.JobStatusSucceeded, "", ""); err != nil {
		t.Fatalf("set succeeded: %v", err)
	}
	job.Status = domain.JobStatusSucceeded
	if err := service.RequireCompletionReady(ctx, owner, job.ID); err != domain.ErrNotFound {
		t.Fatalf("RequireCompletionReady after succeeded = %v, want exact ErrNotFound", err)
	}
	if _, err := service.GetResult(ctx, owner, job.ID); err != nil {
		t.Fatalf("GetResult after succeeded: %v", err)
	}
}

func TestRequireCompletionReadyFailsClosedForEveryIncompleteOutput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*resultScenario)
	}{
		{name: "no outputs", mutate: func(s *resultScenario) { s.job.OutputArtifactIDs = nil }},
		{name: "foreign artifact", mutate: func(s *resultScenario) { s.artifacts[1].OwnerAccountID = uuid.New() }},
		{name: "unready artifact", mutate: func(s *resultScenario) { s.artifacts[1].Status = domain.ArtifactStatusStored }},
		{name: "wrong job link", mutate: func(s *resultScenario) {
			other := uuid.New()
			s.artifacts[1].JobID = &other
		}},
		{name: "missing moderation", mutate: func(s *resultScenario) { s.moderation = s.moderation[:1] }},
		{name: "blocked moderation", mutate: func(s *resultScenario) { s.moderation[1].Decision = domain.ModerationBlock }},
		{name: "conflicting moderation", mutate: func(s *resultScenario) {
			artifactID := s.artifacts[1].ID
			s.moderation = append(s.moderation, &domain.ModerationResult{
				ID:         uuid.New(),
				JobID:      s.job.ID,
				ArtifactID: &artifactID,
				Stage:      domain.ModerationStageOutput,
				Decision:   domain.ModerationReview,
			})
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			scenario := newResultScenario()
			scenario.job.Status = domain.JobStatusResultReady
			test.mutate(scenario)
			service := persistResultScenario(t, ctx, scenario)

			if err := service.RequireCompletionReady(ctx, scenario.owner, scenario.job.ID); err != domain.ErrNotFound {
				t.Fatalf("RequireCompletionReady error = %v, want exact ErrNotFound", err)
			}
		})
	}
}

func TestGetResultRejectsNilForeignAndMissingJobs(t *testing.T) {
	ctx := context.Background()
	repo := &recordingJobRepository{}
	service := New(repo, nil, nil)
	accountID := uuid.New()
	jobID := uuid.New()

	for _, ids := range []struct {
		name      string
		accountID uuid.UUID
		jobID     uuid.UUID
	}{
		{name: "nil account", accountID: uuid.Nil, jobID: jobID},
		{name: "nil job", accountID: accountID, jobID: uuid.Nil},
	} {
		t.Run(ids.name, func(t *testing.T) {
			got, err := service.GetResult(ctx, ids.accountID, ids.jobID)
			if err != domain.ErrNotFound || !reflect.DeepEqual(got, Result{}) {
				t.Fatalf("GetResult = %+v, %v; want zero, exact ErrNotFound", got, err)
			}
		})
	}
	if repo.scopedGetCalls != 0 || repo.legacyGetCalls != 0 {
		t.Fatalf("nil ids performed reads: scoped=%d legacy=%d", repo.scopedGetCalls, repo.legacyGetCalls)
	}

	jobs := memory.NewJobRepo()
	foreignOwner := uuid.New()
	foreignJob := createHistoryJob(t, ctx, jobs, foreignOwner, domain.JobStatusSucceeded)
	service = New(jobs, memory.NewArtifactRepo(), memory.NewModerationRepo())
	for _, id := range []uuid.UUID{foreignJob.ID, uuid.New()} {
		got, err := service.GetResult(ctx, accountID, id)
		if err != domain.ErrNotFound || !reflect.DeepEqual(got, Result{}) {
			t.Fatalf("foreign/missing job %s = %+v, %v; want zero, exact ErrNotFound", id, got, err)
		}
	}
}

func TestGetResultFailsClosedForUnsafeOrIncompleteResults(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*resultScenario)
	}{
		{name: "non-succeeded job", mutate: func(s *resultScenario) { s.job.Status = domain.JobStatusResultReady }},
		{name: "no outputs", mutate: func(s *resultScenario) { s.job.OutputArtifactIDs = nil }},
		{name: "missing artifact", mutate: func(s *resultScenario) { s.artifacts = s.artifacts[:1] }},
		{name: "foreign artifact", mutate: func(s *resultScenario) { s.artifacts[1].OwnerAccountID = uuid.New() }},
		{name: "non-output artifact", mutate: func(s *resultScenario) { s.artifacts[1].Kind = domain.ArtifactKindInput }},
		{name: "unready artifact", mutate: func(s *resultScenario) { s.artifacts[1].Status = domain.ArtifactStatusStored }},
		{name: "missing job link", mutate: func(s *resultScenario) { s.artifacts[1].JobID = nil }},
		{
			name: "wrong job link",
			mutate: func(s *resultScenario) {
				other := uuid.New()
				s.artifacts[1].JobID = &other
			},
		},
		{name: "invalid media type", mutate: func(s *resultScenario) { s.artifacts[1].MediaType = domain.MediaType("private-unsupported") }},
		{name: "missing moderation", mutate: func(s *resultScenario) { s.moderation = s.moderation[:1] }},
		{name: "wrong-stage moderation", mutate: func(s *resultScenario) { s.moderation[1].Stage = domain.ModerationStageInput }},
		{
			name: "wrong-artifact moderation",
			mutate: func(s *resultScenario) {
				firstID := s.artifacts[0].ID
				s.moderation[1].ArtifactID = &firstID
			},
		},
		{name: "blocked moderation", mutate: func(s *resultScenario) { s.moderation[1].Decision = domain.ModerationBlock }},
		{name: "review moderation", mutate: func(s *resultScenario) { s.moderation[1].Decision = domain.ModerationReview }},
		{
			name: "conflicting allowed and blocked moderation",
			mutate: func(s *resultScenario) {
				artifactID := s.artifacts[1].ID
				s.moderation = append(s.moderation, &domain.ModerationResult{
					ID:         uuid.New(),
					JobID:      s.job.ID,
					ArtifactID: &artifactID,
					Stage:      domain.ModerationStageOutput,
					Decision:   domain.ModerationBlock,
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			scenario := newResultScenario()
			test.mutate(scenario)
			service := persistResultScenario(t, ctx, scenario)

			got, err := service.GetResult(ctx, scenario.owner, scenario.job.ID)
			if err != domain.ErrNotFound {
				t.Fatalf("GetResult error = %v, want exact ErrNotFound", err)
			}
			if !reflect.DeepEqual(got, Result{}) {
				t.Fatalf("GetResult leaked partial result: %+v", got)
			}
		})
	}
}

func TestGetResultNormalizesNotFoundAndPropagatesUnexpectedRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("unexpected repository failure")

	tests := []struct {
		name    string
		wantErr error
		build   func(*testing.T) (*Service, uuid.UUID, uuid.UUID)
	}{
		{
			name: "job not found", wantErr: domain.ErrNotFound,
			build: func(t *testing.T) (*Service, uuid.UUID, uuid.UUID) {
				return New(&recordingJobRepository{getErr: domain.ErrNotFound}, nil, nil), uuid.New(), uuid.New()
			},
		},
		{
			name: "job unexpected error", wantErr: boom,
			build: func(t *testing.T) (*Service, uuid.UUID, uuid.UUID) {
				return New(&recordingJobRepository{getErr: boom}, nil, nil), uuid.New(), uuid.New()
			},
		},
		{
			name: "artifact not found", wantErr: domain.ErrNotFound,
			build: func(t *testing.T) (*Service, uuid.UUID, uuid.UUID) {
				accountID, job, _, jobs, artifacts, moderation := createValidResultFixture(t, ctx)
				return New(jobs, &recordingArtifactRepository{ArtifactRepository: artifacts, getErr: domain.ErrNotFound}, moderation), accountID, job.ID
			},
		},
		{
			name: "artifact unexpected error", wantErr: boom,
			build: func(t *testing.T) (*Service, uuid.UUID, uuid.UUID) {
				accountID, job, _, jobs, artifacts, moderation := createValidResultFixture(t, ctx)
				return New(jobs, &recordingArtifactRepository{ArtifactRepository: artifacts, getErr: boom}, moderation), accountID, job.ID
			},
		},
		{
			name: "moderation not found", wantErr: domain.ErrNotFound,
			build: func(t *testing.T) (*Service, uuid.UUID, uuid.UUID) {
				accountID, job, _, jobs, artifacts, moderation := createValidResultFixture(t, ctx)
				return New(jobs, artifacts, &recordingModerationRepository{ModerationResultRepository: moderation, listErr: domain.ErrNotFound}), accountID, job.ID
			},
		},
		{
			name: "moderation unexpected error", wantErr: boom,
			build: func(t *testing.T) (*Service, uuid.UUID, uuid.UUID) {
				accountID, job, _, jobs, artifacts, moderation := createValidResultFixture(t, ctx)
				return New(jobs, artifacts, &recordingModerationRepository{ModerationResultRepository: moderation, listErr: boom}), accountID, job.ID
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, accountID, jobID := test.build(t)
			got, err := service.GetResult(ctx, accountID, jobID)
			if err != test.wantErr {
				t.Fatalf("GetResult error = %v, want original %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(got, Result{}) {
				t.Fatalf("GetResult leaked partial result: %+v", got)
			}
		})
	}
}

type recordingJobRepository struct {
	domain.JobRepository
	listResult     []*domain.Job
	listErr        error
	listCalls      int
	listAccountID  uuid.UUID
	listLimit      int
	listOffset     int
	getResult      *domain.Job
	getErr         error
	scopedGetCalls int
	legacyGetCalls int
}

func (r *recordingJobRepository) ListByAccount(_ context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.Job, error) {
	r.listCalls++
	r.listAccountID, r.listLimit, r.listOffset = accountID, limit, offset
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.listResult, nil
}

func (r *recordingJobRepository) GetByIDForAccount(ctx context.Context, accountID, jobID uuid.UUID) (*domain.Job, error) {
	r.scopedGetCalls++
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.getResult != nil {
		return r.getResult, nil
	}
	return r.JobRepository.GetByIDForAccount(ctx, accountID, jobID)
}

func (r *recordingJobRepository) GetByID(ctx context.Context, jobID uuid.UUID) (*domain.Job, error) {
	r.legacyGetCalls++
	return r.JobRepository.GetByID(ctx, jobID)
}

type recordingArtifactRepository struct {
	domain.ArtifactRepository
	getErr         error
	requestedIDs   []uuid.UUID
	legacyGetCalls int
}

func (r *recordingArtifactRepository) GetByIDForAccount(ctx context.Context, accountID, artifactID uuid.UUID) (*domain.Artifact, error) {
	r.requestedIDs = append(r.requestedIDs, artifactID)
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.ArtifactRepository.GetByIDForAccount(ctx, accountID, artifactID)
}

func (r *recordingArtifactRepository) GetByID(ctx context.Context, artifactID uuid.UUID) (*domain.Artifact, error) {
	r.legacyGetCalls++
	return r.ArtifactRepository.GetByID(ctx, artifactID)
}

type recordingModerationRepository struct {
	domain.ModerationResultRepository
	listErr   error
	listCalls int
	lastJobID uuid.UUID
	appendNil bool
}

func (r *recordingModerationRepository) ListByJob(ctx context.Context, jobID uuid.UUID) ([]*domain.ModerationResult, error) {
	r.listCalls++
	r.lastJobID = jobID
	if r.listErr != nil {
		return nil, r.listErr
	}
	results, err := r.ModerationResultRepository.ListByJob(ctx, jobID)
	if err == nil && r.appendNil {
		results = append(results, nil)
	}
	return results, err
}

type resultScenario struct {
	owner      uuid.UUID
	job        *domain.Job
	artifacts  []*domain.Artifact
	moderation []*domain.ModerationResult
}

func newResultScenario() *resultScenario {
	owner := uuid.New()
	jobID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()
	firstJobID, secondJobID := jobID, jobID
	job := privateJob(owner, domain.JobStatusSucceeded, []uuid.UUID{secondID, firstID})
	job.ID = jobID
	first := privateArtifact(owner, firstID, &firstJobID, domain.MediaTypeImage, "image/png", 101, 640, 480, 0)
	second := privateArtifact(owner, secondID, &secondJobID, domain.MediaTypeVideo, "video/mp4", 202, 1280, 720, 9_000)
	firstModerationID, secondModerationID := first.ID, second.ID

	return &resultScenario{
		owner:     owner,
		job:       job,
		artifacts: []*domain.Artifact{first, second},
		moderation: []*domain.ModerationResult{
			{ID: uuid.New(), JobID: jobID, ArtifactID: &firstModerationID, Stage: domain.ModerationStageOutput, Decision: domain.ModerationAllow, Provider: "private_moderation_provider"},
			{ID: uuid.New(), JobID: jobID, ArtifactID: &secondModerationID, Stage: domain.ModerationStageOutput, Decision: domain.ModerationSanitize, Provider: "private_moderation_provider"},
		},
	}
}

func persistResultScenario(t *testing.T, ctx context.Context, scenario *resultScenario) *Service {
	t.Helper()
	jobs := memory.NewJobRepo()
	artifacts := memory.NewArtifactRepo()
	moderation := memory.NewModerationRepo()
	if err := jobs.Create(ctx, scenario.job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	for _, artifact := range scenario.artifacts {
		if err := artifacts.Create(ctx, artifact); err != nil {
			t.Fatalf("create artifact: %v", err)
		}
	}
	for _, result := range scenario.moderation {
		if err := moderation.Create(ctx, result); err != nil {
			t.Fatalf("create moderation result: %v", err)
		}
	}
	return New(jobs, artifacts, moderation)
}

func createValidResultFixture(t *testing.T, ctx context.Context) (uuid.UUID, *domain.Job, []*domain.Artifact, *memory.JobRepo, *memory.ArtifactRepo, *memory.ModerationRepo) {
	t.Helper()
	scenario := newResultScenario()
	jobs := memory.NewJobRepo()
	artifacts := memory.NewArtifactRepo()
	moderation := memory.NewModerationRepo()
	if err := jobs.Create(ctx, scenario.job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	byID := make(map[uuid.UUID]*domain.Artifact, len(scenario.artifacts))
	for _, artifact := range scenario.artifacts {
		if err := artifacts.Create(ctx, artifact); err != nil {
			t.Fatalf("create artifact: %v", err)
		}
		byID[artifact.ID] = artifact
	}
	for _, result := range scenario.moderation {
		if err := moderation.Create(ctx, result); err != nil {
			t.Fatalf("create moderation result: %v", err)
		}
	}
	ordered := make([]*domain.Artifact, 0, len(scenario.job.OutputArtifactIDs))
	for _, artifactID := range scenario.job.OutputArtifactIDs {
		ordered = append(ordered, byID[artifactID])
	}
	return scenario.owner, scenario.job, ordered, jobs, artifacts, moderation
}

func createHistoryJob(t *testing.T, ctx context.Context, jobs *memory.JobRepo, owner uuid.UUID, status domain.JobStatus) *domain.Job {
	t.Helper()
	job := privateJob(owner, status, []uuid.UUID{uuid.New()})
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create history job: %v", err)
	}
	return job
}

func privateJob(owner uuid.UUID, status domain.JobStatus, outputIDs []uuid.UUID) *domain.Job {
	providerID, modelID := uuid.New(), uuid.New()
	return &domain.Job{
		ID:                uuid.New(),
		AccountID:         owner,
		Source:            "web",
		ResultMode:        domain.ResultModeAccountHistory,
		OperationType:     domain.OperationImageGenerate,
		Modality:          domain.ModalityImage,
		ProviderID:        &providerID,
		ModelID:           &modelID,
		Status:            status,
		Priority:          99,
		IdempotencyKey:    "private_idempotency_" + uuid.NewString(),
		CorrelationID:     "private_correlation",
		OutputArtifactIDs: append([]uuid.UUID(nil), outputIDs...),
		Params:            json.RawMessage(`{"private_params":true}`),
		PricingSnapshot:   json.RawMessage(`{"private_pricing":true}`),
		CostEstimate:      111,
		CostReserved:      222,
		CostCaptured:      333,
		ErrorCode:         "private_raw_error",
		ErrorMessage:      "private_raw_error",
	}
}

func privateArtifact(owner, id uuid.UUID, jobID *uuid.UUID, mediaType domain.MediaType, mimeType string, size int64, width, height int, durationMS int64) *domain.Artifact {
	return &domain.Artifact{
		ID:                      id,
		OwnerAccountID:          owner,
		JobID:                   jobID,
		Kind:                    domain.ArtifactKindOutput,
		MediaType:               mediaType,
		MimeType:                mimeType,
		StorageBucket:           "private_bucket",
		StorageKey:              "private_storage_key",
		PublicURL:               "https://private_url.invalid/object",
		SHA256:                  "private_sha256",
		ValidationPolicyVersion: "private_policy",
		LifecycleClass:          domain.ArtifactLifecycleClass("private_lifecycle"),
		SizeBytes:               size,
		Width:                   width,
		Height:                  height,
		DurationMS:              durationMS,
		Codec:                   "private_codec",
		Container:               "private_container",
		BitrateBPS:              999_999,
		ProbeStatus:             domain.MediaProbePassed,
		Status:                  domain.ArtifactStatusReady,
	}
}

func assertHistoryItem(t *testing.T, got HistoryItem, want *domain.Job) {
	t.Helper()
	if got.ID != want.ID || got.Operation != want.OperationType || got.Modality != want.Modality || got.Status != want.Status ||
		!got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("history item = %+v, want safe projection of %+v", got, want)
	}
}

func assertExactFields(t *testing.T, value any, want []string) {
	t.Helper()
	typ := reflect.TypeOf(value)
	got := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		if field := typ.Field(i); field.IsExported() {
			got = append(got, field.Name)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s exported fields = %v, want %v", typ.Name(), got, want)
	}
}

func assertOmits(t *testing.T, payload string, forbidden ...string) {
	t.Helper()
	for _, value := range forbidden {
		if strings.Contains(payload, value) {
			t.Fatalf("JSON leaked %q: %s", value, payload)
		}
	}
}
