package releasemanifest

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestVerifyAttestationAcceptsCosignObjectAndArray(t *testing.T) {
	predicate := []byte(`{"buildType":"https://mobyproject.org/buildkit@v1","metadata":{"complete":true}}`)
	imageRef := "ghcr.io/" + testRepository + "/api@" + digestFor("api")
	envelope := attestationEnvelope(t, predicate, digestFor("api"))
	array, err := json.Marshal([]json.RawMessage{envelope})
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		raw  []byte
	}{
		{name: "object", raw: envelope},
		{name: "array", raw: array},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := VerifyAttestation(tt.raw, predicate, imageRef); err != nil {
				t.Fatalf("VerifyAttestation() error = %v", err)
			}
		})
	}
}

func TestVerifyAttestationRejectsTamperedPredicate(t *testing.T) {
	expected := []byte(`{"component":"expected"}`)
	envelope := attestationEnvelope(t, []byte(`{"component":"tampered"}`), digestFor("api"))
	imageRef := "ghcr.io/" + testRepository + "/api@" + digestFor("api")

	err := VerifyAttestation(envelope, expected, imageRef)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "predicate") {
		t.Fatalf("VerifyAttestation() error = %v, want predicate mismatch", err)
	}
}

func TestVerifyAttestationRejectsWrongDigest(t *testing.T) {
	predicate := []byte(`{"component":"expected"}`)
	envelope := attestationEnvelope(t, predicate, digestFor("worker"))
	imageRef := "ghcr.io/" + testRepository + "/api@" + digestFor("api")

	err := VerifyAttestation(envelope, predicate, imageRef)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "digest") {
		t.Fatalf("VerifyAttestation() error = %v, want digest mismatch", err)
	}
}

func TestVerifyAttestationRejectsMalformedOrDuplicateJSON(t *testing.T) {
	predicate := []byte(`{"component":"expected"}`)
	imageRef := "ghcr.io/" + testRepository + "/api@" + digestFor("api")
	validEnvelope := attestationEnvelope(t, predicate, digestFor("api"))
	malformedEnvelope := []byte(`{"payloadType":"application/vnd.in-toto+json","payload":"not-base64","signatures":[{"sig":"verified"}]}`)
	validThenMalformed, err := json.Marshal([]json.RawMessage{validEnvelope, malformedEnvelope})
	if err != nil {
		t.Fatal(err)
	}

	duplicatePayload := `{"_type":"https://in-toto.io/Statement/v1","subject":[{"name":"api","digest":{"sha256":"` + strings.TrimPrefix(digestFor("api"), "sha256:") + `"}}],"predicateType":"https://example.test/predicate","predicate":{"component":"expected","component":"tampered"}}`
	duplicateEnvelope := []byte(`{"payloadType":"application/vnd.in-toto+json","payload":"` + base64.StdEncoding.EncodeToString([]byte(duplicatePayload)) + `","signatures":[{"sig":"verified"}]}`)

	tests := []struct {
		name      string
		output    []byte
		expected  []byte
		wantError string
	}{
		{name: "malformed verification output", output: []byte(`{"payload":`), expected: predicate, wantError: "json"},
		{name: "duplicate envelope key", output: []byte(`{"payloadType":"application/vnd.in-toto+json","payload":"first","payload":"second","signatures":[{"sig":"verified"}]}`), expected: predicate, wantError: "duplicate"},
		{name: "duplicate payload key", output: duplicateEnvelope, expected: predicate, wantError: "duplicate"},
		{name: "duplicate expected predicate key", output: validEnvelope, expected: []byte(`{"component":"expected","component":"tampered"}`), wantError: "duplicate"},
		{name: "malformed envelope after valid match", output: validThenMalformed, expected: predicate, wantError: "base64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyAttestation(tt.output, tt.expected, imageRef)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.wantError) {
				t.Fatalf("VerifyAttestation() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func attestationEnvelope(t *testing.T, predicate []byte, digest string) []byte {
	t.Helper()
	var predicateValue any
	if err := json.Unmarshal(predicate, &predicateValue); err != nil {
		t.Fatal(err)
	}
	statement := map[string]any{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []any{map[string]any{
			"name": "ghcr.io/" + testRepository + "/api",
			"digest": map[string]string{
				"sha256": strings.TrimPrefix(digest, "sha256:"),
			},
		}},
		"predicateType": "https://example.test/predicate",
		"predicate":     predicateValue,
	}
	payload, err := json.Marshal(statement)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(map[string]any{
		"payloadType": "application/vnd.in-toto+json",
		"payload":     base64.StdEncoding.EncodeToString(payload),
		"signatures":  []any{map[string]string{"sig": "verified"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}
