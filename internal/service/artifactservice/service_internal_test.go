package artifactservice

import (
	"errors"
	"strings"
	"testing"
)

func TestReadRemoteBodyRejectsLimitPlusOne(t *testing.T) {
	data, err := readRemoteBody(strings.NewReader("abcd"), 3)
	if !errors.Is(err, errRemoteArtifactTooLarge) {
		t.Fatalf("expected too large error, got data=%q err=%v", string(data), err)
	}
}

func TestReadRemoteBodyAllowsExactLimit(t *testing.T) {
	data, err := readRemoteBody(strings.NewReader("abc"), 3)
	if err != nil {
		t.Fatalf("read exact limit: %v", err)
	}
	if string(data) != "abc" {
		t.Fatalf("data = %q, want abc", string(data))
	}
}
