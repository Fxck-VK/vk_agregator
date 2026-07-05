package memory

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestJobRepoReadsAccountFirstWithLegacyFallback(t *testing.T) {
	ctx := context.Background()
	repo := NewJobRepo()
	accountID := uuid.New()
	legacyUserID := uuid.New()

	accountJob := domain.Job{
		UserID:         uuid.New(),
		AccountID:      accountID,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusSucceeded,
		IdempotencyKey: "job-account",
		Source:         "test",
	}
	if err := repo.Create(ctx, &accountJob); err != nil {
		t.Fatalf("create account job: %v", err)
	}

	legacyJob := domain.Job{
		UserID:         legacyUserID,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusSucceeded,
		IdempotencyKey: "job-legacy",
		Source:         "test",
	}
	if err := repo.Create(ctx, &legacyJob); err != nil {
		t.Fatalf("create legacy job: %v", err)
	}

	accountJobs, err := repo.ListByUser(ctx, accountID, 10, 0)
	if err != nil {
		t.Fatalf("list by account: %v", err)
	}
	if len(accountJobs) != 1 || accountJobs[0].ID != accountJob.ID {
		t.Fatalf("account-first list returned %#v, want only account job %s", idsFromJobs(accountJobs), accountJob.ID)
	}

	legacyJobs, err := repo.ListByUser(ctx, legacyUserID, 10, 0)
	if err != nil {
		t.Fatalf("list by legacy user: %v", err)
	}
	if len(legacyJobs) != 1 || legacyJobs[0].ID != legacyJob.ID {
		t.Fatalf("legacy fallback list returned %#v, want only legacy job %s", idsFromJobs(legacyJobs), legacyJob.ID)
	}

	accountSucceeded, err := repo.CountSucceededByUser(ctx, accountID)
	if err != nil {
		t.Fatalf("count account succeeded: %v", err)
	}
	if accountSucceeded != 1 {
		t.Fatalf("account succeeded count = %d, want 1", accountSucceeded)
	}

	legacySucceeded, err := repo.CountSucceededByUser(ctx, legacyUserID)
	if err != nil {
		t.Fatalf("count legacy succeeded: %v", err)
	}
	if legacySucceeded != 1 {
		t.Fatalf("legacy succeeded count = %d, want 1", legacySucceeded)
	}
}

func TestPaymentRepoReadsAccountFirstWithLegacyFallback(t *testing.T) {
	ctx := context.Background()
	repo := NewPaymentRepo()
	accountID := uuid.New()
	legacyUserID := uuid.New()

	accountIntent := domain.PaymentIntent{
		UserID:         uuid.New(),
		AccountID:      accountID,
		Status:         domain.PaymentIntentWaitingForUser,
		Amount:         1000,
		Currency:       domain.CurrencyRUB,
		Credits:        10,
		Provider:       domain.PaymentProviderMock,
		IdempotencyKey: "pay-account",
	}
	if err := repo.CreateIntent(ctx, &accountIntent); err != nil {
		t.Fatalf("create account intent: %v", err)
	}

	legacyIntent := domain.PaymentIntent{
		UserID:         legacyUserID,
		Status:         domain.PaymentIntentWaitingForUser,
		Amount:         1000,
		Currency:       domain.CurrencyRUB,
		Credits:        10,
		Provider:       domain.PaymentProviderMock,
		IdempotencyKey: "pay-legacy",
	}
	if err := repo.CreateIntent(ctx, &legacyIntent); err != nil {
		t.Fatalf("create legacy intent: %v", err)
	}

	accountIntents, err := repo.ListIntentsByUser(ctx, accountID, 10, 0)
	if err != nil {
		t.Fatalf("list account intents: %v", err)
	}
	if len(accountIntents) != 1 || accountIntents[0].ID != accountIntent.ID {
		t.Fatalf("account-first intents returned %#v, want only account intent %s", idsFromIntents(accountIntents), accountIntent.ID)
	}

	legacyIntents, err := repo.ListIntentsByUser(ctx, legacyUserID, 10, 0)
	if err != nil {
		t.Fatalf("list legacy intents: %v", err)
	}
	if len(legacyIntents) != 1 || legacyIntents[0].ID != legacyIntent.ID {
		t.Fatalf("legacy fallback intents returned %#v, want only legacy intent %s", idsFromIntents(legacyIntents), legacyIntent.ID)
	}
}

func TestArtifactRepoReadsAccountFirstWithLegacyFallback(t *testing.T) {
	ctx := context.Background()
	repo := NewArtifactRepo()
	accountID := uuid.New()
	legacyUserID := uuid.New()

	accountArtifact := inputReferenceArtifact(uuid.New(), accountID, "sha-account")
	if err := repo.Create(ctx, &accountArtifact); err != nil {
		t.Fatalf("create account artifact: %v", err)
	}

	legacyArtifact := inputReferenceArtifact(legacyUserID, uuid.Nil, "sha-legacy")
	if err := repo.Create(ctx, &legacyArtifact); err != nil {
		t.Fatalf("create legacy artifact: %v", err)
	}

	gotAccount, err := repo.GetBySHA256(ctx, accountID, "sha-account")
	if err != nil {
		t.Fatalf("get account artifact by sha: %v", err)
	}
	if gotAccount.ID != accountArtifact.ID {
		t.Fatalf("account artifact ID = %s, want %s", gotAccount.ID, accountArtifact.ID)
	}

	gotLegacy, err := repo.GetBySHA256(ctx, legacyUserID, "sha-legacy")
	if err != nil {
		t.Fatalf("get legacy artifact by sha: %v", err)
	}
	if gotLegacy.ID != legacyArtifact.ID {
		t.Fatalf("legacy artifact ID = %s, want %s", gotLegacy.ID, legacyArtifact.ID)
	}

	reusableAccount, err := repo.FindReusableInputReference(ctx, accountID, "sha-account", "policy-v1", "image/png")
	if err != nil {
		t.Fatalf("find reusable account artifact: %v", err)
	}
	if reusableAccount.ID != accountArtifact.ID {
		t.Fatalf("reusable account artifact ID = %s, want %s", reusableAccount.ID, accountArtifact.ID)
	}

	reusableLegacy, err := repo.FindReusableInputReference(ctx, legacyUserID, "sha-legacy", "policy-v1", "image/png")
	if err != nil {
		t.Fatalf("find reusable legacy artifact: %v", err)
	}
	if reusableLegacy.ID != legacyArtifact.ID {
		t.Fatalf("reusable legacy artifact ID = %s, want %s", reusableLegacy.ID, legacyArtifact.ID)
	}
}

func inputReferenceArtifact(ownerUserID, ownerAccountID uuid.UUID, sha string) domain.Artifact {
	return domain.Artifact{
		OwnerUserID:             ownerUserID,
		OwnerAccountID:          ownerAccountID,
		Kind:                    domain.ArtifactKindInput,
		MediaType:               domain.MediaTypeImage,
		MimeType:                "image/png",
		StorageBucket:           "bucket",
		StorageKey:              sha + ".png",
		SHA256:                  sha,
		ValidationPolicyVersion: "policy-v1",
		LifecycleClass:          domain.ArtifactLifecycleInputReference,
		Status:                  domain.ArtifactStatusReady,
		SizeBytes:               1024,
	}
}

func idsFromJobs(jobs []*domain.Job) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(jobs))
	for _, job := range jobs {
		ids = append(ids, job.ID)
	}
	return ids
}

func idsFromIntents(intents []*domain.PaymentIntent) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(intents))
	for _, intent := range intents {
		ids = append(ids, intent.ID)
	}
	return ids
}
