package textapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestPaidPOSTNeverFollowsRedirectOrRetries(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusInternalServerError, http.StatusTooManyRequests} {
		calls, redirects := 0, 0
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirects++ }))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Location", target.URL)
			w.WriteHeader(status)
		}))
		p := New(Config{Provider: domain.ProviderAPIMart, APIKey: "synthetic", BaseURL: srv.URL, EnabledModels: []string{"claude-fable-5.1"}})
		req := domain.ProviderRequest{Provider: domain.ProviderAPIMart, ModelCode: "claude-fable-5.1", Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, Prompt: "Synthetic"}
		_, err := p.Submit(context.Background(), req)
		srv.Close()
		target.Close()
		if err == nil || calls != 1 || redirects != 0 {
			t.Fatalf("status=%d calls=%d redirects=%d", status, calls, redirects)
		}
	}
}

func TestAPIMartChatEnvelopeAndUsageValidation(t *testing.T) {
	valid := `{"choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"hidden"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8192,"completion_tokens":2048}}`
	for _, data := range []string{valid, `{"code":200,"data":` + valid + `}`} {
		text, err := normalize([]byte(data), openAIChat, 8192, 2048)
		if err != nil || text != "answer" {
			t.Fatalf("valid response rejected: %v", err)
		}
	}
	for name, data := range map[string]string{
		"failed code":     `{"code":500,"data":` + valid + `}`,
		"missing code":    `{"data":` + valid + `}`,
		"missing data":    `{"code":200}`,
		"mixed envelope":  `{"code":200,"choices":[],"data":` + valid + `}`,
		"outer error":     `{"code":200,"error":{"message":"private fixture"},"data":` + valid + `}`,
		"missing usage":   `{"code":200,"data":{"choices":[{"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}]}}`,
		"output overflow": `{"code":200,"data":` + strings.Replace(valid, "2048", "2049", 1) + `}`,
		"input overflow":  strings.Replace(valid, "8192", "8193", 1),
		"no user answer":  strings.Replace(valid, `"content":"answer"`, `"content":""`, 1),
		"tools":           strings.Replace(valid, `"stop"`, `"tool_calls"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if text, err := normalize([]byte(data), openAIChat, 8192, 2048); err == nil || text != "" {
				t.Fatal("unsafe response accepted")
			}
		})
	}
}

func TestTextRejectsCrossProviderRouteBeforeHTTP(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	p := New(Config{Provider: domain.ProviderAPIMart, APIKey: "synthetic", BaseURL: srv.URL, EnabledModels: []string{"claude-fable-5.1", "gpt-6-astra"}})
	for _, req := range []domain.ProviderRequest{
		{Provider: domain.ProviderKIE, ModelCode: "claude-fable-5.1"},
		{Provider: domain.ProviderAPIMart, ModelCode: "gpt-6-astra"},
	} {
		req.Operation, req.Modality, req.Prompt = domain.OperationTextGenerate, domain.ModalityText, "Synthetic"
		if _, err := p.Submit(context.Background(), req); err == nil || calls != 0 {
			t.Fatal("cross-provider route sent")
		}
	}
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestQwenChatKeepsThinkingDefault(t *testing.T) {
	transport := testRoundTripper(func(request *http.Request) (*http.Response, error) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, exists := body["enable_thinking"]; exists {
			t.Fatal("ordinary Qwen chat overrides the provider's thinking default")
		}
		if string(body["max_tokens"]) != "2048" {
			t.Fatalf("Qwen output cap = %s", body["max_tokens"])
		}
		// The same paid limit still applies when the provider reports reasoning
		// in its total completion usage.
		result := `{"choices":[{"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":80,"completion_tokens":2800,"completion_tokens_details":{"reasoning_tokens":2200}}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(result))}, nil
	})
	const model = "qwen3.7-flash"
	p := New(Config{Provider: domain.ProviderAPIMart, APIKey: "synthetic", BaseURL: "https://provider.example/v1",
		EnabledModels: []string{model}, HTTPClient: &http.Client{Transport: transport}})
	req := domain.ProviderRequest{Provider: domain.ProviderAPIMart, ModelCode: model, Operation: domain.OperationTextGenerate,
		Modality: domain.ModalityText, Prompt: "Synthetic"}
	_, err := p.Submit(context.Background(), req)
	var providerErr *Error
	if !errors.As(err, &providerErr) || providerErr.Class != domain.ProviderErrSubmitIndeterminate {
		t.Fatalf("over-cap Qwen result returned %v, want submit indeterminate", err)
	}
}
