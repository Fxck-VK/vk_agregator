package apimart

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestSubmitHailuoStandardSuccess(t *testing.T) {
	var seen videoGenerationRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/videos/generations" {
			t.Fatalf("path = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "provider_submit:job:1" {
			t.Fatalf("idempotency header = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_123"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelHailuo23Standard,
		Prompt:         "make a safe clip",
		DurationSec:    6,
		Resolution:     "768p",
		Params:         json.RawMessage(`{"duration_sec":6,"resolution":"768p"}`),
		IdempotencyKey: "provider_submit:job:1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderAPIMart || task.ModelCode != ModelHailuo23Standard || task.ExternalID != "task_123" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if task.Status != domain.ProviderTaskPending {
		t.Fatalf("status = %q, want pending", task.Status)
	}
	if seen.Model != ModelHailuo23Standard || seen.Prompt != "make a safe clip" || seen.Duration != 6 || seen.Resolution != "768p" {
		t.Fatalf("unexpected request body: %+v", seen)
	}
	if !seen.PromptOptimizer || seen.FastPretreatment || seen.Watermark || seen.FirstFrameImage != "" {
		t.Fatalf("unexpected request toggles/frame: %+v", seen)
	}
	if strings.Contains(string(task.Request), "make a safe clip") {
		t.Fatalf("provider task request must not persist prompt: %s", task.Request)
	}
}

func TestSubmitGemini3ProImageSuccess(t *testing.T) {
	var seen imageGenerationRequest
	var rawBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/images/generations" {
			t.Fatalf("path = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "provider_submit:image:1" {
			t.Fatalf("idempotency header = %q", got)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request: %v", err)
		}
		if err := json.Unmarshal(data, &seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if err := json.Unmarshal(data, &rawBody); err != nil {
			t.Fatalf("decode raw request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_image"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		ModelCode:      ModelGemini3ProImage,
		Prompt:         "safe product image",
		Size:           "16:9",
		Resolution:     "4K",
		InputURLs:      []string{" https://cdn.test/reference.png "},
		Params:         json.RawMessage(`{"model_id":"nano_banana_pro","model_name":"Nano Banana Pro"}`),
		IdempotencyKey: "provider_submit:image:1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderAPIMart || task.ModelCode != ModelGemini3ProImage || task.ExternalID != "task_image" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if seen.Model != ModelGemini3ProImage || seen.Prompt != "safe product image" || seen.Size != "16:9" || seen.Resolution != "4K" {
		t.Fatalf("unexpected request body: %+v", seen)
	}
	if seen.N != 1 || seen.OfficialFallback {
		t.Fatalf("unexpected generation options: %+v", seen)
	}
	if _, ok := rawBody["official_fallback"]; ok {
		t.Fatalf("official_fallback must be omitted unless enabled: %#v", rawBody)
	}
	if _, ok := rawBody["quality"]; ok {
		t.Fatalf("APIMart image requests must use resolution, not quality: %#v", rawBody)
	}
	if len(seen.ImageURLs) != 1 || seen.ImageURLs[0] != "https://cdn.test/reference.png" {
		t.Fatalf("image_urls = %#v", seen.ImageURLs)
	}
	if strings.Contains(string(task.Request), "safe product image") {
		t.Fatalf("provider task request must not persist prompt: %s", task.Request)
	}
}

func TestSubmitGPTImage2Success(t *testing.T) {
	var seen imageGenerationRequest
	var rawBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/images/generations" {
			t.Fatalf("path = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header = %q", got)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request: %v", err)
		}
		if err := json.Unmarshal(data, &seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if err := json.Unmarshal(data, &rawBody); err != nil {
			t.Fatalf("decode raw request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_gpt_image_2"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		ModelCode:      ModelGPTImage2,
		Prompt:         "safe editorial image",
		Size:           "9:21",
		Resolution:     "2K",
		InputURLs:      []string{"https://cdn.test/reference-a.png", "https://cdn.test/reference-b.png"},
		IdempotencyKey: "provider_submit:gpt_image_2:1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Provider != domain.ProviderAPIMart || task.ModelCode != ModelGPTImage2 || task.ExternalID != "task_gpt_image_2" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if seen.Model != ModelGPTImage2 || seen.Prompt != "safe editorial image" || seen.Size != "9:21" || seen.Resolution != "2k" {
		t.Fatalf("unexpected request body: %+v", seen)
	}
	if seen.N != 1 || seen.OfficialFallback || len(seen.ImageURLs) != 2 {
		t.Fatalf("unexpected generation options: %+v", seen)
	}
	if _, ok := rawBody["official_fallback"]; ok {
		t.Fatalf("official_fallback must be omitted unless enabled: %#v", rawBody)
	}
	if _, ok := rawBody["quality"]; ok {
		t.Fatalf("APIMart image requests must use resolution, not quality: %#v", rawBody)
	}
}

func TestSubmitHailuoFastRequiresFirstFrame(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:       uuid.New(),
		Operation:   domain.OperationVideoGenerate,
		Modality:    domain.ModalityVideo,
		ModelCode:   ModelHailuo23Fast,
		Prompt:      "clip",
		DurationSec: 6,
		Resolution:  "768p",
	})
	if perr, ok := err.(*Error); !ok || perr.ProviderErrorClass() != domain.ProviderErrInvalidRequest {
		t.Fatalf("expected invalid_request, got %T %v", err, err)
	}
}

func TestSubmitHailuoFastUploadsDataURLFirstFrame(t *testing.T) {
	const uploadedFrame = "https://upload.apimart.ai/f/image/test-first-frame.png"
	firstFrame := testPNGDataURL(t)
	var seen videoGenerationRequest
	var uploadSeen bool
	var submitSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uploads/images":
			uploadSeen = true
			if got := r.Method; got != http.MethodPost {
				t.Fatalf("upload method = %q", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("upload auth header = %q", got)
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
				t.Fatalf("upload content type = %q", r.Header.Get("Content-Type"))
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatalf("upload form file: %v", err)
			}
			defer func() {
				_ = file.Close()
			}()
			data, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("read upload file: %v", err)
			}
			if header.Filename != "first-frame.png" {
				t.Fatalf("upload filename = %q", header.Filename)
			}
			if got := http.DetectContentType(data); got != "image/png" {
				t.Fatalf("upload file content type = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"` + uploadedFrame + `","filename":"test-first-frame.png","content_type":"image/png","bytes":70,"created_at":1743436800}`))
		case "/videos/generations":
			submitSeen = true
			if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_fast"}]}`))
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelHailuo23Fast,
		Prompt:         "clip",
		DurationSec:    10,
		Resolution:     "768p",
		InputURLs:      []string{firstFrame},
		IdempotencyKey: "provider_submit:fast:1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.ExternalID != "task_fast" {
		t.Fatalf("external id = %q", task.ExternalID)
	}
	if !uploadSeen || !submitSeen {
		t.Fatalf("uploadSeen=%v submitSeen=%v", uploadSeen, submitSeen)
	}
	if seen.Model != ModelHailuo23Fast || seen.FirstFrameImage != uploadedFrame {
		t.Fatalf("unexpected request: %+v", seen)
	}
	if strings.HasPrefix(seen.FirstFrameImage, "data:") {
		t.Fatalf("submit leaked data url into generation request")
	}
	if seen.FastPretreatment {
		t.Fatalf("fast_pretreatment must stay independent from Hailuo Fast model")
	}
}

func TestSubmitHailuoFastUsesPublicFirstFrameURLDirectly(t *testing.T) {
	const firstFrame = "https://cdn.test/ref.png"
	var seen videoGenerationRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/videos/generations" {
			t.Fatalf("path = %q, want direct generation submit", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_fast_url"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:       uuid.New(),
		Operation:   domain.OperationVideoGenerate,
		Modality:    domain.ModalityVideo,
		ModelCode:   ModelHailuo23Fast,
		Prompt:      "clip",
		DurationSec: 6,
		Resolution:  "768p",
		InputURLs:   []string{firstFrame},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if seen.FirstFrameImage != firstFrame {
		t.Fatalf("first frame = %q", seen.FirstFrameImage)
	}
}

func TestSubmitHailuoFastRejectsInvalidDataURLFirstFrame(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:       uuid.New(),
		Operation:   domain.OperationVideoGenerate,
		Modality:    domain.ModalityVideo,
		ModelCode:   ModelHailuo23Fast,
		Prompt:      "clip",
		DurationSec: 6,
		Resolution:  "768p",
		InputURLs:   []string{"data:image/png;base64,bm90LWEtcG5n"},
	})
	if perr, ok := err.(*Error); !ok || perr.ProviderErrorClass() != domain.ProviderErrInvalidRequest {
		t.Fatalf("expected invalid_request, got %T %v", err, err)
	}
}

func TestEstimateReportsAPIMartProviderCost(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	req := domain.ProviderRequest{
		JobID:       uuid.New(),
		Operation:   domain.OperationVideoGenerate,
		Modality:    domain.ModalityVideo,
		ModelCode:   ModelHailuo23Standard,
		Prompt:      "clip",
		DurationSec: 6,
		Resolution:  "768p",
	}

	estimate, err := p.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if estimate.AmountCredits != 1 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("unexpected estimate: %+v", estimate)
	}

	params, err := json.Marshal(map[string]any{
		"resolved_video_route": domain.VideoRouteSnapshot{
			Alias:               domain.VideoRouteHailuo23Standard,
			Provider:            domain.ProviderAPIMart,
			ProviderModelID:     ModelHailuo23Standard,
			ModelClass:          "hailuo_2_3_standard",
			DurationSec:         6,
			Resolution:          "768p",
			ProviderCostCredits: 1,
			InternalCostCredits: 2,
			PriceMultiplier:     2,
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	req.Params = params
	estimate, err = p.Estimate(context.Background(), req)
	if err != nil {
		t.Fatalf("snapshot estimate: %v", err)
	}
	if estimate.AmountCredits != 1 || estimate.Estimated {
		t.Fatalf("unexpected snapshot estimate: %+v", estimate)
	}
}

func TestSubmitGemini3ProImageValidation(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	base := domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: domain.OperationImageGenerate,
		Modality:  domain.ModalityImage,
		ModelCode: ModelGemini3ProImage,
		Prompt:    "safe image",
		Size:      "1K",
	}
	estimate, err := p.Estimate(context.Background(), base)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if estimate.AmountCredits != 1 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("unexpected estimate: %+v", estimate)
	}
	twoK := base
	twoK.Size = "2K"
	estimate, err = p.Estimate(context.Background(), twoK)
	if err != nil {
		t.Fatalf("estimate 2K: %v", err)
	}
	if estimate.AmountCredits != 1 || estimate.Currency != "credits" || estimate.Estimated {
		t.Fatalf("unexpected 2K estimate: %+v", estimate)
	}

	cases := []struct {
		name string
		req  domain.ProviderRequest
		want domain.ProviderErrorClass
	}{
		{
			name: "unsupported model",
			req: func() domain.ProviderRequest {
				req := base
				req.ModelCode = ModelHailuo23Standard
				return req
			}(),
			want: domain.ProviderErrUnsupportedCapab,
		},
		{
			name: "bad aspect size",
			req: func() domain.ProviderRequest {
				req := base
				req.Size = "1024x1024"
				return req
			}(),
			want: domain.ProviderErrInvalidRequest,
		},
		{
			name: "too many references",
			req: func() domain.ProviderRequest {
				req := base
				req.InputURLs = make([]string, maxGeminiReferenceImages+1)
				for i := range req.InputURLs {
					req.InputURLs[i] = "https://cdn.test/ref.png"
				}
				return req
			}(),
			want: domain.ProviderErrInvalidRequest,
		},
		{
			name: "unsupported data url",
			req: func() domain.ProviderRequest {
				req := base
				req.InputURLs = []string{"data:text/plain;base64,aGVsbG8="}
				return req
			}(),
			want: domain.ProviderErrInvalidRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Estimate(context.Background(), tc.req)
			if perr, ok := err.(*Error); !ok || perr.ProviderErrorClass() != tc.want {
				t.Fatalf("expected %s, got %T %v", tc.want, err, err)
			}
		})
	}
}

func TestSubmitRejectsUnsupportedVideoShape(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	_, err := p.Submit(context.Background(), domain.ProviderRequest{
		JobID:       uuid.New(),
		Operation:   domain.OperationVideoGenerate,
		Modality:    domain.ModalityVideo,
		ModelCode:   ModelHailuo23Standard,
		Prompt:      "clip",
		DurationSec: 10,
		Resolution:  "1080p",
	})
	if perr, ok := err.(*Error); !ok || perr.ProviderErrorClass() != domain.ProviderErrInvalidRequest {
		t.Fatalf("expected invalid_request, got %T %v", err, err)
	}
}

func TestSubmitIsAdapterIdempotent(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_once"}]}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	req := domain.ProviderRequest{
		JobID:          uuid.New(),
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelHailuo23Standard,
		Prompt:         "clip",
		DurationSec:    6,
		Resolution:     "768p",
		IdempotencyKey: "provider_submit:same:1",
	}
	first, err := p.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := p.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if posts != 1 {
		t.Fatalf("posts = %d, want 1", posts)
	}
	if first.ExternalID != second.ExternalID {
		t.Fatalf("idempotent external id mismatch: %q vs %q", first.ExternalID, second.ExternalID)
	}
}

