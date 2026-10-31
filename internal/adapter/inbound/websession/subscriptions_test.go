package websession

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/paymentservice"
)

func TestWebSubscriptionTrialUsageIsDurableAndOwnerScoped(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name  string
		state *domain.AccountSubscription
		used  bool
	}{
		{"new account", nil, false},
		{"unpaid attempt", &domain.AccountSubscription{TrialUsed: false, PendingIntentID: uuid.New()}, false},
		{"active Lite", &domain.AccountSubscription{TrialUsed: true, PaidUntil: now.Add(time.Hour)}, true},
		{"expired Lite", &domain.AccountSubscription{TrialUsed: true, PaidUntil: now.Add(-time.Hour)}, true},
		{"refunded or cleared period", &domain.AccountSubscription{TrialUsed: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, sessions := newTestHandler(t)
			owner := uuid.New()
			stub := &subscriptionStub{state: tc.state}
			h.deps.Subscriptions = stub
			response := httptest.NewRecorder()
			h.Routes().ServeHTTP(response, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/subscription?trial_used=false&account_id="+uuid.NewString(), sessions, owner))
			var dto struct {
				TrialUsed *bool `json:"trial_used"`
			}
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &dto) != nil || dto.TrialUsed == nil || *dto.TrialUsed != tc.used || stub.owner != owner {
				t.Fatalf("trial usage: status=%d body=%s", response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private eligibility must not be cached")
			}
		})
	}
}

type subscriptionStub struct {
	owner  uuid.UUID
	state  *domain.AccountSubscription
	inputs []paymentservice.CreateSubscriptionInput
}

func (s *subscriptionStub) Subscription(_ context.Context, id uuid.UUID) (*domain.AccountSubscription, error) {
	s.owner = id
	return s.state, nil
}
func (s *subscriptionStub) CancelSubscription(_ context.Context, id uuid.UUID) (*domain.AccountSubscription, error) {
	s.owner = id
	s.state.AutoRenew = false
	return s.state, nil
}
func (s *subscriptionStub) CreateSubscription(_ context.Context, in paymentservice.CreateSubscriptionInput) (paymentservice.CreateIntentResult, error) {
	s.inputs = append(s.inputs, in)
	return paymentservice.CreateIntentResult{Created: true, Intent: &domain.PaymentIntent{ID: uuid.New(), AccountID: in.AccountID, Status: domain.PaymentIntentWaitingForUser, Amount: 19900, Currency: domain.CurrencyRUB, Credits: 400, CreditDenominationVersion: 2, ConfirmationURL: "https://yoomoney.ru/checkout"}}, nil
}
func (s *subscriptionStub) RecurringAvailable() bool     { return true }
func (s *subscriptionStub) SubscriptionsAvailable() bool { return true }

func TestWebSubscriptionUsesPrincipalAndSafeDTO(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	owner := uuid.New()
	now := time.Now().UTC()
	stub := &subscriptionStub{state: &domain.AccountSubscription{Plan: domain.SubscriptionPlanSnapshot{Code: "lite", Name: "Lite", Amount: 19900, Stars: 400}, NextPlan: domain.SubscriptionPlanSnapshot{Code: "start", Amount: 54900}, PeriodStart: now, PaidUntil: now.AddDate(0, 0, 7), AutoRenew: true, PaymentMethodID: "private-saved-method"}}
	h.deps.Subscriptions = stub
	h.cfg.TestPaymentsEnabled = true
	response := httptest.NewRecorder()
	h.Routes().ServeHTTP(response, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/subscription", sessions, owner))
	if response.Code != 200 || stub.owner != owner || !strings.Contains(response.Body.String(), `"next_amount":54900`) {
		t.Fatalf("read: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "private-saved-method") || strings.Contains(response.Body.String(), "payment_method_id") {
		t.Fatal("private method leaked")
	}
	request := safeConversationManagementRequest(t, http.MethodPost, "/web/v1/subscription/cancel", sessions, owner, `{}`)
	response = httptest.NewRecorder()
	h.Routes().ServeHTTP(response, request)
	if response.Code != 200 || stub.state.AutoRenew {
		t.Fatalf("cancel: %d", response.Code)
	}
}

func TestWebSubscriptionCheckoutRequiresServerReceiptAndIdempotency(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	owner := uuid.New()
	stub := &subscriptionStub{}
	h.deps.Subscriptions = stub
	h.deps.ReceiptContacts = &webReceiptContactStub{}
	h.cfg.TestPaymentsEnabled = true
	request := safeConversationManagementRequest(t, http.MethodPost, "/web/v1/subscription/checkout", sessions, owner, `{"plan_code":"lite","auto_renew":false}`)
	request.Header.Set("X-Idempotency-Key", uuid.NewString())
	response := httptest.NewRecorder()
	h.Routes().ServeHTTP(response, request)
	if response.Code != 201 || len(stub.inputs) != 1 || stub.inputs[0].AccountID != owner || stub.inputs[0].ReceiptEmail != "buyer@example.test" {
		t.Fatalf("checkout: %d %s", response.Code, response.Body.String())
	}
	request = safeConversationManagementRequest(t, http.MethodPost, "/web/v1/subscription/checkout", sessions, owner, `{"plan_code":"lite","auto_renew":false,"amount":1}`)
	request.Header.Set("X-Idempotency-Key", uuid.NewString())
	response = httptest.NewRecorder()
	h.Routes().ServeHTTP(response, request)
	if response.Code != 400 || len(stub.inputs) != 1 {
		t.Fatalf("browser price accepted: %d", response.Code)
	}
}

func TestWebSubscriptionCarriesPolicyConsentAndActualRetryTime(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	owner := uuid.New()
	now := time.Now().UTC()
	next := now.Add(24 * time.Hour)
	stub := &subscriptionStub{state: &domain.AccountSubscription{Plan: domain.SubscriptionPlanSnapshot{Code: "lite", Name: "Lite", Amount: 19900, Stars: 400}, NextPlan: domain.SubscriptionPlanSnapshot{Code: "start", Amount: 54900}, PeriodStart: now.Add(-8 * 24 * time.Hour), PaidUntil: now.Add(-24 * time.Hour), NextAttemptAt: next, AutoRenew: true, PastDue: true, PaymentMethodID: "private-retry-method"}}
	h.deps.Subscriptions = stub
	h.deps.ReceiptContacts = &webReceiptContactStub{}
	h.cfg.TestPaymentsEnabled = true
	request := safeConversationManagementRequest(t, http.MethodPost, "/web/v1/subscription/checkout", sessions, owner, `{"plan_code":"lite","auto_renew":true,"retry_policy_version":1}`)
	request.Header.Set("X-Idempotency-Key", uuid.NewString())
	response := httptest.NewRecorder()
	h.Routes().ServeHTTP(response, request)
	if response.Code != 201 || len(stub.inputs) != 1 || stub.inputs[0].RetryPolicyVersion != 1 || !stub.inputs[0].AutoRenew || stub.inputs[0].AccountID != owner {
		t.Fatal("opt-in policy version/principal not forwarded")
	}
	response = httptest.NewRecorder()
	h.Routes().ServeHTTP(response, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/subscription", sessions, owner))
	var dto struct {
		Subscription struct {
			RenewalAt time.Time `json:"renewal_at"`
			Status    string    `json:"status"`
		} `json:"subscription"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &dto) != nil || !dto.Subscription.RenewalAt.Equal(next) || dto.Subscription.Status != "past_due" || strings.Contains(response.Body.String(), "private-retry-method") {
		t.Fatal("actual retry time missing or private data exposed")
	}
}
