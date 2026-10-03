package apimart

import (
	"encoding/base64"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestNativeSingleImageCountsRejectBeforeSubmission(t *testing.T) {
	for _, model := range []string{ModelGemini3ProImage, ModelGPTImage2} {
		for _, count := range []int{-1, 0, 1, 2, 4} {
			req := domain.ProviderRequest{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ModelCode: model, Prompt: "fixture", OutputCount: count}
			err := validateImageShape(req, true)
			wantError := count < 0 || count > 1
			if (err != nil) != wantError {
				t.Errorf("model=%s count=%d error=%v wantError=%v", model, count, err, wantError)
			}
		}
	}
}

func TestGPTImage2ReferenceCountBoundary(t *testing.T) {
	for _, count := range []int{15, 16} {
		req := domain.ProviderRequest{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ModelCode: ModelGPTImage2, Prompt: "fixture"}
		for i := 0; i < count; i++ {
			req.InputURLs = append(req.InputURLs, "https://media.example/reference.png")
		}
		if err := validateImageShape(req, true); (err != nil) != (count > 15) {
			t.Errorf("references=%d error=%v", count, err)
		}
	}
}

func TestGeminiReferenceBytesMatchExistingWebUploadBoundary(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString(strings.SplitN(testPNGDataURL(t), ",", 2)[1])
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{20 << 20, (20 << 20) + 1} {
		data := make([]byte, size)
		copy(data, png)
		req := domain.ProviderRequest{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ModelCode: ModelGemini3ProImage, Prompt: "fixture", InputURLs: []string{"data:image/png;base64," + base64.StdEncoding.EncodeToString(data)}}
		if err := validateImageShape(req, true); (err != nil) != (size > 20<<20) {
			t.Errorf("reference bytes=%d error=%v", size, err)
		}
	}
}
