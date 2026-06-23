package paymentredirect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

type fakePaymentService struct {
	intent *domain.PaymentIntent
	err    error
	calls  *int
}

func (f fakePaymentService) GetIntentAdmin(_ context.Context, _ uuid.UUID) (*domain.PaymentIntent, error) {
	if f.calls != nil {
		*f.calls = *f.calls + 1
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.intent, nil
}

type fakeRateLimiter struct {
	allowed []bool
	keys    []string
}

func (f *fakeRateLimiter) Allow(key string) bool {
	f.keys = append(f.keys, key)
	if len(f.allowed) == 0 {
		return true
	}
	allowed := f.allowed[0]
	f.allowed = f.allowed[1:]
	return allowed
}

func TestVKPaymentRedirectRequiresWaitingVKBotIntent(t *testing.T) {
	intentID := uuid.New()
	metadata, _ := json.Marshal(map[string]string{"source": "vk_bot"})
	handler := NewHandler(Deps{Payment: fakePaymentService{intent: &domain.PaymentIntent{
		ID:              intentID,
		Status:          domain.PaymentIntentWaitingForUser,
		ConfirmationURL: "https://payments.example/continue",
		Metadata:        metadata,
	}}})

	req := httptest.NewRequest(http.MethodGet, "/payments/vk/"+intentID.String(), nil)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "https://payments.example/continue" {
		t.Fatalf("Location = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	if got := rec.Header().Get("X-Robots-Tag"); got != "noindex, nofollow" {
		t.Fatalf("X-Robots-Tag = %q", got)
	}
}

func TestVKPaymentRedirectRejectsInvalidUUIDBeforeLookup(t *testing.T) {
	var calls int
	handler := NewHandler(Deps{Payment: fakePaymentService{calls: &calls}})

	req := httptest.NewRequest(http.MethodGet, "/payments/vk/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if calls != 0 {
		t.Fatalf("payment lookups = %d, want 0", calls)
	}
}

func TestVKPaymentRedirectRejectsUnknownIntent(t *testing.T) {
	var calls int
	handler := NewHandler(Deps{Payment: fakePaymentService{err: domain.ErrNotFound, calls: &calls}})

	req := httptest.NewRequest(http.MethodGet, "/payments/vk/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if calls != 1 {
		t.Fatalf("payment lookups = %d, want 1", calls)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want empty", got)
	}
}

func TestVKPaymentRedirectRejectsOtherSources(t *testing.T) {
	intentID := uuid.New()
	metadata, _ := json.Marshal(map[string]string{"source": "vk_miniapp"})
	handler := NewHandler(Deps{Payment: fakePaymentService{intent: &domain.PaymentIntent{
		ID:              intentID,
		Status:          domain.PaymentIntentWaitingForUser,
		ConfirmationURL: "https://payments.example/continue",
		Metadata:        metadata,
	}}})

	req := httptest.NewRequest(http.MethodGet, "/payments/vk/"+intentID.String(), nil)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestVKPaymentRedirectRejectsNonWaitingIntent(t *testing.T) {
	intentID := uuid.New()
	metadata, _ := json.Marshal(map[string]string{"source": "vk_bot"})
	handler := NewHandler(Deps{Payment: fakePaymentService{intent: &domain.PaymentIntent{
		ID:              intentID,
		Status:          domain.PaymentIntentSucceeded,
		ConfirmationURL: "https://payments.example/continue",
		Metadata:        metadata,
	}}})

	req := httptest.NewRequest(http.MethodGet, "/payments/vk/"+intentID.String(), nil)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
}

func TestVKPaymentRedirectRejectsExpiredIntent(t *testing.T) {
	intentID := uuid.New()
	metadata, _ := json.Marshal(map[string]string{"source": "vk_bot"})
	expired := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	handler := NewHandler(Deps{Payment: fakePaymentService{intent: &domain.PaymentIntent{
		ID:              intentID,
		Status:          domain.PaymentIntentWaitingForUser,
		ConfirmationURL: "https://payments.example/continue",
		Metadata:        metadata,
		ExpiresAt:       &expired,
	}}})
	handler.now = func() time.Time { return expired.Add(time.Second) }

	req := httptest.NewRequest(http.MethodGet, "/payments/vk/"+intentID.String(), nil)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want empty", got)
	}
}

func TestVKPaymentRedirectRejectsUnsafeTarget(t *testing.T) {
	intentID := uuid.New()
	metadata, _ := json.Marshal(map[string]string{"source": "vk_bot"})
	handler := NewHandler(Deps{Payment: fakePaymentService{intent: &domain.PaymentIntent{
		ID:              intentID,
		Status:          domain.PaymentIntentWaitingForUser,
		ConfirmationURL: "http://payments.example/continue",
		Metadata:        metadata,
	}}})

	req := httptest.NewRequest(http.MethodGet, "/payments/vk/"+intentID.String(), nil)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
}

func TestVKPaymentRedirectRateLimitsBeforeSecondLookup(t *testing.T) {
	intentID := uuid.New()
	metadata, _ := json.Marshal(map[string]string{"source": "vk_bot"})
	var calls int
	limiter := &fakeRateLimiter{allowed: []bool{true, false}}
	handler := NewHandler(Deps{
		Payment: fakePaymentService{intent: &domain.PaymentIntent{
			ID:              intentID,
			Status:          domain.PaymentIntentWaitingForUser,
			ConfirmationURL: "https://payments.example/continue",
			Metadata:        metadata,
		}, calls: &calls},
		RateLimiter: limiter,
	})

	req := httptest.NewRequest(http.MethodGet, "/payments/vk/"+intentID.String(), nil)
	req.RemoteAddr = "198.51.100.10:1234"
	first := httptest.NewRecorder()
	handler.Routes().ServeHTTP(first, req)
	if first.Code != http.StatusSeeOther {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/payments/vk/"+intentID.String(), nil)
	req.RemoteAddr = "198.51.100.10:5678"
	req.Header.Set("X-Forwarded-For", "203.0.113.99")
	second := httptest.NewRecorder()
	handler.Routes().ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if calls != 1 {
		t.Fatalf("payment lookups = %d, want 1", calls)
	}
	if got := second.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want 1", got)
	}
	if len(limiter.keys) != 2 || limiter.keys[0] != "198.51.100.10" || limiter.keys[1] != "198.51.100.10" {
		t.Fatalf("limiter keys = %+v", limiter.keys)
	}
}
