package domain

import (
	"context"
	"github.com/google/uuid"
	"strings"
	"time"
)

// AccountSecurityNoticeKind selects a fixed template; caller-provided bodies are forbidden.
type AccountSecurityNoticeKind string

const (
	AccountSecurityNoticePasswordChanged     AccountSecurityNoticeKind = "password_changed"
	AccountSecurityNoticePasswordReset       AccountSecurityNoticeKind = "password_reset"
	AccountSecurityNoticeBackupEmailAdded    AccountSecurityNoticeKind = "backup_email_added"
	AccountSecurityNoticeBackupEmailReplaced AccountSecurityNoticeKind = "backup_email_replaced"
	AccountSecurityNoticeEmailRemoved        AccountSecurityNoticeKind = "email_removed"
	AccountSecurityNoticeRetention                                     = 7 * 24 * time.Hour
	AccountSecurityNoticeSentRetention                                 = 24 * time.Hour
)

func (k AccountSecurityNoticeKind) Valid() bool {
	switch k {
	case AccountSecurityNoticePasswordChanged, AccountSecurityNoticePasswordReset, AccountSecurityNoticeBackupEmailAdded, AccountSecurityNoticeBackupEmailReplaced, AccountSecurityNoticeEmailRemoved:
		return true
	default:
		return false
	}
}

// Recipient is private delivery data and must never appear in logs.
type AccountSecurityNotice struct {
	ID         uuid.UUID
	AccountID  uuid.UUID
	Kind       AccountSecurityNoticeKind
	Recipient  string
	EventAt    time.Time
	Attempts   int
	LeaseToken uuid.UUID
}

// AccountSecurityNoticeRepository fences acknowledgements by the lease token.
type AccountSecurityNoticeRepository interface {
	LeaseAccountSecurityNotices(context.Context, time.Time, int, time.Duration) ([]AccountSecurityNotice, error)
	CompleteAccountSecurityNotice(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	RetryAccountSecurityNotice(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time) error
	CleanupAccountSecurityNotices(context.Context, time.Time, int) error
}
type AccountSecurityNoticeSender interface {
	SendAccountSecurityNotice(context.Context, AccountSecurityNotice) error
}

// UniqueAccountSecurityNoticeRecipients follows normalized email identity semantics.
func UniqueAccountSecurityNoticeRecipients(addresses []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		address = strings.ToLower(strings.TrimSpace(address))
		if address == "" || seen[address] {
			continue
		}
		seen[address] = true
		result = append(result, address)
	}
	return result
}
