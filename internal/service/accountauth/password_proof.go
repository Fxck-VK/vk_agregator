package accountauth

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// PasswordProof is server-captured state for a bounded authentication or recovery
// challenge. It contains private identity/credential data: serialize it only
// inside an authenticated encrypted server token, never as public API JSON.
// The caller owns challenge expiry, purpose binding and atomic replay admission.
type PasswordProof struct {
	EmailBinding     domain.AccountIdentity    `json:"email_binding"`
	ExpectedHash     string                    `json:"expected_hash"`
	Upgrade          *domain.AccountCredential `json:"upgrade,omitempty"`
	PasswordVerified bool                      `json:"password_verified"`
}

func (s *Service) passwordProofDependencies() error {
	if s == nil || s.resolver == nil || s.credentials == nil {
		return ErrPasswordStoreUnavailable
	}
	if _, ok := s.credentials.(domain.AccountPasswordSecurityRepository); !ok {
		return ErrPasswordStoreUnavailable
	}
	if _, ok := s.resolver.(interface {
		PasswordEmailBinding(context.Context, string) (*domain.AccountIdentity, error)
	}); !ok {
		return ErrPasswordStoreUnavailable
	}
	return nil
}

func (s *Service) passwordProofBinding(ctx context.Context, email string) (*domain.AccountIdentity, error) {
	reader := s.resolver.(interface {
		PasswordEmailBinding(context.Context, string) (*domain.AccountIdentity, error)
	})
	binding, err := reader.PasswordEmailBinding(ctx, email)
	if err != nil {
		return nil, passwordProofError(err)
	}
	if binding == nil || binding.VerifiedAt.IsZero() {
		return nil, ErrInvalidPasswordLogin
	}
	return binding, nil
}

// CapturePasswordProof verifies the current password but does not create a
// session, change a legacy credential, or write a successful-login audit yet.
func (s *Service) CapturePasswordProof(ctx context.Context, email, password string) (PasswordProof, error) {
	if err := s.passwordProofDependencies(); err != nil {
		return PasswordProof{}, err
	}
	if err := s.checkLoginMethod(domain.AccountLoginEmailPassword); err != nil {
		return PasswordProof{}, err
	}
	proof, err := s.capturePasswordBinding(ctx, email, "password_login", false)
	if err != nil {
		return PasswordProof{}, err
	}
	valid, rehash, err := verifyPasswordWithUpgrade(password, proof.ExpectedHash)
	if err != nil || !valid {
		return PasswordProof{}, ErrInvalidPasswordLogin
	}
	if rehash {
		upgraded, err := s.newPasswordCredential(proof.EmailBinding.AccountID, password)
		if err != nil {
			return PasswordProof{}, err
		}
		proof.Upgrade = &upgraded
	}
	proof.PasswordVerified = true
	return proof, nil
}

// CaptureRecoveryProof captures a verified email binding and current credential
// version. Only the caller's separate email-ownership challenge authorizes reset.
// Accounts that have no password yet are represented by an empty ExpectedHash.
func (s *Service) CaptureRecoveryProof(ctx context.Context, email string) (PasswordProof, error) {
	if err := s.passwordProofDependencies(); err != nil {
		return PasswordProof{}, err
	}
	return s.capturePasswordBinding(ctx, email, "password_reset", true)
}

func (s *Service) capturePasswordBinding(ctx context.Context, email, rateAction string, allowMissingCredential bool) (PasswordProof, error) {
	_, _, normalized, err := providerIdentity(domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: email, Verified: true})
	if err != nil {
		return PasswordProof{}, ErrInvalidPasswordLogin
	}
	if err := s.checkRateLimit(ctx, rateAction, domain.IdentityProviderEmail, normalized); err != nil {
		return PasswordProof{}, err
	}
	binding, err := s.passwordProofBinding(ctx, normalized)
	if err != nil {
		return PasswordProof{}, passwordProofError(err)
	}
	if binding == nil {
		return PasswordProof{}, ErrInvalidPasswordLogin
	}
	proof := PasswordProof{EmailBinding: *binding}
	stored, err := s.credentials.FindCredential(ctx, binding.AccountID, domain.AccountCredentialPassword)
	if errors.Is(err, domain.ErrNotFound) && allowMissingCredential {
		return proof, nil
	}
	if err != nil {
		return PasswordProof{}, passwordProofError(err)
	}
	if stored == nil || stored.AccountID != binding.AccountID || stored.CredentialType != domain.AccountCredentialPassword || stored.SecretHash == "" {
		return PasswordProof{}, ErrInvalidPasswordLogin
	}
	proof.ExpectedHash = stored.SecretHash
	return proof, nil
}

