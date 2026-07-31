package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// AccountCredentialType names a verifier stored for future account login.
type AccountCredentialType string

const (
	AccountCredentialPassword AccountCredentialType = "password"
	AccountCredentialOTP      AccountCredentialType = "otp"
	AccountCredentialPasskey  AccountCredentialType = "passkey"
)

// AccountLinkAction names an audited identity/account link operation.
type AccountLinkAction string

const (
	AccountLinkActionLinked         AccountLinkAction = "linked"
	AccountLinkActionUnlinked       AccountLinkAction = "unlinked"
	AccountLinkActionLogin          AccountLinkAction = "login"
	AccountLinkActionMergeRequested AccountLinkAction = "merge_requested"
	AccountLinkActionMergeCompleted AccountLinkAction = "merge_completed"
	AccountLinkActionPasswordSet    AccountLinkAction = "password_set"
	AccountLinkActionPasswordReset  AccountLinkAction = "password_reset"
)

// AccountSession stores only hashed access/refresh-token and device/network material.
type AccountSession struct {
	ID               uuid.UUID  `json:"id"`
	AccountID        uuid.UUID  `json:"account_id"`
	IdentityID       *uuid.UUID `json:"identity_id,omitempty"`
	AccessTokenHash  string     `json:"access_token_hash,omitempty"`
	AccessExpiresAt  *time.Time `json:"access_expires_at,omitempty"`
	RefreshTokenHash string     `json:"refresh_token_hash"`
	DeviceID         string     `json:"device_id"`
	IPHash           string     `json:"ip_hash"`
	UserAgentHash    string     `json:"user_agent_hash"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Validate enforces the hash-only session contract.
func (s AccountSession) Validate() error {
	if s.AccountID == uuid.Nil ||
		strings.TrimSpace(s.RefreshTokenHash) == "" ||
		s.ExpiresAt.IsZero() {
		return ErrInvalidIdentity
	}
	accessHash := strings.TrimSpace(s.AccessTokenHash)
	if s.AccessTokenHash != "" && accessHash == "" {
		return ErrInvalidIdentity
	}
	accessHashPresent := accessHash != ""
	accessExpiryPresent := s.AccessExpiresAt != nil && !s.AccessExpiresAt.IsZero()
	if accessHashPresent != accessExpiryPresent {
		return ErrInvalidIdentity
	}
	if accessExpiryPresent && s.AccessExpiresAt.After(s.ExpiresAt) {
		return ErrInvalidIdentity
	}
	return nil
}

// AccountCredential stores only verifier material, never raw passwords or OTPs.
type AccountCredential struct {
	ID             uuid.UUID             `json:"id"`
	AccountID      uuid.UUID             `json:"account_id"`
	CredentialType AccountCredentialType `json:"credential_type"`
	SecretHash     string                `json:"secret_hash"`
	ChangedAt      *time.Time            `json:"changed_at,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

// Validate enforces the hash-only credential contract.
func (c AccountCredential) Validate() error {
	switch c.CredentialType {
	case AccountCredentialPassword, AccountCredentialOTP, AccountCredentialPasskey:
	default:
		return ErrInvalidIdentity
	}
	if c.AccountID == uuid.Nil || strings.TrimSpace(c.SecretHash) == "" {
		return ErrInvalidIdentity
	}
	return nil
}

// AccountLinkAuditEntry is a PII-free link/unlink/login/merge audit record.
type AccountLinkAuditEntry struct {
	ID             uuid.UUID         `json:"id"`
	AccountID      uuid.UUID         `json:"account_id"`
	ActorAccountID *uuid.UUID        `json:"actor_account_id,omitempty"`
	Action         AccountLinkAction `json:"action"`
	Provider       IdentityProvider  `json:"provider,omitempty"`
	IdentityID     *uuid.UUID        `json:"identity_id,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// AccountIdentitySafeRef is a public-safe identity reference without external
// ids, normalized ids, tokens or contact values.
type AccountIdentitySafeRef struct {
	ID         uuid.UUID        `json:"id"`
	AccountID  uuid.UUID        `json:"account_id"`
	Provider   IdentityProvider `json:"provider"`
	Verified   bool             `json:"verified"`
	LastUsedAt *time.Time       `json:"last_used_at,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

// SafeRef returns a PII-free identity DTO for APIs and logs.
func (i AccountIdentity) SafeRef() AccountIdentitySafeRef {
	ref := AccountIdentitySafeRef{
		ID:        i.ID,
		AccountID: i.AccountID,
		Provider:  i.Provider,
		Verified:  !i.VerifiedAt.IsZero(),
		CreatedAt: i.CreatedAt,
	}
	if !i.LastUsedAt.IsZero() {
		lastUsedAt := i.LastUsedAt
		ref.LastUsedAt = &lastUsedAt
	}
	return ref
}
