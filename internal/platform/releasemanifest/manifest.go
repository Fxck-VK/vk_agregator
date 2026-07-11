// Package releasemanifest validates the immutable release bundle consumed by
// deployment workflows. It deliberately knows nothing about credentials or
// mutable image tags.
package releasemanifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const SchemaVersion = 1

const releaseProvenanceKey = "https://github.com/Fxck-VK/vk_agregator/release/v1"

var (
	commitPattern     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	sha256Pattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	repositoryPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$`)
	workflowPattern   = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*/\.github/workflows/[A-Za-z0-9][A-Za-z0-9_.-]*\.ya?ml@refs/heads/[A-Za-z0-9][A-Za-z0-9._/-]*$`)

	expectedServices = []Service{
		{Name: "api", EnvKey: "API_IMAGE"},
		{Name: "worker", EnvKey: "WORKER_IMAGE"},
		{Name: "provider-webhook", EnvKey: "PROVIDER_WEBHOOK_IMAGE"},
		{Name: "provider-balance-bot", EnvKey: "PROVIDER_BALANCE_BOT_IMAGE"},
		{Name: "miniapp", EnvKey: "MINIAPP_IMAGE"},
		{Name: "migrate", EnvKey: "MIGRATE_IMAGE"},
		{Name: "backup", EnvKey: "BACKUP_IMAGE"},
	}
)

type Service struct {
	Name   string
	EnvKey string
}

func ExpectedServices() []Service {
	return slices.Clone(expectedServices)
}

type ArtifactRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type SBOM struct {
	CycloneDX       ArtifactRef  `json:"cyclonedx"`
	SPDX            ArtifactRef  `json:"spdx"`
	SourceCycloneDX *ArtifactRef `json:"source_cyclonedx,omitempty"`
	SourceSPDX      *ArtifactRef `json:"source_spdx,omitempty"`
}

type VulnerabilityScan struct {
	Scanner        string `json:"scanner"`
	ScannerVersion string `json:"scanner_version"`
	Status         string `json:"status"`
	Digest         string `json:"digest"`
}

type Image struct {
	Service           string            `json:"service"`
	Repository        string            `json:"repository"`
	Digest            string            `json:"digest"`
	SBOM              SBOM              `json:"sbom"`
	Provenance        ArtifactRef       `json:"provenance"`
	VulnerabilityScan VulnerabilityScan `json:"vulnerability_scan"`
}

type Manifest struct {
	SchemaVersion    int     `json:"schema_version"`
	Repository       string  `json:"repository"`
	CommitSHA        string  `json:"commit_sha"`
	SourceBranch     string  `json:"source_branch"`
	WorkflowIdentity string  `json:"workflow_identity"`
	Images           []Image `json:"images"`
}

type ManifestHeader struct {
	Repository       string
	CommitSHA        string
	SourceBranch     string
	WorkflowIdentity string
}

type Expectations struct {
	Repository       string
	CommitSHA        string
	WorkflowIdentity string
}

type VerifiedRelease struct {
	Repository       string
	CommitSHA        string
	WorkflowIdentity string
	ManifestSHA256   string
	Images           map[string]string
}

