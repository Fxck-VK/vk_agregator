package accountauth

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
)

func (s *Service) passwordEmailBinding(ctx context.Context, email string) (*domain.AccountIdentity, error) {
	reader, ok := s.resolver.(interface {
		PasswordEmailBinding(context.Context, string) (*domain.AccountIdentity, error)
	})
	if !ok {
		return nil, ErrPasswordStoreUnavailable
	}
	binding, err := reader.PasswordEmailBinding(ctx, email)
	if err != nil {
		return nil, err
	}
	if binding.VerifiedAt.IsZero() {
		return nil, domain.ErrUnverifiedLogin
	}
	return binding, nil
}

func (s *Service) executePasswordSecurity(ctx context.Context, op domain.PasswordSecurityOperation) error {
	writer, ok := s.credentials.(domain.AccountPasswordSecurityRepository)
	if !ok {
		return ErrPasswordStoreUnavailable
	}
	guard, _ := s.resolver.(domain.PasswordEmailGuard)
	deps := domain.PasswordSecurityDependencies{EmailGuard: guard, Sessions: s.sessions, Audit: s.audit}
	if notices, ok := s.resolver.(interface {
		EnqueueAccountSecurityNotifications(context.Context, uuid.UUID, domain.AccountSecurityNoticeKind, []string, time.Time) error
	}); ok {
		deps.EnqueueNotice = notices.EnqueueAccountSecurityNotifications
	}
	return writer.ExecutePasswordSecurity(ctx, op, deps)
}

func (s *Service) commitPasswordMutation(ctx context.Context, credential domain.AccountCredential, binding domain.AccountIdentity, expected string, preserve uuid.UUID, reset bool, actor *uuid.UUID) error {
	if binding.AccountID != credential.AccountID {
		return domain.ErrAccountIdentityOwnershipRequired
	}
	action := domain.AccountLinkActionPasswordSet
	if reset {
		action = domain.AccountLinkActionPasswordReset
	}
	now := s.currentTime()
	return s.executePasswordSecurity(ctx, domain.PasswordSecurityOperation{AccountID: credential.AccountID, EmailBinding: binding, ExpectedHash: expected, Credential: &credential, PreserveSessionID: preserve, Reset: reset, At: now, Audit: domain.AccountLinkAuditEntry{ID: uuid.New(), AccountID: credential.AccountID, ActorAccountID: actor, Action: action, Provider: domain.IdentityProviderEmail, CreatedAt: now}})
}

// AuthenticateEmailPasswordSession verifies a captured credential/binding again
// under the account lock and commits the new session and audit inside it.
func (s *Service) AuthenticateEmailPasswordSession(ctx context.Context, email, password string, meta SessionMetadata) (SessionTokens, error) {
	if s == nil || s.credentials == nil {
		return SessionTokens{}, ErrPasswordStoreUnavailable
	}
	if s.sessions == nil {
		return SessionTokens{}, ErrSessionStoreUnavailable
	}
	_, _, normalized, err := providerIdentity(domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: email, Verified: true})
	if err != nil {
		return SessionTokens{}, ErrInvalidPasswordLogin
	}
	if err := s.checkRateLimit(ctx, "password_login", domain.IdentityProviderEmail, normalized); err != nil {
		return SessionTokens{}, err
	}
	binding, err := s.passwordEmailBinding(ctx, normalized)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnverifiedLogin) {
		return SessionTokens{}, ErrInvalidPasswordLogin
	}
	if err != nil {
		return SessionTokens{}, err
	}
	stored, err := s.credentials.FindCredential(ctx, binding.AccountID, domain.AccountCredentialPassword)
	if errors.Is(err, domain.ErrNotFound) {
		return SessionTokens{}, ErrInvalidPasswordLogin
	}
	if err != nil {
		return SessionTokens{}, err
	}
	valid, rehash, err := verifyPasswordWithUpgrade(password, stored.SecretHash)
	if err != nil || !valid {
		return SessionTokens{}, ErrInvalidPasswordLogin
	}
	var credential *domain.AccountCredential
	if rehash {
		upgraded, err := s.newPasswordCredential(binding.AccountID, password)
		if err != nil {
			return SessionTokens{}, err
		}
		credential = &upgraded
	}
	meta.IdentityID = &binding.ID
	session, tokens, err := s.prepareSession(binding.AccountID, meta)
	if err != nil {
		return SessionTokens{}, err
	}
	now := s.currentTime()
	err = s.executePasswordSecurity(ctx, domain.PasswordSecurityOperation{AccountID: binding.AccountID, EmailBinding: *binding, ExpectedHash: stored.SecretHash, Credential: credential, Session: &session, At: now, Audit: domain.AccountLinkAuditEntry{ID: uuid.New(), AccountID: binding.AccountID, Action: domain.AccountLinkActionLogin, Provider: domain.IdentityProviderEmail, CreatedAt: now}})
	if errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnverifiedLogin) {
		return SessionTokens{}, ErrInvalidPasswordLogin
	}
	if err != nil {
		return SessionTokens{}, err
	}
	return tokens, nil
}
