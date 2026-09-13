package worker

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

type motionTestSigner struct{ calls int }

func (s *motionTestSigner) URL(jobID, artifactID uuid.UUID) (string, error) {
	s.calls++
	return "https://media.example/provider-references/" + jobID.String() + "/" + artifactID.String() + ".mp4?signature=synthetic", nil
}

func TestMotionReferenceUsesOwnedProbedArtifactAndImmutablePrice(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewArtifactRepo()
	owner := uuid.New()
	id := uuid.New()
	jobID := uuid.New()
	a := &domain.Artifact{ID: id, OwnerAccountID: owner, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeVideo, Status: domain.ArtifactStatusReady, SizeBytes: 1234, MimeType: "video/mp4", Width: 1280, Height: 720, DurationMS: 5100, Codec: "h264", Container: "mp4", BitrateBPS: 100000, ProbeStatus: domain.MediaProbePassed}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	price, err := prices.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteKling26Motion, Resolution: "std", DurationSec: 6})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(price)
	job := &domain.Job{ID: jobID, AccountID: owner, InputArtifactIDs: []uuid.UUID{id}, PricingSnapshot: raw}
	pp := promptParams{ReferenceVideoArtifactID: id, CharacterOrientation: "image"}
	signer := &motionTestSigner{}
	p := processor{artifactRepo: repo, providerReferences: signer}
	url, err := p.motionReferenceURL(ctx, job, pp, "kling-v2-6-motion-control", 6)
	if err != nil || !strings.Contains(url, id.String()) || signer.calls != 1 {
		t.Fatal("valid reference not signed")
	}
	for _, mutate := range []func(){func() { job.AccountID = uuid.New() }, func() { job.InputArtifactIDs = nil }, func() { job.PricingSnapshot = nil }} {
		job.AccountID = owner
		job.InputArtifactIDs = []uuid.UUID{id}
		job.PricingSnapshot = raw
		mutate()
		if _, err := p.motionReferenceURL(ctx, job, pp, "kling-v2-6-motion-control", 6); err == nil {
			t.Fatal("invalid reference accepted")
		}
	}
	if signer.calls != 1 {
		t.Fatal("invalid reference signed")
	}
}
