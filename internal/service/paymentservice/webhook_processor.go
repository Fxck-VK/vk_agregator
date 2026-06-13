package paymentservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/service/billingservice"
)

var (
	ErrWebhookInvalid     = errors.New("paymentservice: invalid webhook")
	ErrWebhookUnverified  = errors.New("paymentservice: webhook provider state is not verified")
	ErrWebhookMismatch    = errors.New("paymentservice: webhook provider state mismatch")
	ErrWebhookUnsupported = errors.New("paymentservice: unsupported webhook event")
	ErrRefundNotAllowed   = errors.New("paymentservice: refund is not allowed")
	ErrRefundCreditsSpent = errors.New("paymentservice: refund credits are already spent")
	ErrRefundMismatch     = errors.New("paymentservice: refund provider response mismatch")
)

// PaymentTxRunner executes webhook state changes inside one transaction-bound
// payment/billing repository pair.
type PaymentTxRunner interface {
	RunPaymentTx(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error
}

// TxRunnerFunc adapts a function into a PaymentTxRunner.
type TxRunnerFunc func(context.Context, func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error

// RunPaymentTx executes f.
func (f TxRunnerFunc) RunPaymentTx(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
	return f(ctx, fn)
}

// WebhookProcessor owns provider webhook inbox processing. It never trusts a
// webhook body as proof of payment; every payment event is verified through the
// provider API before ledger mutation.
type WebhookProcessor struct {
	repo     domain.PaymentRepository
	provider domain.PaymentProvider
	billing  *billingservice.Service
	tx       PaymentTxRunner
	now      func() time.Time
}

// NewWebhookProcessor builds a webhook processor.
func NewWebhookProcessor(repo domain.PaymentRepository, provider domain.PaymentProvider, billing *billingservice.Service, tx PaymentTxRunner) *WebhookProcessor {
	return &WebhookProcessor{
		repo:     repo,
		provider: provider,
		billing:  billing,
		tx:       tx,
		now:      time.Now,
	}
}

// IngestWebhook parses a raw provider webhook and stores it in payment_events.
// Duplicate dedup keys are accepted as no-op so providers can retry safely.
func (p *WebhookProcessor) IngestWebhook(ctx context.Context, raw []byte, headers http.Header) (*domain.PaymentEvent, bool, error) {
	if p == nil || p.repo == nil || p.provider == nil {
		return nil, false, errors.New("paymentservice: webhook processor is not configured")
	}
	if len(raw) == 0 {
		return nil, false, ErrWebhookInvalid
	}
	normalized, err := p.provider.ParseWebhook(ctx, raw, headers)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v", ErrWebhookInvalid, err)
	}
	if normalized.Provider == "" {
		normalized.Provider = p.provider.Code()
	}
	if normalized.Provider != p.provider.Code() {
		return nil, false, fmt.Errorf("%w: provider %s", ErrWebhookInvalid, normalized.Provider)
	}
	if strings.TrimSpace(normalized.EventType) == "" || strings.TrimSpace(normalized.DedupKey) == "" {
		return nil, false, ErrWebhookInvalid
	}
	if strings.TrimSpace(normalized.ProviderPaymentID) == "" && strings.TrimSpace(normalized.ProviderRefundID) == "" {
		return nil, false, ErrWebhookInvalid
	}
	payload := normalized.Payload
	if len(payload) == 0 {
		payload = append(json.RawMessage(nil), raw...)
	}
	event := &domain.PaymentEvent{
		Provider:          normalized.Provider,
		EventType:         normalized.EventType,
		ProviderPaymentID: strings.TrimSpace(normalized.ProviderPaymentID),
		ProviderRefundID:  strings.TrimSpace(normalized.ProviderRefundID),
		DedupKey:          strings.TrimSpace(normalized.DedupKey),
		Payload:           payload,
	}
	created, err := p.repo.CreateEvent(ctx, event)
	if err != nil {
		return nil, false, err
	}
	result := "duplicate"
	if created {
		result = "created"
	}
	metrics.PaymentWebhooks.WithLabelValues(string(event.Provider), event.EventType, result).Inc()
	return event, created, nil
}

// ProcessBatch processes up to limit unprocessed events for this provider.
func (p *WebhookProcessor) ProcessBatch(ctx context.Context, limit int) (int, error) {
	if p == nil || p.repo == nil || p.provider == nil {
		return 0, errors.New("paymentservice: webhook processor is not configured")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	events, err := p.repo.ListUnprocessedEvents(ctx, p.provider.Code(), limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	var firstErr error
	for _, event := range events {
		if err := p.ProcessEvent(ctx, event); err != nil {
			metrics.PaymentWebhookProcessingErrors.WithLabelValues(string(p.provider.Code()), webhookProcessingStage(err)).Inc()
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		processed++
	}
	return processed, firstErr
}

// InboxStats returns and observes the current unprocessed webhook inbox backlog
// for this processor's provider.
func (p *WebhookProcessor) InboxStats(ctx context.Context) (domain.PaymentWebhookInboxStats, error) {
	if p == nil || p.repo == nil || p.provider == nil {
		return domain.PaymentWebhookInboxStats{}, errors.New("paymentservice: webhook processor is not configured")
	}
	stats, err := p.repo.WebhookInboxStats(ctx, p.provider.Code())
	if err != nil {
		return domain.PaymentWebhookInboxStats{}, err
	}
	if stats.Provider == "" {
		stats.Provider = p.provider.Code()
	}
	p.observeInboxStats(stats)
	return stats, nil
}

// ProcessEvent verifies and applies one stored webhook event.
func (p *WebhookProcessor) ProcessEvent(ctx context.Context, event *domain.PaymentEvent) error {
	if p == nil || p.repo == nil || p.provider == nil || p.billing == nil || p.tx == nil {
		return errors.New("paymentservice: webhook processor is not configured")
	}
	if event == nil || event.ID == (domain.PaymentEvent{}).ID {
		return ErrWebhookInvalid
	}
	if event.ProcessedAt != nil {
		return nil
	}
	if event.Provider != p.provider.Code() {
		return fmt.Errorf("%w: event provider %s", ErrWebhookInvalid, event.Provider)
	}
	if strings.TrimSpace(event.ProviderPaymentID) == "" {
		return ErrWebhookUnsupported
	}

	providerPayment, err := p.verifiedProviderPayment(ctx, event.ProviderPaymentID)
	if err != nil {
		return err
	}

	return p.tx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billingRepo domain.BillingRepository) error {
		currentEvent, err := payments.GetEventByID(ctx, event.ID)
		if err != nil {
			return err
		}
		if currentEvent.ProcessedAt != nil {
			return nil
		}
		intent, err := payments.GetIntentByProviderPaymentID(ctx, event.Provider, event.ProviderPaymentID)
		if err != nil {
			return err
		}

		if isRefundWebhookEvent(event.EventType) {
			// Refund accounting requires a product policy about spent credits and
			// lot/FIFO attribution. Chapter 6 only inboxes and verifies refund
			// events; balance mutation is intentionally left to a later refund
			// processor.
			return payments.MarkEventProcessed(ctx, event.ID, p.now())
		}
		if _, err := p.applyProviderPayment(ctx, payments, billingRepo, intent, providerPayment); err != nil {
			return err
		}
		return payments.MarkEventProcessed(ctx, event.ID, p.now())
	})
}

// SyncIntent manually syncs one payment intent against provider state. It is a
// protected operator action and uses the same verified path as webhook
// processing.
func (p *WebhookProcessor) SyncIntent(ctx context.Context, intentID uuid.UUID) (*domain.PaymentIntent, error) {
	if p == nil || p.repo == nil || p.provider == nil || p.tx == nil {
		return nil, errors.New("paymentservice: webhook processor is not configured")
	}
	intent, err := p.repo.GetIntentByID(ctx, intentID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(intent.ProviderPaymentID) == "" {
		return nil, ErrWebhookUnsupported
	}
	providerPayment, err := p.verifiedProviderPayment(ctx, intent.ProviderPaymentID)
	if err != nil {
		return nil, err
	}
	if err := p.tx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billingRepo domain.BillingRepository) error {
		current, err := payments.GetIntentByID(ctx, intentID)
		if err != nil {
			return err
		}
		_, err = p.applyProviderPayment(ctx, payments, billingRepo, current, providerPayment)
		return err
	}); err != nil {
		return nil, err
	}
	return p.repo.GetIntentByID(ctx, intentID)
}

// CancelIntent requests provider-side cancellation and then verifies the
// resulting state through the same sync path used by webhook/reconciliation.
// This is a protected operator action; it must not be exposed to user-facing
// surfaces.
func (p *WebhookProcessor) CancelIntent(ctx context.Context, intentID uuid.UUID) (*domain.PaymentIntent, error) {
	if p == nil || p.repo == nil || p.provider == nil || p.tx == nil {
		return nil, errors.New("paymentservice: webhook processor is not configured")
	}
	intent, err := p.repo.GetIntentByID(ctx, intentID)
	if err != nil {
		return nil, err
	}
	if intent.Status == domain.PaymentIntentCanceled ||
		intent.Status == domain.PaymentIntentExpired ||
		intent.Status == domain.PaymentIntentFailed {
		return intent, nil
	}
	if intent.Status == domain.PaymentIntentSucceeded ||
		intent.Status == domain.PaymentIntentRefunded ||
		intent.Status == domain.PaymentIntentPartiallyRefunded {
		return nil, domain.ErrConflict
	}
	if strings.TrimSpace(intent.ProviderPaymentID) == "" {
		return nil, ErrWebhookUnsupported
	}
	if err := p.provider.CancelPayment(ctx, intent.ProviderPaymentID); err != nil {
		recordPaymentProviderError(p.provider.Code(), "cancel_payment", err)
		return nil, fmt.Errorf("%w: cancel provider payment: %v", ErrWebhookUnverified, err)
	}
	return p.SyncIntent(ctx, intentID)
}

// ReconciliationResult summarizes one payment reconciliation pass.
type ReconciliationResult struct {
	Checked    int
	Processed  int
	Mismatches int
}

// ReconcilePending syncs stale pending/waiting intents against the provider.
func (p *WebhookProcessor) ReconcilePending(ctx context.Context, limit int) (ReconciliationResult, error) {
	return p.ReconcilePendingOlderThan(ctx, limit, 30*time.Second)
}

// ReconcilePendingOlderThan syncs pending/waiting intents older than staleAfter.
func (p *WebhookProcessor) ReconcilePendingOlderThan(ctx context.Context, limit int, staleAfter time.Duration) (ReconciliationResult, error) {
	if p == nil || p.repo == nil || p.provider == nil {
		return ReconciliationResult{}, errors.New("paymentservice: webhook processor is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if staleAfter == 0 {
		staleAfter = 30 * time.Second
	}
	intents, err := p.repo.ListIntentsForReconciliation(ctx, domain.PaymentReconciliationFilter{
		Provider: p.provider.Code(),
		Statuses: []domain.PaymentIntentStatus{
			domain.PaymentIntentProviderPending,
			domain.PaymentIntentWaitingForUser,
		},
		UpdatedBefore: p.now().Add(-staleAfter),
	}, limit)
	if err != nil {
		return ReconciliationResult{}, err
	}
	result := ReconciliationResult{Checked: len(intents)}
	var firstErr error
	for _, intent := range intents {
		if _, err := p.SyncIntent(ctx, intent.ID); err != nil {
			if errors.Is(err, ErrWebhookMismatch) {
				result.Mismatches++
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		result.Processed++
	}
	metrics.PaymentReconciliationMismatches.WithLabelValues(string(p.provider.Code())).Set(float64(result.Mismatches))
	return result, firstErr
}

// RefundIntentInput describes a protected manual full-refund request.
type RefundIntentInput struct {
	IntentID       uuid.UUID
	IdempotencyKey string
	Reason         string
}

// RefundIntentResult is returned by manual operator refund.
type RefundIntentResult struct {
	Intent *domain.PaymentIntent
	Refund *domain.PaymentRefund
}

const (
	refundLedgerScanPageSize   = 200
	refundLedgerScanMaxEntries = 5000
)

// RefundIntent performs a manual full refund. It refuses to request money back
// when the user's current balance cannot cover the purchased credits.
func (p *WebhookProcessor) RefundIntent(ctx context.Context, in RefundIntentInput) (RefundIntentResult, error) {
	if p == nil || p.repo == nil || p.provider == nil || p.tx == nil {
		return RefundIntentResult{}, errors.New("paymentservice: webhook processor is not configured")
	}
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	in.Reason = strings.TrimSpace(in.Reason)
	if in.IntentID == uuid.Nil || in.IdempotencyKey == "" {
		return RefundIntentResult{}, ErrInvalidInput
	}
	if existing, err := p.repo.GetRefundByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		if existing.IntentID != in.IntentID {
			return RefundIntentResult{}, ErrForbidden
		}
		intent, getErr := p.repo.GetIntentByID(ctx, existing.IntentID)
		if getErr != nil {
			return RefundIntentResult{}, getErr
		}
		return RefundIntentResult{Intent: intent, Refund: existing}, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return RefundIntentResult{}, err
	}

	var refund *domain.PaymentRefund
	if err := p.tx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billingRepo domain.BillingRepository) error {
		intent, err := payments.GetIntentByID(ctx, in.IntentID)
		if err != nil {
			return err
		}
		if intent.Status != domain.PaymentIntentSucceeded {
			return ErrRefundNotAllowed
		}
		if strings.TrimSpace(intent.ProviderPaymentID) == "" {
			return ErrRefundNotAllowed
		}
		account, err := billingRepo.GetAccountByUser(ctx, intent.UserID, domain.CurrencyCredits)
		if err != nil {
			return err
		}
		if account.BalanceCached < intent.Credits {
			return ErrRefundCreditsSpent
		}
		if err := ensureTopupCreditsRefundable(ctx, billingRepo, account.ID, intent); err != nil {
			return err
		}
		refund = &domain.PaymentRefund{
			IntentID:       intent.ID,
			Amount:         intent.Amount,
			Status:         domain.PaymentRefundProviderPending,
			IdempotencyKey: in.IdempotencyKey,
			Reason:         defaultString(in.Reason, "manual payment refund"),
		}
		if err := payments.CreateRefund(ctx, refund); err != nil {
			return err
		}
		return billingRepo.AppendEntry(ctx, &domain.LedgerEntry{
			AccountID:      account.ID,
			Type:           domain.LedgerAdjustment,
			Amount:         -intent.Credits,
			Status:         domain.LedgerStatusCommitted,
			IdempotencyKey: refundDebitLedgerKey(intent.Provider, intent.ProviderPaymentID, refund.ID),
			Reason:         "payment refund debit",
		})
	}); err != nil {
		return RefundIntentResult{}, err
	}

	intent, err := p.repo.GetIntentByID(ctx, in.IntentID)
	if err != nil {
		return RefundIntentResult{}, err
	}
	providerRefund, err := p.provider.CreateRefund(ctx, domain.CreateRefundInput{
		RefundID:          refund.ID,
		IntentID:          intent.ID,
		ProviderPaymentID: intent.ProviderPaymentID,
		Amount:            intent.Amount,
		Currency:          intent.Currency,
		Description:       paymentIntentDescription(intent, nil),
		Reason:            in.Reason,
		ReceiptEmail:      intent.ReceiptEmail,
		ReceiptPhone:      intent.ReceiptPhone,
		VATCode:           paymentIntentVATCode(intent, nil),
		PaymentSubject:    paymentIntentSubject(intent, nil),
		PaymentMode:       paymentIntentMode(intent, nil),
		IdempotencyKey:    "payrefund:" + refund.ID.String(),
	})
	if err != nil {
		recordPaymentProviderError(p.provider.Code(), "create_refund", err)
		metrics.PaymentRefunds.WithLabelValues(string(p.provider.Code()), "provider_error").Inc()
		if compErr := p.compensateFailedRefund(ctx, intent, refund); compErr != nil {
			metrics.PaymentRefunds.WithLabelValues(string(p.provider.Code()), "rollback_failed").Inc()
			return RefundIntentResult{}, fmt.Errorf("paymentservice: refund provider failed and rollback failed: %w: %v", err, compErr)
		}
		metrics.PaymentRefunds.WithLabelValues(string(p.provider.Code()), "rollback_succeeded").Inc()
		return RefundIntentResult{}, err
	}
	if providerRefund.Amount != intent.Amount || providerRefund.Currency != intent.Currency {
		err := fmt.Errorf("%w: amount/currency", ErrRefundMismatch)
		recordPaymentProviderError(p.provider.Code(), "create_refund", err)
		metrics.PaymentRefunds.WithLabelValues(string(p.provider.Code()), "provider_mismatch").Inc()
		if compErr := p.compensateFailedRefund(ctx, intent, refund); compErr != nil {
			metrics.PaymentRefunds.WithLabelValues(string(p.provider.Code()), "rollback_failed").Inc()
			return RefundIntentResult{}, fmt.Errorf("paymentservice: refund provider mismatch and rollback failed: %w: %v", err, compErr)
		}
		metrics.PaymentRefunds.WithLabelValues(string(p.provider.Code()), "rollback_succeeded").Inc()
		return RefundIntentResult{}, err
	}
	if err := p.tx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billingRepo domain.BillingRepository) error {
		if err := payments.SetRefundProviderState(ctx, refund.ID, providerRefund.ProviderRefundID, providerRefund.Status); err != nil {
			return err
		}
		if providerRefund.Status == domain.PaymentRefundSucceeded {
			current, err := payments.GetIntentByID(ctx, intent.ID)
			if err != nil {
				return err
			}
			if current.Status == domain.PaymentIntentSucceeded {
				if err := payments.UpdateIntentStatus(ctx, current.ID, current.Status, domain.PaymentIntentRefunded); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return RefundIntentResult{}, err
	}
	updatedRefund, err := p.repo.GetRefundByIdempotencyKey(ctx, in.IdempotencyKey)
	if err != nil {
		return RefundIntentResult{}, err
	}
	updatedIntent, err := p.repo.GetIntentByID(ctx, intent.ID)
	if err != nil {
		return RefundIntentResult{}, err
	}
	metrics.PaymentRefunds.WithLabelValues(string(p.provider.Code()), string(updatedRefund.Status)).Inc()
	return RefundIntentResult{Intent: updatedIntent, Refund: updatedRefund}, nil
}

func webhookProcessingStage(err error) string {
	switch {
	case errors.Is(err, ErrWebhookInvalid):
		return "invalid"
	case errors.Is(err, ErrWebhookUnsupported):
		return "unsupported"
	case errors.Is(err, ErrWebhookUnverified):
		return "provider_unverified"
	case errors.Is(err, ErrWebhookMismatch):
		return "provider_mismatch"
	default:
		return "processing"
	}
}

func (p *WebhookProcessor) compensateFailedRefund(ctx context.Context, intent *domain.PaymentIntent, refund *domain.PaymentRefund) error {
	return p.tx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billingRepo domain.BillingRepository) error {
		account, err := billingRepo.GetAccountByUser(ctx, intent.UserID, domain.CurrencyCredits)
		if err != nil {
			return err
		}
		if err := billingRepo.AppendEntry(ctx, &domain.LedgerEntry{
			AccountID:      account.ID,
			Type:           domain.LedgerAdjustment,
			Amount:         intent.Credits,
			Status:         domain.LedgerStatusCommitted,
			IdempotencyKey: refundCompensateLedgerKey(refund.ID),
			Reason:         "payment refund provider failure compensation",
		}); err != nil && !errors.Is(err, domain.ErrConflict) {
			return err
		}
		return payments.SetRefundProviderState(ctx, refund.ID, "", domain.PaymentRefundFailed)
	})
}

