// Package imagepreview stores private, reconstructible image variants.
// Callers must authorize the parent artifact and its moderated completed job.
package imagepreview

import (
	"context"
	"errors"
	"mime"
	"sync"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

const MaxSourceBytes = 20 << 20
const MaxThumbnailBytes = 2 << 20

var errUnavailable = errors.New("imagepreview: unavailable")

type VariantRepository interface {
	ListVariants(context.Context, uuid.UUID) ([]*domain.ArtifactVariant, error)
}
type ObjectReader interface {
	GetObject(context.Context, string, string) ([]byte, error)
}
type boundedReader interface {
	GetObjectBounded(context.Context, string, string, int64) ([]byte, error)
}
type VariantSaver interface {
	SaveVariantWithMetadata(context.Context, *domain.Artifact, domain.VariantType, string, []byte, domain.ArtifactMediaMetadata) (*domain.ArtifactVariant, error)
}
type Renderer interface {
	Render(context.Context, []byte, string) ([]byte, domain.ArtifactMediaMetadata, error)
}
type pendingRead struct {
	done chan struct{}
	data []byte
	err  error
}

type Service struct {
	repo     VariantRepository
	objects  ObjectReader
	saver    VariantSaver
	renderer Renderer
	slots    chan struct{}
	mu       sync.Mutex
	pending  map[uuid.UUID]*pendingRead
}

func New(repo VariantRepository, objects ObjectReader, saver VariantSaver, renderer Renderer) *Service {
	return &Service{repo: repo, objects: objects, saver: saver, renderer: renderer, slots: make(chan struct{}, 4), pending: make(map[uuid.UUID]*pendingRead)}
}

// GetThumbnail never accepts client storage coordinates. The parent is resolved
// by the owner-scoped repository and checked by the authenticated HTTP adapter.
func (s *Service) GetThumbnail(ctx context.Context, a *domain.Artifact) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a == nil || a.ID == uuid.Nil || a.OwnerAccountID == uuid.Nil || a.Status != domain.ArtifactStatusReady || a.Kind != domain.ArtifactKindOutput || a.MediaType != domain.MediaTypeImage || a.SizeBytes <= 0 || a.SizeBytes > MaxSourceBytes || a.StorageBucket == "" || a.StorageKey == "" || !supportedMime(a.MimeType) {
		return nil, errUnavailable
	}
	s.mu.Lock()
	if existing := s.pending[a.ID]; existing != nil {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-existing.done:
			return existing.data, existing.err
		}
	}
	if len(s.pending) >= 36 {
		s.mu.Unlock()
		return nil, errUnavailable
	}
	read := &pendingRead{done: make(chan struct{})}
	s.pending[a.ID] = read
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	read.data, read.err = s.load(ctx, a)
	s.mu.Lock()
	delete(s.pending, a.ID)
	close(read.done)
	s.mu.Unlock()
	return read.data, read.err
}

func (s *Service) load(ctx context.Context, a *domain.Artifact) ([]byte, error) {
	variant, err := s.find(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	if variant != nil {
		return s.readVariant(ctx, a.ID, variant)
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// A different process may have filled persistent storage while we waited.
	variant, err = s.find(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	if variant != nil {
		return s.readVariant(ctx, a.ID, variant)
	}
	original, err := s.readObject(ctx, a.StorageBucket, a.StorageKey, MaxSourceBytes)
	if err != nil {
		return nil, err
	}
	if int64(len(original)) != a.SizeBytes {
		return nil, errUnavailable
	}
	data, metadata, err := s.renderer.Render(ctx, original, a.MimeType)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > MaxThumbnailBytes || metadata.Width <= 0 || metadata.Width > 640 || metadata.Height <= 0 || metadata.Height > 640 {
		return nil, errUnavailable
	}
	variant, err = s.saver.SaveVariantWithMetadata(ctx, a, domain.VariantThumbnail, "image/webp", data, metadata)
	if err != nil {
		return nil, errUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// In a cross-process race, the repository's unique variant row is canonical.
	return s.readVariant(ctx, a.ID, variant)
}

func (s *Service) find(ctx context.Context, id uuid.UUID) (*domain.ArtifactVariant, error) {
	variants, err := s.repo.ListVariants(ctx, id)
	if err != nil {
		return nil, errUnavailable
	}
	for _, variant := range variants {
		if variant != nil && variant.VariantType == domain.VariantThumbnail {
			return variant, nil
		}
	}
	return nil, nil
}
func (s *Service) readVariant(ctx context.Context, id uuid.UUID, v *domain.ArtifactVariant) ([]byte, error) {
	if v == nil || v.ArtifactID != id || v.MimeType != "image/webp" || v.StorageBucket == "" || v.StorageKey == "" || v.SizeBytes <= 0 || v.SizeBytes > MaxThumbnailBytes || v.Width <= 0 || v.Width > 640 || v.Height <= 0 || v.Height > 640 {
		return nil, errUnavailable
	}
	data, err := s.readObject(ctx, v.StorageBucket, v.StorageKey, MaxThumbnailBytes)
	if err != nil || int64(len(data)) != v.SizeBytes {
		return nil, errUnavailable
	}
	return data, nil
}
func (s *Service) readObject(ctx context.Context, bucket, key string, limit int64) ([]byte, error) {
	var data []byte
	var err error
	if reader, ok := s.objects.(boundedReader); ok {
		data, err = reader.GetObjectBounded(ctx, bucket, key, limit)
	} else {
		data, err = s.objects.GetObject(ctx, bucket, key)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil || len(data) == 0 || int64(len(data)) > limit {
		return nil, errUnavailable
	}
	return data, nil
}
func supportedMime(value string) bool {
	kind, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	switch kind {
	case "image/png", "image/jpeg", "image/webp", "image/avif":
		return true
	}
	return false
}
