package imagepreview

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/artifactservice"
)

type testObjects struct {
	mu      sync.Mutex
	data    map[string][]byte
	reads   map[string]int
	puts    int
	failPut bool
}

func (s *testObjects) GetObject(_ context.Context, _, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads[key]++
	value, ok := s.data[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return append([]byte(nil), value...), nil
}
func (s *testObjects) Put(_ context.Context, _, key string, data []byte, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failPut {
		return errors.New("unavailable")
	}
	s.puts++
	s.data[key] = append([]byte(nil), data...)
	return nil
}

type testRenderer struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
	err     error
}

func (r *testRenderer) Render(ctx context.Context, _ []byte, _ string) ([]byte, domain.ArtifactMediaMetadata, error) {
	r.calls.Add(1)
	if r.entered != nil {
		r.entered <- struct{}{}
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return nil, domain.ArtifactMediaMetadata{}, ctx.Err()
		}
	}
	return []byte("saved WebP bytes"), domain.ArtifactMediaMetadata{Width: 320, Height: 640}, r.err
}
func fixture() (*domain.Artifact, *memory.ArtifactRepo, *testObjects, *testRenderer) {
	a := &domain.Artifact{ID: uuid.New(), OwnerAccountID: uuid.New(), Kind: domain.ArtifactKindOutput, MediaType: domain.MediaTypeImage, MimeType: "image/png", Status: domain.ArtifactStatusReady, StorageBucket: "private", StorageKey: "original", SizeBytes: 8}
	return a, memory.NewArtifactRepo(), &testObjects{data: map[string][]byte{"original": []byte("original")}, reads: map[string]int{}}, &testRenderer{}
}
func TestThumbnailPersistsAcrossServicesWithoutReadingOriginalAgain(t *testing.T) {
	a, repo, objects, renderer := fixture()
	saver := artifactservice.New(repo, objects, "private")
	for range 2 {
		service := New(repo, objects, saver, renderer)
		got, err := service.GetThumbnail(context.Background(), a)
		if err != nil || string(got) != "saved WebP bytes" {
			t.Fatalf("thumbnail = %q, error = %v", got, err)
		}
	}
	if renderer.calls.Load() != 1 || objects.reads["original"] != 1 || objects.puts != 1 {
		t.Fatalf("renders=%d original reads=%d puts=%d", renderer.calls.Load(), objects.reads["original"], objects.puts)
	}
	variants, _ := repo.ListVariants(context.Background(), a.ID)
	if len(variants) != 1 || variants[0].VariantType != domain.VariantThumbnail || variants[0].MimeType != "image/webp" || variants[0].LifecycleClass != domain.ArtifactLifecycleDeliveryVariant {
		t.Fatal("missing retained private thumbnail variant")
	}
	if !strings.HasSuffix(variants[0].StorageKey, ".webp") {
		t.Fatal("WebP thumbnail must use a WebP object extension")
	}
}
func TestParallelThumbnailRequestsShareOneConversion(t *testing.T) {
	a, repo, objects, renderer := fixture()
	renderer.release = make(chan struct{})
	renderer.entered = make(chan struct{}, 20)
	service := New(repo, objects, artifactservice.New(repo, objects, "private"), renderer)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := service.GetThumbnail(context.Background(), a); err != nil {
				t.Error(err)
			}
		}()
	}
	<-renderer.entered
	close(renderer.release)
	wg.Wait()
	if renderer.calls.Load() != 1 || objects.puts != 1 {
		t.Fatalf("renders=%d puts=%d", renderer.calls.Load(), objects.puts)
	}
}
func TestThumbnailFailuresRemainRetryableAndNeverSaveFailedOutput(t *testing.T) {
	a, repo, objects, renderer := fixture()
	service := New(repo, objects, artifactservice.New(repo, objects, "private"), renderer)
	renderer.err = errors.New("convert failed")
	if _, err := service.GetThumbnail(context.Background(), a); err == nil {
		t.Fatal("expected conversion failure")
	}
	variants, _ := repo.ListVariants(context.Background(), a.ID)
	if len(variants) != 0 || objects.puts != 0 {
		t.Fatal("failed conversion saved a variant")
	}
	renderer.err = nil
	objects.failPut = true
	if _, err := service.GetThumbnail(context.Background(), a); err == nil {
		t.Fatal("expected storage failure")
	}
	objects.failPut = false
	if _, err := service.GetThumbnail(context.Background(), a); err != nil {
		t.Fatal(err)
	}
}
func TestCancelledThumbnailDoesNotSaveLateOutput(t *testing.T) {
	a, repo, objects, renderer := fixture()
	renderer.release = make(chan struct{})
	renderer.entered = make(chan struct{}, 1)
	service := New(repo, objects, artifactservice.New(repo, objects, "private"), renderer)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, err := service.GetThumbnail(ctx, a); result <- err }()
	<-renderer.entered
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if objects.puts != 0 {
		t.Fatal("cancelled work saved bytes")
	}
}
func TestThumbnailRejectsInvalidParentsAndSourceSizesBeforeRead(t *testing.T) {
	for _, change := range []func(*domain.Artifact){
		func(a *domain.Artifact) { a.Status = domain.ArtifactStatusDeleted },
		func(a *domain.Artifact) { a.OwnerAccountID = uuid.Nil },
		func(a *domain.Artifact) { a.MediaType = domain.MediaTypeVideo },
		func(a *domain.Artifact) { a.MimeType = "image/svg+xml" },
		func(a *domain.Artifact) { a.SizeBytes = 21 << 20 },
	} {
		a, repo, objects, renderer := fixture()
		change(a)
		service := New(repo, objects, artifactservice.New(repo, objects, "private"), renderer)
		if _, err := service.GetThumbnail(context.Background(), a); err == nil {
			t.Fatal("invalid parent accepted")
		}
		if len(objects.reads) != 0 || renderer.calls.Load() != 0 {
			t.Fatal("invalid parent was processed")
		}
	}
}

