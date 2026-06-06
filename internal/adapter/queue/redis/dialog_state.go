package redisqueue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"vk-ai-aggregator/internal/service/dialogstate"
)

// DialogStateStore persists short-lived VK dialog mode state in Redis.
type DialogStateStore struct {
	client redis.Cmdable
}

// NewDialogStateStore builds the Redis implementation used by cmd/api.
func NewDialogStateStore(client redis.Cmdable) *DialogStateStore {
	return &DialogStateStore{client: client}
}

var _ dialogstate.Store = (*DialogStateStore)(nil)

// Get reads one dialog-state key.
func (s *DialogStateStore) Get(ctx context.Context, key string) (string, bool, error) {
	value, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("redisqueue: dialog state get %s: %w", key, err)
	}
	return value, true, nil
}

// Set writes one dialog-state key with TTL.
func (s *DialogStateStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	if err := s.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redisqueue: dialog state set %s: %w", key, err)
	}
	return nil
}

// Delete removes one dialog-state key.
func (s *DialogStateStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redisqueue: dialog state delete %s: %w", key, err)
	}
	return nil
}
