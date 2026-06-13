// Package ratelimit provides a small in-memory token-bucket rate limiter and an
// HTTP middleware that throttles inbound requests per source key (typically the
// client IP). It is an edge-protection primitive (audit S3): it bounds webhook
// flooding and abuse without an external dependency. For multi-instance
// deployments this should be fronted by a shared limiter (e.g. Redis) or an
// ingress/WAF, but it provides a meaningful per-instance ceiling on its own.
package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// bucket is a single token bucket.
type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// Limiter is a per-key token-bucket limiter safe for concurrent use.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64 // bucket capacity
	now     func() time.Time
}

// ConcurrencyLimiter bounds simultaneous in-flight work in one process. It is
// intended for per-instance memory protection, not as a global quota.
type ConcurrencyLimiter struct {
	sem chan struct{}
}

// New builds a Limiter allowing `rate` requests/second with a `burst` ceiling.
func New(rate float64, burst int) *Limiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   float64(burst),
		now:     time.Now,
	}
}

// NewConcurrencyLimiter builds a limiter for simultaneous work. n <= 0 disables
// limiting and returns nil.
func NewConcurrencyLimiter(n int) *ConcurrencyLimiter {
	if n <= 0 {
		return nil
	}
	return &ConcurrencyLimiter{sem: make(chan struct{}, n)}
}

// TryAcquire attempts to reserve one in-flight slot. The returned release
// function must be called when ok is true.
func (l *ConcurrencyLimiter) TryAcquire() (release func(), ok bool) {
	if l == nil {
		return func() {}, true
	}
	select {
	case l.sem <- struct{}{}:
		return func() { <-l.sem }, true
	default:
		return nil, false
	}
}

// Allow reports whether a request for the given key may proceed, consuming a
// token when it does.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.burst - 1, lastSeen: now}
		return true
	}
	// Refill based on elapsed time.
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.lastSeen = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Middleware throttles inbound requests per client IP, returning 429 when the
// limit is exceeded.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(clientIP(r)) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the socket peer IP from the request. Proxy headers are
// intentionally ignored here: callers that need proxy awareness must normalize
// the request only after verifying the proxy is trusted.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
