package paymentservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/economycatalog"
)

var ErrSubscriptionUnavailable = errors.New("subscription unavailable")
var ErrTrialAlreadyUsed = errors.New("subscription trial already used")

type CreateSubscriptionInput struct {
	RetryPolicyVersion int
	AccountID          uuid.UUID
	PlanCode           string
	AutoRenew          bool
	ReceiptEmail       string
	IdempotencyKey     string
}

type subscriptionPurchase struct {
	RetryPolicy      domain.SubscriptionRetryPolicy  `json:"retry_policy"`
	Plan             domain.SubscriptionPlanSnapshot `json:"plan"`
	Next             domain.SubscriptionPlanSnapshot `json:"next"`
	AutoRenew        bool                            `json:"auto_renew"`
	Renewal          bool                            `json:"renewal"`
	PreviousIntentID uuid.UUID                       `json:"previous_intent_id"`
}

func planSnapshot(p economycatalog.Plan) domain.SubscriptionPlanSnapshot {
	return domain.SubscriptionPlanSnapshot{Code: p.Code, Name: p.Name, Amount: p.Amount, Stars: p.Stars, PeriodDays: p.PeriodDays, Trial: p.Trial, NextPlanCode: p.NextPlanCode, PolicyVersion: economycatalog.Version}
}

func subscriptionPurchaseFrom(intent *domain.PaymentIntent) (*subscriptionPurchase, error) {
	if intent == nil {
		return nil, domain.ErrNotFound
	}
	if len(strings.TrimSpace(string(intent.Metadata))) == 0 {
		return nil, nil
	}
	var metadata struct {
		Subscription *subscriptionPurchase `json:"subscription"`
	}
	if err := json.Unmarshal(intent.Metadata, &metadata); err != nil {
		return nil, ErrInvalidInput
	}
	if metadata.Subscription == nil {
		return nil, nil
	}
	s := metadata.Subscription
	if s.RetryPolicy != (domain.SubscriptionRetryPolicy{}) && !s.RetryPolicy.Enabled() {
		return nil, ErrWebhookMismatch
	}
	if intent.AccountID == uuid.Nil || s.Plan.Code == "" || s.Plan.Amount != intent.Amount || s.Plan.Stars != intent.Credits || s.Plan.PeriodDays <= 0 || s.Plan.PeriodDays > 366 || s.Plan.PolicyVersion <= 0 || s.Next.Code == "" || s.Next.Amount <= 0 || s.Next.Stars <= 0 || s.Next.PeriodDays <= 0 || s.Next.PeriodDays > 366 {
		return nil, ErrWebhookMismatch
	}
	return s, nil
}

func (s *Service) Subscription(ctx context.Context, owner uuid.UUID) (*domain.AccountSubscription, error) {
	if owner == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if s.cfg.SubscriptionStore == nil {
		return nil, ErrSubscriptionUnavailable
	}
	state, err := s.cfg.SubscriptionStore.GetSubscription(ctx, owner)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	return state, err
}

func (s *Service) RecurringAvailable() bool {
	return s != nil && s.cfg.RecurringEnabled && s.provider != nil && s.provider.Code() == domain.PaymentProviderYooKassa
}
func (s *Service) SubscriptionsAvailable() bool {
	return s != nil && s.cfg.SubscriptionStore != nil && s.cfg.SubscriptionBilling != nil && s.cfg.SubscriptionTx != nil && s.provider != nil
}

