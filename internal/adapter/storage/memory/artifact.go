package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ArtifactRepo is an in-memory domain.ArtifactRepository.
type ArtifactRepo struct {
	mu       sync.Mutex
	byID     map[uuid.UUID]domain.Artifact
	bySHA    map[string]uuid.UUID
	variants map[uuid.UUID][]domain.ArtifactVariant
}

// NewArtifactRepo builds an empty ArtifactRepo.
func NewArtifactRepo() *ArtifactRepo {
	return &ArtifactRepo{
		byID:     map[uuid.UUID]domain.Artifact{},
		bySHA:    map[string]uuid.UUID{},
		variants: map[uuid.UUID][]domain.ArtifactVariant{},
	}
}

var _ domain.ArtifactRepository = (*ArtifactRepo)(nil)

func shaKey(ownerID uuid.UUID, sha string) string { return ownerID.String() + "|" + sha }

func (r *ArtifactRepo) Create(_ context.Context, a *domain.Artifact) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	normalizeArtifactMetadata(a)
	now := time.Now()
	a.CreatedAt, a.UpdatedAt = now, now
	r.byID[a.ID] = *a
	if a.SHA256 != "" {
		r.bySHA[shaKey(a.OwnerUserID, a.SHA256)] = a.ID
	}
	return nil
}

func (r *ArtifactRepo) Update(_ context.Context, a *domain.Artifact) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.byID[a.ID]
	if !ok {
		return domain.ErrNotFound
	}
	normalizeArtifactMetadata(a)
	a.CreatedAt = cur.CreatedAt
	a.UpdatedAt = time.Now()
	r.byID[a.ID] = *a
	return nil
}

func (r *ArtifactRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Artifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &a, nil
}

func (r *ArtifactRepo) GetBySHA256(_ context.Context, ownerID uuid.UUID, sha256 string) (*domain.Artifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.bySHA[shaKey(ownerID, sha256)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	a := r.byID[id]
	return &a, nil
}

func (r *ArtifactRepo) FindReusableInputReference(_ context.Context, ownerID uuid.UUID, sha256, validationPolicyVersion, mimeType string) (*domain.Artifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, artifact := range r.byID {
		if artifact.OwnerUserID == ownerID &&
			artifact.SHA256 == sha256 &&
			artifact.ValidationPolicyVersion == validationPolicyVersion &&
			artifact.LifecycleClass == domain.ArtifactLifecycleInputReference &&
			artifact.Kind == domain.ArtifactKindInput &&
			artifact.MediaType == domain.MediaTypeImage &&
			artifact.MimeType == mimeType &&
			artifact.Status == domain.ArtifactStatusReady &&
			artifact.StorageBucket != "" &&
			artifact.StorageKey != "" {
			a := artifact
			return &a, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *ArtifactRepo) AddVariant(_ context.Context, v *domain.ArtifactVariant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	for i := range r.variants[v.ArtifactID] {
		if r.variants[v.ArtifactID][i].VariantType == v.VariantType {
			return domain.ErrConflict
		}
	}
	normalizeArtifactVariantMetadata(v)
	now := time.Now()
	v.CreatedAt, v.UpdatedAt = now, now
	r.variants[v.ArtifactID] = append(r.variants[v.ArtifactID], *v)
	return nil
}

func (r *ArtifactRepo) ListVariants(_ context.Context, artifactID uuid.UUID) ([]*domain.ArtifactVariant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.ArtifactVariant
	for i := range r.variants[artifactID] {
		v := r.variants[artifactID][i]
		out = append(out, &v)
	}
	return out, nil
}

func normalizeArtifactMetadata(a *domain.Artifact) {
	m := domain.ArtifactMediaMetadata{
		Width:       a.Width,
		Height:      a.Height,
		DurationMS:  a.DurationMS,
		Codec:       a.Codec,
		Container:   a.Container,
		BitrateBPS:  a.BitrateBPS,
		ProbeStatus: a.ProbeStatus,
	}.Normalize()
	a.Width = m.Width
	a.Height = m.Height
	a.DurationMS = m.DurationMS
	a.Codec = m.Codec
	a.Container = m.Container
	a.BitrateBPS = m.BitrateBPS
	a.ProbeStatus = m.ProbeStatus
	a.LifecycleClass = domain.NormalizeArtifactLifecycleClass(a.LifecycleClass, a.Kind, a.MediaType, a.Status)
}

func normalizeArtifactVariantMetadata(v *domain.ArtifactVariant) {
	m := domain.ArtifactMediaMetadata{
		Width:       v.Width,
		Height:      v.Height,
		DurationMS:  v.DurationMS,
		Codec:       v.Codec,
		Container:   v.Container,
		BitrateBPS:  v.BitrateBPS,
		ProbeStatus: v.ProbeStatus,
	}.Normalize()
	v.Width = m.Width
	v.Height = m.Height
	v.DurationMS = m.DurationMS
	v.Codec = m.Codec
	v.Container = m.Container
	v.BitrateBPS = m.BitrateBPS
	v.ProbeStatus = m.ProbeStatus
	if !v.LifecycleClass.Valid() {
		v.LifecycleClass = domain.ArtifactLifecycleDeliveryVariant
	}
}

// ObjectStore is an in-memory object store satisfying the artifact service's
// ObjectStore contract. Stored objects are kept in a map for assertions.
type ObjectStore struct {
	mu      sync.Mutex
	objects map[string]storedObject
}

type storedObject struct {
	Data        []byte
	ContentType string
}

// NewObjectStore builds an empty in-memory object store.
func NewObjectStore() *ObjectStore {
	return &ObjectStore{objects: map[string]storedObject{}}
}

// Put stores the bytes under bucket/key.
func (s *ObjectStore) Put(_ context.Context, bucket, key string, data []byte, contentType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	s.objects[bucket+"/"+key] = storedObject{Data: cp, ContentType: contentType}
	return nil
}

// Get returns the stored object and whether it exists.
func (s *ObjectStore) Get(bucket, key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	obj, ok := s.objects[bucket+"/"+key]
	return obj.Data, ok
}

// GetObject returns the stored bytes, or domain.ErrNotFound if absent. It
// satisfies the object-fetch contract the delivery worker depends on.
func (s *ObjectStore) GetObject(_ context.Context, bucket, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	obj, ok := s.objects[bucket+"/"+key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := make([]byte, len(obj.Data))
	copy(cp, obj.Data)
	return cp, nil
}

// DeleteObject removes a stored object. Missing objects are treated as already
// deleted so cleanup remains idempotent.
func (s *ObjectStore) DeleteObject(_ context.Context, bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, bucket+"/"+key)
	return nil
}

// Len returns the number of stored objects.
func (s *ObjectStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.objects)
}
