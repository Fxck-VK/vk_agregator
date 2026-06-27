package apimart

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBalanceCheckerRequestsUserBalanceAndParsesResponse(t *testing.T) {
	var seenMethod, seenPath, seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"remain_balance": 88.58,
			"remain_credits": 885.8,
			"used_balance": 12.34,
			"used_credits": 123.4
		}`))
	}))
	defer server.Close()

	checker := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	balance, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want %q", seenMethod, http.MethodGet)
	}
	if seenPath != "/v1/user/balance" {
		t.Fatalf("path = %q, want /v1/user/balance", seenPath)
	}
	if seenAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want Bearer test-key", seenAuth)
	}
	if balance.Provider != "apimart" {
		t.Fatalf("Provider = %q, want apimart", balance.Provider)
	}
	if balance.RemainBalance != 88.58 || balance.RemainCredits != 885.8 || balance.UsedBalance != 12.34 || balance.UsedCredits != 123.4 {
		t.Fatalf("unexpected parsed balance: %+v", balance)
	}
	if balance.CheckedAt.IsZero() {
		t.Fatalf("CheckedAt must be set")
	}
}

func TestBalanceCheckerRejectsUnsuccessfulEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": false}`))
	}))
	defer server.Close()

	checker := New(Config{APIKey: "test-key", BaseURL: server.URL + "/v1"})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "success=false") {
		t.Fatalf("expected success=false error, got %v", err)
	}
}

func TestBalanceCheckerRejectsHTTPUnauthorizedWithoutLeakingAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	const apiKey = "secret-test-key"
	checker := New(Config{APIKey: apiKey, BaseURL: server.URL + "/v1"})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected HTTP 401 error, got %v", err)
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("error leaked API key: %v", err)
	}
}

func TestBalanceCheckerRejectsInvalidJSONWithoutLeakingAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{`))
	}))
	defer server.Close()

	const apiKey = "secret-test-key"
	checker := New(Config{APIKey: apiKey, BaseURL: server.URL + "/v1"})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got %v", err)
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("error leaked API key: %v", err)
	}
}

func TestBalanceCheckerReturnsContextCancellationWithoutLeakingAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	const apiKey = "secret-test-key"
	checker := New(Config{APIKey: apiKey, BaseURL: server.URL + "/v1"})
	_, err := checker.Check(ctx)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("error leaked API key: %v", err)
	}
}