func (s *Service) CreateSubscription(ctx context.Context, in CreateSubscriptionInput) (CreateIntentResult, error) {
	in.PlanCode = strings.TrimSpace(in.PlanCode)
	in.ReceiptEmail = strings.TrimSpace(in.ReceiptEmail)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if !s.SubscriptionsAvailable() {
		return CreateIntentResult{}, ErrSubscriptionUnavailable
	}
	plan, ok := economycatalog.FindPlan(in.PlanCode)
	if !ok || in.AccountID == uuid.Nil || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 160 {
		return CreateIntentResult{}, ErrInvalidInput
	}
	if in.ReceiptEmail == "" {
		return CreateIntentResult{}, ErrReceiptContactRequired
	}
	if in.AutoRenew && !s.RecurringAvailable() {
		return CreateIntentResult{}, ErrSubscriptionUnavailable
	}
	if in.RetryPolicyVersion != 0 && (in.RetryPolicyVersion != economycatalog.RenewalRetries().Version || !in.AutoRenew) {
		return CreateIntentResult{}, ErrInvalidInput
	}
	next := plan
	if plan.NextPlanCode != "" {
		var found bool
		next, found = economycatalog.FindPlan(plan.NextPlanCode)
		if !found {
			return CreateIntentResult{}, ErrSubscriptionUnavailable
		}
	}
	purchase := subscriptionPurchase{Plan: planSnapshot(plan), Next: planSnapshot(next), AutoRenew: in.AutoRenew}
	if in.RetryPolicyVersion != 0 {
		purchase.RetryPolicy = economycatalog.RenewalRetries()
	}
	var result CreateIntentResult
	err := s.cfg.SubscriptionTx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billing domain.BillingRepository) error {
		store, ok := billing.(domain.SubscriptionRepository)
		if !ok {
			return ErrSubscriptionUnavailable
		}
		state, err := store.LockSubscription(ctx, in.AccountID)
		if err != nil {
			return err
		}
		if old, err := payments.GetIntentByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
			previous, e := subscriptionPurchaseFrom(old)
			if e != nil {
				return e
			}
			if old.AccountID != in.AccountID || previous == nil || previous.Plan.Code != in.PlanCode || previous.AutoRenew != in.AutoRenew || previous.RetryPolicy != purchase.RetryPolicy || previous.Renewal || old.ReceiptEmail != in.ReceiptEmail {
				return domain.ErrConflict
			}
			result.Intent = old
			return nil
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		if !state.PaidUntil.IsZero() && state.PaidUntil.After(time.Now().UTC()) {
			return domain.ErrConflict
		}
		if state.PendingIntentID != uuid.Nil {
			pending, e := payments.GetIntentByID(ctx, state.PendingIntentID)
			if e != nil {
				return e
			}
			if !pending.Status.IsTerminal() {
				return domain.ErrConflict
			}
			if pending.Status == domain.PaymentIntentSucceeded {
				return domain.ErrConflict
			}
		}
		if plan.Trial && state.TrialUsed {
			return ErrTrialAlreadyUsed
		}
		raw, err := json.Marshal(map[string]any{"source": "web", "return_url": s.cfg.AccountReturnURL, "capture": true, "subscription": purchase, subscriptionSubmitMarker: time.Now().UTC().Format(time.RFC3339Nano)})
		if err != nil {
			return err
		}
		intent := &domain.PaymentIntent{ID: uuid.New(), AccountID: in.AccountID, Status: domain.PaymentIntentCreated, Amount: plan.Amount, Currency: domain.CurrencyRUB, Credits: plan.Stars, CreditDenominationVersion: domain.CurrentCreditDenominationVersion, PriceVersion: economycatalog.Version, ReceiptDescription: "Epifera " + plan.Name, PaymentSubject: "service", PaymentMode: "full_prepayment", Provider: s.provider.Code(), IdempotencyKey: in.IdempotencyKey, ReceiptEmail: in.ReceiptEmail, Metadata: raw}
		if err := payments.CreateIntent(ctx, intent); err != nil {
			return err
		}
		state.PendingIntentID = intent.ID
		state.CancelledAt = nil
		state.AutoRenew = false
		state.PastDue = false
		state.PaymentMethodID = ""
		resetSubscriptionRetry(state)
		if err := store.SaveSubscription(ctx, state); err != nil {
			return err
		}
		result.Intent = intent
		result.Created = true
		return nil
	})
	if err != nil {
		return CreateIntentResult{}, err
	}
	intent, err := s.ensureProviderPayment(ctx, result.Intent, nil, s.cfg.AccountReturnURL)
	result.Intent = intent
	return result, err
}

