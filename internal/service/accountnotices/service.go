// Package accountnotices dispatches private transactional security notices.
package accountnotices

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"vk-ai-aggregator/internal/domain"
)

const (
	sendTimeout     = 10 * time.Second
	storeTimeout    = 5 * time.Second
	leaseDuration   = time.Minute
	pollInterval    = 5 * time.Second
	cleanupInterval = time.Hour
)

type Service struct {
	repo   domain.AccountSecurityNoticeRepository
	sender domain.AccountSecurityNoticeSender
	logger *slog.Logger
	now    func() time.Time
}

func New(repo domain.AccountSecurityNoticeRepository, sender domain.AccountSecurityNoticeSender, logger *slog.Logger) *Service {
	return &Service{repo: repo, sender: sender, logger: logger, now: time.Now}
}

// DispatchOnce claims one message, bounding processing below the one-minute lease.
// SMTP errors are recorded as retry state, never returned to an account mutation.
func (s *Service) DispatchOnce(ctx context.Context) error {
	if s == nil || s.repo == nil || s.sender == nil {
		return errors.New("security notice dispatcher unavailable")
	}
	storeCtx, cancel := context.WithTimeout(ctx, storeTimeout)
	notices, err := s.repo.LeaseAccountSecurityNotices(storeCtx, s.now(), 1, leaseDuration)
	cancel()
	if err != nil {
		return err
	}
	for _, notice := range notices {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		sendCtx, sendCancel := context.WithTimeout(ctx, sendTimeout)
		sendErr := s.sender.SendAccountSecurityNotice(sendCtx, notice)
		sendCancel()
		at := s.now()
		status := "sent"
		storeCtx, cancel = context.WithTimeout(ctx, storeTimeout)
		if sendErr != nil {
			status = "retry"
			err = s.repo.RetryAccountSecurityNotice(storeCtx, notice.ID, notice.LeaseToken, at, at.Add(retryDelay(notice.Attempts)))
		} else {
			err = s.repo.CompleteAccountSecurityNotice(storeCtx, notice.ID, notice.LeaseToken, at)
		}
		cancel()
		if err != nil {
			status = "ack_failed"
		}
		if s.logger != nil {
			s.logger.Info("account security notice", "notice_id", notice.ID.String(), "event", string(notice.Kind), "status", status)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// CleanupOnce bounds private snapshots to seven days (one day after delivery).
// Bounded sweeps may require multiple passes for an existing backlog.
func (s *Service) CleanupOnce(ctx context.Context) error {
	if s == nil || s.repo == nil {
		return errors.New("security notice retention unavailable")
	}
	storeCtx, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()
	return s.repo.CleanupAccountSecurityNotices(storeCtx, s.now(), 1000)
}

// RunRetention cleans even when SMTP is disabled. It never attempts delivery.
// A healthy runtime sweeps hourly; restart immediately resumes cleanup.
func (s *Service) RunRetention(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		if err := s.CleanupOnce(ctx); err != nil && ctx.Err() == nil && s.logger != nil {
			s.logger.Warn("account security notice retention", "status", "store_unavailable")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func retryDelay(attempt int) time.Duration {
	delay := time.Minute
	for i := 1; i < attempt && delay < time.Hour; i++ {
		delay *= 2
	}
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

// Run uses a single bounded loop. Store errors are deliberately not logged verbatim.
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if err := s.CleanupOnce(ctx); err != nil && ctx.Err() == nil && s.logger != nil {
			s.logger.Warn("account security notice retention", "status", "store_unavailable")
		}
		if err := s.DispatchOnce(ctx); err != nil && ctx.Err() == nil && s.logger != nil {
			s.logger.Warn("account security notice dispatcher", "status", "store_unavailable")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
