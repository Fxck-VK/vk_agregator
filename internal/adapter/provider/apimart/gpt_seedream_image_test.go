package apimart

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestGPTImage25ContractAndReplay(t *testing.T) {
	req := gptImage25Request(ModelGPTImage25Flare)
	want := jsonMap(t, `{
		"model":"gpt-image-2.5-flare",
		"prompt":"Synthetic editorial image",
		"size":"9:21",
		"resolution":"1k",
		"quality":"xhigh",
		"n":4,
		"output_format":"png",
		"moderation":"auto",
		"image_urls":["https://cdn.test/ref-a.png"]
	}`)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" || r.Header.Get("Idempotency-Key") != req.IdempotencyKey {
			t.Error("unexpected GPT-Image-2.5 submit route or idempotency header")
		}
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("contract mismatch: %#v", got)
		}
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"gpt25-task"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := p.Submit(context.Background(), req)
			if err != nil || task.ExternalID != "gpt25-task" || task.ModelCode != ModelGPTImage25Flare {
				t.Error("GPT-Image-2.5 submission failed")
			}
			if strings.Contains(string(task.Request), "Synthetic editorial image") || strings.Contains(string(task.Request), "cdn.test") {
				t.Error("provider task request persisted prompt or reference URL")
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("duplicate GPT-Image-2.5 paid submit count = %d", calls.Load())
	}
}

func TestGPTImage25UploadsDataURIReferencesAndDefaultsMedium(t *testing.T) {
	const uploaded = "https://upload.apimart.ai/f/image/reference.png"
	req := gptImage25Request(ModelGPTImage25Sunburst)
	req.Resolution = "2K"
	req.AspectRatio = "1:1"
	req.OutputCount = 1
	req.InputURLs = []string{pngDataURL(t, 32, 32)}
	req.Params = nil
	var uploadSeen, submitSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/uploads/images":
			uploadSeen = true
			if r.Method != http.MethodPost || !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
				t.Error("unexpected upload request")
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			if header.Filename != "reference-image.png" || http.DetectContentType(data) != "image/png" {
				t.Fatalf("unexpected upload file %q %q", header.Filename, http.DetectContentType(data))
			}
			_, _ = w.Write([]byte(`{"url":"` + uploaded + `"}`))
		case "/v1/images/generations":
			submitSeen = true
			got := decodeJSONMap(t, r.Body)
			want := jsonMap(t, `{
				"model":"gpt-image-2.5-sunburst",
				"prompt":"Synthetic editorial image",
				"size":"1:1",
				"resolution":"2k",
				"quality":"medium",
				"n":1,
				"output_format":"png",
				"moderation":"auto",
				"image_urls":["https://upload.apimart.ai/f/image/reference.png"]
			}`)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("contract mismatch: %#v", got)
			}
			if strings.Contains(asJSON(t, got), "data:image") {
				t.Error("data URI leaked into GPT-Image-2.5 generation request")
			}
			_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"gpt25-edit"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), req)
	if err != nil || task.ExternalID != "gpt25-edit" {
		t.Fatalf("submit: task=%+v err=%v", task, err)
	}
	if !uploadSeen || !submitSeen {
		t.Fatalf("uploadSeen=%v submitSeen=%v", uploadSeen, submitSeen)
	}
}