func TestSubmitGPTImage2ValidationUsesModelSpecificLimits(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	base := domain.ProviderRequest{
		JobID:      uuid.New(),
		Operation:  domain.OperationImageGenerate,
		Modality:   domain.ModalityImage,
		ModelCode:  ModelGPTImage2,
		Prompt:     "safe image",
		Size:       "9:21",
		Resolution: "4k",
	}
	if _, err := p.Estimate(context.Background(), base); err != nil {
		t.Fatalf("estimate accepted gpt-image-2 9:21/4k: %v", err)
	}
	base.Size = "1881x836"
	if _, err := p.Estimate(context.Background(), base); err != nil {
		t.Fatalf("estimate accepted gpt-image-2 pixel size: %v", err)
	}
	base.Size = "9:21"
	base.InputURLs = make([]string, maxGPTImage2ReferenceImages)
	for i := range base.InputURLs {
		base.InputURLs[i] = "https://cdn.test/ref.png"
	}
	if _, err := p.Estimate(context.Background(), base); err != nil {
		t.Fatalf("estimate accepted 16 refs: %v", err)
	}
	base.InputURLs = append(base.InputURLs, "https://cdn.test/ref-extra.png")
	if _, err := p.Estimate(context.Background(), base); err == nil {
		t.Fatalf("expected too many refs error")
	}
}

