package domain

import (
	"context"
	"github.com/google/uuid"
	"time"
)

// AccountRegistrationRepository creates account, verified email, password hash
// and security audit atomically. A retry with the same server-generated account
// ID is idempotent and never changes credentials. Another owner is a conflict.
type AccountRegistrationRepository interface {
	RegisterEmailAccount(context.Context, uuid.UUID, string, string, time.Time) (IdentityResolution, error)
}
