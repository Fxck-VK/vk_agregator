package postgres

import (
	"context"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
)

// EnqueueAccountSecurityNotifications runs in the account mutation transaction,
// holding its account row lock. Extras are previously verified removed addresses.
func EnqueueAccountSecurityNotifications(ctx context.Context, db Querier, accountID uuid.UUID, kind domain.AccountSecurityNoticeKind, extraRecipients []string, at time.Time) error {
	if db == nil || accountID == uuid.Nil || !kind.Valid() || at.IsZero() {
		return domain.ErrInvalidIdentity
	}
	rows, err := db.Query(ctx, `SELECT DISTINCT lower(trim(normalized_id)) FROM account_identities WHERE account_id=$1 AND provider='email' AND verified_at IS NOT NULL`, accountID)
	if err != nil {
		return mapError(err)
	}
	recipients := make([]string, 0)
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			rows.Close()
			return mapError(err)
		}
		recipients = append(recipients, address)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return mapError(err)
	}
	recipients = append(recipients, extraRecipients...)
	for _, recipient := range domain.UniqueAccountSecurityNoticeRecipients(recipients) {
		if _, err := db.Exec(ctx, `INSERT INTO account_security_notices(id,account_id,kind,recipient,event_at,next_attempt_at) VALUES($1,$2,$3,$4,$5,$5)`, uuid.New(), accountID, kind, recipient, at); err != nil {
			return mapError(err)
		}
	}
	return nil
}

type AccountSecurityNoticeRepository struct{ db Querier }

func NewAccountSecurityNoticeRepository(db Querier) *AccountSecurityNoticeRepository {
	return &AccountSecurityNoticeRepository{db: db}
}

var _ domain.AccountSecurityNoticeRepository = (*AccountSecurityNoticeRepository)(nil)

func (r *AccountSecurityNoticeRepository) LeaseAccountSecurityNotices(ctx context.Context, at time.Time, limit int, lease time.Duration) ([]domain.AccountSecurityNotice, error) {
	if limit < 1 || limit > 100 || lease <= 0 || lease > 10*time.Minute {
		return nil, domain.ErrInvalidIdentity
	}
	rows, err := r.db.Query(ctx, `WITH due AS (
 SELECT id FROM account_security_notices WHERE sent_at IS NULL AND next_attempt_at <= $1
 AND event_at > $4 AND (lease_until IS NULL OR lease_until <= $1)
 ORDER BY next_attempt_at,id LIMIT $2 FOR UPDATE SKIP LOCKED
 ) UPDATE account_security_notices n SET lease_token=$3,lease_until=$5,attempts=n.attempts+1
 FROM due WHERE n.id=due.id RETURNING n.id,n.account_id,n.kind,n.recipient,n.event_at,n.attempts,n.lease_token`, at, limit, uuid.New(), at.Add(-domain.AccountSecurityNoticeRetention), at.Add(lease))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	notices := make([]domain.AccountSecurityNotice, 0)
	for rows.Next() {
		var notice domain.AccountSecurityNotice
		if err := rows.Scan(&notice.ID, &notice.AccountID, &notice.Kind, &notice.Recipient, &notice.EventAt, &notice.Attempts, &notice.LeaseToken); err != nil {
			return nil, mapError(err)
		}
		notices = append(notices, notice)
	}
	return notices, mapError(rows.Err())
}
func (r *AccountSecurityNoticeRepository) CompleteAccountSecurityNotice(ctx context.Context, id, token uuid.UUID, at time.Time) error {
	tag, err := r.db.Exec(ctx, `UPDATE account_security_notices SET sent_at=$3,lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2 AND lease_until>$3 AND sent_at IS NULL`, id, token, at)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrConflict
	}
	return nil
}
func (r *AccountSecurityNoticeRepository) RetryAccountSecurityNotice(ctx context.Context, id, token uuid.UUID, at, next time.Time) error {
	tag, err := r.db.Exec(ctx, `UPDATE account_security_notices SET next_attempt_at=$4,lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2 AND lease_until>$3 AND sent_at IS NULL`, id, token, at, next)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrConflict
	}
	return nil
}
func (r *AccountSecurityNoticeRepository) CleanupAccountSecurityNotices(ctx context.Context, at time.Time, limit int) error {
	if limit < 1 || limit > 1000 {
		return domain.ErrInvalidIdentity
	}
	_, err := r.db.Exec(ctx, `WITH expired AS (SELECT id FROM account_security_notices WHERE
 (event_at <= $1 OR sent_at <= $2) AND (lease_until IS NULL OR lease_until <= $3)
 ORDER BY event_at LIMIT $4 FOR UPDATE SKIP LOCKED)
 DELETE FROM account_security_notices n USING expired WHERE n.id=expired.id`, at.Add(-domain.AccountSecurityNoticeRetention), at.Add(-domain.AccountSecurityNoticeSentRetention), at, limit)
	return mapError(err)
}
