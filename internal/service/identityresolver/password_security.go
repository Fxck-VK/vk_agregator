package identityresolver

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
)

func (s *Service) PasswordEmailBinding(ctx context.Context, email string) (*domain.AccountIdentity, error) {
	if s == nil || s.identities == nil {
		return nil, domain.ErrNotFound
	}
	return s.identities.ResolveIdentity(ctx, domain.IdentityProviderEmail, email)
}

func (s *Service) EnqueueAccountSecurityNotifications(ctx context.Context, accountID uuid.UUID, kind domain.AccountSecurityNoticeKind, recipients []string, at time.Time) error {
	writer, ok := s.identities.(interface {
		EnqueueAccountSecurityNotifications(context.Context, uuid.UUID, domain.AccountSecurityNoticeKind, []string, time.Time) error
	})
	if !ok {
		return errors.New("identityresolver: security notice transaction unavailable")
	}
	return writer.EnqueueAccountSecurityNotifications(ctx, accountID, kind, recipients, at)
}

func (s *Service) WithPasswordEmailLock(ctx context.Context, binding domain.AccountIdentity, action func(context.Context) error) error {
	guard, ok := s.identities.(domain.PasswordEmailGuard)
	if !ok {
		return errors.New("identityresolver: password transaction unavailable")
	}
	return guard.WithPasswordEmailLock(ctx, binding, action)
}