// AssembleDirectory requires exactly one <service>.metadata.json file for each
// service returned by ExpectedServices and rejects other *.metadata.json files.
func AssembleDirectory(dir string, header ManifestHeader) (*Manifest, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("releasemanifest: open metadata directory: %w", err)
	}
	defer root.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("releasemanifest: read metadata directory: %w", err)
	}

	expectedFiles := make(map[string]Service, len(expectedServices))
	for _, service := range expectedServices {
		expectedFiles[service.Name+".metadata.json"] = service
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".metadata.json") {
			continue
		}
		if _, ok := expectedFiles[entry.Name()]; !ok {
			return nil, fmt.Errorf("releasemanifest: unexpected metadata file %q", entry.Name())
		}
	}

	imagesByService := make(map[string]Image, len(expectedServices))
	for _, service := range expectedServices {
		filename := service.Name + ".metadata.json"
		raw, err := root.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("releasemanifest: read metadata %q: %w", filename, err)
		}
		var image Image
		if err := decodeStrict(raw, &image); err != nil {
			return nil, fmt.Errorf("releasemanifest: decode metadata %q: %w", filename, err)
		}
		if image.Service != service.Name {
			return nil, fmt.Errorf("releasemanifest: metadata %q declares service %q", filename, image.Service)
		}
		imagesByService[image.Service] = image
	}

	manifest := &Manifest{
		SchemaVersion:    SchemaVersion,
		Repository:       header.Repository,
		CommitSHA:        header.CommitSHA,
		SourceBranch:     header.SourceBranch,
		WorkflowIdentity: header.WorkflowIdentity,
		Images:           make([]Image, 0, len(expectedServices)),
	}
	for _, service := range expectedServices {
		image, ok := imagesByService[service.Name]
		if !ok {
			return nil, fmt.Errorf("releasemanifest: missing service %q", service.Name)
		}
		manifest.Images = append(manifest.Images, image)
	}
	if _, err := validate(manifest, dir, Expectations{
		Repository:       header.Repository,
		CommitSHA:        header.CommitSHA,
		WorkflowIdentity: header.WorkflowIdentity,
	}, ""); err != nil {
		return nil, err
	}
	return manifest, nil
}

func WriteManifestFile(filename string, manifest *Manifest) error {
	if manifest == nil {
		return errors.New("releasemanifest: manifest is nil")
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("releasemanifest: encode manifest: %w", err)
	}
	raw = append(raw, '\n')
	return writeAtomic(filename, raw)
}

func VerifyFile(manifestPath, bundleDir string, expected Expectations) (*VerifiedRelease, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("releasemanifest: read manifest: %w", err)
	}
	var manifest Manifest
	if err := decodeStrict(raw, &manifest); err != nil {
		return nil, fmt.Errorf("releasemanifest: decode manifest: %w", err)
	}
	return validate(&manifest, bundleDir, expected, hashHex(raw))
}

func WriteEnvFile(filename string, release *VerifiedRelease) error {
	if release == nil {
		return errors.New("releasemanifest: verified release is nil")
	}
	if !repositoryPattern.MatchString(release.Repository) {
		return errors.New("releasemanifest: verified release repository is invalid")
	}
	if !commitPattern.MatchString(release.CommitSHA) {
		return errors.New("releasemanifest: verified release commit SHA is invalid")
	}
	if !sha256Pattern.MatchString(release.ManifestSHA256) {
		return errors.New("releasemanifest: verified release manifest sha256 is invalid")
	}
	if !workflowPattern.MatchString(release.WorkflowIdentity) {
		return errors.New("releasemanifest: verified release workflow identity is invalid")
	}
	if len(release.Images) != len(expectedServices) {
		return fmt.Errorf("releasemanifest: verified release must contain exactly seven images")
	}

	var builder strings.Builder
	for _, service := range expectedServices {
		image, ok := release.Images[service.Name]
		if !ok {
			return fmt.Errorf("releasemanifest: verified release missing service %q", service.Name)
		}
		prefix := "ghcr.io/" + release.Repository + "/" + service.Name + "@"
		digest := strings.TrimPrefix(image, prefix)
		if digest == image || !digestPattern.MatchString(digest) {
			return fmt.Errorf("releasemanifest: verified release image is invalid for %q", service.Name)
		}
		fmt.Fprintf(&builder, "%s=%s\n", service.EnvKey, image)
	}
	fmt.Fprintf(&builder, "RELEASE_COMMIT_SHA=%s\n", release.CommitSHA)
	fmt.Fprintf(&builder, "RELEASE_MANIFEST_SHA256=%s\n", release.ManifestSHA256)
	fmt.Fprintf(&builder, "RELEASE_WORKFLOW_IDENTITY=%s\n", release.WorkflowIdentity)
	return writeAtomic(filename, []byte(builder.String()))
}

