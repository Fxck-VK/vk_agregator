package redisqueue

import "testing"

func TestConversationTitleStreamIsCataloguedAndClassified(t *testing.T) {
	const titleStream = "stream:conversations:title"
	if !containsStream(AllStreams, titleStream) {
		t.Fatalf("AllStreams does not contain %q: %v", titleStream, AllStreams)
	}
	if !containsStream(AllStreamsWithDLQ, titleStream) {
		t.Fatalf("AllStreamsWithDLQ does not contain %q: %v", titleStream, AllStreamsWithDLQ)
	}
	if got := queueClassForStream(titleStream); got != "conversation_title" {
		t.Fatalf("queueClassForStream(%q) = %q, want conversation_title", titleStream, got)
	}
}

func containsStream(streams []string, want string) bool {
	for _, stream := range streams {
		if stream == want {
			return true
		}
	}
	return false
}
