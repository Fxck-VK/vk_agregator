// Package artifactdeletion revokes account-owned output access before purging
// bytes. Existing expired-media maintenance retries any unconfirmed deletion.
package artifactdeletion

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

type Repository interface {
	RequestOutputDeletionForAccount(context.Context, uuid.UUID, uuid.UUID) ([]domain.MediaCleanupCandidate, error)
}
type CleanupRecorder interface {
	MarkMediaCleanupDeleted(context.Context, domain.MediaCleanupCandidate) error
}
type ObjectStore interface {
	DeleteObject(context.Context, string, string) error
}
type Service struct {
	repo    Repository
	cleanup CleanupRecorder
	objects ObjectStore
}

func New(repo Repository, cleanup CleanupRecorder, objects ObjectStore) *Service {
	return &Service{repo, cleanup, objects}
}
func (s *Service) Delete(ctx context.Context, owner, id uuid.UUID) error {
	if owner == uuid.Nil || id == uuid.Nil {
		return domain.ErrNotFound
	}
	if s.repo == nil || s.cleanup == nil || s.objects == nil {
		return errors.New("artifact deletion unavailable")
	}
	candidates, err := s.repo.RequestOutputDeletionForAccount(ctx, owner, id)
	if err != nil {
		return err
	}
	// The committed DB expiration is the durable queue. A cancelled request or
	// failed store operation must never restore access or lose the cleanup task.
	cleanupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for _, candidate := range candidates {
		if cleanupCtx.Err() != nil {
			break
		}
		if candidate.StorageBucket == "" || candidate.StorageKey == "" {
			continue
		}
		if s.objects.DeleteObject(cleanupCtx, candidate.StorageBucket, candidate.StorageKey) != nil {
			continue
		}
		_ = s.cleanup.MarkMediaCleanupDeleted(cleanupCtx, candidate)
	}
	return nil
}
