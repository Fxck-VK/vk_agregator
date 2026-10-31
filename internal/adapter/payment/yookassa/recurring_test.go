package yookassa

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestRecurringPaymentUsesVerifiedMethodWithoutRedirect(t *testing.T) {
	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"provider-renew","status":"succeeded","paid":true,"amount":{"value":"549.00","currency":"RUB"}}`))
	}))
	defer server.Close()
	p, err := New(Config{ShopID: "shop", SecretKey: "test_key", BaseURL: server.URL, ReturnURL: "https://epifera.test/return", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.CreatePayment(context.Background(), domain.CreatePaymentInput{IntentID: uuid.New(), Amount: 54900, Currency: domain.CurrencyRUB, ReceiptEmail: "test@example.test", IdempotencyKey: "renew-key", PaymentMethodID: "verified-method"})
	if err != nil {
		t.Fatal(err)
	}
	if request["payment_method_id"] != "verified-method" {
		t.Fatal("saved method not used")
	}
	if _, ok := request["confirmation"]; ok {
		t.Fatal("recurring request must not request redirect")
	}
	if _, ok := request["save_payment_method"]; ok {
		t.Fatal("renewal must not ask new consent")
	}
}

func TestInitialRecurringConsentAndSavedMethodNormalization(t *testing.T) {
	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&request)
		}
		_, _ = w.Write([]byte(`{"id":"provider-initial","status":"succeeded","paid":true,"amount":{"value":"199.00","currency":"RUB"},"payment_method":{"id":"saved-id","saved":true}}`))
	}))
	defer server.Close()
	p, err := New(Config{ShopID: "shop", SecretKey: "test_key", BaseURL: server.URL, ReturnURL: "https://epifera.test/return", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.CreatePayment(context.Background(), domain.CreatePaymentInput{Amount: 19900, Currency: domain.CurrencyRUB, ReceiptEmail: "test@example.test", IdempotencyKey: "initial-key", SavePaymentMethod: true})
	if err != nil {
		t.Fatal(err)
	}
	if request["save_payment_method"] != true {
		t.Fatal("explicit consent lost")
	}
	verified, err := p.GetPayment(context.Background(), "provider-initial")
	if err != nil {
		t.Fatal(err)
	}
	if verified.PaymentMethodID != "saved-id" || !verified.PaymentMethodSaved {
		t.Fatal("saved method verification missing")
	}
}

func TestRecurringCancellationReasonIsNormalizedFromAuthenticatedGET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shop, key, ok := r.BasicAuth()
		if r.Method != http.MethodGet || r.URL.Path != "/payments/provider-retry" || !ok || shop != "shop" || key != "test_key" {
			t.Error("expected authenticated payment GET")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"provider-retry","status":"canceled","paid":false,"amount":{"value":"549.00","currency":"RUB"},"cancellation_details":{"party":"payment_network","reason":"insufficient_funds"}}`))
	}))
	defer server.Close()
	p, err := New(Config{ShopID: "shop", SecretKey: "test_key", BaseURL: server.URL, ReturnURL: "https://epifera.test/return", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := p.GetPayment(context.Background(), "provider-retry")
	if err != nil || verified.CancellationReason != "insufficient_funds" || verified.Paid || verified.Captured || verified.Status != domain.PaymentIntentCanceled {
		t.Fatal("authenticated cancellation proof not normalized")
	}
}
