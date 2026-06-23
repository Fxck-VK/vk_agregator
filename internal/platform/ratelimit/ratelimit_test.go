package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterCapsBucketGrowth(t *testing.T) {
	limiter := New(100, 1, WithMaxBuckets(3), WithBucketTTL(time.Hour))

	for _, key := range []string{"user-1", "user-2", "user-3"} {
		if !limiter.Allow(key) {
			t.Fatalf("Allow(%q) = false, want true", key)
		}
	}
	if limiter.Allow("user-4") {
		t.Fatal("Allow(user-4) = true, want false after max buckets")
	}
	if got := len(limiter.buckets); got != 3 {
		t.Fatalf("bucket count = %d, want 3", got)
	}
}

func TestLimiterEvictsExpiredBucketsBeforeDenyingNewKeys(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	limiter := New(100, 1,
		WithMaxBuckets(3),
		WithBucketTTL(time.Minute),
		WithClock(func() time.Time { return now }),
	)

	for _, key := range []string{"user-1", "user-2", "user-3"} {
		if !limiter.Allow(key) {
			t.Fatalf("Allow(%q) = false, want true", key)
		}
	}
	now = now.Add(time.Minute + time.Nanosecond)

	if !limiter.Allow("user-4") {
		t.Fatal("Allow(user-4) = false, want true after expired buckets are evicted")
	}
	if got := len(limiter.buckets); got > 3 {
		t.Fatalf("bucket count = %d, want <= 3", got)
	}
	if _, ok := limiter.buckets["user-4"]; !ok {
		t.Fatal("user-4 bucket was not retained")
	}
}

func TestLimiterResetsExpiredExistingBucket(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	limiter := New(1, 1,
		WithMaxBuckets(1),
		WithBucketTTL(time.Minute),
		WithClock(func() time.Time { return now }),
	)

	if !limiter.Allow("user-1") {
		t.Fatal("first Allow(user-1) = false, want true")
	}
	if limiter.Allow("user-1") {
		t.Fatal("second Allow(user-1) = true, want false before refill")
	}
	now = now.Add(time.Minute + time.Nanosecond)
	if !limiter.Allow("user-1") {
		t.Fatal("Allow(user-1) = false, want true after TTL")
	}
	if got := len(limiter.buckets); got != 1 {
		t.Fatalf("bucket count = %d, want 1", got)
	}
}

func TestMiddlewareIgnoresSpoofedXForwardedFor(t *testing.T) {
	limiter := New(1, 1)
	var hits int
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRequest(http.MethodPost, "/", nil)
	first.RemoteAddr = "198.51.100.10:1000"
	first.Header.Set("X-Forwarded-For", "203.0.113.1")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", firstRec.Code, http.StatusNoContent)
	}

	second := httptest.NewRequest(http.MethodPost, "/", nil)
	second.RemoteAddr = "198.51.100.10:1000"
	second.Header.Set("X-Forwarded-For", "203.0.113.2")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", secondRec.Code, http.StatusTooManyRequests)
	}
	if hits != 1 {
		t.Fatalf("handler hits = %d, want 1", hits)
	}
}
