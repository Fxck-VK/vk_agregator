package accountlink

import (
	"context"
	"sync"
	"time"
)

type memoryChallenge struct {
	value     Challenge
	expiresAt time.Time
}

type memoryCounter struct {
	value     int64
	expiresAt time.Time
}

// MemoryStore is used by tests and local mock flows.
type MemoryStore struct {
	mu         sync.Mutex
	challenges map[string]memoryChallenge
	counters   map[string]memoryCounter
	now        func() time.Time
}

// NewMemoryStore creates an in-memory account-link store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		challenges: make(map[string]memoryChallenge),
		counters:   make(map[string]memoryCounter),
		now:        time.Now,
	}
}

// SetNow overrides the clock for tests.
func (s *MemoryStore) SetNow(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now == nil {
		now = time.Now
	}
	s.now = now
}

func (s *MemoryStore) SaveChallenge(_ context.Context, key string, challenge Challenge, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.challenges[key] = memoryChallenge{
		value:     challenge,
		expiresAt: s.now().Add(ttl),
	}
	return nil
}

func (s *MemoryStore) LoadChallenge(_ context.Context, key string) (Challenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.challenges[key]
	if !ok {
		return Challenge{}, ErrInvalidCode
	}
	if !item.expiresAt.IsZero() && s.now().After(item.expiresAt) {
		delete(s.challenges, key)
		return Challenge{}, ErrExpiredCode
	}
	return item.value, nil
}

func (s *MemoryStore) DeleteChallenge(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.challenges, key)
	return nil
}

func (s *MemoryStore) Increment(_ context.Context, key string, ttl time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	item := s.counters[key]
	if item.expiresAt.IsZero() || now.After(item.expiresAt) {
		item = memoryCounter{expiresAt: now.Add(ttl)}
	}
	item.value++
	s.counters[key] = item
	return item.value, nil
}