func (s *Service) CancelSubscription(ctx context.Context, owner uuid.UUID) (*domain.AccountSubscription, error) {
	if !s.SubscriptionsAvailable() {
		return nil, ErrSubscriptionUnavailable
	}
	if owner == uuid.Nil {
		return nil, ErrInvalidInput
	}
	var result *domain.AccountSubscription
	err := s.cfg.SubscriptionTx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billing domain.BillingRepository) error {
		store, ok := billing.(domain.SubscriptionRepository)
		if !ok {
			return ErrSubscriptionUnavailable
		}
		state, err := store.LockSubscription(ctx, owner)
		if err != nil {
			return err
		}
		if state.CancelledAt == nil {
			now := time.Now().UTC()
			state.CancelledAt = &now
		}
		state.AutoRenew = false
		state.PaymentMethodID = ""
		resetSubscriptionRetry(state)
		if err := retireUnsubmittedRenewal(ctx, payments, state); err != nil {
			return err
		}
		if err := store.SaveSubscription(ctx, state); err != nil {
			return err
		}
		result = state
		return nil
	})
	return result, err
}

// settleSubscription is called inside the verified payment transaction. An
// ordinary top-up cannot enter this branch without a server purchase snapshot.
func (p *WebhookProcessor) settleSubscription(ctx context.Context, billing domain.BillingRepository, intent *domain.PaymentIntent, verified domain.ProviderPayment, purchase *subscriptionPurchase) error {
	store, ok := billing.(domain.SubscriptionRepository)
	if !ok {
		return ErrSubscriptionUnavailable
	}
	state, err := store.LockSubscription(ctx, intent.AccountID)
	if err != nil {
		return err
	}
	if state.ActiveIntentID == intent.ID {
		return nil
	}
	if state.PendingIntentID != intent.ID {
		return domain.ErrConflict
	}
	if purchase.Renewal && state.ActiveIntentID != purchase.PreviousIntentID {
		return domain.ErrConflict
	}
	if purchase.Plan.Trial && state.TrialUsed {
		return ErrTrialAlreadyUsed
	}
	now := p.now().UTC()
	until := now.AddDate(0, 0, purchase.Plan.PeriodDays)
	acc, err := p.billing.EnsureAccountForAccount(ctx, billing, intent.AccountID, domain.CurrencyCredits)
	if err != nil {
		return err
	}
	entry := &domain.LedgerEntry{AccountID: acc.ID, OwnerAccountID: intent.AccountID, Type: domain.LedgerTopup, Amount: purchase.Plan.Stars, CreditDenominationVersion: domain.CurrentCreditDenominationVersion, Status: domain.LedgerStatusCommitted, IdempotencyKey: topUpLedgerKey(intent.Provider, intent.ProviderPaymentID), Reason: "subscription period " + purchase.Plan.Code, LotKind: domain.CreditLotSubscription, LotExpiresAt: &until}
	if err := billing.AppendEntry(ctx, entry); err != nil {
		return err
	}
	state.Plan = purchase.Plan
	state.NextPlan = purchase.Next
	state.PeriodStart = now
	state.PaidUntil = until
	state.TrialUsed = state.TrialUsed || purchase.Plan.Trial
	state.ActiveIntentID = intent.ID
	state.PendingIntentID = uuid.Nil
	state.PastDue = false
	state.NextAttemptAt = time.Time{}
	// A late paid retry remains payable, but cannot revive canceled/exhausted
	// consent. An initial checkout enables only its own fresh opt-in.
	state.AutoRenew = purchase.AutoRenew && (!purchase.Renewal || (state.AutoRenew && !retryWindowExhausted(state, now))) && state.CancelledAt == nil && verified.PaymentMethodSaved && verified.PaymentMethodID != ""
	state.RetryPolicy = purchase.RetryPolicy
	resetSubscriptionRetry(state)
	if state.AutoRenew {
		state.PaymentMethodID = verified.PaymentMethodID
	} else {
		state.PaymentMethodID = ""
	}
	return store.SaveSubscription(ctx, state)
}

