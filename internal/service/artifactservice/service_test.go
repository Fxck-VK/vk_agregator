package artifactservice_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/artifactservice"
)

const testBucket = "artifacts"

type stubDownloader struct {
	data        []byte
	contentType string
	err         error
}

func (d stubDownloader) Download(_ context.Context, _ string) ([]byte, string, error) {
	return d.data, d.contentType, d.err
}

func TestSaveTextArtifact(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	svc := artifactservice.New(repo, store, testBucket)
	owner := uuid.New()

	art, err := svc.SaveTextArtifact(context.Background(), owner, nil, domain.ArtifactKindOutput, "hello world")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if art.MediaType != domain.MediaTypeText || art.Status != domain.ArtifactStatusReady {
		t.Fatalf("unexpected artifact: %+v", art)
	}
	want := sha256.Sum256([]byte("hello world"))
	if art.SHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("sha mismatch: %s", art.SHA256)
	}
	if data, ok := store.Get(art.StorageBucket, art.StorageKey); !ok || string(data) != "hello world" {
		t.Fatalf("object not stored correctly: %q ok=%v", string(data), ok)
	}
}

func TestSaveInputReferenceImageDeduplicatesByOwnerHashAndPolicy(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	svc := artifactservice.New(repo, store, testBucket)
	owner := uuid.New()
	payload := []byte{0x1, 0x2, 0x3}

	first, err := svc.SaveBytesArtifact(context.Background(), owner, nil, domain.ArtifactKindInput, domain.MediaTypeImage, "image/png", payload)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	second, err := svc.SaveBytesArtifact(context.Background(), owner, nil, domain.ArtifactKindInput, domain.MediaTypeImage, "image/png", payload)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected dedup to return same artifact, got %s vs %s", first.ID, second.ID)
	}
	if first.LifecycleClass != domain.ArtifactLifecycleInputReference {
		t.Fatalf("LifecycleClass = %q, want input_reference", first.LifecycleClass)
	}
	if first.ValidationPolicyVersion != artifactservice.ReferenceImageValidationPolicyVersion {
		t.Fatalf("ValidationPolicyVersion = %q", first.ValidationPolicyVersion)
	}
	if store.Len() != 1 {
		t.Fatalf("expected one stored object, got %d", store.Len())
	}
}

func TestSaveInputReferenceDedupeIsOwnerIsolated(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	svc := artifactservice.New(repo, store, testBucket)
	payload := []byte{0x1, 0x2, 0x3}

	first, err := svc.SaveBytesArtifact(context.Background(), uuid.New(), nil, domain.ArtifactKindInput, domain.MediaTypeImage, "image/png", payload)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	second, err := svc.SaveBytesArtifact(context.Background(), uuid.New(), nil, domain.ArtifactKindInput, domain.MediaTypeImage, "image/png", payload)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("expected different owners to keep separate artifacts: %s", first.ID)
	}
	if store.Len() != 2 {
		t.Fatalf("expected two stored objects, got %d", store.Len())
	}
}

func TestSaveInputReferenceDedupeIgnoresOldPolicy(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	svc := artifactservice.New(repo, store, testBucket)
	owner := uuid.New()
	payload := []byte{0x1, 0x2, 0x3}
	sum := sha256.Sum256(payload)
	old := &domain.Artifact{
		ID:                      uuid.New(),
		OwnerUserID:             owner,
		Kind:                    domain.ArtifactKindInput,
		MediaType:               domain.MediaTypeImage,
		MimeType:                "image/png",
		StorageBucket:           testBucket,
		StorageKey:              "old-policy.png",
		SHA256:                  hex.EncodeToString(sum[:]),
		ValidationPolicyVersion: "image_reference_old",
		LifecycleClass:          domain.ArtifactLifecycleInputReference,
		SizeBytes:               int64(len(payload)),
		Status:                  domain.ArtifactStatusReady,
	}
	if err := repo.Create(context.Background(), old); err != nil {
		t.Fatalf("seed old policy artifact: %v", err)
	}

	art, err := svc.SaveBytesArtifact(context.Background(), owner, nil, domain.ArtifactKindInput, domain.MediaTypeImage, "image/png", payload)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if art.ID == old.ID {
		t.Fatalf("unexpected reuse of old policy artifact %s", old.ID)
	}
	if art.ValidationPolicyVersion != artifactservice.ReferenceImageValidationPolicyVersion {
		t.Fatalf("ValidationPolicyVersion = %q", art.ValidationPolicyVersion)
	}
}

func TestSaveOutputArtifactsDoNotReuseReferenceDedupe(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	svc := artifactservice.New(repo, store, testBucket)
	owner := uuid.New()
	payload := []byte("same provider output")

	first, err := svc.SaveBytesArtifact(context.Background(), owner, nil, domain.ArtifactKindOutput, domain.MediaTypeImage, "image/png", payload)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	second, err := svc.SaveBytesArtifact(context.Background(), owner, nil, domain.ArtifactKindOutput, domain.MediaTypeImage, "image/png", payload)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("output artifacts should keep separate ids, got %s", first.ID)
	}
	if first.StorageKey == second.StorageKey {
		t.Fatalf("output artifacts should not share storage key %q", first.StorageKey)
	}
	if first.LifecycleClass != domain.ArtifactLifecycleProviderOriginal {
		t.Fatalf("LifecycleClass = %q, want provider_original", first.LifecycleClass)
	}
}