func ensureTopupCreditsRefundable(ctx context.Context, repo domain.BillingRepository, accountID uuid.UUID, intent *domain.PaymentIntent) error {
	if intent == nil || strings.TrimSpace(intent.ProviderPaymentID) == "" || intent.Credits <= 0 {
		return ErrRefundNotAllowed
	}
	topupKey := topUpLedgerKey(intent.Provider, intent.ProviderPaymentID)
	scanned := 0
	for offset := 0; scanned < refundLedgerScanMaxEntries; offset += refundLedgerScanPageSize {
		limit := refundLedgerScanPageSize
		if remaining := refundLedgerScanMaxEntries - scanned; remaining < limit {
			limit = remaining
		}
		entries, err := repo.ListEntries(ctx, accountID, limit, offset)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return ErrRefundNotAllowed
		}
		for _, entry := range entries {
			if entry == nil {
				continue
			}
			if entry.IdempotencyKey == topupKey {
				if entry.Type != domain.LedgerTopup ||
					entry.Status != domain.LedgerStatusCommitted ||
					entry.Amount != intent.Credits {
					return ErrRefundNotAllowed
				}
				return nil
			}
			if entry.Amount < 0 && (entry.Status == domain.LedgerStatusCommitted || entry.Status == domain.LedgerStatusPending) {
				return ErrRefundCreditsSpent
			}
		}
		scanned += len(entries)
		if len(entries) < limit {
			return ErrRefundNotAllowed
		}
	}
	return ErrRefundNotAllowed
}

