package websession

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestSafeImageJobProjectsOnlyBoundedPublicGeometry(t *testing.T) {
	for _, tc := range []struct {
		ratio, wantRatio string
		count, wantCount int
	}{
		{"9:16", "9:16", 4, 4},
		{"1:1", "1:1", 15, 15},
		{"", "", 0, 0},
		{"0:16", "", 99, 0},
		{"private-routing-value", "", -1, 0},
	} {
		params, _ := json.Marshal(map[string]any{"prompt": "Test", "model_id": "test", "model_name": "Test", "image_quality": "2K", "aspect_ratio": tc.ratio, "output_count": tc.count, "provider": "private-provider", "model_code": "private-model"})
		job, ok := newSafeImageJob(&domain.Job{ID: uuid.New(), Source: "web", OperationType: domain.OperationImageGenerate, Modality: domain.ModalityImage, CostEstimate: 10, Params: params})
		if !ok || job.AspectRatio != tc.wantRatio || job.OutputCount != tc.wantCount {
			t.Fatalf("unexpected public geometry: %+v, valid=%v", job, ok)
		}
		data, _ := json.Marshal(job)
		var dto map[string]any
		_ = json.Unmarshal(data, &dto)
		if _, exists := dto["provider"]; exists {
			t.Fatal("provider leaked")
		}
		if _, exists := dto["model_code"]; exists {
			t.Fatal("worker model leaked")
		}
	}
}
