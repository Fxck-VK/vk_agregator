package artifactdeletion

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

type repository struct {
	candidates []domain.MediaCleanupCandidate
	err        error
	marked     int
	owner, id  uuid.UUID
}

func (r *repository) RequestOutputDeletionForAccount(_ context.Context, owner, id uuid.UUID) ([]domain.MediaCleanupCandidate, error) {
	r.owner, r.id = owner, id
	return r.candidates, r.err
}
func (r *repository) MarkMediaCleanupDeleted(_ context.Context, _ domain.MediaCleanupCandidate) error {
	r.marked++
	return nil
}

type failingObjects struct{}

func (failingObjects) DeleteObject(context.Context, string, string) error {
	return errors.New("temporary storage failure")
}

func TestPurgeDeletesOriginalAndVariantsAfterOwnedTombstone(t *testing.T) {
	ctx := context.Background()
	objects := memory.NewObjectStore()
	owner, id := uuid.New(), uuid.New()
	repo := &repository{candidates: []domain.MediaCleanupCandidate{{ArtifactID: id, StorageBucket: "private", StorageKey: "original"}, {ArtifactID: id, StorageBucket: "private", StorageKey: "thumbnail"}}}
	for _, item := range repo.candidates {
		_ = objects.Put(ctx, item.StorageBucket, item.StorageKey, []byte("bytes"), "image/png")
	}
	if err := New(repo, repo, objects).Delete(ctx, owner, id); err != nil {
		t.Fatal(err)
	}
	if repo.owner != owner || repo.id != id || repo.marked != 2 || objects.Len() != 0 {
		t.Fatal("owner scope or physical purge failed")
	}
}
func TestStorageFailureLeavesDurableCandidatesForRetry(t *testing.T) {
	repo := &repository{candidates: []domain.MediaCleanupCandidate{{StorageBucket: "private", StorageKey: "original"}}}
	if err := New(repo, repo, failingObjects{}).Delete(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal("accepted tombstone must survive cleanup failure", err)
	}
	if repo.marked != 0 {
		t.Fatal("failed physical deletion marked as purged")
	}
}
func TestForeignAndBusyArtifactsNeverReachStorage(t *testing.T) {
	for _, err := range []error{domain.ErrNotFound, domain.ErrConflict} {
		repo := &repository{err: err}
		objects := memory.NewObjectStore()
		if got := New(repo, repo, objects).Delete(context.Background(), uuid.New(), uuid.New()); !errors.Is(got, err) {
			t.Fatal(got)
		}
		if repo.marked != 0 {
			t.Fatal("foreign/busy file changed")
		}
	}
}
