package yookassa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestNPDManualPaymentsAndRefundsOmitFiscalReceipt(t *testing.T) {
	var bodies []map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/payments" {
			_, _ = w.Write([]byte(`{"id":"fixture-payment","status":"pending","amount":{"value":"10.00","currency":"RUB"},"confirmation":{"confirmation_url":"https://yookassa.ru/fixture"}}`))
		} else if r.URL.Path == "/refunds" {
			_, _ = w.Write([]byte(`{"id":"fixture-refund","payment_id":"fixture-payment","status":"succeeded","amount":{"value":"10.00","currency":"RUB"}}`))
		} else {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	p, err := New(Config{ShopID: "synthetic", SecretKey: "test_synthetic", BaseURL: server.URL, ReturnURL: "https://app.example.test/return", ReceiptMode: "npd_manual"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.CreatePayment(context.Background(), domain.CreatePaymentInput{IntentID: uuid.New(), Amount: 1000, Currency: domain.CurrencyRUB, IdempotencyKey: uuid.NewString()}); err == nil {
		t.Fatal("manual NPD mode bypassed the verified buyer contact requirement")
	}
	if len(bodies) != 0 {
		t.Fatal("invalid contact reached the provider")
	}
	for _, recurring := range []bool{false, true} {
		input := domain.CreatePaymentInput{IntentID: uuid.New(), AccountID: uuid.New(), Amount: 1000, Currency: domain.CurrencyRUB, ReceiptEmail: "buyer@example.test", IdempotencyKey: uuid.NewString()}
		if recurring {
			input.PaymentMethodID = "synthetic-saved-method"
		}
		if _, err := p.CreatePayment(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.CreateRefund(context.Background(), domain.CreateRefundInput{ProviderPaymentID: "fixture-payment", Amount: 1000, Currency: domain.CurrencyRUB, ReceiptEmail: "buyer@example.test", IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 3 {
		t.Fatalf("requests=%d", len(bodies))
	}
	for _, body := range bodies {
		if _, present := body["receipt"]; present {
			t.Fatal("NPD manual request unexpectedly contains a 54-FZ fiscal receipt")
		}
		if _, present := body["amount"]; !present {
			t.Fatal("payment amount lost")
		}
	}
}

func TestUnknownReceiptModeRejected(t *testing.T) {
	if _, err := New(Config{ShopID: "synthetic", SecretKey: "test_synthetic", ReturnURL: "https://app.example.test/return", ReceiptMode: "unknown"}); err == nil {
		t.Fatal("unknown receipt policy accepted")
	}
}
