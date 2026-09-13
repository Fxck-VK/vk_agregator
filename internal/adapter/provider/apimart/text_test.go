package apimart

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestFable51TextContractAndDisabledGate(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Model     string                           `json:"model"`
			Stream    bool                             `json:"stream"`
			MaxTokens int                              `json:"max_tokens"`
			Messages  []struct{ Role, Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer fixture-key" || body.Model != "claude-fable-5.1" || body.Stream || body.MaxTokens != 2048 || len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[1].Content != "Synthetic" {
			t.Error("wrong Fable contract")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"hidden"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":8}}`))
	}))
	defer srv.Close()
	cfg := Config{APIKey: "fixture-key", BaseURL: srv.URL + "/v1"}
	req := domain.ProviderRequest{Provider: domain.ProviderAPIMart, ModelCode: "claude-fable-5.1", Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, Prompt: "Synthetic"}
	if _, err := New(cfg).Submit(context.Background(), req); err == nil || calls != 0 {
		t.Fatal("disabled text sent")
	}
	cfg.EnabledTextModels = []string{"claude-fable-5.1"}
	p := New(cfg)
	task, err := p.Submit(context.Background(), req)
	if err != nil || calls != 1 || task.Provider != domain.ProviderAPIMart || task.ImmediateResult == nil || task.ImmediateResult.Text != "answer" {
		t.Fatalf("Fable output: calls=%d err=%v", calls, err)
	}
	req.Provider = ""
	if _, err := p.Estimate(context.Background(), req); err == nil {
		t.Fatal("paid Fable used for default chat")
	}
	if _, err := p.Poll(context.Background(), domain.ProviderTaskRef{ExternalID: "text:fixture"}); err == nil || calls != 1 {
		t.Fatal("synchronous text polled as a media task")
	}
}
