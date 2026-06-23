// Package ratelimit provides a small in-memory token-bucket rate limiter and an
// HTTP middleware that throttles inbound requests per source key (typically the
// client IP). It is an edge-protection primitive (audit S3): it bounds webhook
// flooding and abuse without an external dependency. Buckets have bounded
// retention and a hard per-process cap, so high-cardinality keys cannot grow
// memory forever. For multi-instance deployments this should be fronted by a
// shared limiter (e.g. Redis) or an ingress/WAF, but it provides a meaningful
// per-instance ceiling on its own.
package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	defaultBucketTTL       = 10 * time.Minute
	defaultMaxBuckets      = 100_000
	maxSweepChecksPerAllow = 64
)

// bucket is a single token bucket.
type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// Limiter is a per-key token-bucket limiter safe for concurrent use.
type Limiter struct {
	mu          sync.Mutex
	buckets     map[string]*bucket
	keys        []string
	sweepCursor int
	rate        float64 // tokens per second
	burst       float64 // bucket capacity
	bucketTTL   time.Duration
	maxBuckets  int
	now         func() time.Time
}

// Option customizes a Limiter.
type Option func(*Limiter)

// ConcurrencyLimiter bounds simultaneous in-flight work in one process. It is
// intended for per-instance memory protection, not as a global quota.
type ConcurrencyLimiter struct {
	sem chan struct{}
}

// New builds a Limiter allowing `rate` requests/second with a `burst` ceiling.
func New(rate float64, burst int, opts ...Option) *Limiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = 1
	}
	l := &Limiter{
		buckets:    make(map[string]*bucket),
		rate:       rate,
		burst:      float64(burst),
		bucketTTL:  defaultBucketTTL,
		maxBuckets: defaultMaxBuckets,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(l)
	}
	if l.bucketTTL <= 0 {
		l.bucketTTL = defaultBucketTTL
	}
	if l.maxBuckets <= 0 {
		l.maxBuckets = defaultMaxBuckets
	}
	if l.now == nil {
		l.now = time.Now
	}
	return l
}

// WithBucketTTL sets how long an idle key bucket is retained.
func WithBucketTTL(ttl time.Duration) Option {
	return func(l *Limiter) {
		l.bucketTTL = ttl
	}
}

// WithMaxBuckets sets a hard cap on the number of buckets retained per process.
func WithMaxBuckets(n int) Option {
	return func(l *Limiter) {
		l.maxBuckets = n
	}
}

// WithClock injects a clock for deterministic tests.
func WithClock(now func() time.Time) Option {
	return func(l *Limiter) {
		l.now = now
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
	l.evictExpired(now)
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxBuckets {
			return false
		}
		l.buckets[key] = &bucket{tokens: l.burst - 1, lastSeen: now}
		l.keys = append(l.keys, key)
		return true
	}
	if l.expired(b, now) {
		b.tokens = l.burst - 1
		b.lastSeen = now
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

func (l *Limiter) evictExpired(now time.Time) {
	if l.bucketTTL <= 0 || len(l.keys) == 0 {
		return
	}
	limit := maxSweepChecksPerAllow
	if len(l.keys) < limit {
		limit = len(l.keys)
	}
	for checked := 0; checked < limit && len(l.keys) > 0; checked++ {
		if l.sweepCursor >= len(l.keys) {
			l.sweepCursor = 0
		}
		key := l.keys[l.sweepCursor]
		b, ok := l.buckets[key]
		if !ok || l.expired(b, now) {
			if ok {
				delete(l.buckets, key)
			}
			last := len(l.keys) - 1
			l.keys[l.sweepCursor] = l.keys[last]
			l.keys = l.keys[:last]
			if l.sweepCursor >= len(l.keys) {
				l.sweepCursor = 0
			}
			continue
		}
		l.sweepCursor++
	}
}

func (l *Limiter) expired(b *bucket, now time.Time) bool {
	return l.bucketTTL > 0 && now.Sub(b.lastSeen) > l.bucketTTL
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
