// Package ratelimit provides in-memory token-bucket limiters for per-process
// edge protection and Redis-backed fixed-window limiters for quotas that must
// be shared across API instances.
package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultBucketTTL       = 10 * time.Minute
	defaultMaxBuckets      = 100_000
	maxSweepChecksPerAllow = 64
	defaultSharedNamespace = "default"
)

var redisFixedWindowScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

// KeyLimiter is a shared per-key limiter used by authenticated surfaces where
// all API instances must enforce the same quota.
type KeyLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// RedisFixedWindowLimiter implements a simple shared fixed-window quota in
// Redis. It stores hashed keys only, so operator/user identifiers are not
// exposed in Redis key names.
type RedisFixedWindowLimiter struct {
	client    redis.Cmdable
	namespace string
	limit     int
	window    time.Duration
}

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

// NewRedisFixedWindowLimiter builds a Redis-backed fixed-window limiter.
func NewRedisFixedWindowLimiter(client redis.Cmdable, namespace string, limit int, window time.Duration) *RedisFixedWindowLimiter {
	namespace = safeNamespace(namespace)
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RedisFixedWindowLimiter{
		client:    client,
		namespace: namespace,
		limit:     limit,
		window:    window,
	}
}

// Allow increments a shared Redis counter and returns true while the current
// window count is within the configured limit.
func (l *RedisFixedWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	if l == nil || l.client == nil {
		return false, errors.New("redis rate limiter is not configured")
	}
	key = sharedKey(l.namespace, key)
	windowMillis := int64(l.window / time.Millisecond)
	if windowMillis <= 0 {
		windowMillis = 1
	}
	count, err := redisFixedWindowScript.Run(ctx, l.client, []string{key}, windowMillis).Int64()
	if err != nil {
		return false, err
	}
	return count <= int64(l.limit), nil
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

func safeNamespace(namespace string) string {
	out := make([]byte, 0, len(namespace))
	for i := 0; i < len(namespace); i++ {
		ch := namespace[i]
		if (ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '_' || ch == '-' || ch == ':' {
			out = append(out, ch)
		}
	}
	if len(out) == 0 {
		return defaultSharedNamespace
	}
	return string(out)
}

func sharedKey(namespace, key string) string {
	sum := sha256.Sum256([]byte(key))
	return "rate_limit:" + safeNamespace(namespace) + ":" + hex.EncodeToString(sum[:])
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
