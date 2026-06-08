package yookassa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestCreatePaymentSendsYooKassaContract(t *testing.T) {
	intentID := uuid.New()
	userID := uuid.New()
	var seen struct {
		Auth           string
		IdempotencyKey string
		Body           createPaymentRequest
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/payments" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		seen.Auth = r.Header.Get("Authorization")
		seen.IdempotencyKey = r.Header.Get("Idempotence-Key")
		if err := json.NewDecoder(r.Body).Decode(&seen.Body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"pay-1",
			"status":"pending",
			"paid":false,
			"amount":{"value":"100.00","currency":"RUB"},
			"confirmation":{"type":"redirect","confirmation_url":"https://yookassa.example/confirm/pay-1"},
			"refundable":false
		}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL)
	result, err := provider.CreatePayment(context.Background(), domain.CreatePaymentInput{
		IntentID:       intentID,
		UserID:         userID,
		Amount:         10000,
		Currency:       domain.CurrencyRUB,
		Credits:        500,
		Description:    "NeiroHub credits",
		ReturnURL:      "https://neiirohub.ru/return/custom",
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: "pay:" + intentID.String(),
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if result.ProviderPaymentID != "pay-1" || result.ConfirmationURL == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Status != domain.PaymentIntentWaitingForUser {
		t.Fatalf("status = %q, want waiting_for_user", result.Status)
	}
	if seen.Auth != "Basic "+base64.StdEncoding.EncodeToString([]byte("shop-1:secret")) {
		t.Fatalf("unexpected auth header: %q", seen.Auth)
	}
	if seen.IdempotencyKey != "pay:"+intentID.String() {
		t.Fatalf("idempotency key = %q", seen.IdempotencyKey)
	}
	if !seen.Body.Capture {
		t.Fatal("capture = false, want true")
	}
	if seen.Body.Amount.Value != "100.00" || seen.Body.Amount.Currency != "RUB" {
		t.Fatalf("unexpected amount: %#v", seen.Body.Amount)
	}
	if seen.Body.Confirmation.Type != "redirect" || seen.Body.Confirmation.ReturnURL != "https://neiirohub.ru/return/custom" {
		t.Fatalf("unexpected confirmation: %#v", seen.Body.Confirmation)
	}
	if seen.Body.Receipt.Customer.Email != "user@example.com" {
		t.Fatalf("unexpected receipt customer: %#v", seen.Body.Receipt.Customer)
	}
	if len(seen.Body.Receipt.Items) != 1 {
		t.Fatalf("receipt item count = %d, want 1", len(seen.Body.Receipt.Items))
	}
	item := seen.Body.Receipt.Items[0]
	if item.Amount.Value != "100.00" || item.VATCode != defaultVATCode || item.PaymentSubject != defaultPaymentSubject || item.PaymentMode != defaultPaymentMode {
		t.Fatalf("unexpected receipt item: %#v", item)
	}
	if seen.Body.Metadata["intent_id"] != intentID.String() || seen.Body.Metadata["user_id"] != userID.String() {
		t.Fatalf("unexpected metadata: %#v", seen.Body.Metadata)
	}
}

func TestCreatePaymentRequiresReceiptContact(t *testing.T) {
	provider := newTestProvider(t, "https://example.com")
	_, err := provider.CreatePayment(context.Background(), domain.CreatePaymentInput{
		IntentID:       uuid.New(),
		UserID:         uuid.New(),
		Amount:         10000,
		Currency:       domain.CurrencyRUB,
		IdempotencyKey: "pay:" + uuid.NewString(),
	})
	if err == nil || !strings.Contains(err.Error(), "receipt email or phone") {
		t.Fatalf("expected receipt contact error, got %v", err)
	}
}

func TestCreatePaymentRejectsLongHTTPIdempotencyKey(t *testing.T) {
	provider := newTestProvider(t, "https://example.com")
	_, err := provider.CreatePayment(context.Background(), domain.CreatePaymentInput{
		IntentID:       uuid.New(),
		UserID:         uuid.New(),
		Amount:         10000,
		Currency:       domain.CurrencyRUB,
		ReceiptEmail:   "user@example.com",
		IdempotencyKey: strings.Repeat("x", maxHTTPIdempotencySize+1),
	})
	if err == nil || !strings.Contains(err.Error(), "idempotence key") {
		t.Fatalf("expected idempotency error, got %v", err)
	}
}

func TestGetPaymentParsesAmountAndStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/payments/pay-1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"pay-1",
			"status":"succeeded",
			"paid":true,
			"amount":{"value":"250.50","currency":"RUB"},
			"refundable":true
		}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL)
	payment, err := provider.GetPayment(context.Background(), "pay-1")
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if payment.Status != domain.PaymentIntentSucceeded || payment.Amount != 25050 || payment.Currency != domain.CurrencyRUB {
		t.Fatalf("unexpected payment: %#v", payment)
	}
	if !payment.Paid || !payment.Captured || !payment.Refundable {
		t.Fatalf("unexpected flags: %#v", payment)
	}
}

