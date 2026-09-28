package apimart

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestAPIMartDocumentedTextModels(t *testing.T) {
	models := []string{"gpt-5", "gpt-5.1", "gpt-5-chat-latest", "gpt-5-mini", "claude-opus-4-6", "claude-sonnet-4-6", "claude-opus-4-5-20251101", "gemini-3.5-flash", "gemini-3.1-pro-preview", "gemini-3-pro-preview", "gemini-3-pro-preview-thinking", "gemini-3-flash-preview", "gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite", "deepseek-v4-pro", "deepseek-v4-flash", "deepseek-v3.2", "deepseek-v3.2-exp", "deepseek-r1-250528", "deepseek-v3-0324"}
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
