package apimart

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func expansionImageRequest(model, resolution string) domain.ProviderRequest {
	return domain.ProviderRequest{Provider: domain.ProviderAPIMart, ModelCode: model, Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, Prompt: "Synthetic landscape", Resolution: resolution, AspectRatio: "16:9", OutputCount: 1, IdempotencyKey: "image-expansion:" + model + resolution}
}

func TestImageExpansionWireAndRetry(t *testing.T) {
	for _, tc := range []struct{ model, resolution string }{
		{"seedream-5-0-flash", "1K"}, {"seedream-5-0-flash", "1.5K"}, {"seedream-5-0-flash", "2K"},
		{"z-image-turbo", "1K"}, {"z-image-turbo", "2K"},
		{"flux-2-max", "1MP"}, {"flux-2-max", "4MP"}, {"flux-2-flex", "2MP"}, {"flux-2-flex", "3MP"},
		{"qwen-image-3.0-pro", "1K"}, {"qwen-image-3.0-pro", "2K"},
	} {
		t.Run(tc.model+tc.resolution, func(t *testing.T) {
			r := expansionImageRequest(tc.model, tc.resolution)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
				calls++
				if q.Method != "POST" || q.URL.Path != "/v1/images/generations" || q.Header.Get("Idempotency-Key") != r.IdempotencyKey {
					t.Error("incorrect endpoint or idempotency")
				}
				var body map[string]any
				if err := json.NewDecoder(q.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["model"] != tc.model || body["resolution"] != tc.resolution || body["size"] != "16:9" || body["nsfw_check"] != true {
					t.Errorf("incorrect wire dimensions: %v", body)
				}
				if tc.model == "z-image-turbo" {
					if _, exists := body["n"]; exists {
						t.Error("Z-Image rejects n")
					}
				} else if body["n"] != float64(1) {
					t.Error("missing single-output bound")
				}
				for _, key := range []string{"image_urls", "width", "height", "layer_decomposition", "tools", "stream"} {
					if _, exists := body[key]; exists {
						t.Errorf("unexpected %s", key)
					}
				}
				_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"fixture-image","status":"submitted"}]}`))
			}))
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			if _, err := p.Estimate(context.Background(), r); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if _, err := p.Submit(context.Background(), r); err != nil {
					t.Fatal(err)
				}
			}
			if calls != 1 {
				t.Fatalf("duplicate submit: %d", calls)
			}
		})
	}
}

func TestImageExpansionRejectsUnpricedInputsBeforeHTTP(t *testing.T) {
	for _, model := range []string{"seedream-5-0-flash", "z-image-turbo", "flux-2-max", "flux-2-flex", "qwen-image-3.0-pro"} {
		quality := "1K"
		if strings.HasPrefix(model, "flux-") {
			quality = "1MP"
		}
		for _, change := range []func(*domain.ProviderRequest){
			func(r *domain.ProviderRequest) { r.Resolution = "4K" },
			func(r *domain.ProviderRequest) { r.Size = "2048x2048" },
			func(r *domain.ProviderRequest) { r.OutputCount = 2 },
			func(r *domain.ProviderRequest) { r.InputURLs = []string{"https://example.com/image.png"} },
			func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"resolution":"4K"}`) },
			func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"layer_decomposition":true}`) },
			func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"prompt_extend":true}`) },
			func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"width":2048,"height":2048}`) },
			func(r *domain.ProviderRequest) { r.Prompt = "" },
			func(r *domain.ProviderRequest) { r.Operation = domain.OperationTextGenerate },
		} {
			r := expansionImageRequest(model, quality)
			change(&r)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(500) }))
			p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			if _, err := p.Submit(context.Background(), r); err == nil {
				t.Errorf("%s accepted invalid input", model)
			}
			if _, err := p.Estimate(context.Background(), r); err == nil {
				t.Errorf("%s priced invalid input", model)
			}
			srv.Close()
			if calls != 0 {
				t.Fatal("invalid input reached network")
			}
		}
	}
	r := expansionImageRequest("z-image-turbo", "1K")
	r.Prompt = strings.Repeat("я", 800)
	if _, err := buildNextVisualBody(r); err != nil {
		t.Fatal(err)
	}
	r.Prompt += "я"
	if _, err := buildNextVisualBody(r); err == nil {
		t.Fatal("801-character Z-Image prompt accepted")
	}
}
