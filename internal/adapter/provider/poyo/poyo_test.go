package poyo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	providertest "vk-ai-aggregator/internal/adapter/provider/providertest"
	"vk-ai-aggregator/internal/domain"
)

func TestCapabilitiesAdvertiseSupportedMedia(t *testing.T) {
	provider := New(Config{APIKey: "test-key"})
	caps, err := provider.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	providertest.RequireCapability(t, caps, domain.OperationImageGenerate, domain.ModalityImage, ModelNanoBanana2New)
	providertest.RequireCapability(t, caps, domain.OperationImageGenerate, domain.ModalityImage, ModelNanoBananaPro)
	providertest.RequireCapability(t, caps, domain.OperationImageGenerate, domain.ModalityImage, "seedream-4.5")
	providertest.RequireCapability(t, caps, domain.OperationVideoGenerate, domain.ModalityVideo, ModelKlingO3Standard)
	providertest.RequireCapability(t, caps, domain.OperationVideoGenerate, domain.ModalityVideo, ModelSeedance20Fast)
	providertest.RequireCapability(t, caps, domain.OperationVideoGenerate, domain.ModalityVideo, ModelRunwayGen45)
}

func TestSubmitKlingO3SuccessAndIdempotency(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "idem-1" {
			t.Fatalf("idempotency header = %q", got)
		}
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != ModelKlingO3Standard {
			t.Fatalf("model = %q", body.Model)
		}
		if body.Input["prompt"] != "safe prompt" {
			t.Fatalf("bad input prompt: %+v", body.Input)
		}
		if refs, ok := body.Input["reference_image_urls"].([]any); !ok || len(refs) != 1 || refs[0] != "https://cdn.test/input.png" {
			t.Fatalf("reference_image_urls = %#v", body.Input["reference_image_urls"])
		}
		if body.Input["duration"].(float64) != 10 || body.Input["aspect_ratio"] != "16:9" {
			t.Fatalf("bad input options: %+v", body.Input)
		}
		if body.Input["multi_shots"] != false {
			t.Fatalf("kling multi_shots must be explicitly disabled, got %+v", body.Input)
		}
		if body.Input["sound"] != false {
			t.Fatalf("kling sound must be explicitly disabled, got %+v", body.Input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"poyo_task_1","status":"not_started","created_time":"2026-06-19T15:00:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseVideoRequest(ModelKlingO3Standard)
	req.IdempotencyKey = "idem-1"
	req.DurationSec = 10
	req.Resolution = "1080p"
	req.AspectRatio = "16:9"
	req.InputURLs = []string{"https://cdn.test/input.png"}

	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderPoYo || task.ExternalID != "poyo_task_1" || task.Status != domain.ProviderTaskPending {
		t.Fatalf("bad task: %+v", task)
	}
	task2, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if task2.ExternalID != task.ExternalID || calls != 1 {
		t.Fatalf("idempotency failed task2=%+v calls=%d", task2, calls)
	}
}

func TestSubmitNanoBanana2TextOnlyUsesGenerationModel(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header = %q", got)
		}
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != ModelNanoBanana2New {
			t.Fatalf("model = %q", body.Model)
		}
		if body.Input["prompt"] != "safe prompt" || body.Input["size"] != "16:9" || body.Input["resolution"] != "4K" {
			t.Fatalf("bad image input: %+v", body.Input)
		}
		if body.Input["n"].(float64) != 3 {
			t.Fatalf("output count = %#v, want 3", body.Input["n"])
		}
		if _, ok := body.Input["image_urls"]; ok {
			t.Fatalf("image_urls must be omitted without references: %+v", body.Input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"image_task_1","status":"not_started","created_time":"2026-06-20T10:30:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseImageRequest(ModelNanoBanana2New)
	req.AspectRatio = "16:9"
	req.Resolution = "4K"
	req.OutputCount = 3

	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderPoYo || task.ExternalID != "image_task_1" || task.Status != domain.ProviderTaskPending {
		t.Fatalf("bad task: %+v", task)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestSubmitNanoBanana2ReferencesUseEditModelAndImageURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != ModelNanoBanana2NewEdit {
			t.Fatalf("model = %q, want %q", body.Model, ModelNanoBanana2NewEdit)
		}
		if body.Input["prompt"] != "safe prompt" || body.Input["size"] != "16:9" || body.Input["resolution"] != "4K" {
			t.Fatalf("bad image input: %+v", body.Input)
		}
		if body.Input["n"].(float64) != 2 {
			t.Fatalf("output count = %#v, want 2", body.Input["n"])
		}
		refs, ok := body.Input["image_urls"].([]any)
		if !ok || len(refs) != 2 || refs[0] != "https://cdn.test/ref-a.png" || refs[1] != "https://cdn.test/ref-b.png" {
			t.Fatalf("image_urls = %#v", body.Input["image_urls"])
		}
		if _, ok := body.Input["reference_image_urls"]; ok {
			t.Fatalf("reference_image_urls must not be used for Nano Banana 2: %+v", body.Input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"image_edit_task_1","status":"not_started","created_time":"2026-06-20T10:30:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseImageRequest(ModelNanoBanana2New)
	req.AspectRatio = "16:9"
	req.Resolution = "4K"
	req.InputURLs = []string{" https://cdn.test/ref-a.png ", "https://cdn.test/ref-b.png"}
	req.OutputCount = 2

	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderPoYo || task.ModelCode != ModelNanoBanana2New || task.ExternalID != "image_edit_task_1" || task.Status != domain.ProviderTaskPending {
		t.Fatalf("bad task: %+v", task)
	}
}

func TestSubmitNanoBananaProTextOnlyUsesGenerationModel(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != ModelNanoBananaPro {
			t.Fatalf("model = %q, want %q", body.Model, ModelNanoBananaPro)
		}
		if body.Input["prompt"] != "safe prompt" || body.Input["size"] != "auto" || body.Input["resolution"] != "1K" {
			t.Fatalf("bad pro image input: %+v", body.Input)
		}
		if body.Input["n"].(float64) != 3 || body.Input["output_format"] != "png" || body.Input["enable_web_search"] != false {
			t.Fatalf("bad pro options: %+v", body.Input)
		}
		if _, ok := body.Input["image_urls"]; ok {
			t.Fatalf("image_urls must be omitted without references: %+v", body.Input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"pro_image_task_1","status":"not_started","created_time":"2026-06-20T10:30:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseImageRequest(ModelNanoBananaPro)
	req.AspectRatio = ""
	req.Size = "auto"
	req.OutputCount = 3

	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderPoYo || task.ModelCode != ModelNanoBananaPro || task.ExternalID != "pro_image_task_1" || task.Status != domain.ProviderTaskPending {
		t.Fatalf("bad task: %+v", task)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestSubmitNanoBananaProReferencesUseEditModelAndImageURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != ModelNanoBananaProEdit {
			t.Fatalf("model = %q, want %q", body.Model, ModelNanoBananaProEdit)
		}
		refs, ok := body.Input["image_urls"].([]any)
		if !ok || len(refs) != 2 || refs[0] != "https://cdn.test/ref-a.png" || refs[1] != "https://cdn.test/ref-b.png" {
			t.Fatalf("image_urls = %#v", body.Input["image_urls"])
		}
		if _, ok := body.Input["reference_image_urls"]; ok {
			t.Fatalf("reference_image_urls must not be used for Nano Banana Pro: %+v", body.Input)
		}
		if body.Input["size"] != "auto" || body.Input["resolution"] != "4K" || body.Input["output_format"] != "png" {
			t.Fatalf("bad pro edit options: %+v", body.Input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"pro_edit_task_1","status":"not_started","created_time":"2026-06-20T10:30:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseImageRequest(ModelNanoBananaPro)
	req.AspectRatio = ""
	req.Size = "auto"
	req.Resolution = "4K"
	req.InputURLs = []string{" https://cdn.test/ref-a.png ", "https://cdn.test/ref-b.png"}

	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderPoYo || task.ModelCode != ModelNanoBananaPro || task.ExternalID != "pro_edit_task_1" {
		t.Fatalf("bad task: %+v", task)
	}
}

func TestSubmitSeedream45TextOnlyUsesGenerationModel(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != "seedream-4.5" {
			t.Fatalf("model = %q, want seedream-4.5", body.Model)
		}
		if body.Input["prompt"] != "safe prompt" || body.Input["size"] != "4K" || body.Input["n"].(float64) != 1 {
			t.Fatalf("bad Seedream input: %+v", body.Input)
		}
		if _, ok := body.Input["image_urls"]; ok {
			t.Fatalf("image_urls must be omitted without references: %+v", body.Input)
		}
		if _, ok := body.Input["reference_image_urls"]; ok {
			t.Fatalf("reference_image_urls must not be used for Seedream 4.5: %+v", body.Input)
		}
		if _, ok := body.Input["resolution"]; ok {
			t.Fatalf("resolution must not be sent for Seedream 4.5: %+v", body.Input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"seedream_task_1","status":"not_started","created_time":"2026-07-05T10:30:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseImageRequest("seedream-4.5")
	req.Resolution = "4K"

	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderPoYo || task.ModelCode != "seedream-4.5" || task.ExternalID != "seedream_task_1" || task.Status != domain.ProviderTaskPending {
		t.Fatalf("bad task: %+v", task)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestSubmitSeedream45ReferencesUseEditModelAndImageURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != "seedream-4.5-edit" {
			t.Fatalf("model = %q, want seedream-4.5-edit", body.Model)
		}
		if body.Input["prompt"] != "safe prompt" || body.Input["size"] != "2K" || body.Input["n"].(float64) != 1 {
			t.Fatalf("bad Seedream edit input: %+v", body.Input)
		}
		refs, ok := body.Input["image_urls"].([]any)
		if !ok || len(refs) != 2 || refs[0] != "https://cdn.test/ref-a.png" || refs[1] != "https://cdn.test/ref-b.png" {
			t.Fatalf("image_urls = %#v", body.Input["image_urls"])
		}
		if _, ok := body.Input["reference_image_urls"]; ok {
			t.Fatalf("reference_image_urls must not be used for Seedream 4.5: %+v", body.Input)
		}
		if _, ok := body.Input["resolution"]; ok {
			t.Fatalf("resolution must not be sent for Seedream 4.5: %+v", body.Input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"seedream_edit_task_1","status":"not_started","created_time":"2026-07-05T10:30:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseImageRequest("seedream-4.5")
	req.Resolution = "2K"
	req.InputURLs = []string{" https://cdn.test/ref-a.png ", "https://cdn.test/ref-b.png"}

	task, err := provider.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderPoYo || task.ModelCode != "seedream-4.5" || task.ExternalID != "seedream_edit_task_1" {
		t.Fatalf("bad task: %+v", task)
	}
}

func TestSeedream45ReferenceValidationAndEstimate(t *testing.T) {
	provider := New(Config{APIKey: "test-key", BaseURL: "http://127.0.0.1"})
	req := baseImageRequest("seedream-4.5")
	req.Resolution = "2K"
	req.InputURLs = make([]string, 10)
	for i := range req.InputURLs {
		req.InputURLs[i] = "https://cdn.test/ref.png"
	}
	estimate, err := provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate accepted 10 refs: %v", err)
	}
	if estimate.AmountCredits != 10 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("bad 2K estimate: %+v", estimate)
	}

	req.InputURLs = nil
	req.Resolution = "4K"
	estimate, err = provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate 4K: %v", err)
	}
	if estimate.AmountCredits != 15 {
		t.Fatalf("4K estimate = %d, want 15", estimate.AmountCredits)
	}

	req.Resolution = "1K"
	_, err = provider.Estimate(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req.Resolution = "2K"
	req.InputURLs = make([]string, 11)
	for i := range req.InputURLs {
		req.InputURLs[i] = "https://cdn.test/ref.png"
	}
	_, err = provider.Estimate(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)
}

func TestSeedream45AllowedSizes(t *testing.T) {
	wantSize := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Input["size"] != wantSize {
			t.Fatalf("size = %#v, want %s; input=%+v", body.Input["size"], wantSize, body.Input)
		}
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"seedream_task_1","status":"not_started"}}`))
	}))
	defer srv.Close()

	for _, size := range []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "21:9", "2K", "4K"} {
		t.Run(size, func(t *testing.T) {
			wantSize = size
			provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			req := baseImageRequest("seedream-4.5")
			req.AspectRatio = ""
			req.Size = size
			if _, err := provider.Submit(context.Background(), req); err != nil {
				t.Fatalf("submit: %v", err)
			}
		})
	}
}

func TestNanoBananaProReferenceValidation(t *testing.T) {
	provider := New(Config{APIKey: "test-key", BaseURL: "http://127.0.0.1"})
	req := baseImageRequest(ModelNanoBananaPro)
	req.AspectRatio = ""
	req.Size = "auto"
	req.InputURLs = make([]string, 14)
	for i := range req.InputURLs {
		req.InputURLs[i] = "https://cdn.test/ref.png"
	}
	if _, err := provider.Estimate(context.Background(), req); err != nil {
		t.Fatalf("estimate accepted 14 refs: %v", err)
	}

	req.InputURLs = append(req.InputURLs, "https://cdn.test/ref-extra.png")
	_, err := provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)
}

func TestSubmitUploadsDataURLReferencesBeforeGenerate(t *testing.T) {
	const dataURL = "data:image/png;base64,aW1hZ2U="
	cases := []struct {
		name      string
		req       domain.ProviderRequest
		wantModel string
		wantField string
	}{
		{
			name:      "nano banana image",
			req:       baseImageRequest(ModelNanoBanana2New),
			wantModel: ModelNanoBanana2NewEdit,
			wantField: "image_urls",
		},
		{
			name:      "nano banana pro image",
			req:       baseImageRequest(ModelNanoBananaPro),
			wantModel: ModelNanoBananaProEdit,
			wantField: "image_urls",
		},
		{
			name:      "seedream image",
			req:       baseImageRequest("seedream-4.5"),
			wantModel: "seedream-4.5-edit",
			wantField: "image_urls",
		},
		{
			name:      "kling video",
			req:       baseVideoRequest(ModelKlingO3Standard),
			wantModel: ModelKlingO3Standard,
			wantField: "reference_image_urls",
		},
		{
			name:      "seedance video",
			req:       baseVideoRequest(ModelSeedance20Fast),
			wantModel: ModelSeedance20Fast,
			wantField: "reference_image_urls",
		},
		{
			name:      "runway video",
			req:       baseVideoRequest(ModelRunwayGen45),
			wantModel: ModelRunwayGen45,
			wantField: "image_urls",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantURL := "https://storage.poyo.ai/temp/ref.png"
			uploadCalls := 0
			submitCalls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/common/upload/base64":
					uploadCalls++
					var body uploadBase64Request
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatalf("decode upload body: %v", err)
					}
					if body.Base64Data != dataURL {
						t.Fatalf("base64_data = %q", body.Base64Data)
					}
					_, _ = w.Write([]byte(`{"success":true,"code":200,"data":{"file_url":"` + wantURL + `"}}`))
				case "/api/generate/submit":
					submitCalls++
					var body submitRequest
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatalf("decode submit body: %v", err)
					}
					if body.Model != tc.wantModel {
						t.Fatalf("model = %q, want %q", body.Model, tc.wantModel)
					}
					switch tc.wantField {
					case "image_urls":
						refs, ok := body.Input["image_urls"].([]any)
						if !ok || len(refs) != 1 || refs[0] != wantURL {
							t.Fatalf("image_urls = %#v", body.Input["image_urls"])
						}
					case "reference_image_urls":
						refs, ok := body.Input["reference_image_urls"].([]any)
						if !ok || len(refs) != 1 || refs[0] != wantURL {
							t.Fatalf("reference_image_urls = %#v", body.Input["reference_image_urls"])
						}
					default:
						t.Fatalf("unexpected field %q", tc.wantField)
					}
					_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"task_1","status":"not_started"}}`))
				default:
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
			}))
			defer srv.Close()

			provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			req := tc.req
			req.InputURLs = []string{dataURL}
			if _, err := provider.Submit(context.Background(), req); err != nil {
				t.Fatalf("submit: %v", err)
			}
			if uploadCalls != 1 || submitCalls != 1 {
				t.Fatalf("calls upload=%d submit=%d, want 1/1", uploadCalls, submitCalls)
			}
		})
	}
}

func TestSubmitRejectsInvalidReferenceURLBeforeHTTP(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseVideoRequest(ModelSeedance20Fast)
	req.InputURLs = []string{"not-a-url"}

	_, err := provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)
	if called {
		t.Fatal("invalid reference URL must be rejected before HTTP calls")
	}
}

func TestSubmitNanoBanana2ImageAcceptsDataID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"id":"I46OXVK51SS6JRZJ","status":"not_started"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := provider.Submit(context.Background(), baseImageRequest(ModelNanoBanana2New))
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.ExternalID != "I46OXVK51SS6JRZJ" || task.Status != domain.ProviderTaskPending {
		t.Fatalf("bad task: %+v", task)
	}
}

func TestSubmitNanoBanana2ImageDefaultsTo1K(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != ModelNanoBanana2New {
			t.Fatalf("model = %q", body.Model)
		}
		if body.Input["resolution"] != "1K" {
			t.Fatalf("resolution = %#v, want 1K", body.Input["resolution"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"image_task_1","status":"not_started","created_time":"2026-06-20T10:30:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	if _, err := provider.Submit(context.Background(), baseImageRequest(ModelNanoBanana2New)); err != nil {
		t.Fatalf("submit: %v", err)
	}
}

func TestNanoBanana2EstimateAndValidation(t *testing.T) {
	provider := New(Config{APIKey: "test-key", BaseURL: "http://127.0.0.1"})
	req := baseImageRequest(ModelNanoBanana2New)

	estimate, err := provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if estimate.AmountCredits != 5 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("bad estimate: %+v", estimate)
	}

	req.Resolution = "2K"
	estimate, err = provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate 2K: %v", err)
	}
	if estimate.AmountCredits != 8 {
		t.Fatalf("2K estimate = %d, want 8", estimate.AmountCredits)
	}

	req.Resolution = "4K"
	estimate, err = provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate 4K: %v", err)
	}
	if estimate.AmountCredits != 12 {
		t.Fatalf("4K estimate = %d, want 12", estimate.AmountCredits)
	}

	req.Resolution = ""
	req.InputURLs = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15"}
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req.InputURLs = nil
	req.AspectRatio = "7:7"
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)
}

