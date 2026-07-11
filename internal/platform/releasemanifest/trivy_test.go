package releasemanifest

import (
	"fmt"
	"strings"
	"testing"
)

func TestVerifyTrivyPolicy(t *testing.T) {
	digest := strings.Repeat("a", 64)
	imageRef := "ghcr.io/fxck-vk/vk_agregator/api@sha256:" + digest
	clean := []byte(fmt.Sprintf(`{"SchemaVersion":2,"ArtifactName":%q,"Results":[{"Target":"runtime","Vulnerabilities":null}]}`, imageRef))
	if err := VerifyTrivyPolicy(clean, imageRef); err != nil {
		t.Fatalf("VerifyTrivyPolicy(clean) error = %v", err)
	}

	for _, severity := range []string{"HIGH", "CRITICAL"} {
		t.Run(severity, func(t *testing.T) {
			report := []byte(fmt.Sprintf(`{"SchemaVersion":2,"ArtifactName":%q,"Results":[{"Vulnerabilities":[{"VulnerabilityID":"TEST-1","Severity":%q}]}]}`, imageRef, severity))
			if err := VerifyTrivyPolicy(report, imageRef); err == nil {
				t.Fatalf("VerifyTrivyPolicy accepted %s vulnerability", severity)
			}
		})
	}

	if err := VerifyTrivyPolicy(clean, "ghcr.io/fxck-vk/vk_agregator/api@sha256:"+strings.Repeat("b", 64)); err == nil {
		t.Fatal("VerifyTrivyPolicy accepted report for another digest")
	}
	duplicate := []byte(fmt.Sprintf(`{"SchemaVersion":2,"SchemaVersion":2,"ArtifactName":%q,"Results":[]}`, imageRef))
	if err := VerifyTrivyPolicy(duplicate, imageRef); err == nil {
		t.Fatal("VerifyTrivyPolicy accepted duplicate JSON keys")
	}
}
