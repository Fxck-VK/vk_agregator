package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/domain"
)

func TestTurboH3IgnoresUnsupportedRuntimeDraft(t *testing.T) {
	for _, model := range []string{"MiniMax-H3", "kling-3.0-turbo"} {
		for _, configured := range []bool{false, true} {
			t.Run(model, func(t *testing.T) {
				resolution, duration := "720p", 3
				if model == "MiniMax-H3" {
					resolution, duration = "768p", 4
				}
				params, _ := json.Marshal(promptParams{Provider: domain.ProviderAPIMart, ModelCode: model, Prompt: "Synthetic shapes move slowly", DurationSec: duration, AspectRatio: "16:9"})
				p := processor{videoDraft: configured, videoResolution: resolution}
				req, err := p.buildRequest(context.Background(), &domain.Job{ID: uuid.New(), Modality: domain.ModalityVideo, OperationType: domain.OperationVideoGenerate, Params: params}, 1)
				if err != nil {
					t.Fatal(err)
				}
				if req.Draft {
					t.Fatal("unsupported runtime draft leaked into request")
				}
				if _, err := apimart.New(apimart.Config{}).Estimate(context.Background(), req); err != nil {
					t.Fatalf("worker request rejected by real adapter validation: %v", err)
				}
			})
		}
	}
}
func TestLegacyVideoKeepsRuntimeDraft(t *testing.T) {
	params, _ := json.Marshal(promptParams{Provider: domain.ProviderDeepInfra, ModelCode: "PrunaAI/p-video", Prompt: "Synthetic shapes move slowly", DurationSec: 2, AspectRatio: "16:9"})
	p := processor{videoDraft: true, videoResolution: "480p"}
	req, err := p.buildRequest(context.Background(), &domain.Job{ID: uuid.New(), Modality: domain.ModalityVideo, OperationType: domain.OperationVideoGenerate, Params: params}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !req.Draft {
		t.Fatal("legacy draft behavior changed")
	}
}
