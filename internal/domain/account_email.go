package domain

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

const MaxAccountEmails = 2

type AccountEmailRole string

const (
	AccountEmailPrimary    AccountEmailRole = "primary"
	AccountEmailBackup     AccountEmailRole = "backup"
	AccountEmailAdditional AccountEmailRole = "additional"
)

var (
	ErrAccountEmailLimit         = errors.New("domain: account email limit reached")
	ErrAccountBackupEmailChanged = errors.New("domain: backup email changed")
	ErrAccountEmailAlreadyLinked = errors.New("domain: email already linked")
)

// AccountEmailRoles derives stable roles without deleting legacy bindings.
func AccountEmailRoles(rows []*AccountIdentity) map[uuid.UUID]AccountEmailRole {
	emails := make([]*AccountIdentity, 0)
	for _, row := range rows {
		if row != nil && row.Provider == IdentityProviderEmail && !row.VerifiedAt.IsZero() {
			emails = append(emails, row)
		}
	}
	sort.Slice(emails, func(i, j int) bool {
		if emails[i].CreatedAt.Equal(emails[j].CreatedAt) {
			return emails[i].ID.String() < emails[j].ID.String()
		}
		return emails[i].CreatedAt.Before(emails[j].CreatedAt)
	})
	roles := make(map[uuid.UUID]AccountEmailRole, len(emails))
	for i, row := range emails {
		role := AccountEmailAdditional
		if i == 0 {
			role = AccountEmailPrimary
		}
		if i == 1 {
			role = AccountEmailBackup
		}
		roles[row.ID] = role
	}
	return roles
}

// AccountBackupEmailRepository must check role and version under the same lock
// as link/unlink, and replace the binding together with its audit records.
type AccountBackupEmailRepository interface {
	ReplaceBackupEmailIdentity(context.Context, uuid.UUID, uuid.UUID, string, string, time.Time) (*AccountIdentity, error)
}