func validate(manifest *Manifest, bundleDir string, expected Expectations, manifestHash string) (*VerifiedRelease, error) {
	root, err := os.OpenRoot(bundleDir)
	if err != nil {
		return nil, fmt.Errorf("releasemanifest: open bundle directory: %w", err)
	}
	defer root.Close()

	if manifest.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("releasemanifest: schema version %d is unsupported", manifest.SchemaVersion)
	}
	if !repositoryPattern.MatchString(manifest.Repository) {
		return nil, fmt.Errorf("releasemanifest: repository %q is invalid", manifest.Repository)
	}
	if manifest.Repository != expected.Repository {
		return nil, fmt.Errorf("releasemanifest: repository mismatch")
	}
	if !commitPattern.MatchString(manifest.CommitSHA) {
		return nil, fmt.Errorf("releasemanifest: commit SHA is invalid")
	}
	if manifest.CommitSHA != expected.CommitSHA {
		return nil, fmt.Errorf("releasemanifest: commit mismatch")
	}
	if strings.TrimSpace(manifest.SourceBranch) == "" || containsControl(manifest.SourceBranch) {
		return nil, fmt.Errorf("releasemanifest: source branch is invalid")
	}
	if manifest.WorkflowIdentity != expected.WorkflowIdentity || !workflowPattern.MatchString(manifest.WorkflowIdentity) {
		return nil, fmt.Errorf("releasemanifest: workflow identity mismatch")
	}
	if len(manifest.Images) != len(expectedServices) {
		return nil, fmt.Errorf("releasemanifest: expected exactly seven images, got %d", len(manifest.Images))
	}

	expectedByName := make(map[string]Service, len(expectedServices))
	for _, service := range expectedServices {
		expectedByName[service.Name] = service
	}
	seen := make(map[string]struct{}, len(expectedServices))
	images := make(map[string]string, len(expectedServices))
	for index := range manifest.Images {
		image := &manifest.Images[index]
		service, ok := expectedByName[image.Service]
		if !ok {
			return nil, fmt.Errorf("releasemanifest: unknown service %q", image.Service)
		}
		if _, duplicate := seen[image.Service]; duplicate {
			return nil, fmt.Errorf("releasemanifest: duplicate service %q", image.Service)
		}
		seen[image.Service] = struct{}{}
		wantRepository := "ghcr.io/" + manifest.Repository + "/" + service.Name
		if image.Repository != wantRepository {
			return nil, fmt.Errorf("releasemanifest: image repository mismatch for %q", service.Name)
		}
		if !digestPattern.MatchString(image.Digest) {
			return nil, fmt.Errorf("releasemanifest: digest is invalid for %q", service.Name)
		}
		if strings.TrimSpace(image.VulnerabilityScan.Scanner) == "" || strings.TrimSpace(image.VulnerabilityScan.ScannerVersion) == "" {
			return nil, fmt.Errorf("releasemanifest: vulnerability scan metadata missing for %q", service.Name)
		}
		if image.VulnerabilityScan.Status != "passed" {
			return nil, fmt.Errorf("releasemanifest: vulnerability scan did not pass for %q", service.Name)
		}
		if image.VulnerabilityScan.Digest != image.Digest {
			return nil, fmt.Errorf("releasemanifest: scan digest mismatch for %q", service.Name)
		}
		if err := validateArtifact(root, service.Name+" cyclonedx SBOM", image.SBOM.CycloneDX); err != nil {
			return nil, err
		}
		if err := validateArtifact(root, service.Name+" spdx SBOM", image.SBOM.SPDX); err != nil {
			return nil, err
		}
		if (image.SBOM.SourceCycloneDX == nil) != (image.SBOM.SourceSPDX == nil) {
			return nil, fmt.Errorf("releasemanifest: source SBOM refs must be provided together for %q", service.Name)
		}
		if service.Name == "miniapp" && image.SBOM.SourceCycloneDX == nil {
			return nil, fmt.Errorf("releasemanifest: source SBOM refs are required for %q", service.Name)
		}
		if service.Name != "miniapp" && image.SBOM.SourceCycloneDX != nil {
			return nil, fmt.Errorf("releasemanifest: source SBOM refs are not allowed for %q", service.Name)
		}
		if image.SBOM.SourceCycloneDX != nil {
			if err := validateArtifact(root, service.Name+" source cyclonedx SBOM", *image.SBOM.SourceCycloneDX); err != nil {
				return nil, err
			}
			if err := validateArtifact(root, service.Name+" source spdx SBOM", *image.SBOM.SourceSPDX); err != nil {
				return nil, err
			}
		}
		provenanceRaw, err := validateArtifactBytes(root, service.Name+" provenance", image.Provenance)
		if err != nil {
			return nil, err
		}
		if err := validateProvenanceBinding(provenanceRaw, manifest.CommitSHA, image.Repository, image.Digest); err != nil {
			return nil, fmt.Errorf("releasemanifest: %s provenance %w", service.Name, err)
		}
		images[service.Name] = image.Repository + "@" + image.Digest
	}
	for _, service := range expectedServices {
		if _, ok := seen[service.Name]; !ok {
			return nil, fmt.Errorf("releasemanifest: missing service %q", service.Name)
		}
	}

	return &VerifiedRelease{
		Repository:       manifest.Repository,
		CommitSHA:        manifest.CommitSHA,
		WorkflowIdentity: manifest.WorkflowIdentity,
		ManifestSHA256:   manifestHash,
		Images:           images,
	}, nil
}