func TestCreateRefundSendsYooKassaContract(t *testing.T) {
	refundID := uuid.New()
	var seen struct {
		IdempotencyKey string
		Body           createRefundRequest
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/refunds" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		seen.IdempotencyKey = r.Header.Get("Idempotence-Key")
		if err := json.NewDecoder(r.Body).Decode(&seen.Body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"refund-1",
			"status":"succeeded",
			"amount":{"value":"10.50","currency":"RUB"},
			"payment_id":"pay-1"
		}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, server.URL)
	result, err := provider.CreateRefund(context.Background(), domain.CreateRefundInput{
		RefundID:          refundID,
		IntentID:          uuid.New(),
		ProviderPaymentID: "pay-1",
		Amount:            1050,
		Currency:          domain.CurrencyRUB,
		Reason:            "manual refund",
		ReceiptEmail:      "user@example.com",
		IdempotencyKey:    "payrefund:" + refundID.String(),
	})
	if err != nil {
		t.Fatalf("create refund: %v", err)
	}
	if result.ProviderRefundID != "refund-1" || result.Status != domain.PaymentRefundSucceeded || result.Amount != 1050 {
		t.Fatalf("unexpected refund result: %#v", result)
	}
	if seen.IdempotencyKey != "payrefund:"+refundID.String() {
		t.Fatalf("idempotency key = %q", seen.IdempotencyKey)
	}
	if seen.Body.PaymentID != "pay-1" || seen.Body.Amount.Value != "10.50" || seen.Body.Description != "manual refund" {
		t.Fatalf("unexpected refund request: %#v", seen.Body)
	}
	if seen.Body.Receipt.Customer.Email != "user@example.com" ||
		len(seen.Body.Receipt.Items) != 1 ||
		seen.Body.Receipt.Items[0].Amount.Value != "10.50" {
		t.Fatalf("unexpected refund receipt: %#v", seen.Body.Receipt)
	}
}

func TestParseWebhookUsesPaymentAndRefundDedupKeys(t *testing.T) {
	provider := newTestProvider(t, "https://example.com")
	paymentEvent, err := provider.ParseWebhook(
		context.Background(),
		[]byte(`{"type":"notification","event":"payment.succeeded","object":{"id":"pay-1","status":"succeeded"}}`),
		http.Header{},
	)
	if err != nil {
		t.Fatalf("parse payment webhook: %v", err)
	}
	if paymentEvent.Provider != domain.PaymentProviderYooKassa ||
		paymentEvent.ProviderPaymentID != "pay-1" ||
		paymentEvent.DedupKey != "webhook:yookassa:payment.succeeded:pay-1" {
		t.Fatalf("unexpected payment event: %#v", paymentEvent)
	}

	refundEvent, err := provider.ParseWebhook(
		context.Background(),
		[]byte(`{"type":"notification","event":"refund.succeeded","object":{"id":"refund-1","payment_id":"pay-1","status":"succeeded"}}`),
		http.Header{},
	)
	if err != nil {
		t.Fatalf("parse refund webhook: %v", err)
	}
	if refundEvent.ProviderPaymentID != "pay-1" ||
		refundEvent.ProviderRefundID != "refund-1" ||
		refundEvent.DedupKey != "webhook:yookassa:refund.succeeded:refund-1" {
		t.Fatalf("unexpected refund event: %#v", refundEvent)
	}
}

func TestKopeckConversion(t *testing.T) {
	for _, tc := range []struct {
		kopecks int64
		value   string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{105, "1.05"},
		{10000, "100.00"},
	} {
		got, err := formatKopecks(tc.kopecks)
		if err != nil {
			t.Fatalf("format %d: %v", tc.kopecks, err)
		}
		if got != tc.value {
			t.Fatalf("format %d = %q, want %q", tc.kopecks, got, tc.value)
		}
		parsed, err := parseKopecks(tc.value)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.value, err)
		}
		if parsed != tc.kopecks {
			t.Fatalf("parse %q = %d, want %d", tc.value, parsed, tc.kopecks)
		}
	}
	if _, err := parseKopecks("1.234"); err == nil {
		t.Fatal("expected invalid precision error")
	}
}

func newTestProvider(t *testing.T, baseURL string) *Provider {
	t.Helper()
	provider, err := New(Config{
		ShopID:    "shop-1",
		SecretKey: "secret",
		BaseURL:   baseURL,
		ReturnURL: "https://neiirohub.ru/payments/return",
		HTTPClient: &http.Client{
			Transport: http.DefaultTransport,
		},
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	return provider
}
