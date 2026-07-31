package domain

import "github.com/google/uuid"

// AuthenticationMethod identifies the credential kind that established a
// request principal.
type AuthenticationMethod string

const AuthenticationMethodAccountSession AuthenticationMethod = "account_session"

// RequestPrincipal is the canonical authenticated account for a request.
type RequestPrincipal struct {
	AccountID uuid.UUID
	SessionID uuid.UUID
	Method    AuthenticationMethod
}

// Validate ensures the principal can authorize account-owned operations.
func (p RequestPrincipal) Validate() error {
	if p.AccountID == uuid.Nil || p.SessionID == uuid.Nil ||
		p.Method != AuthenticationMethodAccountSession {
		return ErrInvalidIdentity
	}
	return nil
}
