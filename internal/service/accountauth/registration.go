package accountauth

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func WithRegistrationRepository(repo domain.AccountRegistrationRepository) Option {
	return func(s *Service) { s.registration = repo }
}

// ValidatePassword is shared by proof-based registration and password storage.
// Validate before consuming proof, so invalid input cannot destroy a valid code.
func ValidatePassword(password string) error { return validatePassword(password) }

// RegisterVerifiedEmailPassword is an internal Account Layer operation. The
// caller must verify email ownership; neither Verified nor accountID comes from
// a public client. Repository retries preserve the original password.
func (s *Service) RegisterVerifiedEmailPassword(ctx context.Context, accountID uuid.UUID, email, password string) (domain.IdentityResolution, error) {
	if s == nil || s.registration == nil {
		return domain.IdentityResolution{}, errors.New("accountauth: registration unavailable")
	}
	if accountID == uuid.Nil {
		return domain.IdentityResolution{}, domain.ErrInvalidIdentity
	}
	_, _, normalizedEmail, err := providerIdentity(domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: email, Verified: true})
	if err != nil {
		return domain.IdentityResolution{}, err
	}
	if err := s.checkRateLimit(ctx, "registration", domain.IdentityProviderEmail, normalizedEmail); err != nil {
		return domain.IdentityResolution{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return domain.IdentityResolution{}, err
	}
	return s.registration.RegisterEmailAccount(ctx, accountID, normalizedEmail, hash, s.currentTime())
}
