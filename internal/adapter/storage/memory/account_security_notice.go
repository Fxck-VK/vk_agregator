package memory

import (
	"context"
	"github.com/google/uuid"
	"sort"
	"time"
	"vk-ai-aggregator/internal/domain"
)

type noticeIdentityLockKey struct{}
type memorySecurityNotice struct {
	notice                 domain.AccountSecurityNotice
	next, leaseUntil, sent time.Time
}

// EnqueueAccountSecurityNotifications requires the mock account identity transaction.
func (r *AccountIdentityRepo) EnqueueAccountSecurityNotifications(ctx context.Context, accountID uuid.UUID, kind domain.AccountSecurityNoticeKind, extras []string, at time.Time) error {
	if ctx.Value(noticeIdentityLockKey{}) != r || accountID == uuid.Nil || !kind.Valid() || at.IsZero() {
		return domain.ErrConflict
	}
	r.enqueueSecurityNoticesLocked(accountID, kind, extras, at)
	return nil
}
func (r *AccountIdentityRepo) enqueueSecurityNoticesLocked(accountID uuid.UUID, kind domain.AccountSecurityNoticeKind, extras []string, at time.Time) {
	recipients := append([]string(nil), extras...)
	for _, identity := range r.byID {
		if identity.AccountID == accountID && identity.Provider == domain.IdentityProviderEmail && !identity.VerifiedAt.IsZero() {
			recipients = append(recipients, identity.NormalizedID)
		}
	}
	if r.notices == nil {
		r.notices = make(map[uuid.UUID]*memorySecurityNotice)
	}
	for _, recipient := range domain.UniqueAccountSecurityNoticeRecipients(recipients) {
		id := uuid.New()
		r.notices[id] = &memorySecurityNotice{notice: domain.AccountSecurityNotice{ID: id, AccountID: accountID, Kind: kind, Recipient: recipient, EventAt: at}, next: at}
	}
}

// SecurityNotices returns private notice snapshots only for local tests, never logging.
func (r *AccountIdentityRepo) SecurityNotices() []domain.AccountSecurityNotice {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.AccountSecurityNotice, 0, len(r.notices))
	for _, row := range r.notices {
		out = append(out, row.notice)
	}
	return out
}
func (r *AccountIdentityRepo) LeaseAccountSecurityNotices(_ context.Context, at time.Time, limit int, lease time.Duration) ([]domain.AccountSecurityNotice, error) {
	if limit < 1 || limit > 100 || lease <= 0 || lease > 10*time.Minute {
		return nil, domain.ErrInvalidIdentity
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	due := make([]*memorySecurityNotice, 0)
	for _, row := range r.notices {
		if row.sent.IsZero() && !row.next.After(at) && !row.leaseUntil.After(at) && row.notice.EventAt.After(at.Add(-domain.AccountSecurityNoticeRetention)) {
			due = append(due, row)
		}
	}
	sort.Slice(due, func(i, j int) bool { return due[i].next.Before(due[j].next) })
	if len(due) > limit {
		due = due[:limit]
	}
	out := make([]domain.AccountSecurityNotice, 0, len(due))
	for _, row := range due {
		row.notice.LeaseToken = uuid.New()
		row.leaseUntil = at.Add(lease)
		row.notice.Attempts++
		out = append(out, row.notice)
	}
	return out, nil
}
func (r *AccountIdentityRepo) CompleteAccountSecurityNotice(_ context.Context, id, token uuid.UUID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.notices[id]
	if row == nil || row.notice.LeaseToken != token || !row.leaseUntil.After(at) || !row.sent.IsZero() {
		return domain.ErrConflict
	}
	row.sent = at
	row.notice.LeaseToken = uuid.Nil
	row.leaseUntil = time.Time{}
	return nil
}
func (r *AccountIdentityRepo) RetryAccountSecurityNotice(_ context.Context, id, token uuid.UUID, at, next time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.notices[id]
	if row == nil || row.notice.LeaseToken != token || !row.leaseUntil.After(at) || !row.sent.IsZero() {
		return domain.ErrConflict
	}
	row.next = next
	row.notice.LeaseToken = uuid.Nil
	row.leaseUntil = time.Time{}
	return nil
}
func (r *AccountIdentityRepo) CleanupAccountSecurityNotices(_ context.Context, at time.Time, limit int) error {
	if limit < 1 || limit > 1000 {
		return domain.ErrInvalidIdentity
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, row := range r.notices {
		if limit == 0 {
			break
		}
		if !row.leaseUntil.After(at) && (!row.notice.EventAt.After(at.Add(-domain.AccountSecurityNoticeRetention)) || (!row.sent.IsZero() && !row.sent.After(at.Add(-domain.AccountSecurityNoticeSentRetention)))) {
			delete(r.notices, id)
			limit--
		}
	}
	return nil
}
