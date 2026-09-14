package websession

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/paymentservice"
)

type webPaymentStub struct {
	inputs []paymentservice.CreateAccountIntentInput
	intent *domain.PaymentIntent
}

type webReceiptContactStub struct {
	accountID uuid.UUID
	err       error
}

func (s *webReceiptContactStub) VerifiedReceiptEmail(_ context.Context, accountID uuid.UUID) (string, error) {
	s.accountID = accountID
	if s.err != nil {
		return "", s.err
	}
	return "buyer@example.test", nil
}

func (s *webPaymentStub) ListActiveProducts(context.Context) ([]*domain.PaymentProduct, error) {
	return []*domain.PaymentProduct{{Code: "tokens_800", Amount: 40000, Currency: domain.CurrencyRUB, Credits: 800, CreditDenominationVersion: 2, IsActive: true}}, nil
}
func (s *webPaymentStub) CreateAccountIntent(_ context.Context, in paymentservice.CreateAccountIntentInput) (paymentservice.CreateIntentResult, error) {
	s.inputs = append(s.inputs, in)
	s.intent.AccountID = in.AccountID
	return paymentservice.CreateIntentResult{Intent: s.intent, Created: len(s.inputs) == 1}, nil
}
func (s *webPaymentStub) GetAccountIntent(_ context.Context, accountID, id uuid.UUID) (*domain.PaymentIntent, error) {
	if s.intent.ID != id || s.intent.AccountID != accountID {
		return nil, domain.ErrNotFound
	}
	return s.intent, nil
}

func TestWebPaymentCatalogAndCreate(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	h.cfg.TestPaymentsEnabled = true
	stub := &webPaymentStub{intent: &domain.PaymentIntent{ID: uuid.New(), Status: domain.PaymentIntentWaitingForUser, Amount: 40000, Currency: domain.CurrencyRUB, Credits: 800, CreditDenominationVersion: 2, Provider: domain.PaymentProviderYooKassa, ConfirmationURL: "https://yoomoney.ru/checkout/payments/v2/contract?orderId=test"}}
	h.deps.Payments = stub
	contacts := &webReceiptContactStub{}
	h.deps.ReceiptContacts = contacts
	owner := uuid.New()
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/payment-products", sessions, owner))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"credits":800`) {
		t.Fatalf("catalog: %d %s", rec.Code, rec.Body.String())
	}
	key := uuid.NewString()
	for range 2 {
		req := safeConversationManagementRequest(t, http.MethodPost, "/web/v1/payments/intents", sessions, owner, `{"product_code":"tokens_800"}`)
		req.Header.Set("X-Idempotency-Key", key)
		rec = httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != 200 && rec.Code != 201 {
			t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "account_id") || strings.Contains(rec.Body.String(), "provider_payment_id") || strings.Contains(rec.Body.String(), "buyer@example.test") {
			t.Fatal("unsafe payment DTO")
		}
	}
	if contacts.accountID != owner || stub.inputs[0].ReceiptEmail != "buyer@example.test" {
		t.Fatal("receipt contact not resolved from authenticated account")
	}
	if len(stub.inputs) != 2 || stub.inputs[0].AccountID != owner || stub.inputs[0].IdempotencyKey != stub.inputs[1].IdempotencyKey || !strings.Contains(stub.inputs[0].IdempotencyKey, owner.String()) {
		t.Fatal("principal/idempotency not preserved")
	}
	for _, foreign := range []bool{false, true} {
		id := owner
		if foreign {
			id = uuid.New()
		}
		rec = httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/payments/"+stub.intent.ID.String(), sessions, id))
		want := 200
		if foreign {
			want = 404
		}
		if rec.Code != want {
			t.Fatalf("read: %d", rec.Code)
		}
	}
}

func TestWebPaymentRejectsUnsafeRequests(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		mutate     func(*http.Request, *Handler)
		want       int
	}{
		{name: "client price", body: `{"product_code":"tokens_800","amount":1}`, want: 400},
		{name: "client email", body: `{"product_code":"tokens_800","receipt_email":"attacker@example.test"}`, want: 400},
		{name: "missing key", mutate: func(r *http.Request, _ *Handler) { r.Header.Del("X-Idempotency-Key") }, want: 400},
		{name: "csrf", mutate: func(r *http.Request, _ *Handler) { r.Header.Del("X-CSRF-Token") }, want: 403},
		{name: "origin", mutate: func(r *http.Request, _ *Handler) { r.Header.Set("Origin", "https://evil.test") }, want: 403},
		{name: "session", mutate: func(r *http.Request, _ *Handler) { r.Header.Del("Cookie") }, want: 403},
		{name: "unavailable test shop", mutate: func(_ *http.Request, h *Handler) { h.cfg.TestPaymentsEnabled = false }, want: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, sessions := newTestHandler(t)
			h.cfg.TestPaymentsEnabled = true
			stub := &webPaymentStub{}
			h.deps.Payments = stub
			body := tc.body
			if body == "" {
				body = `{"product_code":"tokens_800"}`
			}
			req := safeConversationManagementRequest(t, http.MethodPost, "/web/v1/payments/intents", sessions, uuid.New(), body)
			req.Header.Set("X-Idempotency-Key", uuid.NewString())
			if tc.mutate != nil {
				tc.mutate(req, h)
			}
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != tc.want || len(stub.inputs) != 0 {
				t.Fatalf("status %d; service calls %d", rec.Code, len(stub.inputs))
			}
		})
	}
}

func TestWebPaymentRequiresVerifiedAccountReceiptContact(t *testing.T) {
	for _, tc := range []struct {
		name       string
		contactErr error
		want       int
	}{
		{"missing email", accountservice.ErrReceiptEmailUnavailable, 422},
		{"store unavailable", errors.New("unavailable"), 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, sessions := newTestHandler(t)
			h.cfg.TestPaymentsEnabled = true
			stub := &webPaymentStub{}
			h.deps.Payments = stub
			h.deps.ReceiptContacts = &webReceiptContactStub{err: tc.contactErr}
			req := safeConversationManagementRequest(t, http.MethodPost, "/web/v1/payments/intents", sessions, uuid.New(), `{"product_code":"tokens_800"}`)
			req.Header.Set("X-Idempotency-Key", uuid.NewString())
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != tc.want || len(stub.inputs) != 0 {
				t.Fatalf("status %d; service calls %d", rec.Code, len(stub.inputs))
			}
		})
	}
}

func TestWebPaymentDoesNotExposeUnsafeOrTerminalCheckout(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	owner := uuid.New()
	stub := &webPaymentStub{intent: &domain.PaymentIntent{ID: uuid.New(), AccountID: owner, Amount: 40000, Credits: 800, CreditDenominationVersion: 2, Currency: domain.CurrencyRUB, Status: domain.PaymentIntentSucceeded, ConfirmationURL: "https://evil.test/checkout"}}
	h.deps.Payments = stub
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/payments/"+stub.intent.ID.String(), sessions, owner))
	var dto map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &dto)
	if rec.Code != 200 || dto["confirmation_url"] != nil {
		t.Fatalf("unsafe DTO: %d %s", rec.Code, rec.Body.String())
	}
}
