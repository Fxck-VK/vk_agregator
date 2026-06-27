package deepinfra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBalanceCheckerRequestsChecklistAndParsesStripeBalance(t *testing.T) {
	var seenMethod, seenPath, seenRawQuery, seenAuth, seenRequestURI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenRawQuery = r.URL.RawQuery
		seenAuth = r.Header.Get("Authorization")
		seenRequestURI = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stripe_balance":-25.5,"recent":3.5,"limit":500,"suspended":false}`))
	}))
	defer server.Close()

	checker := New(Config{APIKey: "x", BalanceBaseURL: server.URL})
	balance, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want %q", seenMethod, http.MethodGet)
	}
	if seenPath != "/payment/checklist" {
		t.Fatalf("path = %q, want /payment/checklist", seenPath)
	}
	if seenRawQuery != "" {
		t.Fatalf("query = %q, want empty", seenRawQuery)
	}
	if strings.Contains(seenRequestURI, "x") {
		t.Fatalf("request URI leaked API key: %s", seenRequestURI)
	}
	if seenAuth != "Bearer x" {
		t.Fatalf("Authorization = %q, want Bearer x", seenAuth)
	}
	if balance.Provider != "deepinfra" {
		t.Fatalf("Provider = %q, want deepinfra", balance.Provider)
	}
	if balance.RemainBalance != 22 {
		t.Fatalf("RemainBalance = %v, want 22", balance.RemainBalance)
	}
	if balance.UsedBalance != 3.5 {
		t.Fatalf("UsedBalance = %v, want 3.5", balance.UsedBalance)
	}
	if balance.RemainCredits != 0 || balance.UsedCredits != 0 {
		t.Fatalf("unexpected credit fields: %+v", balance)
	}
	if balance.CheckedAt.IsZero() {
		t.Fatal("CheckedAt must be set")
	}
}

func TestBalanceCheckerParsesDebtAsNegativeBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stripe_balance":7.25,"recent":0}`))
	}))
	defer server.Close()

	checker := New(Config{APIKey: "x", BalanceBaseURL: server.URL})
	balance, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if balance.RemainBalance != -7.25 {
		t.Fatalf("RemainBalance = %v, want -7.25", balance.RemainBalance)
	}
}

func TestBalanceCheckerSubtractsRecentUsageFromDebt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stripe_balance":7.25,"recent":2.75}`))
	}))
	defer server.Close()

	checker := New(Config{APIKey: "x", BalanceBaseURL: server.URL})
	balance, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if balance.RemainBalance != -10 {
		t.Fatalf("RemainBalance = %v, want -10", balance.RemainBalance)
	}
	if balance.UsedBalance != 2.75 {
		t.Fatalf("UsedBalance = %v, want 2.75", balance.UsedBalance)
	}
}

func TestBalanceCheckerRejectsHTTPUnauthorizedWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"body-marker","billing_address":"addr-marker","payment_method":"card-marker"}`))
	}))
	defer server.Close()

	const apiKey = "zz"
	checker := New(Config{APIKey: apiKey, BalanceBaseURL: server.URL})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected HTTP 401 error, got %v", err)
	}
	assertErrorDoesNotLeak(t, err, apiKey, "Bearer", server.URL, "body-marker", "addr-marker", "card-marker")
}

func TestBalanceCheckerRejectsInvalidJSONWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{`))
	}))
	defer server.Close()

	const apiKey = "zz"
	checker := New(Config{APIKey: apiKey, BalanceBaseURL: server.URL})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got %v", err)
	}
	assertErrorDoesNotLeak(t, err, apiKey, "Bearer", server.URL)
}

func TestBalanceCheckerRejectsMissingStripeBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"recent":123}`))
	}))
	defer server.Close()

	checker := New(Config{APIKey: "x", BalanceBaseURL: server.URL})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "stripe_balance") {
		t.Fatalf("expected stripe_balance error, got %v", err)
	}
}

func TestBalanceCheckerReturnsContextCancellationWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	const apiKey = "zz"
	checker := New(Config{APIKey: apiKey, BalanceBaseURL: server.URL})
	_, err := checker.Check(ctx)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	assertErrorDoesNotLeak(t, err, apiKey, "Bearer", server.URL)
}

func assertErrorDoesNotLeak(t *testing.T, err error, forbidden ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, value := range forbidden {
		if value != "" && strings.Contains(err.Error(), value) {
			t.Fatalf("error leaked %q: %v", value, err)
		}
	}
}
