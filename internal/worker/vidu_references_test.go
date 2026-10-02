package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestViduReferencesRequireOwnedJobBoundReadyImages(t *testing.T) {
	for _, model := range []string{"viduq3", "viduq3-mix"} {
		for _, scenario := range []string{"valid", "missing", "foreign-owner", "unbound", "mismatched-order", "not-ready", "output", "corrupt"} {
			t.Run(model+"/"+scenario, func(t *testing.T) {
				ctx := context.Background()
				owner, id := uuid.New(), uuid.New()
				artifacts, objects := memory.NewArtifactRepo(), memory.NewObjectStore()
				artifact := &domain.Artifact{ID: id, OwnerAccountID: owner, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeImage, Status: domain.ArtifactStatusReady, StorageBucket: "inputs", StorageKey: "reference.png", MimeType: "image/png"}
				var buf bytes.Buffer
				if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 128, 128))); err != nil {
					t.Fatal(err)
				}
				data := buf.Bytes()
				job := &domain.Job{ID: uuid.New(), AccountID: owner, OperationType: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, InputArtifactIDs: []uuid.UUID{id}}
				params := promptParams{Provider: domain.ProviderAPIMart, ModelCode: model, Prompt: "A paper boat on a pond", ReferenceArtifactIDs: []uuid.UUID{id}, AspectRatio: "16:9", DurationSec: 5}
				switch scenario {
				case "missing":
					params.ReferenceArtifactIDs = nil
					job.InputArtifactIDs = nil
				case "foreign-owner":
					artifact.OwnerAccountID = uuid.New()
				case "unbound":
					job.InputArtifactIDs = []uuid.UUID{uuid.New()}
				case "mismatched-order":
					job.InputArtifactIDs = []uuid.UUID{id, uuid.New()}
					params.ReferenceArtifactIDs = []uuid.UUID{id, id}
				case "not-ready":
					artifact.Status = domain.ArtifactStatus("pending")
				case "output":
					artifact.Kind = domain.ArtifactKindOutput
				case "corrupt":
					data = []byte("not an image")
				}
				if err := artifacts.Create(ctx, artifact); err != nil {
					t.Fatal(err)
				}
				if err := objects.Put(ctx, artifact.StorageBucket, artifact.StorageKey, data, artifact.MimeType); err != nil {
					t.Fatal(err)
				}
				job.Params, _ = json.Marshal(params)
				p := processor{artifactRepo: artifacts, objects: objects, videoDurationSec: 5, videoResolution: "720p"}
				req, err := p.buildRequest(ctx, job, 1)
				if scenario != "valid" {
					if err == nil {
						t.Fatal("invalid reference accepted")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(req.InputURLs) != 1 || !strings.HasPrefix(req.InputURLs[0], "data:image/png;base64,") || req.ReferenceArtifactIDs[0] != id || req.KeepOriginalSound {
					t.Fatal("owned reference not hydrated into the restricted Vidu request")
				}
				if strings.Contains(string(job.Params), "data:") || strings.Contains(string(req.Params), "data:") {
					t.Fatal("hydrated bytes leaked into persisted or native params")
				}
			})
		}
	}
}
