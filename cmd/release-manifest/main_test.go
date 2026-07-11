package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vk-ai-aggregator/internal/platform/releasemanifest"
)

func TestRunVerifyAttestation(t *testing.T) {
	dir := t.TempDir()
	digest := cliDigest("api")
	predicate := []byte(`{"component":"expected"}`)
	predicatePath := filepath.Join(dir, "predicate.json")
	verificationPath := filepath.Join(dir, "cosign-verification.json")
	if err := os.WriteFile(predicatePath, predicate, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(verificationPath, cliAttestationEnvelope(t, predicate, digest), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"verify-attestation",
		"--verification-output", verificationPath,
		"--predicate", predicatePath,
		"--image-ref", "ghcr.io/fxck-vk/vk_agregator/api@" + digest,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify-attestation code = %d, stderr = %s", code, stderr.String())
	}

	if err := os.WriteFile(predicatePath, []byte(`{"component":"tampered"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"verify-attestation",
		"--verification-output", verificationPath,
		"--predicate", predicatePath,
		"--image-ref", "ghcr.io/fxck-vk/vk_agregator/api@" + digest,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("tampered verify-attestation code = %d, stderr = %s", code, stderr.String())
	}
}

func TestRunAssembleAndVerify(t *testing.T) {
	dir := t.TempDir()
	for _, service := range releasemanifest.ExpectedServices() {
		artifactDir := filepath.Join(dir, service.Name)
		if err := os.MkdirAll(artifactDir, 0o700); err != nil {
			t.Fatal(err)
		}
		cycloneDX := cliArtifact(t, dir, service.Name+"/runtime.cdx.json", []byte(`{"components":[{"name":"runtime"}]}`))
		spdx := cliArtifact(t, dir, service.Name+"/runtime.spdx.json", []byte(`{"packages":[{"name":"runtime"}]}`))
		digest := cliDigest(service.Name)
		repository := "ghcr.io/fxck-vk/vk_agregator/" + service.Name
		provenanceRaw, err := json.Marshal(map[string]any{
			"buildType": "https://mobyproject.org/buildkit@v1",
			"metadata": map[string]any{
				"https://github.com/Fxck-VK/vk_agregator/release/v1": map[string]string{
					"commit_sha":       "0123456789abcdef0123456789abcdef01234567",
					"image_digest":     digest,
					"image_repository": repository,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		provenance := cliArtifact(t, dir, service.Name+"/provenance.json", provenanceRaw)
		metadata := releasemanifest.Image{
			Service:    service.Name,
			Repository: repository,
			Digest:     digest,
			SBOM: releasemanifest.SBOM{
				CycloneDX: cycloneDX,
				SPDX:      spdx,
			},
			Provenance: provenance,
			VulnerabilityScan: releasemanifest.VulnerabilityScan{
				Scanner: "trivy", ScannerVersion: "v0.72.0", Status: "passed", Digest: digest,
			},
		}
		if service.Name == "miniapp" {
			sourceCycloneDX := cliArtifact(t, dir, service.Name+"/source.cdx.json", []byte(`{"components":[{"purl":"pkg:npm/example@1.0.0"}]}`))
			sourceSPDX := cliArtifact(t, dir, service.Name+"/source.spdx.json", []byte(`{"packages":[{"name":"example"}]}`))
			metadata.SBOM.SourceCycloneDX = &sourceCycloneDX
			metadata.SBOM.SourceSPDX = &sourceSPDX
		}
		raw, err := json.Marshal(metadata)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, service.Name+".metadata.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	manifestPath := filepath.Join(dir, "release-manifest.json")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"assemble", "--input-dir", dir, "--output", manifestPath,
		"--repository", "fxck-vk/vk_agregator",
		"--commit", "0123456789abcdef0123456789abcdef01234567",
		"--branch", "main",
		"--workflow-identity", "https://github.com/Fxck-VK/vk_agregator/.github/workflows/docker-images.yml@refs/heads/main",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("assemble code = %d, stderr = %s", code, stderr.String())
	}

	envPath := filepath.Join(dir, "release-images.env")
	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"verify", "--manifest", manifestPath, "--bundle-dir", dir,
		"--expected-repository", "fxck-vk/vk_agregator",
		"--expected-commit", "0123456789abcdef0123456789abcdef01234567",
		"--expected-workflow-identity", "https://github.com/Fxck-VK/vk_agregator/.github/workflows/docker-images.yml@refs/heads/main",
		"--output-env", envPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify code = %d, stderr = %s", code, stderr.String())
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("verified env missing: %v", err)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"unknown"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run code = %d, want 2", code)
	}
}

func cliArtifact(t *testing.T, dir, relative string, data []byte) releasemanifest.ArtifactRef {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(relative))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return releasemanifest.ArtifactRef{Path: relative, SHA256: hex.EncodeToString(sum[:])}
}

func cliDigest(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cliAttestationEnvelope(t *testing.T, predicate []byte, digest string) []byte {
	t.Helper()
	var predicateValue any
	if err := json.Unmarshal(predicate, &predicateValue); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []any{map[string]any{
			"name": "ghcr.io/fxck-vk/vk_agregator/api",
			"digest": map[string]string{
				"sha256": digest[len("sha256:"):],
			},
		}},
		"predicateType": "https://example.test/predicate",
		"predicate":     predicateValue,
	})
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
