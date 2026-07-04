package redis

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"vk-ai-aggregator/internal/service/accountlink"
)

var incrementWithTTL = goredis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

// AccountLinkStore persists short-lived account-link challenges in Redis.
type AccountLinkStore struct {
	client goredis.Cmdable
}

// NewAccountLinkStore creates a Redis-backed account-link store.
func NewAccountLinkStore(client goredis.Cmdable) *AccountLinkStore {
	return &AccountLinkStore{client: client}
}

func (s *AccountLinkStore) SaveChallenge(ctx context.Context, key string, challenge accountlink.Challenge, ttl time.Duration) error {
	body, err := json.Marshal(challenge)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, body, ttl).Err()
}

func (s *AccountLinkStore) LoadChallenge(ctx context.Context, key string) (accountlink.Challenge, error) {
	body, err := s.client.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return accountlink.Challenge{}, accountlink.ErrInvalidCode
	}
	if err != nil {
		return accountlink.Challenge{}, err
	}
	var challenge accountlink.Challenge
	if err := json.Unmarshal(body, &challenge); err != nil {
		return accountlink.Challenge{}, err
	}
	return challenge, nil
}

func (s *AccountLinkStore) DeleteChallenge(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *AccountLinkStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	ttlMillis := ttl.Milliseconds()
	if ttlMillis <= 0 {
		ttlMillis = int64(time.Minute / time.Millisecond)
	}
	return incrementWithTTL.Run(ctx, s.client, []string{key}, ttlMillis).Int64()
}
