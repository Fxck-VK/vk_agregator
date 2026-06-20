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
