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

func TestPollRefundNoticePreservesFailureCause(t *testing.T) {
	cases := []struct {
		name, code, message string
		want                domain.ProviderErrorClass
	}{
		{"motion media format", "task_failed", "InvalidParameter.MediaFormat task failed with status: FAIL, message: The input was rejected, No complete upper body detected in the video; ensure the upper body is clearly visible.", domain.ProviderErrInvalidRequest},
		{"moderation", "task_failed", "Content moderation rejected the input.", domain.ProviderErrContentRejected},
		{"timeout", "task_failed", "Upstream timed out.", domain.ProviderErrTimeout},
		{"unavailable", "task_failed", "Provider unavailable.", domain.ProviderErrOverloaded},
		{"unknown", "task_failed", "Task failed.", domain.ProviderErrInternal},
		{"insufficient pose", "task_failed", "Invalid input: insufficient body visibility.", domain.ProviderErrInvalidRequest},
		{"actual balance", "task_failed", "Insufficient account balance.", domain.ProviderErrInsufficientBalance},
		{"quota code", "insufficient_quota", "Credit allowance exhausted.", domain.ProviderErrInsufficientBalance},
		{"provider balance code", "insufficient_provider_balance", "Not enough funds.", domain.ProviderErrInsufficientBalance},
		{"quota exhausted", "task_failed", "Quota exceeded.", domain.ProviderErrInsufficientBalance},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": map[string]any{"id": "task_refund", "status": "failed", "error": map[string]any{"code": tc.code, "type": "task_failed", "message": tc.message + " Any deducted quota has been refunded automatically."}}})
			}))
			defer srv.Close()
			p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			result, err := p.Poll(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderAPIMart, ExternalID: "task_refund"})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != domain.ProviderTaskFailed || result.ErrorClass != tc.want {
				t.Fatalf("status=%s class=%s want=%s", result.Status, result.ErrorClass, tc.want)
			}
			if len(result.OutputURLs) != 0 {
				t.Fatal("failed task returned outputs")
			}
			if strings.Contains(result.ErrorMessage, tc.message) || strings.Contains(string(result.Raw), "refunded") {
				t.Fatal("provider message leaked")
			}
		})
	}
}