// PrepareSubscriptionRenewals stores each next intent before any provider call.
// A single cycle has one durable key, even across process crashes and retries.
func (s *Service) ProcessSubscriptionRenewals(ctx context.Context, limit int) (int, error) {
	if !s.SubscriptionsAvailable() || !s.RecurringAvailable() {
		return 0, nil
	}
	now := time.Now().UTC()
	owners, err := s.cfg.SubscriptionStore.ListDueSubscriptions(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	count := 0
	var firstErr error
	for _, owner := range owners {
		observed, readErr := s.cfg.SubscriptionStore.GetSubscription(ctx, owner)
		if readErr != nil || observed == nil {
			if readErr == nil {
				readErr = ErrSubscriptionUnavailable
			}
			if firstErr == nil {
				firstErr = readErr
			}
			continue
		}
		if err := s.renewSubscription(ctx, owner, now); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			// Failed early owners must not occupy the first bounded page forever.
			backoffErr := s.cfg.SubscriptionTx.RunPaymentTx(ctx, func(ctx context.Context, _ domain.PaymentRepository, billing domain.BillingRepository) error {
				store, ok := billing.(domain.SubscriptionRepository)
				if !ok {
					return ErrSubscriptionUnavailable
				}
				state, e := store.LockSubscription(ctx, owner)
				if e != nil {
					return e
				}
				if !state.AutoRenew || state.CancelledAt != nil || state.ActiveIntentID != observed.ActiveIntentID || state.RetryFailures != observed.RetryFailures || !state.RetryUntil.Equal(observed.RetryUntil) {
					return nil
				}
				next := currentRenewalTime(now).Add(5 * time.Minute)
				if !state.RetryUntil.IsZero() && next.After(state.RetryUntil) {
					next = state.RetryUntil
				}
				if !state.NextAttemptAt.Before(next) {
					return nil
				}
				state.NextAttemptAt = next
				return store.SaveSubscription(ctx, state)
			})
			if backoffErr != nil {
				firstErr = errors.Join(firstErr, backoffErr)
			}
			continue
		}
		count++
	}
	return count, firstErr
}