func TestSaveRemoteArtifact(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	dl := stubDownloader{data: []byte("video-bytes"), contentType: "video/mp4"}
	svc := artifactservice.New(repo, store, testBucket, artifactservice.WithDownloader(dl))
	owner := uuid.New()

	art, err := svc.SaveRemoteArtifact(context.Background(), owner, nil, domain.ArtifactKindOutput, domain.MediaTypeVideo, "mock://x/output.mp4")
	if err != nil {
		t.Fatalf("save remote: %v", err)
	}
	if art.MimeType != "video/mp4" || art.SizeBytes != int64(len("video-bytes")) {
		t.Fatalf("unexpected artifact: %+v", art)
	}
	if _, ok := store.Get(art.StorageBucket, art.StorageKey); !ok {
		t.Fatalf("remote bytes not stored")
	}
}

func TestSaveBytesArtifactWithMetadata(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	svc := artifactservice.New(repo, store, testBucket)
	owner := uuid.New()

	art, err := svc.SaveBytesArtifactWithMetadata(
		context.Background(),
		owner,
		nil,
		domain.ArtifactKindOutput,
		domain.MediaTypeVideo,
		"video/mp4",
		[]byte("video-bytes-with-metadata"),
		domain.ArtifactMediaMetadata{
			Width:       1920,
			Height:      1080,
			DurationMS:  5000,
			Codec:       " H.264 ",
			Container:   "MP4",
			BitrateBPS:  4500000,
			ProbeStatus: domain.MediaProbePassed,
		},
	)
	if err != nil {
		t.Fatalf("save with metadata: %v", err)
	}
	if art.Width != 1920 || art.Height != 1080 || art.DurationMS != 5000 || art.BitrateBPS != 4500000 {
		t.Fatalf("metadata numeric fields not stored: %+v", art)
	}
	if art.Codec != "h.264" || art.Container != "mp4" || art.ProbeStatus != domain.MediaProbePassed {
		t.Fatalf("metadata tokens not normalized: %+v", art)
	}
}

func TestSaveVariantWithMetadataIsIdempotent(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	svc := artifactservice.New(repo, store, testBucket)
	owner := uuid.New()

	parent, err := svc.SaveBytesArtifact(context.Background(), owner, nil, domain.ArtifactKindOutput, domain.MediaTypeVideo, "video/webm", []byte("raw-provider-video"))
	if err != nil {
		t.Fatalf("save parent: %v", err)
	}
	first, err := svc.SaveVariantWithMetadata(context.Background(), parent, domain.VariantVKVideo, "video/mp4", []byte("vk-ready-video"), domain.ArtifactMediaMetadata{
		Width:       1280,
		Height:      720,
		DurationMS:  5000,
		Codec:       "H264",
		Container:   "MP4",
		BitrateBPS:  2400000,
		ProbeStatus: domain.MediaProbePassed,
	})
	if err != nil {
		t.Fatalf("save variant: %v", err)
	}
	second, err := svc.SaveVariantWithMetadata(context.Background(), parent, domain.VariantVKVideo, "video/mp4", []byte("different retry bytes ignored"), domain.ArtifactMediaMetadata{})
	if err != nil {
		t.Fatalf("save variant retry: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("retry returned different variant: %s vs %s", first.ID, second.ID)
	}
	if store.Len() != 2 {
		t.Fatalf("expected original plus one variant object, got %d", store.Len())
	}
	if data, ok := store.Get(first.StorageBucket, first.StorageKey); !ok || string(data) != "vk-ready-video" {
		t.Fatalf("variant bytes not stored correctly: %q ok=%v", string(data), ok)
	}
	if first.Codec != "h264" || first.Container != "mp4" || first.ProbeStatus != domain.MediaProbePassed {
		t.Fatalf("variant metadata not normalized: %+v", first)
	}
	variants, err := repo.ListVariants(context.Background(), parent.ID)
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(variants) != 1 || variants[0].VariantType != domain.VariantVKVideo {
		t.Fatalf("expected one vk_video variant, got %+v", variants)
	}
}

func TestSaveBytesArtifactDefaultsProbeStatus(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	svc := artifactservice.New(repo, store, testBucket)
	owner := uuid.New()

	art, err := svc.SaveBytesArtifact(context.Background(), owner, nil, domain.ArtifactKindOutput, domain.MediaTypeVideo, "video/mp4", []byte("video-bytes"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if art.ProbeStatus != domain.MediaProbeUnknown {
		t.Fatalf("ProbeStatus = %q, want unknown", art.ProbeStatus)
	}
}

func TestSaveRemoteArtifactRedactsURLFromDownloadError(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	dl := stubDownloader{err: errors.New("provider fetch failed for https://provider.example/private/output.png?token=secret")}
	svc := artifactservice.New(repo, store, testBucket, artifactservice.WithDownloader(dl))
	owner := uuid.New()

	_, err := svc.SaveRemoteArtifact(context.Background(), owner, nil, domain.ArtifactKindOutput, domain.MediaTypeImage, "https://provider.example/private/output.png?token=secret")
	if err == nil {
		t.Fatal("expected download error")
	}
	msg := err.Error()
	for _, forbidden := range []string{"provider.example", "private/output.png", "token=secret"} {
		if strings.Contains(msg, forbidden) {
			t.Fatalf("error leaked %q in %q", forbidden, msg)
		}
	}
	if !strings.Contains(msg, "[redacted-url]") {
		t.Fatalf("error was not redacted: %q", msg)
	}
}
