package websession

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/paymentservice"
)

type WebSubscriptionService interface {
	Subscription(context.Context, uuid.UUID) (*domain.AccountSubscription, error)
	CreateSubscription(context.Context, paymentservice.CreateSubscriptionInput) (paymentservice.CreateIntentResult, error)
	CancelSubscription(context.Context, uuid.UUID) (*domain.AccountSubscription, error)
	RecurringAvailable() bool
	SubscriptionsAvailable() bool
}
type WebSubscriptionBalances interface {
	CreditLotBalancesForOwner(context.Context, uuid.UUID, time.Time) ([]domain.CreditLotBalance, error)
}
type safeSubscription struct {
	PlanCode     string          `json:"plan_code"`
	PlanName     string          `json:"plan_name"`
	Amount       int64           `json:"amount"`
	Stars        int64           `json:"stars"`
	Currency     domain.Currency `json:"currency"`
	PeriodStart  time.Time       `json:"period_start"`
	PaidUntil    time.Time       `json:"paid_until"`
	RenewalAt    *time.Time      `json:"renewal_at,omitempty"`
	NextPlanCode string          `json:"next_plan_code"`
	NextAmount   int64           `json:"next_amount"`
	AutoRenew    bool            `json:"auto_renew"`
	Status       string          `json:"status"`
}
type safeSubscriptionBalance struct {
	Kind      domain.CreditLotKind `json:"kind"`
	Available int64                `json:"available"`
	Reserved  int64                `json:"reserved"`
	ExpiresAt *time.Time           `json:"expires_at,omitempty"`
}

func (h *Handler) getWebSubscription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Subscriptions == nil {
		writeError(w, 503, "subscriptions unavailable")
		return
	}
	owner, _ := PrincipalFromContext(r.Context())
	state, err := h.deps.Subscriptions.Subscription(r.Context(), owner.AccountID)
	if err != nil {
		writeError(w, 503, "subscriptions unavailable")
		return
	}
	h.writeWebSubscription(w, r, state)
}
func (h *Handler) writeWebSubscription(w http.ResponseWriter, r *http.Request, state *domain.AccountSubscription) {
	now := time.Now().UTC()
	var current *safeSubscription
	if state != nil && !state.PaidUntil.IsZero() {
		status := "active"
		if state.CancelledAt != nil {
			status = "cancelled"
		}
		if !state.PaidUntil.After(now) {
			status = "expired"
		}
		if state.PastDue {
			status = "past_due"
		}
		current = &safeSubscription{PlanCode: state.Plan.Code, PlanName: state.Plan.Name, Amount: state.Plan.Amount, Stars: state.Plan.Stars, Currency: domain.CurrencyRUB, PeriodStart: state.PeriodStart, PaidUntil: state.PaidUntil, NextPlanCode: state.NextPlan.Code, NextAmount: state.NextPlan.Amount, AutoRenew: state.AutoRenew, Status: status}
		if state.AutoRenew {
			until := state.PaidUntil
			if state.NextAttemptAt.After(until) {
				until = state.NextAttemptAt
			}
			current.RenewalAt = &until
		}
	}
	balances := []safeSubscriptionBalance{}
	if h.deps.SubscriptionBalances != nil {
		owner, _ := PrincipalFromContext(r.Context())
		lots, err := h.deps.SubscriptionBalances.CreditLotBalancesForOwner(r.Context(), owner.AccountID, now)
		if err != nil {
			writeError(w, 503, "balance unavailable")
			return
		}
		for _, lot := range lots {
			balances = append(balances, safeSubscriptionBalance{Kind: lot.Kind, Available: lot.AvailableCredits, Reserved: lot.ReservedCredits, ExpiresAt: lot.ExpiresAt})
		}
	}
	writeJSON(w, 200, struct {
		Subscription       *safeSubscription         `json:"subscription"`
		Balances           []safeSubscriptionBalance `json:"balances"`
		CheckoutAvailable  bool                      `json:"checkout_available"`
		RecurringAvailable bool                      `json:"recurring_available"`
		TrialUsed          bool                      `json:"trial_used"`
	}{current, balances, h.cfg.TestPaymentsEnabled && h.deps.Subscriptions.SubscriptionsAvailable(), h.cfg.TestPaymentsEnabled && h.deps.Subscriptions.RecurringAvailable(), state != nil && state.TrialUsed})
}
func (h *Handler) createWebSubscription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.cfg.TestPaymentsEnabled || h.deps.Subscriptions == nil || !h.deps.Subscriptions.SubscriptionsAvailable() {
		writeError(w, 503, "subscription checkout unavailable")
		return
	}
	owner, _ := PrincipalFromContext(r.Context())
	key, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Idempotency-Key")))
	if err != nil || key == uuid.Nil {
		writeError(w, 400, "invalid idempotency key")
		return
	}
	var req struct {
		PlanCode           string `json:"plan_code"`
		AutoRenew          bool   `json:"auto_renew"`
		RetryPolicyVersion int    `json:"retry_policy_version"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if h.deps.PaymentCreateLimiter != nil {
		allowed, err := h.deps.PaymentCreateLimiter.Allow(r.Context(), owner.AccountID.String())
		if err != nil {
			writeError(w, 503, "checkout unavailable")
			return
		}
		if !allowed {
			writeError(w, 429, "too many payment requests")
			return
		}
	}
	if h.deps.ReceiptContacts == nil {
		writeError(w, 503, "checkout unavailable")
		return
	}
	email, err := h.deps.ReceiptContacts.VerifiedReceiptEmail(r.Context(), owner.AccountID)
	if err != nil {
		if errors.Is(err, accountservice.ErrReceiptEmailUnavailable) {
			writeError(w, 422, "verified account email required")
		} else {
			writeError(w, 503, "checkout unavailable")
		}
		return
	}
	result, err := h.deps.Subscriptions.CreateSubscription(r.Context(), paymentservice.CreateSubscriptionInput{AccountID: owner.AccountID, PlanCode: req.PlanCode, AutoRenew: req.AutoRenew, RetryPolicyVersion: req.RetryPolicyVersion, ReceiptEmail: email, IdempotencyKey: "web-subscription:" + owner.AccountID.String() + ":" + key.String()})
	if err != nil {
		if errors.Is(err, domain.ErrConflict) || errors.Is(err, paymentservice.ErrTrialAlreadyUsed) {
			writeError(w, 409, "subscription purchase conflicts with current state")
		} else if errors.Is(err, paymentservice.ErrInvalidInput) {
			writeError(w, 400, "invalid subscription details")
		} else {
			writeError(w, 503, "checkout unavailable")
		}
		return
	}
	if result.Intent == nil || result.Intent.AccountID != owner.AccountID {
		writeError(w, 503, "checkout unavailable")
		return
	}
	status := 200
	if result.Created {
		status = 201
	}
	writeSafeWebPayment(w, status, result.Intent)
}
func (h *Handler) cancelWebSubscription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Subscriptions == nil {
		writeError(w, 503, "subscriptions unavailable")
		return
	}
	var body struct{}
	if !decodeJSON(w, r, &body) {
		return
	}
	owner, _ := PrincipalFromContext(r.Context())
	state, err := h.deps.Subscriptions.CancelSubscription(r.Context(), owner.AccountID)
	if err != nil {
		writeError(w, 503, "subscriptions unavailable")
		return
	}
	h.writeWebSubscription(w, r, state)
}
