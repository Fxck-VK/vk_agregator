package releasemanifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestVerifyRejectsDuplicateManifestKeys(t *testing.T) {
	dir, manifestPath := writeValidBundle(t)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(raw)
	needle := []byte(`"commit_sha": "` + testCommit + `"`)
	replacement := []byte(`"commit_sha": "` + strings.Repeat("a", 40) + `", "commit_sha": "` + testCommit + `"`)
	raw = bytes.Replace(raw, needle, replacement, 1)
	if bytes.Equal(raw, original) {
		t.Fatal("test did not inject a duplicate manifest key")
	}
	if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = VerifyFile(manifestPath, dir, Expectations{
		Repository:       testRepository,
		CommitSHA:        testCommit,
		WorkflowIdentity: testWorkflowIdentity,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Fatalf("VerifyFile() error = %v, want duplicate-key rejection", err)
	}
}

const (
	testRepository       = "fxck-vk/vk_agregator"
	testCommit           = "0123456789abcdef0123456789abcdef01234567"
	testWorkflowIdentity = "https://github.com/Fxck-VK/vk_agregator/.github/workflows/docker-images.yml@refs/heads/main"
)

func TestVerifyAcceptsCompleteDigestRelease(t *testing.T) {
	dir, manifestPath := writeValidBundle(t)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	verified, err := VerifyFile(manifestPath, dir, Expectations{
		Repository:       testRepository,
		CommitSHA:        testCommit,
		WorkflowIdentity: testWorkflowIdentity,
	})
	if err != nil {
		t.Fatalf("VerifyFile() error = %v", err)
	}
	if len(verified.Images) != len(ExpectedServices()) {
		t.Fatalf("images = %d, want %d", len(verified.Images), len(ExpectedServices()))
	}

	envPath := filepath.Join(dir, "release-images.env")
	if err := WriteEnvFile(envPath, verified); err != nil {
		t.Fatalf("WriteEnvFile() error = %v", err)
	}
	envBytes, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	var expectedEnv strings.Builder
	for _, service := range ExpectedServices() {
		expectedEnv.WriteString(service.EnvKey + "=ghcr.io/" + testRepository + "/" + service.Name + "@" + digestFor(service.Name) + "\n")
	}
	expectedEnv.WriteString("RELEASE_COMMIT_SHA=" + testCommit + "\n")
	expectedEnv.WriteString("RELEASE_MANIFEST_SHA256=" + hashHex(manifestBytes) + "\n")
	expectedEnv.WriteString("RELEASE_WORKFLOW_IDENTITY=" + testWorkflowIdentity + "\n")
	if got, want := string(envBytes), expectedEnv.String(); got != want {
		t.Fatalf("env bytes differ\ngot:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(string(envBytes), ":sha-") || strings.Contains(string(envBytes), "IMAGE_TAG") {
		t.Fatalf("env contains mutable tag material:\n%s", envBytes)
	}
}

func TestVerifyRejectsTrustBoundaryViolations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{
			name: "missing service",
			mutate: func(m *Manifest) {
				m.Images = m.Images[:len(m.Images)-1]
			},
			want: "exactly seven",
		},
		{
			name: "duplicate service",
			mutate: func(m *Manifest) {
				m.Images[len(m.Images)-1].Service = m.Images[0].Service
			},
			want: "duplicate service",
		},
		{
			name: "wrong commit",
			mutate: func(m *Manifest) {
				m.CommitSHA = strings.Repeat("a", 40)
			},
			want: "commit",
		},
		{
			name: "wrong repository",
			mutate: func(m *Manifest) {
				m.Repository = "attacker/repository"
			},
			want: "repository",
		},
		{
			name: "wrong workflow identity",
			mutate: func(m *Manifest) {
				m.WorkflowIdentity = "https://github.com/attacker/repository/.github/workflows/docker-images.yml@refs/heads/main"
			},
			want: "workflow identity",
		},
		{
			name: "malformed digest",
			mutate: func(m *Manifest) {
				m.Images[0].Digest = "sha256:not-a-digest"
			},
			want: "digest",
		},
		{
			name: "wrong image repository",
			mutate: func(m *Manifest) {
				m.Images[0].Repository = "ghcr.io/attacker/api"
			},
			want: "image repository",
		},
		{
			name: "failed vulnerability scan",
			mutate: func(m *Manifest) {
				m.Images[0].VulnerabilityScan.Status = "failed"
			},
			want: "vulnerability scan",
		},
		{
			name: "scan digest mismatch",
			mutate: func(m *Manifest) {
				m.Images[0].VulnerabilityScan.Digest = digestFor("different")
			},
			want: "scan digest",
		},
		{
			name: "missing provenance",
			mutate: func(m *Manifest) {
				m.Images[0].Provenance = ArtifactRef{}
			},
			want: "provenance",
		},
		{
			name: "missing cyclonedx sbom",
			mutate: func(m *Manifest) {
				m.Images[0].SBOM.CycloneDX = ArtifactRef{}
			},
			want: "cyclonedx",
		},
		{
			name: "missing spdx sbom",
			mutate: func(m *Manifest) {
				m.Images[0].SBOM.SPDX = ArtifactRef{}
			},
			want: "spdx",
		},
		{
			name: "miniapp missing source sboms",
			mutate: func(m *Manifest) {
				m.Images[4].SBOM.SourceCycloneDX = nil
				m.Images[4].SBOM.SourceSPDX = nil
			},
			want: "source sbom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, manifestPath := writeValidBundle(t)
			manifest := readManifest(t, manifestPath)
			tt.mutate(&manifest)
			writeManifestJSON(t, manifestPath, manifest)

			_, err := VerifyFile(manifestPath, dir, Expectations{
				Repository:       testRepository,
				CommitSHA:        testCommit,
				WorkflowIdentity: testWorkflowIdentity,
			})
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("VerifyFile() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestVerifyRejectsUnsafeOrTamperedArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, dir string, manifest *Manifest)
		want   string
	}{
		{
			name: "path traversal",
			mutate: func(_ *testing.T, _ string, manifest *Manifest) {
				manifest.Images[0].Provenance.Path = "../provenance.json"
			},
			want: "relative",
		},
		{
			name: "absolute path",
			mutate: func(_ *testing.T, dir string, manifest *Manifest) {
				manifest.Images[0].Provenance.Path = filepath.Join(dir, "provenance.json")
			},
			want: "relative",
		},
		{
			name: "missing artifact",
			mutate: func(t *testing.T, dir string, manifest *Manifest) {
				path := filepath.Join(dir, filepath.FromSlash(manifest.Images[0].SBOM.CycloneDX.Path))
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			},
			want: "read artifact",
		},
		{
			name: "artifact hash mismatch",
			mutate: func(t *testing.T, dir string, manifest *Manifest) {
				path := filepath.Join(dir, filepath.FromSlash(manifest.Images[0].SBOM.SPDX.Path))
				if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "sha256",
		},
		{
			name: "source sbom hash mismatch",
			mutate: func(t *testing.T, dir string, manifest *Manifest) {
				miniapp := &manifest.Images[4]
				path := filepath.Join(dir, filepath.FromSlash(miniapp.SBOM.SourceCycloneDX.Path))
				if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "sha256",
		},
		{
			name: "provenance release commit mismatch",
			mutate: func(t *testing.T, dir string, manifest *Manifest) {
				ref := &manifest.Images[0].Provenance
				path := filepath.Join(dir, filepath.FromSlash(ref.Path))
				data := provenanceFor(manifest.Images[0].Repository, strings.Repeat("b", 40), manifest.Images[0].Digest)
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
				ref.SHA256 = hashHex(data)
			},
			want: "provenance commit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, manifestPath := writeValidBundle(t)
			manifest := readManifest(t, manifestPath)
			tt.mutate(t, dir, &manifest)
			writeManifestJSON(t, manifestPath, manifest)

			_, err := VerifyFile(manifestPath, dir, Expectations{
				Repository:       testRepository,
				CommitSHA:        testCommit,
				WorkflowIdentity: testWorkflowIdentity,
			})
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("VerifyFile() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestVerifyRejectsArtifactSymlinkEscape(t *testing.T) {
	dir, manifestPath := writeValidBundle(t)
	manifest := readManifest(t, manifestPath)
	externalDir := t.TempDir()
	externalData := []byte(`{"outside":true}`)
	externalPath := filepath.Join(externalDir, "outside.json")
	if err := os.WriteFile(externalPath, externalData, 0o600); err != nil {
		t.Fatal(err)
	}

	ref := &manifest.Images[0].Provenance
	insidePath := filepath.Join(dir, filepath.FromSlash(ref.Path))
	if err := os.Remove(insidePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalPath, insidePath); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation requires Windows Developer Mode or SeCreateSymbolicLinkPrivilege: %v", err)
		}
		t.Fatalf("create test symlink: %v", err)
	}
	ref.SHA256 = hashHex(externalData)
	writeManifestJSON(t, manifestPath, manifest)

	_, err := VerifyFile(manifestPath, dir, Expectations{
		Repository:       testRepository,
		CommitSHA:        testCommit,
		WorkflowIdentity: testWorkflowIdentity,
	})
	if err == nil {
		t.Fatal("VerifyFile() accepted an artifact symlink escaping the bundle")
	}
}

func TestVerifyRejectsUnsafeWorkflowIdentityEvenWhenExpectedMatches(t *testing.T) {
	dir, manifestPath := writeValidBundle(t)
	manifest := readManifest(t, manifestPath)
	malicious := testWorkflowIdentity + "\nINJECTED=value"
	manifest.WorkflowIdentity = malicious
	writeManifestJSON(t, manifestPath, manifest)

	_, err := VerifyFile(manifestPath, dir, Expectations{
		Repository:       testRepository,
		CommitSHA:        testCommit,
		WorkflowIdentity: malicious,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "workflow identity") {
		t.Fatalf("VerifyFile() error = %v, want workflow identity rejection", err)
	}
}

func TestWriteEnvFileRevalidatesVerifiedRelease(t *testing.T) {
	dir := t.TempDir()
	release := &VerifiedRelease{
		Repository:       testRepository,
		CommitSHA:        testCommit,
		ManifestSHA256:   strings.Repeat("a", 64),
		WorkflowIdentity: testWorkflowIdentity + "\nINJECTED=value",
		Images:           make(map[string]string),
	}
	for _, service := range ExpectedServices() {
		release.Images[service.Name] = "ghcr.io/" + testRepository + "/" + service.Name + "@" + digestFor(service.Name)
	}
	if err := WriteEnvFile(filepath.Join(dir, "release.env"), release); err == nil || !strings.Contains(strings.ToLower(err.Error()), "workflow identity") {
		t.Fatalf("WriteEnvFile() error = %v, want workflow identity rejection", err)
	}
}

func TestVerifyRejectsUnknownOrMutableTagFields(t *testing.T) {
	dir, manifestPath := writeValidBundle(t)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document["tag"] = "sha-" + testCommit
	raw, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = VerifyFile(manifestPath, dir, Expectations{
		Repository:       testRepository,
		CommitSHA:        testCommit,
		WorkflowIdentity: testWorkflowIdentity,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown") {
		t.Fatalf("VerifyFile() error = %v, want unknown-field rejection", err)
	}
}

func TestAssembleDirectoryIsStableAndStrict(t *testing.T) {
	dir := t.TempDir()
	manifest := validManifest(t, dir)
	for index := len(manifest.Images) - 1; index >= 0; index-- {
		image := manifest.Images[index]
		writeManifestJSON(t, filepath.Join(dir, image.Service+".metadata.json"), image)
	}

	assembled, err := AssembleDirectory(dir, ManifestHeader{
		Repository:       testRepository,
		CommitSHA:        testCommit,
		SourceBranch:     "main",
		WorkflowIdentity: testWorkflowIdentity,
	})
	if err != nil {
		t.Fatalf("AssembleDirectory() error = %v", err)
	}
	gotOrder := make([]string, 0, len(assembled.Images))
	for _, image := range assembled.Images {
		gotOrder = append(gotOrder, image.Service)
	}
	wantOrder := make([]string, 0, len(ExpectedServices()))
	for _, service := range ExpectedServices() {
		wantOrder = append(wantOrder, service.Name)
	}
	if !slices.Equal(gotOrder, wantOrder) {
		t.Fatalf("service order = %v, want %v", gotOrder, wantOrder)
	}

	writeManifestJSON(t, filepath.Join(dir, "unknown.metadata.json"), manifest.Images[0])
	if _, err := AssembleDirectory(dir, ManifestHeader{
		Repository:       testRepository,
		CommitSHA:        testCommit,
		SourceBranch:     "main",
		WorkflowIdentity: testWorkflowIdentity,
	}); err == nil {
		t.Fatal("AssembleDirectory() accepted an extra metadata file")
	}

	if err := os.Remove(filepath.Join(dir, "unknown.metadata.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "api.metadata.json"), filepath.Join(dir, "renamed.metadata.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := AssembleDirectory(dir, ManifestHeader{
		Repository:       testRepository,
		CommitSHA:        testCommit,
		SourceBranch:     "main",
		WorkflowIdentity: testWorkflowIdentity,
	}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "renamed.metadata.json") {
		t.Fatalf("AssembleDirectory() error = %v, want exact metadata filename rejection", err)
	}
}

func writeValidBundle(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	manifest := validManifest(t, dir)
	manifestPath := filepath.Join(dir, "release-manifest.json")
	writeManifestJSON(t, manifestPath, manifest)
	return dir, manifestPath
}

func validManifest(t *testing.T, dir string) Manifest {
	t.Helper()
	manifest := Manifest{
		SchemaVersion:    SchemaVersion,
		Repository:       testRepository,
		CommitSHA:        testCommit,
		SourceBranch:     "main",
		WorkflowIdentity: testWorkflowIdentity,
	}
	for _, service := range ExpectedServices() {
		prefix := service.Name + "/"
		cycloneDX := writeArtifact(t, dir, prefix+"runtime.cdx.json", []byte(`{"components":[{"name":"runtime"}]}`))
		spdx := writeArtifact(t, dir, prefix+"runtime.spdx.json", []byte(`{"packages":[{"name":"runtime"}]}`))
		provenance := writeArtifact(t, dir, prefix+"provenance.json", provenanceFor("ghcr.io/"+testRepository+"/"+service.Name, testCommit, digestFor(service.Name)))
		image := Image{
			Service:    service.Name,
			Repository: "ghcr.io/" + testRepository + "/" + service.Name,
			Digest:     digestFor(service.Name),
			SBOM: SBOM{
				CycloneDX: cycloneDX,
				SPDX:      spdx,
			},
			Provenance: provenance,
			VulnerabilityScan: VulnerabilityScan{
				Scanner:        "trivy",
				ScannerVersion: "v0.72.0",
				Status:         "passed",
				Digest:         digestFor(service.Name),
			},
		}
		if service.Name == "miniapp" {
			sourceCycloneDX := writeArtifact(t, dir, prefix+"source.cdx.json", []byte(`{"components":[{"purl":"pkg:npm/example@1.0.0"}]}`))
			sourceSPDX := writeArtifact(t, dir, prefix+"source.spdx.json", []byte(`{"packages":[{"name":"example"}]}`))
			image.SBOM.SourceCycloneDX = &sourceCycloneDX
			image.SBOM.SourceSPDX = &sourceSPDX
		}
		manifest.Images = append(manifest.Images, image)
	}
	return manifest
}

func provenanceFor(repository, commit, digest string) []byte {
	value := map[string]any{
		"buildType": "https://mobyproject.org/buildkit@v1",
		"metadata": map[string]any{
			"https://github.com/Fxck-VK/vk_agregator/release/v1": map[string]string{
				"commit_sha":       commit,
				"image_digest":     digest,
				"image_repository": repository,
			},
		},
	}
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func writeArtifact(t *testing.T, dir, relative string, data []byte) ArtifactRef {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return ArtifactRef{Path: relative, SHA256: hex.EncodeToString(sum[:])}
}

func digestFor(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func readManifest(t *testing.T, path string) Manifest {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func writeManifestJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
