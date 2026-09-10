package kie

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestNativeTextContracts(t *testing.T) {
	for _, tc := range []struct{ model, path, cap, response string }{
		{providermodels.ModelGPT55, "/codex/v1/responses", "max_output_tokens", `{"status":"completed","output":[{"type":"reasoning","content":[{"type":"output_text","text":"private"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer"}]}],"usage":{"input_tokens":9,"output_tokens":5}}`},
		{providermodels.ModelClaudeOpus47, "/claude/v1/messages", "max_tokens", `{"stop_reason":"end_turn","content":[{"type":"thinking","text":"private"},{"type":"text","text":"answer"}],"usage":{"input_tokens":9,"output_tokens":5}}`},
		{providermodels.ModelGemini31Pro, "/gemini-3.1-pro/v1/chat/completions", "max_tokens", `{"choices":[{"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":5}}`},
	} {
		t.Run(tc.model, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != tc.path || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("invalid route/auth")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body[tc.cap] != float64(2048) || body["stream"] != false {
					t.Errorf("missing bounded nonstream contract: %v", body)
				}
				if _, ok := body["tools"]; ok {
					t.Error("tools unexpectedly sent")
				}
				if tc.model == providermodels.ModelGemini31Pro {
					if body["include_thoughts"] != false {
						t.Error("Gemini reasoning output must be explicitly disabled")
					}
					messages, ok := body["messages"].([]any)
					if !ok || len(messages) != 2 {
						t.Error("Gemini requires separate system and user messages")
					} else {
						for i, role := range []string{"system", "user"} {
							message, _ := messages[i].(map[string]any)
							content, ok := message["content"].([]any)
							if message["role"] != role || !ok || len(content) != 1 {
								t.Errorf("Gemini %s message must use a content array", role)
								continue
							}
							block, _ := content[0].(map[string]any)
							text, _ := block["text"].(string)
							if block["type"] != "text" || (role == "user" && text != "test") || (role == "system" && !strings.HasSuffix(text, "\n\ntrusted")) {
								t.Errorf("Gemini %s text block lost its content", role)
							}
						}
					}
				}
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			p := New(Config{APIKey: "test-key", BaseURL: server.URL, EnabledModels: []string{tc.model}})
			task, err := p.Submit(context.Background(), domain.ProviderRequest{JobID: uuid.New(), Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, ModelCode: tc.model, Prompt: "test", TrustedFacts: "trusted"})
			if err != nil || task.ImmediateResult == nil || task.ImmediateResult.Text != "answer" || calls != 1 {
				t.Fatalf("bad output: %v", err)
			}
			raw, _ := json.Marshal(task)
			if strings.Contains(string(raw), "answer") {
				t.Fatal("text leaked into task snapshot")
			}
		})
	}
}

func TestTextFailsClosedBeforeHTTPAndOnMalformedOutput(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"answer"}}]}`))
	}))
	defer server.Close()
	p := New(Config{APIKey: "key", BaseURL: server.URL, EnabledModels: []string{providermodels.ModelGemini31Pro}})
	req := domain.ProviderRequest{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, ModelCode: providermodels.ModelGemini31Pro, Prompt: strings.Repeat("x", 9000)}
	if _, err := p.Submit(context.Background(), req); err == nil || calls != 0 {
		t.Fatal("oversize request sent")
	}
	req.Prompt = "test"
	if _, err := p.Submit(context.Background(), req); err == nil || calls != 1 {
		t.Fatal("missing usage accepted")
	}
	req.ModelCode = providermodels.ModelGPT55
	if _, err := p.Submit(context.Background(), req); err == nil || calls != 1 {
		t.Fatal("disabled model submitted")
	}
}
