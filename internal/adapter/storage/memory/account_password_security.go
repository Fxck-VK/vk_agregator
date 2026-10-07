package memory

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

// ExecutePasswordSecurity holds identity, credential, session and audit locks
// through validation and commits only operations that cannot subsequently fail.
func (r *AccountSecurityRepo) ExecutePasswordSecurity(ctx context.Context, op domain.PasswordSecurityOperation, deps domain.PasswordSecurityDependencies) error {
	if err := op.Validate(); err != nil {
		return err
	}
	if deps.EmailGuard == nil {
		return errors.New("memory: password email guard unavailable")
	}
	sessions, sessionOK := deps.Sessions.(*AccountSessionRepo)
	if deps.Sessions != nil && !sessionOK {
		return errors.New("memory: session transaction unavailable")
	}
	if op.Session != nil && sessions == nil {
		return errors.New("memory: session transaction unavailable")
	}
	audit := r
	if deps.Audit != nil {
		var ok bool
		audit, ok = deps.Audit.(*AccountSecurityRepo)
		if !ok {
			return errors.New("memory: audit transaction unavailable")
		}
	}
	return deps.EmailGuard.WithPasswordEmailLock(ctx, op.EmailBinding, func(lockedCtx context.Context) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		if sessions != nil {
			sessions.mu.Lock()
			defer sessions.mu.Unlock()
		}
		if audit != r {
			audit.mu.Lock()
			defer audit.mu.Unlock()
		}
		stored := r.credentials[accountCredentialKey(op.AccountID, domain.AccountCredentialPassword)]
		if !op.Reset && ((op.ExpectedHash == "" && stored != nil) || (op.ExpectedHash != "" && (stored == nil || stored.SecretHash != op.ExpectedHash))) {
			return domain.ErrConflict
		}
		if op.PreserveSessionID != uuid.Nil {
			if sessions == nil {
				return domain.ErrNotFound
			}
			current := sessions.byID[op.PreserveSessionID]
			if current == nil || current.AccountID != op.AccountID || current.RevokedAt != nil || !current.ExpiresAt.After(op.At) || current.AccessExpiresAt == nil || !current.AccessExpiresAt.After(op.At) {
				return domain.ErrNotFound
			}
		}
		if op.Session != nil {
			if _, exists := sessions.byID[op.Session.ID]; exists {
				return domain.ErrConflict
			}
			if _, exists := sessions.byRefreshHash[op.Session.RefreshTokenHash]; exists {
				return domain.ErrConflict
			}
			if _, exists := sessions.byAccessHash[op.Session.AccessTokenHash]; exists {
				return domain.ErrConflict
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if op.Audit.Action != domain.AccountLinkActionLogin {
			if deps.EnqueueNotice == nil {
				return errors.New("memory: security notice transaction unavailable")
			}
			kind := domain.AccountSecurityNoticePasswordChanged
			if op.Reset {
				kind = domain.AccountSecurityNoticePasswordReset
			}
			if err := deps.EnqueueNotice(lockedCtx, op.AccountID, kind, nil, op.At); err != nil {
				return err
			}
		}
		if op.Credential != nil {
			r.upsertCredentialLocked(*op.Credential)
		}
		if op.Audit.Action != domain.AccountLinkActionLogin && sessions != nil {
			for _, session := range sessions.byID {
				if session.AccountID == op.AccountID && session.RevokedAt == nil && session.ID != op.PreserveSessionID {
					sessions.revokeLocked(session, op.At)
				}
			}
		}
		if op.Session != nil {
			if _, err := sessions.createSessionLocked(*op.Session); err != nil {
				panic("validated memory session failed")
			}
		}
		audit.audits = append(audit.audits, op.Audit)
		return nil
	})
}
