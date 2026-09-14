package accountservice

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

// ErrReceiptEmailUnavailable means the account has no usable verified email.
var ErrReceiptEmailUnavailable = errors.New("accountservice: verified receipt email unavailable")

// VerifiedReceiptEmail resolves a receipt contact for the server-side payment
// flow only. Never expose its result in profile DTOs, browser responses or logs.
func (s *Service) VerifiedReceiptEmail(ctx context.Context, accountID uuid.UUID) (string, error) {
	if s == nil || s.identities == nil {
		return "", errMissingDependency
	}
	if accountID == uuid.Nil {
		return "", domain.ErrInvalidIdentity
	}
	for offset := 0; ; offset += maxIdentityLimit {
		rows, err := s.identities.ListIdentitiesByAccount(ctx, accountID, maxIdentityLimit, offset)
		if err != nil {
			return "", err
		}
		for _, identity := range rows {
			if identity == nil || identity.AccountID != accountID || identity.VerifiedAt.IsZero() || domain.NormalizeIdentityProvider(identity.Provider) != domain.IdentityProviderEmail {
				continue
			}
			email := strings.TrimSpace(identity.NormalizedID)
			address, err := mail.ParseAddress(email)
			if err == nil && address.Address == email && len(email) <= 254 {
				return email, nil
			}
		}
		if len(rows) < maxIdentityLimit {
			return "", ErrReceiptEmailUnavailable
		}
	}
}
