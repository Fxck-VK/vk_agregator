package runway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBalanceCheckerRequestsOrganizationAndParsesCreditBalance(t *testing.T) {
	var seenMethod, seenPath, seenAuth, seenVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		seenVersion = r.Header.Get("X-Runway-Version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"creditBalance":1000,"usage":{"models":{}}}`))
	}))
	defer server.Close()

	checker := New(Config{APISecret: "test-secret", BaseURL: server.URL + "/v1"})
	balance, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want %q", seenMethod, http.MethodGet)
	}
	if seenPath != "/v1/organization" {
		t.Fatalf("path = %q, want /v1/organization", seenPath)
	}
	if seenAuth != "Bearer test-secret" {
		t.Fatalf("Authorization = %q, want Bearer test-secret", seenAuth)
	}
	if seenVersion != "2024-11-06" {
		t.Fatalf("X-Runway-Version = %q, want 2024-11-06", seenVersion)
	}
	if balance.Provider != "runway" {
		t.Fatalf("Provider = %q, want runway", balance.Provider)
	}
	if balance.RemainCredits != 1000 {
		t.Fatalf("RemainCredits = %v, want 1000", balance.RemainCredits)
	}
	if balance.RemainBalance != 0 || balance.UsedBalance != 0 || balance.UsedCredits != 0 {
		t.Fatalf("unexpected non-credit fields: %+v", balance)
	}
	if balance.CheckedAt.IsZero() {
		t.Fatal("CheckedAt must be set")
	}
}

func TestBalanceCheckerRejectsHTTPUnauthorizedWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"raw provider body must not be returned"}`))
	}))
	defer server.Close()

	const secret = "secret-runway-token"
	checker := New(Config{APISecret: secret, BaseURL: server.URL + "/v1"})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected HTTP 401 error, got %v", err)
	}
	assertErrorDoesNotLeak(t, err, secret, "Bearer", server.URL, "raw provider body")
}

func TestBalanceCheckerRejectsInvalidJSONWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{`))
	}))
	defer server.Close()

	const secret = "secret-runway-token"
	checker := New(Config{APISecret: secret, BaseURL: server.URL + "/v1"})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got %v", err)
	}
	assertErrorDoesNotLeak(t, err, secret, "Bearer", server.URL)
}

func TestBalanceCheckerReturnsContextCancellationWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	const secret = "secret-runway-token"
	checker := New(Config{APISecret: secret, BaseURL: server.URL + "/v1"})
	_, err := checker.Check(ctx)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	assertErrorDoesNotLeak(t, err, secret, "Bearer", server.URL)
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
