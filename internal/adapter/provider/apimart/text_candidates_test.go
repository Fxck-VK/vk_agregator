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

func TestAPIMartDocumentedTextModels(t *testing.T) {
	models := []string{"gpt-5", "gpt-5.1", "gpt-5-chat-latest", "gpt-5-mini", "claude-opus-4-6", "claude-sonnet-4-6", "claude-opus-4-5-20251101", "gemini-3.5-flash", "gemini-3.1-pro-preview", "gemini-3-pro-preview", "gemini-3-pro-preview-thinking", "gemini-3-flash-preview", "gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite", "deepseek-v4-pro", "deepseek-v4-flash", "deepseek-v3.2", "deepseek-v3.2-exp", "deepseek-r1-250528", "deepseek-v3-0324"}
	models = append(models, "gpt-6-sol", "gpt-6-luna", "gpt-5.4", "gpt-5.3-codex", "gpt-5.2", "claude-opus-5-5", "claude-haiku-4-5-20251001", "kimi-k3", "qwen3.7-flash", "grok-4.5", "grok-4.6", "grok-4.7")
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body struct {
					Model     string                           `json:"model"`
					Stream    *bool                            `json:"stream"`
					MaxTokens int                              `json:"max_tokens"`
					Messages  []struct{ Role, Content string } `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" || body.Model != model || body.Stream == nil || *body.Stream || body.MaxTokens != 2048 || len(body.Messages) != 2 || body.Messages[1].Content != "Synthetic" {
					t.Error("incorrect chat contract")
				}
				_, _ = w.Write([]byte(`{"code":200,"data":{"choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"hidden"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":8}}}`))
			}))
			defer srv.Close()
			cfg := Config{APIKey: "fixture-key", BaseURL: srv.URL + "/v1"}
			req := domain.ProviderRequest{Provider: domain.ProviderAPIMart, ModelCode: model, Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, Prompt: "Synthetic"}
			if _, err := New(cfg).Submit(context.Background(), req); err == nil || calls != 0 {
				t.Fatal("disabled model sent")
			}
			cfg.EnabledTextModels = []string{model}
			p := New(cfg)
			task, err := p.Submit(context.Background(), req)
			if err != nil || calls != 1 || task.ImmediateResult == nil || task.ImmediateResult.Text != "answer" {
				t.Fatalf("chat failed calls=%d err=%v", calls, err)
			}
			if estimate, err := p.Estimate(context.Background(), req); err != nil || estimate.AmountCredits <= 0 {
				t.Fatal("missing paid estimate")
			}
			req.InputURLs = []string{"https://example.com/fixture.png"}
			if _, err := p.Submit(context.Background(), req); err == nil || calls != 1 {
				t.Fatal("unwired attachment sent")
			}
		})
	}
}

func TestQwen38ResponsesBoundsReasoningAndRejectsUnwiredInputs(t *testing.T) {
	valid := `{"id":"resp-fixture","object":"response","status":"completed","output":[{"type":"reasoning","summary":[]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer"}]}],"usage":{"input_tokens":8192,"output_tokens":2048,"output_tokens_details":{"reasoning_tokens":2000}}}`
	for _, tc := range []struct {
		name, response string
		wantOK         bool
	}{
		{"bounded answer", valid, true},
		{"reasoning exceeds budget", strings.Replace(valid, `"output_tokens":2048`, `"output_tokens":2049`, 1), false},
		{"input exceeds budget", strings.Replace(valid, `"input_tokens":8192`, `"input_tokens":8193`, 1), false},
		{"incomplete answer", strings.Replace(valid, `"completed"`, `"incomplete"`, 1), false},
		{"reasoning only", `{"status":"completed","output":[{"type":"reasoning","summary":[]}],"usage":{"input_tokens":20,"output_tokens":2048}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if r.Method != "POST" || r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer fixture-key" || string(body["model"]) != `"qwen3.8-max"` || string(body["stream"]) != "false" || string(body["max_output_tokens"]) != "2048" || len(body) != 4 {
					t.Error("incorrect bounded Responses request or extra unpriced options")
				}
				var input []struct {
					Role    string
					Content []struct{ Type, Text string }
				}
				if err := json.Unmarshal(body["input"], &input); err != nil || len(input) != 2 || input[0].Role != "system" || input[1].Role != "user" || len(input[1].Content) != 1 || input[1].Content[0].Type != "input_text" || input[1].Content[0].Text != "Synthetic" {
					t.Error("invalid text input")
				}
				_, _ = w.Write([]byte(tc.response))
			}))
			defer srv.Close()
			cfg := Config{APIKey: "fixture-key", BaseURL: srv.URL + "/v1"}
			req := domain.ProviderRequest{Provider: domain.ProviderAPIMart, ModelCode: "qwen3.8-max", Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, Prompt: "Synthetic"}
			if _, err := New(cfg).Submit(context.Background(), req); err == nil || calls != 0 {
				t.Fatal("disabled model sent")
			}
			cfg.EnabledTextModels = []string{"qwen3.8-max"}
			p := New(cfg)
			task, err := p.Submit(context.Background(), req)
			if (err == nil) != tc.wantOK || calls != 1 {
				t.Fatalf("unexpected result: calls=%d err=%v", calls, err)
			}
			if tc.wantOK && (task.ImmediateResult == nil || task.ImmediateResult.Text != "answer") {
				t.Fatal("reasoning leaked or answer missing")
			}
			for _, prompt := range []string{"", strings.Repeat("x", 7681)} {
				req.Prompt = prompt
				if _, err := p.Submit(context.Background(), req); err == nil || calls != 1 {
					t.Fatal("invalid input sent")
				}
			}
			req.Prompt = "Synthetic"
			req.InputURLs = []string{"https://example.com/fixture.pdf"}
			if _, err := p.Submit(context.Background(), req); err == nil || calls != 1 {
				t.Fatal("unwired PDF sent")
			}
		})
	}
}