func (s *Service) renewSubscription(ctx context.Context, owner uuid.UUID, now time.Time) error {
	var pending uuid.UUID
	err := s.cfg.SubscriptionTx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billing domain.BillingRepository) error {
		store, ok := billing.(domain.SubscriptionRepository)
		if !ok {
			return ErrSubscriptionUnavailable
		}
		state, err := store.LockSubscription(ctx, owner)
		if err != nil {
			return err
		}
		if !state.AutoRenew || state.CancelledAt != nil || state.PaymentMethodID == "" {
			return nil
		}
		if retryWindowExhausted(state, currentRenewalTime(now)) {
			disableSubscriptionRenewal(state)
			if err := retireUnsubmittedRenewal(ctx, payments, state); err != nil {
				return err
			}
			return store.SaveSubscription(ctx, state)
		}
		if state.PaidUntil.After(now) || state.NextAttemptAt.After(now) {
			return nil
		}
		if state.PendingIntentID != uuid.Nil {
			pending = state.PendingIntentID
			return nil
		}
		previous, err := payments.GetIntentByID(ctx, state.ActiveIntentID)
		if err != nil {
			return err
		}
		purchase := subscriptionPurchase{Plan: state.NextPlan, Next: state.NextPlan, AutoRenew: true, Renewal: true, PreviousIntentID: state.ActiveIntentID, RetryPolicy: state.RetryPolicy}
		raw, err := json.Marshal(map[string]any{"source": "web", "capture": true, "subscription": purchase, "subscription_payment_method_id": state.PaymentMethodID})
		if err != nil {
			return err
		}
		key := "subscription-renew:" + state.ActiveIntentID.String()
		if state.RetryFailures > 0 && state.RetryPolicy.Enabled() {
			key += fmt.Sprintf(":retry:%d", state.RetryFailures)
		}
		next := &domain.PaymentIntent{ID: uuid.New(), AccountID: owner, Status: domain.PaymentIntentCreated, Amount: purchase.Plan.Amount, Currency: domain.CurrencyRUB, Credits: purchase.Plan.Stars, CreditDenominationVersion: domain.CurrentCreditDenominationVersion, PriceVersion: purchase.Plan.PolicyVersion, ReceiptDescription: "Epifera " + purchase.Plan.Name, PaymentSubject: "service", PaymentMode: "full_prepayment", Provider: s.provider.Code(), IdempotencyKey: key, ReceiptEmail: previous.ReceiptEmail, Metadata: raw}
		if err := payments.CreateIntent(ctx, next); err != nil {
			return err
		}
		state.PendingIntentID = next.ID
		pending = next.ID
		return store.SaveSubscription(ctx, state)
	})
	if err != nil || pending == uuid.Nil {
		return err
	}
	receipt, err := s.markSubscriptionSubmit(ctx, owner, pending, now)
	if err != nil {
		return err
	}
	// Hold the same subscription row through submit so cancellation is linearized
	// before or after the network call. The intent was committed above already.
	var deferredErr error
	err = s.cfg.SubscriptionTx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billing domain.BillingRepository) error {
		store := billing.(domain.SubscriptionRepository)
		state, err := store.LockSubscription(ctx, owner)
		if err != nil {
			return err
		}
		if state.PendingIntentID != pending {
			return nil
		}
		intent, err := payments.GetIntentByID(ctx, pending)
		if err != nil {
			return err
		}
		if retryWindowExhausted(state, currentRenewalTime(now)) {
			return expireOwnedSubscriptionRetry(ctx, payments, store, state, intent, receipt)
		}
		if intent.Status == domain.PaymentIntentCanceled || intent.Status == domain.PaymentIntentFailed || intent.Status == domain.PaymentIntentExpired {
			if !state.AutoRenew || state.CancelledAt != nil {
				return nil
			}
			state.PastDue = true
			disableSubscriptionRenewal(state)
			return store.SaveSubscription(ctx, state)
		}
		if intent.ProviderPaymentID != "" {
			// Provider-bound payments are settled by verified reconciliation,
			// not submitted again by the renewal scheduler.
			state.NextAttemptAt = now.Add(time.Minute)
			return store.SaveSubscription(ctx, state)
		}
		if !ownsSubmissionReceipt(intent, receipt) {
			return nil
		}
		if !state.AutoRenew || state.CancelledAt != nil {
			// Only this invocation owns the first receipt and it has not called
			// the provider yet. Retried/ambiguous receipts must keep their binding.
			if receipt.FirstAttempt && intent.Status == domain.PaymentIntentCreated && intent.ProviderPaymentID == "" {
				if err := payments.UpdateIntentStatus(ctx, intent.ID, intent.Status, domain.PaymentIntentCanceled); err != nil {
					return err
				}
				state.PendingIntentID = uuid.Nil
				return store.SaveSubscription(ctx, state)
			}
			return nil
		}
		started, marked, markErr := submissionStartedAt(intent)
		if markErr != nil {
			return markErr
		}
		if !marked {
			started = intent.CreatedAt
		}
		if intent.ProviderPaymentID == "" && currentRenewalTime(now).Sub(started) > 23*time.Hour {
			state.PastDue = true
			state.AutoRenew = false
			deferredErr = fmt.Errorf("%w: ambiguous expired renewal requires reconciliation", ErrSubscriptionUnavailable)
			return store.SaveSubscription(ctx, state)
		}
		sender := *s
		sender.repo = payments
		if retryWindowExhausted(state, currentRenewalTime(now)) {
			return expireOwnedSubscriptionRetry(ctx, payments, store, state, intent, receipt)
		}
		_, err = sender.ensureProviderPayment(ctx, intent, nil, "")
		if err != nil {
			return err
		}
		state.NextAttemptAt = now.Add(time.Minute)
		return store.SaveSubscription(ctx, state)
	})
	if err != nil {
		return err
	}
	return deferredErr
}