func TestGPTImage25RejectsUnsafeOptionsBeforeSubmit(t *testing.T) {
	cases := map[string]func(*domain.ProviderRequest){
		"bad quality":          func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"image_quality":"1K-ultra"}`) },
		"quality without tier": func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"image_quality":"medium"}`) },
		"tier conflict":        func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"image_quality":"2K-medium"}`) },
		"raw quality":          func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"quality":"high"}`) },
		"raw output format":    func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"output_format":"jpeg"}`) },
		"background":           func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"background":"transparent"}`) },
		"moderation":           func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"moderation":"low"}`) },
		"resolved image route": func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"image_quality":"1K-xhigh","resolved_image_route":{"provider":"apimart","provider_model_id":"gpt-image-2.5-flare","provider_cost_credits":2.2}}`)
		},
		"provider cost credits": func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"image_quality":"1K-xhigh","provider_cost_credits":2.2}`)
		},
		"unknown params": func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"model_id":"gpt_image_25","surprise":true}`)
		},
		"auto size":              func(r *domain.ProviderRequest) { r.Size = "auto"; r.AspectRatio = "" },
		"pixel size":             func(r *domain.ProviderRequest) { r.Size = "1600x1200"; r.AspectRatio = "" },
		"unsupported resolution": func(r *domain.ProviderRequest) { r.Resolution = "8K"; r.Params = nil },
		"too many outputs":       func(r *domain.ProviderRequest) { r.OutputCount = 5 },
		"too many refs":          func(r *domain.ProviderRequest) { r.InputURLs = repeatedRefs(17) },
		"http reference":         func(r *domain.ProviderRequest) { r.InputURLs = []string{"http://cdn.test/ref.png"} },
		"unsupported data uri type": func(r *domain.ProviderRequest) {
			r.InputURLs = []string{"data:image/gif;base64," + base64.StdEncoding.EncodeToString([]byte("gif"))}
		},
		"malformed params": func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{`) },
	}
	assertInvalidNoSubmit(t, cases, func() domain.ProviderRequest { return gptImage25Request(ModelGPTImage25Flare) })
}

func TestSeedream50LiteContractSequentialAndReplay(t *testing.T) {
	req := seedream50LiteRequest()
	req.OutputCount = 3
	req.InputURLs = []string{"https://cdn.test/a.png", "https://cdn.test/b.png"}
	req.Params = json.RawMessage(`{"model_id":"seedream_5_0_lite","model_name":"Seedream 5 Lite","provider":"apimart","model_code":"seedream-5-0-lite","resolution":"4K","image_quality":"4K","aspect_ratio":"21:9","output_count":3}`)
	want := jsonMap(t, `{
		"model":"seedream-5-0-lite",
		"prompt":"Synthetic product set",
		"size":"21:9",
		"resolution":"4K",
		"n":3,
		"image_urls":["https://cdn.test/a.png","https://cdn.test/b.png"],
		"output_format":"png",
		"watermark":false,
		"sequential_image_generation":"auto",
		"sequential_image_generation_options":{"max_images":3}
	}`)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		got := decodeJSONMap(t, r.Body)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("contract mismatch: %#v", got)
		}
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"seedream-lite-task"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
	for range 2 {
		task, err := p.Submit(context.Background(), req)
		if err != nil || task.ExternalID != "seedream-lite-task" || task.ModelCode != ModelSeedream50Lite {
			t.Fatalf("submit: task=%+v err=%v", task, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("duplicate Seedream Lite paid submit count = %d", calls.Load())
	}
}

func TestSeedream50LiteSingleOutputDisablesSequentialMode(t *testing.T) {
	req := seedream50LiteRequest()
	req.Size = "1:1"
	req.Resolution = "2K"
	req.OutputCount = 1
	req.Params = json.RawMessage(`{"model_id":"seedream_5_0_lite","model_name":"Seedream 5 Lite","provider":"apimart","model_code":"seedream-5-0-lite","resolution":"2K","image_quality":"2K","aspect_ratio":"1:1","output_count":1}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := decodeJSONMap(t, r.Body)
		want := jsonMap(t, `{
			"model":"seedream-5-0-lite",
			"prompt":"Synthetic product set",
			"size":"1:1",
			"resolution":"2K",
			"n":1,
			"output_format":"png",
			"watermark":false,
			"sequential_image_generation":"disabled"
		}`)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("contract mismatch: %#v", got)
		}
		_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"seedream-lite-one"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
	if _, err := p.Submit(context.Background(), req); err != nil {
		t.Fatal(err)
	}
}

func TestSeedream50LiteRejectsUnsafeOptionsBeforeSubmit(t *testing.T) {
	cases := map[string]func(*domain.ProviderRequest){
		"1K":                 func(r *domain.ProviderRequest) { r.Resolution = "1K" },
		"unsupported aspect": func(r *domain.ProviderRequest) { r.Size = "9:21" },
		"two to one aspect": func(r *domain.ProviderRequest) {
			r.Size = "2:1"
			r.Params = json.RawMessage(`{"model_id":"seedream_5_0_lite","provider":"apimart","model_code":"seedream-5-0-lite","resolution":"4K","image_quality":"4K","aspect_ratio":"2:1","output_count":1}`)
		},
		"one to two aspect": func(r *domain.ProviderRequest) {
			r.Size = "1:2"
			r.Params = json.RawMessage(`{"model_id":"seedream_5_0_lite","provider":"apimart","model_code":"seedream-5-0-lite","resolution":"4K","image_quality":"4K","aspect_ratio":"1:2","output_count":1}`)
		},
		"auto":             func(r *domain.ProviderRequest) { r.Size = "auto" },
		"pixel size":       func(r *domain.ProviderRequest) { r.Size = "2048x2048" },
		"too many outputs": func(r *domain.ProviderRequest) { r.OutputCount = 16 },
		"refs plus outputs": func(r *domain.ProviderRequest) {
			r.OutputCount = 14
			r.InputURLs = []string{"https://cdn.test/ref.png", "https://cdn.test/ref2.png"}
		},
		"too many refs": func(r *domain.ProviderRequest) { r.InputURLs = repeatedRefs(16) },
		"tiny data uri": func(r *domain.ProviderRequest) { r.InputURLs = []string{pngDataURL(t, 10, 20)} },
		"wide data uri": func(r *domain.ProviderRequest) { r.InputURLs = []string{pngDataURL(t, 90, 20)} },
		"webp data uri": func(r *domain.ProviderRequest) {
			r.InputURLs = []string{"data:image/webp;base64," + base64.StdEncoding.EncodeToString([]byte("webp"))}
		},
		"native sequential": func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"sequential_image_generation":"auto"}`) },
		"native watermark":  func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"watermark":true}`) },
		"native nsfw":       func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"nsfw_check":true}`) },
		"malformed params":  func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{`) },
	}
	assertInvalidNoSubmit(t, cases, seedream50LiteRequest)
}