func TestPollCompletedImageReturnsProviderURLWithSanitizedRaw(t *testing.T) {
	const outputURL = "https://upload.apimart.ai/f/image/private-output.png?token=secret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/tasks/task_image" {
			t.Fatalf("path = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code":200,
			"data":{
				"id":"task_image",
				"status":"completed",
				"cost":0.1,
				"credits_cost":1,
				"progress":100,
				"result":{"images":[{"url":["` + outputURL + `"],"expires_at":1763174708}]},
				"created":1763088289,
				"completed":1763088308
			}
		}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderAPIMart, ExternalID: "task_image"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded || len(res.OutputURLs) != 1 || res.OutputURLs[0] != outputURL {
		t.Fatalf("unexpected result: %+v", res)
	}
	if strings.Contains(string(res.Raw), "upload.apimart.ai") || strings.Contains(string(res.Raw), "secret") {
		t.Fatalf("raw metadata leaked provider URL: %s", string(res.Raw))
	}
}

func TestPollCompletedReturnsProviderURLWithSanitizedRaw(t *testing.T) {
	const outputURL = "https://upload.apimart.ai/f/video/private-output.mp4?token=secret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/tasks/task_123" {
			t.Fatalf("path = %q", got)
		}
		if got := r.URL.Query().Get("language"); got != "en" {
			t.Fatalf("language = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code":200,
			"data":{
				"id":"task_123",
				"status":"completed",
				"cost":0.1,
				"credits_cost":1,
				"progress":100,
				"result":{"videos":[{"url":["` + outputURL + `"],"expires_at":1763174708}]},
				"created":1763088289,
				"completed":1763088308
			}
		}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderAPIMart, ExternalID: "task_123"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded || len(res.OutputURLs) != 1 || res.OutputURLs[0] != outputURL {
		t.Fatalf("unexpected result: %+v", res)
	}
	if strings.Contains(string(res.Raw), "upload.apimart.ai") || strings.Contains(string(res.Raw), "secret") {
		t.Fatalf("raw metadata leaked provider URL: %s", string(res.Raw))
	}
}

func TestPollProcessingAndFailedStatus(t *testing.T) {
	responses := []string{
		`{"code":200,"data":{"id":"task_123","status":"processing","progress":45}}`,
		`{"code":200,"data":{"id":"task_123","status":"failed","error":{"type":"content_policy","message":"moderation rejected content"}}}`,
	}
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responses[calls]))
		calls++
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderAPIMart, ExternalID: "task_123"})
	if err != nil {
		t.Fatalf("poll processing: %v", err)
	}
	if res.Status != domain.ProviderTaskProcessing {
		t.Fatalf("status = %q, want processing", res.Status)
	}
	res, err = p.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderAPIMart, ExternalID: "task_123"})
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.Status != domain.ProviderTaskFailed || res.ErrorClass != domain.ProviderErrContentRejected {
		t.Fatalf("unexpected failed result: %+v", res)
	}
}

