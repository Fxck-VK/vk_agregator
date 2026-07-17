package accountdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/service/accountlink"
)

func TestDisabledSenderFailsClosed(t *testing.T) {
	sender, err := NewSender(Config{})
	if err != nil {
		t.Fatalf("new sender: %v", err)
	}
	if err := sender.SendEmailLinkCode(context.Background(), "user@example.com", "123456", time.Now()); !errors.Is(err, accountlink.ErrDeliveryUnavailable) {
		t.Fatalf("email disabled error = %v, want delivery unavailable", err)
	}
	if err := sender.SendPhoneLinkOTP(context.Background(), "+79991234567", "123456", time.Now()); !errors.Is(err, accountlink.ErrSMSDeliveryUnavailable) {
		t.Fatalf("phone disabled error = %v, want sms delivery unavailable", err)
	}
}

func TestHTTPPhoneSenderSendsTemplatedOTP(t *testing.T) {
	var gotAuth string
	var gotPayload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		gotAuth = r.Header.Get("X-Test-Auth")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sender, err := NewSender(Config{
		PhoneProvider: ProviderHTTP,
		PhoneHTTP: HTTPPhoneConfig{
			URL:             server.URL,
			AuthHeaderName:  "X-Test-Auth",
			AuthHeaderValue: "secret",
			BodyTemplate:    `{"to":"{{phone}}","otp":"{{code}}","expires":"{{expires_at}}"}`,
		},
	})
	if err != nil {
		t.Fatalf("new sender: %v", err)
	}
	expiresAt := time.Date(2026, 7, 5, 12, 30, 0, 0, time.UTC)
	if err := sender.SendPhoneLinkOTP(context.Background(), "+79991234567", "654321", expiresAt); err != nil {
		t.Fatalf("send otp: %v", err)
	}
	if gotAuth != "secret" {
		t.Fatalf("auth header = %q, want secret", gotAuth)
	}
	if gotPayload["to"] != "+79991234567" || gotPayload["otp"] != "654321" || gotPayload["expires"] != expiresAt.Format(time.RFC3339) {
		t.Fatalf("payload = %+v", gotPayload)
	}
}

func TestHTTPPhoneSenderRejectsUnsafeTemplate(t *testing.T) {
	_, err := NewSender(Config{
		PhoneProvider: ProviderHTTP,
		PhoneHTTP: HTTPPhoneConfig{
			URL:          "https://sms.example.test/send",
			BodyTemplate: `{"to":"{{phone}}"}`,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "{{phone}} and {{code}}") {
		t.Fatalf("error = %v, want missing placeholder validation", err)
	}
}

func TestBuildEmailMessageSanitizesHeaders(t *testing.T) {
	msg := string(buildEmailMessage(
		"noreply@example.com\r\nBcc: attacker@example.com",
		"user@example.com",
		"Subject\r\nInjected: true",
		"123456",
		time.Date(2026, 7, 5, 12, 30, 0, 0, time.UTC),
	))
	if strings.Contains(msg, "\r\nBcc:") || strings.Contains(msg, "\r\nInjected:") {
		t.Fatalf("message contains injected header: %q", msg)
	}
	if !strings.Contains(msg, "123456") || !strings.Contains(msg, "2026-07-05T12:30:00Z") {
		t.Fatalf("message missing code/expiry: %q", msg)
	}
}
