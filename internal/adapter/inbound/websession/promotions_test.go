package websession

import (
	"context"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/domain"
)

type promoSubscriptionStub struct {
	subscriptionStub
	code string
}

func (s *promoSubscriptionStub) QuotePromotion(_ context.Context, owner uuid.UUID, code string) (*domain.PromotionQuote, error) {
	s.owner = owner
	s.code = code
	return &domain.PromotionQuote{Code: "FIRST15", Revision: 1, Percent: 15, Plans: []domain.PromotionQuotePlan{{PlanCode: "start", BaseAmount: 54900, Amount: 46665, Currency: "RUB", Stars: 1250}}}, nil
}
func TestWebPromotionQuotePrincipalCSRFAndCheckout(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	owner := uuid.New()
	stub := &promoSubscriptionStub{}
	h.deps.Subscriptions = stub
	h.cfg.TestPaymentsEnabled = true
	h.deps.ReceiptContacts = &webReceiptContactStub{}
	request := safeConversationManagementRequest(t, http.MethodPost, "/web/v1/subscription/promo", sessions, owner, `{"code":"first15"}`)
	response := httptest.NewRecorder()
	h.Routes().ServeHTTP(response, request)
	if response.Code != 200 || stub.owner != owner || stub.code != "first15" || !strings.Contains(response.Body.String(), `"amount":46665`) {
		t.Fatalf("quote %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("cacheable eligibility")
	}
	request = safeConversationManagementRequest(t, http.MethodPost, "/web/v1/subscription/promo", sessions, owner, `{"code":"first15","account_id":"`+uuid.NewString()+`"}`)
	response = httptest.NewRecorder()
	h.Routes().ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatal("accepts spoofed owner", response.Code)
	}
	request = safeConversationManagementRequest(t, http.MethodPost, "/web/v1/subscription/promo", sessions, owner, `{"code":"first15"}`)
	request.Header.Del("X-CSRF-Token")
	response = httptest.NewRecorder()
	h.Routes().ServeHTTP(response, request)
	if response.Code == 200 {
		t.Fatal("missing csrf accepted")
	}
	request = safeConversationManagementRequest(t, http.MethodPost, "/web/v1/subscription/checkout", sessions, owner, `{"plan_code":"start","promo_code":"FIRST15","promo_revision":1}`)
	request.Header.Set("X-Idempotency-Key", uuid.NewString())
	response = httptest.NewRecorder()
	h.Routes().ServeHTTP(response, request)
	if response.Code != 201 || len(stub.inputs) != 1 || stub.inputs[0].PromoCode != "FIRST15" || stub.inputs[0].PromoRevision != 1 {
		t.Fatalf("checkout %d %s", response.Code, response.Body.String())
	}
}
