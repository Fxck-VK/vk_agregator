package yookassa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

type metadataRecoveryTransport func(*http.Request) (*http.Response, error)

func (f metadataRecoveryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGetPaymentNormalizesVerifiedBindingOnlyFromGetMetadata(t *testing.T) {
	intent, owner := uuid.New(), uuid.New()
	cases := []struct {
		name, metadata string
		valid          bool
	}{
		{"authoritative", fmt.Sprintf(`{"intent_id":%q,"account_id":%q}`, intent.String(), owner.String()), true},
		{"absent", `{}`, false},
		{"malformed", `{"intent_id":"not-a-uuid","account_id":42}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: metadataRecoveryTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet || r.URL.Path != "/v3/payments/accepted" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				body := fmt.Sprintf(`{"id":"accepted","status":"succeeded","paid":true,"amount":{"value":"199.00","currency":"RUB"},"metadata":%s}`, tc.metadata)
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			provider, err := New(Config{ShopID: "test-shop", SecretKey: "test_key", ReturnURL: "https://epifera.test/return", BaseURL: "https://payments.invalid/v3", HTTPClient: client})
			if err != nil {
				t.Fatal(err)
			}
			verified, err := provider.GetPayment(context.Background(), "accepted")
			if err != nil {
				t.Fatal(err)
			}
			if tc.valid {
				if verified.VerifiedIntentID != intent || verified.VerifiedAccountID != owner {
					t.Fatal("GET metadata binding lost")
				}
			} else if verified.VerifiedIntentID != uuid.Nil || verified.VerifiedAccountID != uuid.Nil {
				t.Fatal("invalid metadata became verified binding")
			}
			verified.Raw = nil
			raw, err := json.Marshal(verified)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), intent.String()) || strings.Contains(string(raw), owner.String()) || strings.Contains(string(raw), "verified_") {
				t.Fatal("private recovery identifiers serialized")
			}
		})
	}
}

func TestPaymentMetadataOmitsPrivateSubscriptionState(t *testing.T) {
	intent, owner := uuid.New(), uuid.New()
	metadata := metadataForPayment(domain.CreatePaymentInput{IntentID: intent, AccountID: owner, Credits: 400,
		Metadata: json.RawMessage(`{"source":"web","custom":"kept","subscription":{"plan":{"code":"lite"}},"subscription_submit_started_at":"2026-10-27T10:00:00Z","subscription_submit_attempt_id":"private-attempt","subscription_payment_method_id":"private-saved","payment_method_id":"private-method","save_payment_method":true,"intent_id":"forged","account_id":"forged"}`)})
	for _, private := range []string{"subscription", "subscription_submit_started_at", "subscription_submit_attempt_id", "subscription_payment_method_id", "payment_method_id", "save_payment_method"} {
		if _, ok := metadata[private]; ok {
			t.Errorf("private subscription field forwarded: %s", private)
		}
	}
	if metadata["intent_id"] != intent.String() || metadata["account_id"] != owner.String() || metadata["custom"] != "kept" || metadata["source"] != "web" || metadata["credits"] != int64(400) {
		t.Fatal("authoritative/legacy metadata changed")
	}
}