func validPasswordProof(proof PasswordProof) bool {
	binding := proof.EmailBinding
	if binding.AccountID == uuid.Nil || binding.ID == uuid.Nil || binding.Provider != domain.IdentityProviderEmail || binding.VerifiedAt.IsZero() || binding.UpdatedAt.IsZero() {
		return false
	}
	_, _, normalized, err := providerIdentity(domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: binding.NormalizedID, Verified: true})
	if err != nil || normalized != binding.NormalizedID {
		return false
	}
	if proof.PasswordVerified && proof.ExpectedHash == "" {
		return false
	}
	if proof.Upgrade != nil && (!proof.PasswordVerified || proof.Upgrade.Validate() != nil || proof.Upgrade.AccountID != binding.AccountID || proof.Upgrade.CredentialType != domain.AccountCredentialPassword) {
		return false
	}
	return true
}

// ValidatePasswordProof is a read-only snapshot check for a trusted-browser
// receipt. A subsequent mutation can invalidate it; CompletePasswordProof and
// ResetPasswordProof independently recheck captured versions under the account
// lock before any security operation commits.
func (s *Service) ValidatePasswordProof(ctx context.Context, proof PasswordProof) error {
	if err := s.passwordProofDependencies(); err != nil {
		return err
	}
	if !validPasswordProof(proof) {
		return ErrInvalidPasswordLogin
	}
	current, err := s.passwordProofBinding(ctx, proof.EmailBinding.NormalizedID)
	if err != nil {
		return passwordProofError(err)
	}
	if current == nil || current.ID != proof.EmailBinding.ID || current.AccountID != proof.EmailBinding.AccountID || !current.UpdatedAt.Equal(proof.EmailBinding.UpdatedAt) {
		return ErrInvalidPasswordLogin
	}
	stored, err := s.credentials.FindCredential(ctx, current.AccountID, domain.AccountCredentialPassword)
	if errors.Is(err, domain.ErrNotFound) && proof.ExpectedHash == "" {
		return nil
	}
	if err != nil {
		return passwordProofError(err)
	}
	if stored == nil || stored.AccountID != current.AccountID || stored.CredentialType != domain.AccountCredentialPassword || stored.SecretHash != proof.ExpectedHash {
		return ErrInvalidPasswordLogin
	}
	return nil
}

// CompletePasswordProof rechecks the captured binding/credential and commits
// session issuance, optional legacy upgrade and one audit inside the same lock.
func (s *Service) CompletePasswordProof(ctx context.Context, proof PasswordProof, meta SessionMetadata) (SessionTokens, error) {
	if err := s.passwordProofDependencies(); err != nil {
		return SessionTokens{}, err
	}
	if s.sessions == nil {
		return SessionTokens{}, ErrSessionStoreUnavailable
	}
	if !validPasswordProof(proof) || !proof.PasswordVerified {
		return SessionTokens{}, ErrInvalidPasswordLogin
	}
	if err := s.checkLoginMethod(domain.AccountLoginEmailPassword); err != nil {
		return SessionTokens{}, err
	}
	binding := proof.EmailBinding
	meta.IdentityID = &binding.ID
	session, tokens, err := s.prepareSession(binding.AccountID, meta)
	if err != nil {
		return SessionTokens{}, err
	}
	now := s.currentTime()
	err = s.executePasswordSecurity(ctx, domain.PasswordSecurityOperation{
		AccountID: binding.AccountID, EmailBinding: binding, ExpectedHash: proof.ExpectedHash,
		Credential: proof.Upgrade, Session: &session, At: now,
		Audit: domain.AccountLinkAuditEntry{ID: uuid.New(), AccountID: binding.AccountID, Action: domain.AccountLinkActionLogin, Provider: domain.IdentityProviderEmail, CreatedAt: now},
	})
	if err != nil {
		return SessionTokens{}, passwordProofError(err)
	}
	return tokens, nil
}

// ResetPasswordProof replaces only the credential version captured before the
// recovery email was issued. Password/binding changes invalidate old links;
// resetting also revokes sessions and enqueues the security notice atomically.
func (s *Service) ResetPasswordProof(ctx context.Context, proof PasswordProof, newPassword string) error {
	if err := s.passwordProofDependencies(); err != nil {
		return err
	}
	if s.sessions == nil {
		return ErrSessionStoreUnavailable
	}
	if !validPasswordProof(proof) {
		return ErrInvalidPasswordLogin
	}
	credential, err := s.newPasswordCredential(proof.EmailBinding.AccountID, newPassword)
	if err != nil {
		return err
	}
	now := s.currentTime()
	err = s.executePasswordSecurity(ctx, domain.PasswordSecurityOperation{
		AccountID: proof.EmailBinding.AccountID, EmailBinding: proof.EmailBinding, ExpectedHash: proof.ExpectedHash,
		Credential: &credential, Reset: true, RequireCredentialMatch: true, At: now,
		Audit: domain.AccountLinkAuditEntry{ID: uuid.New(), AccountID: proof.EmailBinding.AccountID, Action: domain.AccountLinkActionPasswordReset, Provider: domain.IdentityProviderEmail, CreatedAt: now},
	})
	return passwordProofError(err)
}

func passwordProofError(err error) error {
	if errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnverifiedLogin) {
		return ErrInvalidPasswordLogin
	}
	return err
}
