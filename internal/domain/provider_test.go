package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProviderRequestTrustedFactsAreBackendOnlyAndTextOnly(t *testing.T) {
	req := ProviderRequest{
		Prompt:       "untrusted user text",
		TrustedFacts: "Факты НейроХаб: canonical backend context",
	}

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal provider request: %v", err)
	}
	if strings.Contains(string(raw), "canonical backend context") || strings.Contains(string(raw), "trusted_facts") {
		t.Fatalf("trusted facts must not be serialized in provider snapshots: %s", raw)
	}

	var decoded ProviderRequest
	if err := json.Unmarshal([]byte(`{"prompt":"user","trusted_facts":"forged"}`), &decoded); err != nil {
		t.Fatalf("unmarshal provider request: %v", err)
	}
	if decoded.TrustedFacts != "" {
		t.Fatalf("serialized input populated backend-only facts: %q", decoded.TrustedFacts)
	}

	if image := req.ImageRequest(); image.Prompt != req.Prompt {
		t.Fatalf("image prompt = %q, want untrusted prompt", image.Prompt)
	}
	if video := req.VideoRequest(); video.Prompt != req.Prompt {
		t.Fatalf("video prompt = %q, want untrusted prompt", video.Prompt)
	}
}

func TestDurableProviderTaskSnapshotsExcludePayloadMaterial(t *testing.T) {
	request := DurableProviderTaskRequestJSON()
	if string(request) != `{}` {
		t.Fatalf("durable request = %q, want empty object", request)
	}

	result := DurableProviderTaskResultJSON(ProviderTaskResult{
		Status:       ProviderTaskSucceeded,
		OutputURLs:   []string{"https://provider.example/temporary"},
		Text:         "private generated output",
		ErrorMessage: "private provider error",
		Raw:          json.RawMessage(`{"provider_payload":"private"}`),
	})
	serialized := string(result)
	for _, forbidden := range []string{"temporary", "private", "output_urls", "text", "raw", "error_message"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("durable result contains forbidden material %q: %s", forbidden, serialized)
		}
	}
	if !strings.Contains(serialized, `"status":"succeeded"`) {
		t.Fatalf("durable result lost terminal status: %s", serialized)
	}
}