func TestSubmitRejectsKlingAudioByDefault(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := baseVideoRequest(ModelKlingO3Standard)
	req.Params = rawJSON(t, map[string]any{"audio": true})

	_, err := provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)
	if called {
		t.Fatal("audio request must be rejected before HTTP submit")
	}
}

func TestSeedanceEstimateAndReferenceValidation(t *testing.T) {
	provider := New(Config{APIKey: "test-key", BaseURL: "http://127.0.0.1"})
	req := baseVideoRequest(ModelSeedance20Fast)
	req.DurationSec = 10
	req.Resolution = "720p"

	estimate, err := provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if estimate.AmountCredits != 280 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("bad estimate: %+v", estimate)
	}

	req.InputURLs = []string{"1", "2", "3", "4", "5"}
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req.InputURLs = nil
	req.Resolution = "1080p"
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)
}

func TestRunwayGen45DurationAndReferenceValidation(t *testing.T) {
	provider := New(Config{APIKey: "test-key", BaseURL: "http://127.0.0.1"})
	req := baseVideoRequest(ModelRunwayGen45)
	req.DurationSec = 7

	_, err := provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req.DurationSec = 5
	req.InputURLs = []string{"https://cdn.test/a.png", "https://cdn.test/b.png"}
	_, err = provider.Submit(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrInvalidRequest)

	req.InputURLs = nil
	_, err = provider.Estimate(context.Background(), req)
	requireErrorClass(t, err, domain.ProviderErrUnsupportedCapab)
}