func validateArtifact(root *os.Root, label string, ref ArtifactRef) error {
	_, err := validateArtifactBytes(root, label, ref)
	return err
}

func validateArtifactBytes(root *os.Root, label string, ref ArtifactRef) ([]byte, error) {
	if strings.TrimSpace(ref.Path) == "" {
		return nil, fmt.Errorf("releasemanifest: %s artifact path is missing", label)
	}
	if strings.Contains(ref.Path, `\`) || path.IsAbs(ref.Path) {
		return nil, fmt.Errorf("releasemanifest: %s artifact path must be relative", label)
	}
	clean := path.Clean(ref.Path)
	if clean == "." || clean != ref.Path || clean == ".." || strings.HasPrefix(clean, "../") {
		return nil, fmt.Errorf("releasemanifest: %s artifact path must be a clean relative path", label)
	}
	if !sha256Pattern.MatchString(ref.SHA256) {
		return nil, fmt.Errorf("releasemanifest: %s artifact sha256 is invalid", label)
	}
	raw, err := root.ReadFile(filepath.FromSlash(clean))
	if err != nil {
		return nil, fmt.Errorf("releasemanifest: read artifact %q: %w", clean, err)
	}
	if got := hashHex(raw); got != ref.SHA256 {
		return nil, fmt.Errorf("releasemanifest: %s artifact sha256 mismatch", label)
	}
	return raw, nil
}

func validateProvenanceBinding(raw []byte, commitSHA, repository, digest string) error {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return fmt.Errorf("is not valid JSON: %w", err)
	}
	var document struct {
		Metadata map[string]json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return fmt.Errorf("is not valid JSON: %w", err)
	}
	bindingRaw, ok := document.Metadata[releaseProvenanceKey]
	if !ok {
		return errors.New("release binding is missing")
	}
	var binding struct {
		CommitSHA       string `json:"commit_sha"`
		ImageDigest     string `json:"image_digest"`
		ImageRepository string `json:"image_repository"`
	}
	if err := decodeStrict(bindingRaw, &binding); err != nil {
		return fmt.Errorf("release binding is invalid: %w", err)
	}
	if binding.CommitSHA != commitSHA {
		return errors.New("commit mismatch")
	}
	if binding.ImageRepository != repository {
		return errors.New("repository mismatch")
	}
	if binding.ImageDigest != digest {
		return errors.New("digest mismatch")
	}
	return nil
}

func decodeStrict(raw []byte, target any) error {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func writeAtomic(filename string, data []byte) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("releasemanifest: create output directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".release-*.tmp")
	if err != nil {
		return fmt.Errorf("releasemanifest: create temporary output: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("releasemanifest: chmod temporary output: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("releasemanifest: write temporary output: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("releasemanifest: sync temporary output: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("releasemanifest: close temporary output: %w", err)
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return fmt.Errorf("releasemanifest: replace output: %w", err)
	}
	if err := os.Chmod(filename, 0o600); err != nil {
		return fmt.Errorf("releasemanifest: chmod output: %w", err)
	}
	return nil
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func containsControl(value string) bool {
	return strings.ContainsAny(value, "\r\n\x00")
}
