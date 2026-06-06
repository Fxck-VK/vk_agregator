package redisqueue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"vk-ai-aggregator/internal/service/antispam"
)

// AntiSpamStore persists user-level anti-spam counters in Redis.
type AntiSpamStore struct {
	client redis.Cmdable
}

// NewAntiSpamStore builds the Redis implementation used by the VK API.
func NewAntiSpamStore(client redis.Cmdable) *AntiSpamStore {
	return &AntiSpamStore{client: client}
}

var _ antispam.Store = (*AntiSpamStore)(nil)

var incrWithTTLScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {current, ttl}
`)

// Increment increments a key and applies the window as TTL on first hit.
func (s *AntiSpamStore) Increment(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	if window <= 0 {
		return 0, 0, fmt.Errorf("redisqueue: antispam window must be positive")
	}
	res, err := incrWithTTLScript.Run(ctx, s.client, []string{key}, int64(window/time.Millisecond)).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("redisqueue: antispam incr %s: %w", key, err)
	}
	values, ok := res.([]any)
	if !ok || len(values) != 2 {
		return 0, 0, fmt.Errorf("redisqueue: antispam incr %s returned %T", key, res)
	}
	count, ok := values[0].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("redisqueue: antispam count %s returned %T", key, values[0])
	}
	ttlMillis, ok := values[1].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("redisqueue: antispam ttl %s returned %T", key, values[1])
	}
	return count, ttlFromMillis(ttlMillis), nil
}

// TTL returns the remaining key TTL, or zero when the key is missing or has no
// expiry.
func (s *AntiSpamStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := s.client.PTTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redisqueue: antispam ttl %s: %w", key, err)
	}
	if ttl < 0 {
		return 0, nil
	}
	return ttl, nil
}

// SetTTL stores a marker key with the requested TTL.
func (s *AntiSpamStore) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	if err := s.client.Set(ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("redisqueue: antispam set ttl %s: %w", key, err)
	}
	return nil
}

func ttlFromMillis(ms int64) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}