func (p *WebhookProcessor) verifiedProviderPayment(ctx context.Context, providerPaymentID string) (domain.ProviderPayment, error) {
	providerPayment, err := p.provider.GetPayment(ctx, providerPaymentID)
	if err != nil {
		recordPaymentProviderError(p.provider.Code(), "get_payment", err)
		return domain.ProviderPayment{}, fmt.Errorf("%w: get provider payment: %v", ErrWebhookUnverified, err)
	}
	if strings.TrimSpace(providerPayment.ProviderPaymentID) != strings.TrimSpace(providerPaymentID) {
		return domain.ProviderPayment{}, fmt.Errorf("%w: provider payment id", ErrWebhookMismatch)
	}
	if providerPayment.Status == "" || !providerPayment.Status.Valid() {
		return domain.ProviderPayment{}, fmt.Errorf("%w: provider status %q", ErrWebhookMismatch, providerPayment.Status)
	}
	return providerPayment, nil
}

func (p *WebhookProcessor) observeInboxStats(stats domain.PaymentWebhookInboxStats) {
	provider := stats.Provider
	if provider == "" && p != nil && p.provider != nil {
		provider = p.provider.Code()
	}
	if provider == "" {
		provider = "unknown"
	}
	metrics.PaymentWebhookUnprocessedEvents.WithLabelValues(string(provider)).Set(float64(stats.UnprocessedEvents))
	ageSeconds := 0.0
	if stats.UnprocessedEvents > 0 && stats.OldestUnprocessedReceivedAt != nil {
		age := p.now().Sub(*stats.OldestUnprocessedReceivedAt)
		if age > 0 {
			ageSeconds = age.Seconds()
		}
	}
	metrics.PaymentWebhookOldestUnprocessedAgeSeconds.WithLabelValues(string(provider)).Set(ageSeconds)
}

