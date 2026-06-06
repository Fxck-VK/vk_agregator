package dialogstate

import (
	"context"
	"testing"
	"time"
)

func TestServiceStoresGetsAndClearsMode(t *testing.T) {
	store := newMemoryStore()
	svc := New(store, Config{TTL: 30 * time.Minute})

	if err := svc.Set(context.Background(), 123, ModeGPT); err != nil {
		t.Fatalf("set: %v", err)
	}
	mode, ok, err := svc.Get(context.Background(), 123)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || mode != ModeGPT {
		t.Fatalf("mode=%q ok=%v, want %q true", mode, ok, ModeGPT)
	}
	if got := store.ttls[modeKey(123)]; got != 30*time.Minute {
		t.Fatalf("ttl=%s, want 30m", got)
	}

	if err := svc.Clear(context.Background(), 123); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok, err := svc.Get(context.Background(), 123); err != nil || ok {
		t.Fatalf("after clear ok=%v err=%v, want false nil", ok, err)
	}
}

func TestServiceDefaultsTTL(t *testing.T) {
	store := newMemoryStore()
	svc := New(store, Config{})

	if err := svc.Set(context.Background(), 456, ModeGPT); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := store.ttls[modeKey(456)]; got != time.Hour {
		t.Fatalf("ttl=%s, want 1h", got)
	}
}

type memoryStore struct {
	values map[string]string
	ttls   map[string]time.Duration
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		values: map[string]string{},
		ttls:   map[string]time.Duration{},
	}
}

func (s *memoryStore) Get(_ context.Context, key string) (string, bool, error) {
	value, ok := s.values[key]
	return value, ok, nil
}

func (s *memoryStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	s.values[key] = value
	s.ttls[key] = ttl
	return nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	delete(s.ttls, key)
	return nil
}
