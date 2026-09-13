package apimart

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func flux2Request() domain.ProviderRequest {
	return domain.ProviderRequest{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ModelCode: "flux-2-pro", Prompt: "Synthetic landscape", Resolution: "1MP", AspectRatio: "9:21", OutputCount: 1, IdempotencyKey: "flux-test"}
}

func TestFlux2ProContract(t *testing.T) {
	for _, resolution := range []string{"1MP", "2MP", "3MP", "4MP"} {
		t.Run(resolution, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "POST" || r.URL.Path != "/v1/images/generations" || r.Header.Get("Idempotency-Key") == "" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("incorrect endpoint or headers")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["model"] != "flux-2-pro" || body["resolution"] != resolution || body["size"] != "9:21" || body["n"] != float64(1) || body["prompt_upsampling"] != false || body["output_format"] != "png" {
					t.Error("incorrect FLUX.2 parameters")
				}
				for _, key := range []string{"image_urls", "width", "height", "steps", "guidance", "negative_prompt"} {
					if _, ok := body[key]; ok {
						t.Errorf("unexpected native field %s", key)
					}
				}
				_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"flux-task","status":"submitted"}]}`))
			}))
			defer srv.Close()
			p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			req := flux2Request()
			req.Resolution = resolution
			if _, err := p.Estimate(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					task, err := p.Submit(context.Background(), req)
					if err != nil || task.ExternalID != "flux-task" {
						t.Error("submission failed")
					}
				}()
			}
			wg.Wait()
			if calls.Load() != 1 {
				t.Fatal("concurrent replay created multiple paid tasks")
			}
		})
	}
}

func TestFlux2ProRejectsUnpricedOptions(t *testing.T) {
	changes := []func(*domain.ProviderRequest){
		func(r *domain.ProviderRequest) { r.Resolution = "1K" },
		func(r *domain.ProviderRequest) { r.Resolution = "2K" },
		func(r *domain.ProviderRequest) { r.Resolution = "4K" },
		func(r *domain.ProviderRequest) { r.Resolution = "" },
		func(r *domain.ProviderRequest) { r.AspectRatio = "auto" },
		func(r *domain.ProviderRequest) { r.Size = "1024x1024" },
		func(r *domain.ProviderRequest) { r.AspectRatio = "4:5" },
		func(r *domain.ProviderRequest) { r.OutputCount = 2 },
		func(r *domain.ProviderRequest) { r.Prompt = "" },
		func(r *domain.ProviderRequest) { r.NegativePrompt = "blur" },
		func(r *domain.ProviderRequest) { r.InputURLs = []string{"https://example.com/input.png"} },
	}
	for _, raw := range []string{`{"width":1024}`, `{"height":1024}`, `{"n":2}`, `{"steps":30}`, `{"guidance":4.5}`, `{"image_urls":[]}`, `{"prompt_upsampling":true}`, `{"resolution":"4MP"}`, `{"output_format":"webp"}`, `{`} {
		changes = append(changes, func(r *domain.ProviderRequest) { r.Params = json.RawMessage(raw) })
	}
	for i, change := range changes {
		req := flux2Request()
		change(&req)
		p := New(Config{BaseURL: "http://127.0.0.1:1/v1"})
		_, err := p.Submit(context.Background(), req)
		var pe *Error
		if !errors.As(err, &pe) || pe.Class != domain.ProviderErrInvalidRequest {
			t.Fatalf("case %d: expected local rejection, got %v", i, err)
		}
	}
}
