package artifactservice_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func TestSaveBytesDeduplicates(t *testing.T) {
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	svc := artifactservice.New(repo, store, testBucket)
	owner := uuid.New()
	payload := []byte{0x1, 0x2, 0x3}

	first, err := svc.SaveBytesArtifact(context.Background(), owner, nil, domain.ArtifactKindOutput, domain.MediaTypeImage, "image/png", payload)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	second, err := svc.SaveBytesArtifact(context.Background(), owner, nil, domain.ArtifactKindOutput, domain.MediaTypeImage, "image/png", payload)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected dedup to return same artifact, got %s vs %s", first.ID, second.ID)
	}
	if store.Len() != 1 {
		t.Fatalf("expected one stored object, got %d", store.Len())
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