func TestSeedream50ProContractReplayAndFlatPoll(t *testing.T) {
	req := seedream50ProRequest()
	want := jsonMap(t, `{
		"model":"seedream-5-0-pro",
		"prompt":"Synthetic premium product",
		"resolution":"1.5K",
		"size":"16:9",
		"n":1,
		"image_urls":["https://cdn.test/person.jpg","https://cdn.test/product.png"],
		"background":"opaque",
		"output_format":"png",
		"watermark":false
	}`)
	var submits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/images/generations":
			submits.Add(1)
			got := decodeJSONMap(t, r.Body)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("contract mismatch: %#v", got)
			}
			_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"seedream-pro-task"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/tasks/seedream-pro-task":
			_, _ = w.Write([]byte(`{
				"id":"seedream-pro-task",
				"status":"success",
				"progress":100,
				"cost":0.045,
				"result":{"images":[{"url":["https://cdn.example.com/images/pro.png"],"sizes":["2048x1152"],"output_formats":["png"]}]}
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
	for range 2 {
		task, err := p.Submit(context.Background(), req)
		if err != nil || task.ExternalID != "seedream-pro-task" || task.ModelCode != ModelSeedream50Pro {
			t.Fatalf("submit: task=%+v err=%v", task, err)
		}
	}
	if submits.Load() != 1 {
		t.Fatalf("duplicate Seedream Pro paid submit count = %d", submits.Load())
	}
	result, err := p.Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "seedream-pro-task"})
	if err != nil || result.Status != domain.ProviderTaskSucceeded || len(result.OutputURLs) != 1 {
		t.Fatalf("poll: result=%+v err=%v", result, err)
	}
	if strings.Contains(string(result.Raw), "cdn.example.com") {
		t.Fatalf("raw metadata leaked provider URL: %s", string(result.Raw))
	}
}

func TestSeedream50ProRejectsUnsafeOptionsBeforeSubmit(t *testing.T) {
	cases := map[string]func(*domain.ProviderRequest){
		"3K":                 func(r *domain.ProviderRequest) { r.Resolution = "3K" },
		"4K":                 func(r *domain.ProviderRequest) { r.Resolution = "4K" },
		"unsupported aspect": func(r *domain.ProviderRequest) { r.Size = "9:21" },
		"auto":               func(r *domain.ProviderRequest) { r.Size = "auto" },
		"pixel size":         func(r *domain.ProviderRequest) { r.Size = "1600x1600" },
		"batch":              func(r *domain.ProviderRequest) { r.OutputCount = 2 },
		"too many refs":      func(r *domain.ProviderRequest) { r.InputURLs = repeatedRefs(11) },
		"transparent":        func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"background":"transparent"}`) },
		"layers":             func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"layer_decomposition":true}`) },
		"tools":              func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"tools":[{"type":"x"}]}`) },
		"stream":             func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{"stream":true}`) },
		"optimize prompt": func(r *domain.ProviderRequest) {
			r.Params = json.RawMessage(`{"optimize_prompt_options":{"mode":"standard"}}`)
		},
		"malformed params": func(r *domain.ProviderRequest) { r.Params = json.RawMessage(`{`) },
	}
	assertInvalidNoSubmit(t, cases, seedream50ProRequest)
}

func TestGPTImage25EstimateRejectsInputRefsUntilPricingBoundsKnown(t *testing.T) {
	req := gptImage25Request(ModelGPTImage25Flare)
	req.OutputCount = 1
	req.Params = json.RawMessage(`{"image_quality":"1K-medium"}`)
	_, err := New(Config{}).Estimate(context.Background(), req)
	var providerErr *Error
	if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrInvalidRequest {
		t.Fatalf("estimate err=%v, want fail-closed invalid request while GPT ref costs are unknown", err)
	}
}

func TestNewImageModelsEstimateUsesLocalPricingFallback(t *testing.T) {
	p := New(Config{})

	gpt := gptImage25Request(ModelGPTImage25Flare)
	gpt.AspectRatio = "1:1"
	gpt.Resolution = "4K"
	gpt.OutputCount = 4
	gpt.InputURLs = nil
	gpt.Params = json.RawMessage(`{"model_id":"gpt_image_25","provider":"apimart","model_code":"gpt-image-2.5-flare","resolution":"4K","image_quality":"4K-max","aspect_ratio":"1:1","output_count":4}`)
	estimate, err := p.Estimate(context.Background(), gpt)
	if err != nil {
		t.Fatal(err)
	}
	if estimate.AmountCredits != 23 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("GPT-Image-2.5 local estimate = %+v, want conservative token ceiling", estimate)
	}

	lite := seedream50LiteRequest()
	lite.OutputCount = 15
	lite.Params = json.RawMessage(`{"model_id":"seedream_5_0_lite","provider":"apimart","model_code":"seedream-5-0-lite","resolution":"4K","image_quality":"4K","aspect_ratio":"21:9","output_count":15}`)
	estimate, err = p.Estimate(context.Background(), lite)
	if err != nil {
		t.Fatal(err)
	}
	if estimate.AmountCredits != 5 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("Seedream Lite local estimate = %+v, want 0.28*n ceiling", estimate)
	}

	pro := seedream50ProRequest()
	pro.Resolution = "2K"
	pro.InputURLs = repeatedRefs(10)
	pro.Params = json.RawMessage(`{"model_id":"seedream_5_0_pro","provider":"apimart","model_code":"seedream-5-0-pro","resolution":"2K","image_quality":"2K","aspect_ratio":"16:9","output_count":1}`)
	estimate, err = p.Estimate(context.Background(), pro)
	if err != nil {
		t.Fatal(err)
	}
	if estimate.AmountCredits != 1 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("Seedream Pro local estimate = %+v, want base plus reference surcharge ceiling", estimate)
	}
}

func gptImage25Request(model string) domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		ModelCode:      model,
		Prompt:         "Synthetic editorial image",
		AspectRatio:    "9:21",
		Resolution:     "1K",
		OutputCount:    4,
		InputURLs:      []string{"https://cdn.test/ref-a.png"},
		Params:         json.RawMessage(`{"model_id":"gpt_image_25","model_name":"GPT Image 2.5","provider":"apimart","model_code":"gpt-image-2.5-flare","resolution":"1K","image_quality":"1K-xhigh","aspect_ratio":"9:21","output_count":4}`),
		IdempotencyKey: "provider_submit:gpt25:test",
	}
}

func seedream50LiteRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		ModelCode:      ModelSeedream50Lite,
		Prompt:         "Synthetic product set",
		Size:           "21:9",
		Resolution:     "4K",
		OutputCount:    1,
		Params:         json.RawMessage(`{"model_id":"seedream_5_0_lite","model_name":"Seedream 5 Lite","provider":"apimart","model_code":"seedream-5-0-lite","resolution":"4K","image_quality":"4K","aspect_ratio":"21:9","output_count":1}`),
		IdempotencyKey: "provider_submit:seedream50-lite:test",
	}
}

func seedream50ProRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		ModelCode:      ModelSeedream50Pro,
		Prompt:         "Synthetic premium product",
		Size:           "16:9",
		Resolution:     "1.5K",
		OutputCount:    1,
		InputURLs:      []string{"https://cdn.test/person.jpg", "https://cdn.test/product.png"},
		Params:         json.RawMessage(`{"model_id":"seedream_5_0_pro","model_name":"Seedream 5 Pro","provider":"apimart","model_code":"seedream-5-0-pro","resolution":"1.5K","image_quality":"1.5K","aspect_ratio":"16:9","output_count":1}`),
		IdempotencyKey: "provider_submit:seedream50-pro:test",
	}
}

func assertInvalidNoSubmit(t *testing.T, cases map[string]func(*domain.ProviderRequest), base func() domain.ProviderRequest) {
	t.Helper()
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()
			req := base()
			change(&req)
			p := New(Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			_, err := p.Submit(context.Background(), req)
			var providerErr *Error
			if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrInvalidRequest || calls.Load() != 0 {
				t.Fatalf("local rejection: err=%v calls=%d", err, calls.Load())
			}
		})
	}
}

func repeatedRefs(n int) []string {
	values := make([]string, n)
	for i := range values {
		values[i] = "https://cdn.test/ref.png"
	}
	return values
}

func pngDataURL(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 64, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func jsonMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func decodeJSONMap(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func asJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