type applyResult struct {
	StatusChanged bool
	TopupGranted  bool
}

func (p *WebhookProcessor) applyProviderPayment(ctx context.Context, payments domain.PaymentRepository, billingRepo domain.BillingRepository, intent *domain.PaymentIntent, providerPayment domain.ProviderPayment) (applyResult, error) {
	target := providerPayment.Status
	transitionAllowed := intent.Status == target || intent.Status.CanTransitionTo(target)
	if !transitionAllowed {
		return applyResult{}, nil
	}
	if target == domain.PaymentIntentSucceeded {
		if !providerPayment.Paid || !providerPayment.Captured {
			return applyResult{}, fmt.Errorf("%w: succeeded payment is not paid/captured", ErrWebhookUnverified)
		}
		if providerPayment.Amount != intent.Amount || providerPayment.Currency != intent.Currency {
			return applyResult{}, fmt.Errorf("%w: amount/currency", ErrWebhookMismatch)
		}
	}
	var result applyResult
	if intent.Status != target {
		if err := payments.UpdateIntentStatus(ctx, intent.ID, intent.Status, target); err != nil {
			return result, err
		}
		result.StatusChanged = true
		switch target {
		case domain.PaymentIntentSucceeded:
			metrics.PaymentsSucceeded.WithLabelValues(string(intent.Provider), paymentSource(intent)).Inc()
		case domain.PaymentIntentCanceled:
			metrics.PaymentsCanceled.WithLabelValues(string(intent.Provider)).Inc()
		}
	}
	if target == domain.PaymentIntentSucceeded {
		if err := p.billing.GrantWith(
			ctx,
			billingRepo,
			intent.UserID,
			intent.Credits,
			topUpLedgerKey(intent.Provider, intent.ProviderPaymentID),
			"payment top-up via "+string(intent.Provider),
		); err != nil {
			return result, err
		}
		if result.StatusChanged {
			result.TopupGranted = true
			metrics.PaymentTopups.WithLabelValues(string(intent.Provider)).Inc()
			metrics.LedgerEntries.WithLabelValues(string(domain.LedgerTopup), paymentSource(intent)).Inc()
			metrics.ObserveProductEvent(paymentSource(intent), "payment", "ledger_topup", "top_up", "credits", "success")
			metrics.AddProductCreditsFlow(paymentSource(intent), "topup", "success", intent.Credits)
			if !intent.CreatedAt.IsZero() {
				duration := p.now().Sub(intent.CreatedAt)
				if duration > 0 {
					metrics.PaymentToLedgerDuration.WithLabelValues(string(intent.Provider)).Observe(duration.Seconds())
				}
			}
		}
	}
	return result, nil
}