func TestPollImagePolicyFilteredWithoutImagesIsContentRejected(t *testing.T) {
	const message = "No images found in AI response. Unable to show the generated image. The image was filtered out because it violated Google's Generative AI Prohibited Use Policy."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/tasks/task_filtered" {
			t.Fatalf("path = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code":200,
			"data":{
				"id":"task_filtered",
				"status":"completed",
				"progress":100,
				"error_message":` + strconv.Quote(message) + `,
				"result":{"images":[]}
			}
		}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderAPIMart, ExternalID: "task_filtered"})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskFailed || res.ErrorClass != domain.ProviderErrContentRejected {
		t.Fatalf("unexpected result: %+v", res)
	}
	if strings.Contains(string(res.Raw), "Google") || strings.Contains(string(res.Raw), "No images found") {
		t.Fatalf("raw metadata leaked provider error text: %s", string(res.Raw))
	}
}

func TestPollEnvelopeErrorUsesTopLevelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":402,"error":{"message":"insufficient balance"}}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderAPIMart, ExternalID: "task_123"})
	if perr, ok := err.(*Error); !ok || perr.ProviderErrorClass() != domain.ProviderErrInsufficientBalance {
		t.Fatalf("expected insufficient balance, got %T %v", err, err)
	}
}

func TestHTTPErrorClasses(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   domain.ProviderErrorClass
	}{
		{name: "auth", status: http.StatusUnauthorized, body: `{"error":{"message":"invalid token"}}`, want: domain.ProviderErrAuthFailed},
		{name: "balance", status: http.StatusPaymentRequired, body: `{"error":{"message":"insufficient balance"}}`, want: domain.ProviderErrInsufficientBalance},
		{name: "rate", status: http.StatusTooManyRequests, body: `{"error":{"message":"rate limit"}}`, want: domain.ProviderErrRateLimited},
		{name: "validation", status: http.StatusBadRequest, body: `{"error":{"type":"validation_error","message":"bad request"}}`, want: domain.ProviderErrInvalidRequest},
		{name: "unavailable", status: http.StatusBadGateway, body: `{"error":{"message":"provider unavailable"}}`, want: domain.ProviderErrOverloaded},
		{name: "timeout", status: http.StatusGatewayTimeout, body: `{"error":{"message":"upstream timeout"}}`, want: domain.ProviderErrTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			_, err := p.Submit(context.Background(), domain.ProviderRequest{
				JobID:       uuid.New(),
				Operation:   domain.OperationVideoGenerate,
				Modality:    domain.ModalityVideo,
				ModelCode:   ModelHailuo23Standard,
				Prompt:      "clip",
				DurationSec: 6,
				Resolution:  "768p",
			})
			if perr, ok := err.(*Error); !ok || perr.ProviderErrorClass() != tc.want {
				t.Fatalf("class = %T %v, want %s", err, err, tc.want)
			}
		})
	}
}

func TestPollEmptyTaskIDReturnsTaskNotFound(t *testing.T) {
	p := New(Config{APIKey: "test-key"})
	res, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderAPIMart})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskFailed || res.ErrorClass != domain.ProviderErrTaskNotFound {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func testPNGDataURL(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
