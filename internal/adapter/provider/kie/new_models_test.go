package kie

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestNewModelNativeContracts(t *testing.T) {
	for _, tc := range []struct{ model, path, cap, kind string }{
		{"claude-opus-4-8", "/claude/v1/messages", "max_tokens", "messages"},
		{"gpt-5-6-terra", "/codex/v1/responses", "max_output_tokens", "responses"},
		{"gpt-6-astra", "/codex/v1/responses", "max_output_tokens", "responses"},
		{"claude-opus-5", "/claude/v1/messages", "max_tokens", "messages"},
		{"claude-fable-5", "/claude/v1/messages", "max_tokens", "messages"},
		{"gemini-3-7-flash-openai", "/gemini-3-7-flash-openai/v1/chat/completions", "max_tokens", "chat"},
		{"gemini-3-6-flash-openai", "/gemini-3-6-flash-openai/v1/chat/completions", "max_tokens", "chat"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if r.URL.Path != tc.path || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fixture-key" || body[tc.cap] != float64(123) || body["stream"] != false {
					t.Error("wrong native request")
				}
				if tc.kind != "chat" && body["model"] != tc.model {
					t.Error("wrong model ID")
				}
				if tc.kind == "chat" && body["include_thoughts"] != false {
					t.Error("reasoning output enabled")
				}
				switch tc.kind {
				case "responses":
					_, _ = w.Write([]byte(`{"status":"completed","output":[{"type":"reasoning","content":[{"type":"output_text","text":"hidden"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer"}]}],"usage":{"input_tokens":20,"output_tokens":12}}`))
				case "messages":
					_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"thinking","text":"hidden"},{"type":"text","text":"answer"}],"usage":{"input_tokens":20,"output_tokens":12}}`))
				case "chat":
					_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"hidden"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":12,"completion_tokens_details":{"reasoning_tokens":8}}}`))
				}
			}))
			defer srv.Close()
			p := New(Config{APIKey: "fixture-key", BaseURL: srv.URL, EnabledModels: []string{tc.model}})
			req := domain.ProviderRequest{Provider: domain.ProviderKIE, ModelCode: tc.model, Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, Prompt: "Synthetic", MaxOutputTokens: 123}
			result, err := p.Submit(context.Background(), req)
			if err != nil || calls != 1 || result.ImmediateResult == nil || result.ImmediateResult.Text != "answer" {
				t.Fatalf("native output: calls=%d err=%v", calls, err)
			}
			req.Provider = ""
			if _, err := p.Estimate(context.Background(), req); err == nil {
				t.Fatal("paid model used for default chat")
			}
		})
	}
}
