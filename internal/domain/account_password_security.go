package domain

import (
	"context"
	"github.com/google/uuid"
	"time"
)

// PasswordSecurityOperation contains only server-captured verification state.
// Empty ExpectedHash means insert-only setup; Reset deliberately ignores it.
type PasswordSecurityOperation struct {
	AccountID         uuid.UUID
	EmailBinding      AccountIdentity
	ExpectedHash      string
	Credential        *AccountCredential
	Session           *AccountSession
	PreserveSessionID uuid.UUID
	Reset             bool
	Audit             AccountLinkAuditEntry
	At                time.Time
}

type PasswordEmailGuard interface {
	WithPasswordEmailLock(context.Context, AccountIdentity, func(context.Context) error) error
}

// PasswordSecurityDependencies are used by the mock transaction coordinator.
// Durable adapters own the transaction and all repositories inside it.
type PasswordSecurityDependencies struct {
	EmailGuard    PasswordEmailGuard
	Sessions      AccountSessionRepository
	Audit         AccountLinkAuditRepository
	EnqueueNotice func(context.Context, uuid.UUID, AccountSecurityNoticeKind, []string, time.Time) error
}

// AccountPasswordSecurityRepository linearizes verified binding/credential
// checks, credential writes, audit, session issuance/revocation and notices.
type AccountPasswordSecurityRepository interface {
	ExecutePasswordSecurity(context.Context, PasswordSecurityOperation, PasswordSecurityDependencies) error
}

func (op PasswordSecurityOperation) Validate() error {
	if op.AccountID == uuid.Nil || op.EmailBinding.AccountID != op.AccountID || op.EmailBinding.ID == uuid.Nil || op.EmailBinding.Provider != IdentityProviderEmail || op.EmailBinding.VerifiedAt.IsZero() || op.At.IsZero() || op.Audit.AccountID != op.AccountID {
		return ErrInvalidIdentity
	}
	if op.Credential != nil && (op.Credential.Validate() != nil || op.Credential.AccountID != op.AccountID || op.Credential.CredentialType != AccountCredentialPassword) {
		return ErrInvalidIdentity
	}
	if op.Session != nil && (op.Session.Validate() != nil || op.Session.AccountID != op.AccountID || op.Audit.Action != AccountLinkActionLogin) {
		return ErrInvalidIdentity
	}
	if op.Reset && (op.Credential == nil || op.PreserveSessionID != uuid.Nil || op.Audit.Action != AccountLinkActionPasswordReset) {
		return ErrInvalidIdentity
	}
	switch op.Audit.Action {
	case AccountLinkActionLogin:
		if op.ExpectedHash == "" || op.Reset || op.PreserveSessionID != uuid.Nil {
			return ErrInvalidIdentity
		}
	case AccountLinkActionPasswordSet:
		if op.Credential == nil || op.Session != nil || op.Reset {
			return ErrInvalidIdentity
		}
	case AccountLinkActionPasswordReset:
		if !op.Reset || op.Session != nil {
			return ErrInvalidIdentity
		}
	default:
		return ErrInvalidIdentity
	}
	if op.Session != nil && op.Session.ID == uuid.Nil {
		return ErrInvalidIdentity
	}
	return nil
}
