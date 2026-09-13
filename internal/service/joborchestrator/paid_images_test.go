package joborchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestCreatePaidImageRejectsMalformedOrMissingParamsForBoundedSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params json.RawMessage
	}{
		{name: "malformed", params: json.RawMessage(`{`)},
		{name: "missing", params: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			userID := uuid.New()
			snapshot := seedream50ProSnapshot(t, "16:9", 0)

			job, err := f.orch.CreateJob(context.Background(), joborchestrator.CreateJobInput{
				UserID:          userID,
				Source:          "miniapp",
				Operation:       domain.OperationImageGenerate,
				Modality:        domain.ModalityImage,
				IdempotencyKey:  "miniapp:image:bad-params:" + tc.name,
				Params:          tc.params,
				PricingSnapshot: snapshot,
			})

			if !errors.Is(err, joborchestrator.ErrBackendPriceRequired) {
				t.Fatalf("CreateJob() job=%+v err=%v, want ErrBackendPriceRequired", job, err)
			}
			assertNoJobReservationOrTask(t, f, userID)
		})
	}
}

func TestCreateLegacyPricedImageStillAllowsNilParamsWithoutSnapshot(t *testing.T) {
	f := newFixture()

	job, err := f.orch.CreateJob(context.Background(), joborchestrator.CreateJobInput{
		UserID:              uuid.New(),
		Source:              "miniapp",
		Operation:           domain.OperationImageGenerate,
		Modality:            domain.ModalityImage,
		IdempotencyKey:      "miniapp:image:legacy-nil-params",
		CostEstimateCredits: 15,
	})

	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if job.Params != nil || job.CostReserved != 15 {
		t.Fatalf("legacy job params/reserved = %q/%d, want nil/15", string(job.Params), job.CostReserved)
	}
}

func TestCreatePaidImageIdempotentReplayAllowsSameIntentWithNewPriceSnapshot(t *testing.T) {
	f := newFixture()
	userID := uuid.New()
	in := seedream50ProInput(t, userID, "miniapp:image:replay-same-intent", "make a poster", "16:9", nil)

	first, err := f.orch.CreateJob(context.Background(), in)
	if err != nil {
		t.Fatalf("first CreateJob() error = %v", err)
	}

	replay := in
	replay.PricingSnapshot = changedValidImageSnapshot(replay.PricingSnapshot, replay.PricingSnapshot.InternalCredits+7)
	second, err := f.orch.CreateJob(context.Background(), replay)
	if err != nil {
		t.Fatalf("idempotent replay error = %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("replay returned job %s, want %s", second.ID, first.ID)
	}
	if second.CostEstimate != first.CostEstimate {
		t.Fatalf("replay cost = %d, want original %d", second.CostEstimate, first.CostEstimate)
	}
}

func TestCreatePaidImageIdempotentReplayRejectsChangedIntent(t *testing.T) {
	f := newFixture()
	userID := uuid.New()
	in := seedream50ProInput(t, userID, "miniapp:image:replay-changed-prompt", "make a poster", "16:9", nil)
	if _, err := f.orch.CreateJob(context.Background(), in); err != nil {
		t.Fatalf("first CreateJob() error = %v", err)
	}

	replay := seedream50ProInput(t, userID, in.IdempotencyKey, "make a logo", "16:9", nil)
	if _, err := f.orch.CreateJob(context.Background(), replay); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed prompt replay err=%v, want ErrConflict", err)
	}
}

func TestCreatePaidImageIdempotentReplayRejectsReferenceOrderChange(t *testing.T) {
	f := newFixture()
	userID := uuid.New()
	firstRef := seedInputArtifact(t, f, userID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, "inputs", "a.png")
	secondRef := seedInputArtifact(t, f, userID, domain.ArtifactKindInput, domain.MediaTypeImage, domain.ArtifactStatusReady, "inputs", "b.png")
	in := seedream50ProInput(t, userID, "miniapp:image:replay-ref-order", "make a poster", "16:9", []uuid.UUID{firstRef, secondRef})
	if _, err := f.orch.CreateJob(context.Background(), in); err != nil {
		t.Fatalf("first CreateJob() error = %v", err)
	}

	replay := seedream50ProInput(t, userID, in.IdempotencyKey, "make a poster", "16:9", []uuid.UUID{secondRef, firstRef})
	if _, err := f.orch.CreateJob(context.Background(), replay); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("reference-order replay err=%v, want ErrConflict", err)
	}
}

func seedream50ProInput(t *testing.T, userID uuid.UUID, idempotencyKey, prompt, aspectRatio string, refs []uuid.UUID) joborchestrator.CreateJobInput {
	t.Helper()
	params, err := json.Marshal(struct {
		Provider             domain.ProviderName `json:"provider"`
		ModelID              string              `json:"model_id"`
		ModelCode            string              `json:"model_code"`
		Prompt               string              `json:"prompt"`
		ImageQuality         string              `json:"image_quality"`
		Resolution           string              `json:"resolution"`
		Size                 string              `json:"size"`
		AspectRatio          string              `json:"aspect_ratio"`
		OutputCount          int                 `json:"output_count"`
		ReferenceArtifactIDs []uuid.UUID         `json:"reference_artifact_ids,omitempty"`
	}{
		Provider:             domain.ProviderAPIMart,
		ModelID:              pricingcatalog.PublicImageSeedream50Pro,
		ModelCode:            providermodels.ProviderModelSeedream50Pro,
		Prompt:               prompt,
		ImageQuality:         "1K",
		Resolution:           "1K",
		Size:                 aspectRatio,
		AspectRatio:          aspectRatio,
		OutputCount:          1,
		ReferenceArtifactIDs: refs,
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	return joborchestrator.CreateJobInput{
		UserID:           userID,
		Source:           "miniapp",
		Operation:        domain.OperationImageGenerate,
		Modality:         domain.ModalityImage,
		IdempotencyKey:   idempotencyKey,
		InputArtifactIDs: append([]uuid.UUID(nil), refs...),
		Params:           params,
		PricingSnapshot:  seedream50ProSnapshot(t, aspectRatio, len(refs)),
	}
}

func seedream50ProSnapshot(t *testing.T, aspectRatio string, refs int) pricingcatalog.PricingSnapshot {
	t.Helper()
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static catalog: %v", err)
	}
	snapshot, err := catalog.Snapshot(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: pricingcatalog.PublicImageSeedream50Pro,
		Quality:      "1K",
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshot, err = pricingcatalog.QuoteAPIMartImage(snapshot, aspectRatio, refs)
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	return snapshot
}

func changedValidImageSnapshot(snapshot pricingcatalog.PricingSnapshot, credits int64) pricingcatalog.PricingSnapshot {
	snapshot.Version++
	snapshot.Source = "runtime_db"
	snapshot.InternalCredits = credits
	snapshot.InternalCreditCap = credits
	snapshot.DefaultDisplayCredits = credits
	return snapshot
}