func TestThumbnailConversionsAndWaitingQueueAreBounded(t *testing.T) {
	a, repo, objects, renderer := fixture()
	renderer.release = make(chan struct{})
	renderer.entered = make(chan struct{}, 36)
	service := New(repo, objects, artifactservice.New(repo, objects, "private"), renderer)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for range 36 {
		parent := *a
		parent.ID = uuid.New()
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = service.GetThumbnail(ctx, &parent) }()
	}
	for range 4 {
		select {
		case <-renderer.entered:
		case <-ctx.Done():
			t.Fatal("conversions did not start")
		}
	}
	// Wait until the queued requests have all reached admission, without assuming
	// goroutine scheduling order. This permits testing refusal before any work.
	for {
		service.mu.Lock()
		pending := len(service.pending)
		service.mu.Unlock()
		if pending == 36 {
			break
		}
		select {
		case <-time.After(time.Millisecond):
		case <-ctx.Done():
			t.Fatal("requests did not reach the queue")
		}
	}
	if renderer.calls.Load() != 4 {
		t.Fatal("too many simultaneous conversions")
	}
	parent := *a
	parent.ID = uuid.New()
	if _, err := service.GetThumbnail(ctx, &parent); err == nil {
		t.Fatal("accepted a request beyond the waiting limit")
	}
	cancel()
	wg.Wait()
	if objects.puts != 0 {
		t.Fatal("cancelled queue saved output")
	}
}

func TestThumbnailRejectsMismatchedOriginalBytesBeforeRendering(t *testing.T) {
	a, repo, objects, renderer := fixture()
	objects.data["original"] = []byte("short")
	service := New(repo, objects, artifactservice.New(repo, objects, "private"), renderer)
	if _, err := service.GetThumbnail(context.Background(), a); err == nil || renderer.calls.Load() != 0 || objects.puts != 0 {
		t.Fatalf("err=%v renders=%d puts=%d", err, renderer.calls.Load(), objects.puts)
	}
}