func isRefundWebhookEvent(eventType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(eventType)), "refund.")
}

func topUpLedgerKey(provider domain.PaymentProviderCode, providerPaymentID string) string {
	return "topup:" + string(provider) + ":" + strings.TrimSpace(providerPaymentID)
}

func refundDebitLedgerKey(provider domain.PaymentProviderCode, providerPaymentID string, refundID uuid.UUID) string {
	return "payment_refund_debit:" + string(provider) + ":" + strings.TrimSpace(providerPaymentID) + ":" + refundID.String()
}

func refundCompensateLedgerKey(refundID uuid.UUID) string {
	return "payment_refund_compensate:" + refundID.String()
}

func paymentSource(intent *domain.PaymentIntent) string {
	if intent == nil || len(intent.Metadata) == 0 {
		return "unknown"
	}
	var metadata struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(intent.Metadata, &metadata); err != nil {
		return "unknown"
	}
	return metricLabel(metadata.Source)
}

func recordPaymentProviderError(provider domain.PaymentProviderCode, operation string, err error) {
	if provider == "" {
		provider = "unknown"
	}
	if strings.TrimSpace(operation) == "" {
		operation = "unknown"
	}
	metrics.PaymentProviderErrors.WithLabelValues(string(provider), operation, paymentProviderErrorClass(err)).Inc()
}

func paymentProviderErrorClass(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, domain.ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrRefundMismatch):
		return "provider_mismatch"
	default:
		return "provider_error"
	}
}
