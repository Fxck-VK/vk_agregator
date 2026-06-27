package poyo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBalanceCheckerRequestsUserBalanceAndParsesCredits(t *testing.T) {
	var seenMethod, seenPath, seenAuth string
	testEmail := "owner" + "@" + "example.test"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"code": 200,
			"data": {
				"email": %q,
				"credits_amount": 17276
			}
		}`, testEmail)
	}))
	defer server.Close()

	checker := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	balance, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want %q", seenMethod, http.MethodGet)
	}
	if seenPath != "/api/user/balance" {
		t.Fatalf("path = %q, want /api/user/balance", seenPath)
	}
	if seenAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want Bearer test-key", seenAuth)
	}
	if balance.Provider != "poyo" {
		t.Fatalf("Provider = %q, want poyo", balance.Provider)
	}
	if balance.RemainCredits != 17276 {
		t.Fatalf("RemainCredits = %v, want 17276", balance.RemainCredits)
	}
	if balance.RemainBalance != 0 || balance.UsedBalance != 0 || balance.UsedCredits != 0 {
		t.Fatalf("unexpected non-credit fields: %+v", balance)
	}
	if strings.Contains(fmt.Sprintf("%+v", balance), testEmail) {
		t.Fatalf("balance leaked email: %+v", balance)
	}
	if balance.CheckedAt.IsZero() {
		t.Fatalf("CheckedAt must be set")
	}
}

func TestBalanceCheckerRejectsNonOKEnvelopeWithoutLeakingSecrets(t *testing.T) {
	testEmail := "owner" + "@" + "example.test"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"code": 401,
			"data": {
				"email": %q,
				"credits_amount": 17276
			}
		}`, testEmail)
	}))
	defer server.Close()

	const apiKey = "secret-test-key"
	checker := New(Config{APIKey: apiKey, BaseURL: server.URL})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "code=401") {
		t.Fatalf("expected code=401 error, got %v", err)
	}
	assertErrorDoesNotLeak(t, err, apiKey, "Bearer", server.URL, testEmail)
}

func TestBalanceCheckerRejectsHTTPUnauthorizedWithoutLeakingSecrets(t *testing.T) {
	testEmail := "owner" + "@" + "example.test"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintf(w, `{"code":401,"data":{"email":%q}}`, testEmail)
	}))
	defer server.Close()

	const apiKey = "secret-test-key"
	checker := New(Config{APIKey: apiKey, BaseURL: server.URL})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected HTTP 401 error, got %v", err)
	}
	assertErrorDoesNotLeak(t, err, apiKey, "Bearer", server.URL, testEmail)
}

func TestBalanceCheckerRejectsInvalidJSONWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{`))
	}))
	defer server.Close()

	const apiKey = "secret-test-key"
	checker := New(Config{APIKey: apiKey, BaseURL: server.URL})
	_, err := checker.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got %v", err)
	}
	assertErrorDoesNotLeak(t, err, apiKey, "Bearer", server.URL)
}

func TestBalanceCheckerReturnsContextCancellationWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	const apiKey = "secret-test-key"
	checker := New(Config{APIKey: apiKey, BaseURL: server.URL})
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
