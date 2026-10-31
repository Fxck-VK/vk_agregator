package postgres

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
)

var _ domain.SubscriptionRepository = (*BillingRepository)(nil)

func (r *BillingRepository) GetSubscription(ctx context.Context, owner uuid.UUID) (*domain.AccountSubscription, error) {
	var raw []byte
	var revision int64
	if err := r.q().QueryRow(ctx, `SELECT state,revision FROM account_subscriptions WHERE owner_account_id=$1`, owner).Scan(&raw, &revision); err != nil {
		return nil, mapError(err)
	}
	var s domain.AccountSubscription
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	s.AccountID = owner
	s.Revision = revision
	return &s, nil
}
func (r *BillingRepository) LockSubscription(ctx context.Context, owner uuid.UUID) (*domain.AccountSubscription, error) {
	if owner == uuid.Nil {
		return nil, domain.ErrConflict
	}
	// Account lock also serializes the first row creation; identity ownership is
	// always the canonical account, never a legacy user or browser field.
	var id uuid.UUID
	if err := r.q().QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, owner).Scan(&id); err != nil {
		return nil, mapError(err)
	}
	if _, err := r.q().Exec(ctx, `INSERT INTO account_subscriptions(owner_account_id,state) VALUES($1,'{}') ON CONFLICT DO NOTHING`, owner); err != nil {
		return nil, mapError(err)
	}
	var raw []byte
	var revision int64
	if err := r.q().QueryRow(ctx, `SELECT state,revision FROM account_subscriptions WHERE owner_account_id=$1 FOR UPDATE`, owner).Scan(&raw, &revision); err != nil {
		return nil, mapError(err)
	}
	var s domain.AccountSubscription
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	s.AccountID = owner
	s.Revision = revision
	return &s, nil
}
func (r *BillingRepository) SaveSubscription(ctx context.Context, s *domain.AccountSubscription) error {
	if s == nil || s.AccountID == uuid.Nil {
		return domain.ErrConflict
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var due *time.Time
	if s.AutoRenew && !s.PaidUntil.IsZero() {
		d := s.PaidUntil
		if s.NextAttemptAt.After(d) {
			d = s.NextAttemptAt
		}
		due = &d
	}
	tag, err := r.q().Exec(ctx, `UPDATE account_subscriptions SET state=$2,revision=revision+1,due_at=$3,updated_at=now() WHERE owner_account_id=$1 AND revision=$4`, s.AccountID, raw, due, s.Revision)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrConflict
	}
	s.Revision++
	return nil
}
func (r *BillingRepository) ListDueSubscriptions(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := r.q().Query(ctx, `SELECT owner_account_id FROM account_subscriptions WHERE due_at<=$1 ORDER BY due_at,owner_account_id LIMIT $2`, now, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapError(err)
		}
		out = append(out, id)
	}
	return out, mapError(rows.Err())
}
