package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestAccountNativePreparedScopedRepositoriesAreStrict(t *testing.T) {
	ctx := context.Background()
	owner := uuid.New()
	foreign := uuid.New()
	jobs := NewJobRepo()
	job := &domain.Job{ID: uuid.New(), AccountID: owner, Status: domain.JobStatusPrepared, IdempotencyKey: "native-prepared"}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if _, err := jobs.GetByIDForAccount(ctx, foreign, job.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign job read = %v, want ErrNotFound", err)
	}
	if _, err := jobs.GetByIdempotencyKeyForAccount(ctx, foreign, job.IdempotencyKey); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign key read = %v, want ErrNotFound", err)
	}
	if listed, err := jobs.ListByAccount(ctx, foreign, 10, 0); err != nil || len(listed) != 0 {
		t.Fatalf("foreign list = %+v, %v", listed, err)
	}

	artifacts := NewArtifactRepo()
	artifact := &domain.Artifact{ID: uuid.New(), OwnerAccountID: owner, OwnerUserID: uuid.Nil, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeImage, MimeType: "image/png", StorageBucket: "artifacts", StorageKey: "input.png", SHA256: "native-sha", ValidationPolicyVersion: "policy", LifecycleClass: domain.ArtifactLifecycleInputReference, Status: domain.ArtifactStatusReady}
	if err := artifacts.Create(ctx, artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	if _, err := artifacts.GetByIDForAccount(ctx, foreign, artifact.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign artifact read = %v, want ErrNotFound", err)
	}
	if _, err := artifacts.FindReusableInputReferenceForAccount(ctx, foreign, artifact.SHA256, "policy", artifact.MimeType); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign artifact reuse = %v, want ErrNotFound", err)
	}
}

func TestCountActiveByAccountOperationNeverFallsBackToLegacyUser(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	legacyAccountID := uuid.New()
	jobs := NewJobRepo()

	legacy := &domain.Job{
		ID:             uuid.New(),
		UserID:         accountID,
		AccountID:      legacyAccountID,
		Source:         "legacy",
		ResultMode:     domain.ResultModeLegacyUnknown,
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "legacy-user-capacity",
	}
	if err := jobs.Create(ctx, legacy); err != nil {
		t.Fatalf("create legacy job: %v", err)
	}

	count, err := jobs.CountActiveByAccountOperation(ctx, accountID, domain.OperationVideoGenerate)
	if err != nil {
		t.Fatalf("count account-only active jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("account-only active jobs = %d, want 0 without legacy user fallback", count)
	}

	native := &domain.Job{
		ID:             uuid.New(),
		AccountID:      accountID,
		Source:         "web",
		ResultMode:     domain.ResultModeLegacyUnknown,
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "native-account-capacity",
	}
	if err := jobs.Create(ctx, native); err != nil {
		t.Fatalf("create native job: %v", err)
	}

	count, err = jobs.CountActiveByAccountOperation(ctx, accountID, domain.OperationVideoGenerate)
	if err != nil {
		t.Fatalf("count native active jobs: %v", err)
	}
	if count != 1 {
		t.Fatalf("account-only active jobs = %d, want 1", count)
	}
}

func TestJobRepoListCursorFiltersExactAccountSourceAndOperation(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	jobs := NewJobRepo()
	for _, job := range []*domain.Job{
		{ID: uuid.New(), AccountID: accountID, Source: "web", OperationType: domain.OperationImageGenerate, Modality: domain.ModalityImage, Status: domain.JobStatusPrepared, IdempotencyKey: "web-image"},
		{ID: uuid.New(), AccountID: accountID, Source: "miniapp", OperationType: domain.OperationImageGenerate, Modality: domain.ModalityImage, Status: domain.JobStatusPrepared, IdempotencyKey: "miniapp-image"},
		{ID: uuid.New(), AccountID: accountID, Source: "web", OperationType: domain.OperationTextGenerate, Modality: domain.ModalityText, Status: domain.JobStatusPrepared, IdempotencyKey: "web-text"},
	} {
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatalf("create job: %v", err)
		}
	}

	got, err := jobs.ListCursor(ctx, domain.JobFilter{
		AccountID: &accountID,
		Source:    "web",
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
	}, 10, nil)
	if err != nil {
		t.Fatalf("list filtered jobs: %v", err)
	}
	if len(got) != 1 || got[0].Source != "web" || got[0].OperationType != domain.OperationImageGenerate || got[0].Modality != domain.ModalityImage {
		t.Fatalf("filtered jobs = %+v", got)
	}
}