func TestSubmitRunwayGen45UsesOptionalImageURLsList(t *testing.T) {
	cases := []struct {
		name      string
		inputURLs []string
		wantRefs  bool
	}{
		{name: "text only"},
		{name: "single reference", inputURLs: []string{"https://cdn.test/ref.png"}, wantRefs: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/generate/submit" {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				var body submitRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if body.Model != ModelRunwayGen45 {
					t.Fatalf("model = %q, want %q", body.Model, ModelRunwayGen45)
				}
				if _, ok := body.Input["image_url"]; ok {
					t.Fatalf("image_url must not be used for Runway Gen-4.5: %+v", body.Input)
				}
				refs, ok := body.Input["image_urls"].([]any)
				if !tc.wantRefs {
					if ok {
						t.Fatalf("image_urls must be omitted without references: %+v", body.Input)
					}
				} else if !ok || len(refs) != 1 || refs[0] != tc.inputURLs[0] {
					t.Fatalf("image_urls = %#v", body.Input["image_urls"])
				}
				_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"runway_task_1","status":"not_started"}}`))
			}))
			defer srv.Close()

			provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			req := baseVideoRequest(ModelRunwayGen45)
			req.InputURLs = tc.inputURLs
			if _, err := provider.Submit(context.Background(), req); err != nil {
				t.Fatalf("submit: %v", err)
			}
		})
	}
}

func TestSubmitRunwayGen45AcceptsDocumentedAspectRatios(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body submitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"runway_task_1","status":"not_started"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	for _, aspectRatio := range []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"} {
		t.Run(aspectRatio, func(t *testing.T) {
			req := baseVideoRequest(ModelRunwayGen45)
			req.AspectRatio = aspectRatio
			if _, err := provider.Submit(context.Background(), req); err != nil {
				t.Fatalf("submit: %v", err)
			}
		})
	}
}

func TestEstimateUsesResolvedRouteSnapshot(t *testing.T) {
	provider := New(Config{APIKey: "test-key", BaseURL: "http://127.0.0.1"})
	req := baseVideoRequest(ModelRunwayGen45)
	req.Params = rawJSON(t, map[string]any{
		"resolved_video_route": domain.VideoRouteSnapshot{
			Alias:                  domain.VideoRouteRunwayGen45,
			Provider:               domain.ProviderPoYo,
			ProviderModelID:        ModelRunwayGen45,
			ModelClass:             "runway_gen4_5",
			DurationSec:            5,
			Resolution:             "720p",
			ProviderCostCredits:    25,
			InternalCostCredits:    50,
			PriceMultiplier:        2,
			MaxProviderCostCredits: 25,
			MaxInternalCostCredits: 50,
		},
	})

	estimate, err := provider.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if estimate.AmountCredits != 25 || estimate.Estimated {
		t.Fatalf("bad estimate: %+v", estimate)
	}
}

func TestPollCompletedReturnsOutputAndSanitizesRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/generate/status/task_1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"task_1","status":"finished","files":[{"file_type":"video","file_url":"https://private.poyo.ai/output.mp4?token=secret","format":"mp4"}],"credits_amount":50,"created_time":"2026-06-19T15:00:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	result, err := provider.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderPoYo, ExternalID: "task_1"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 {
		t.Fatalf("bad result: %+v", result)
	}
	raw := string(result.Raw)
	if strings.Contains(raw, "private.poyo.ai") || strings.Contains(raw, "secret") || strings.Contains(raw, "video_url") {
		t.Fatalf("raw metadata leaked private output URL: %s", raw)
	}
}

func TestPollCompletedImageReturnsOutputAndSanitizesRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/generate/status/image_task_1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"image_task_1","status":"finished","files":[{"file_type":"image","file_url":"https://private.poyo.ai/output.jpg?token=secret","format":"jpg"}],"credits_amount":5,"created_time":"2026-06-20T10:30:00Z"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	result, err := provider.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderPoYo, ExternalID: "image_task_1"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 {
		t.Fatalf("bad result: %+v", result)
	}
	raw := string(result.Raw)
	if strings.Contains(raw, "private.poyo.ai") || strings.Contains(raw, "secret") || strings.Contains(raw, "file_url") {
		t.Fatalf("raw metadata leaked private output URL: %s", raw)
	}
}

func TestPollCompletedImageAcceptsDataResultImageURLs(t *testing.T) {
	const outputURL = "https://private.poyo.ai/output.jpg?token=secret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/generate/status/I46OXVK51SS6JRZJ" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"id":"I46OXVK51SS6JRZJ","status":"finished","progress":0.0,"result":{"image_urls":["` + outputURL + `"]}}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	result, err := provider.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderPoYo, ExternalID: "I46OXVK51SS6JRZJ"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 || result.OutputURLs[0] != outputURL {
		t.Fatalf("unexpected result: %+v", result)
	}
	raw := string(result.Raw)
	if strings.Contains(raw, "private.poyo.ai") || strings.Contains(raw, "secret") || strings.Contains(raw, "image_urls") {
		t.Fatalf("raw metadata leaked private output URL: %s", raw)
	}
}

func TestPollFailureNormalizesModeration(t *testing.T) {
	cases := []struct {
		name string
		body string
		leak string
	}{
		{
			name: "policy error message",
			body: `{"code":200,"data":{"task_id":"task_1","status":"failed","error_message":"policy rejected prompt"}}`,
			leak: "policy rejected prompt",
		},
		{
			name: "platform regulations message",
			body: `{"code":200,"data":{"task_id":"task_1","status":"failed","error_message":"The content does not comply with the platform regulations. Please modify it and try again."}}`,
			leak: "platform regulations",
		},
		{
			name: "copyright policy message",
			body: `{"code":200,"data":{"task_id":"task_1","status":"failed","error_message":"blocked by copyright policy"}}`,
			leak: "copyright policy",
		},
		{
			name: "top level status message",
			body: `{"code":200,"task_id":"task_1","status":"failed","message":"The content does not comply with the platform regulations."}`,
			leak: "platform regulations",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			result, err := provider.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderPoYo, ExternalID: "task_1"})
			if err != nil {
				t.Fatalf("poll: %v", err)
			}
			if result.Status != domain.ProviderTaskFailed || result.ErrorClass != domain.ProviderErrContentRejected {
				t.Fatalf("bad result: %+v", result)
			}
			if strings.Contains(string(result.Raw), tc.leak) {
				t.Fatalf("raw metadata leaked provider error text: %s", string(result.Raw))
			}
		})
	}
}

func TestPoYoClassifiesModelUnavailable(t *testing.T) {
	const providerMessage = "unknown model poyo-model-v9"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":{"type":"validation_error","message":` + strconv.Quote(providerMessage) + `}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := provider.Submit(context.Background(), baseVideoRequest(ModelKlingO3Standard))
	requireErrorClass(t, err, domain.ProviderErrModelUnavailable)
	if err != nil && (strings.Contains(err.Error(), providerMessage) || strings.Contains(err.Error(), "poyo-model-v9")) {
		t.Fatalf("provider message leaked: %v", err)
	}
}

func TestSubmitHTTPErrorIsNormalized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"unauthorized token"}}`))
	}))
	defer srv.Close()

	provider := New(Config{APIKey: "bad-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := provider.Submit(context.Background(), baseVideoRequest(ModelKlingO3Standard))
	requireErrorClass(t, err, domain.ProviderErrAuthFailed)
	if err != nil && strings.Contains(err.Error(), "bad-key") {
		t.Fatalf("error leaked api key: %v", err)
	}
}

func TestPoYoInvalidPromptForModelStaysInvalidRequest(t *testing.T) {
	class := classifyPoYoError(http.StatusUnprocessableEntity, "validation_error", "", "invalid prompt length for model")
	if class != domain.ProviderErrInvalidRequest {
		t.Fatalf("class = %q, want invalid_request", class)
	}
}

func baseVideoRequest(model string) domain.ProviderRequest {
	return domain.ProviderRequest{
		JobID:          uuid.New(),
		UserID:         uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      model,
		Provider:       domain.ProviderPoYo,
		Prompt:         "safe prompt",
		DurationSec:    5,
		Resolution:     "720p",
		AspectRatio:    "16:9",
		IdempotencyKey: "idem-" + uuid.NewString(),
	}
}

func baseImageRequest(model string) domain.ProviderRequest {
	return domain.ProviderRequest{
		JobID:          uuid.New(),
		UserID:         uuid.New(),
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		ModelCode:      model,
		Provider:       domain.ProviderPoYo,
		Prompt:         "safe prompt",
		AspectRatio:    "1:1",
		IdempotencyKey: "idem-" + uuid.NewString(),
	}
}

func rawJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}

func requireErrorClass(t *testing.T, err error, class domain.ProviderErrorClass) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error, got nil", class)
	}
	var classified interface {
		ProviderErrorClass() domain.ProviderErrorClass
	}
	if !errors.As(err, &classified) {
		t.Fatalf("error has no provider class: %v", err)
	}
	if got := classified.ProviderErrorClass(); got != class {
		t.Fatalf("error class = %s, want %s; err=%v", got, class, err)
	}
}
