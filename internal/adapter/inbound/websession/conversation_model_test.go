package websession

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestSafeConversationExposesOnlyPublicOriginModelMetadata(t *testing.T) {
	owner := uuid.New()
	encoded, err := json.Marshal(newSafeConversation(domain.Conversation{
		ID: uuid.New(), AccountID: owner, ExternalThreadID: "private-thread",
		FirstModelID: "claude_sonnet_4_6",
	}))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 5 || string(fields["first_model_id"]) != `"claude_sonnet_4_6"` {
		t.Fatalf("unexpected safe metadata fields: %v", fields)
	}
	for _, key := range []string{"account_id", "external_thread_id", "params", "messages"} {
		if fields[key] != nil {
			t.Fatalf("private field exposed: %s", key)
		}
	}
}
