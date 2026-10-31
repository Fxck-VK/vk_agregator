package maintenance

import (
	"context"
	"errors"
	"testing"
	"time"
)

type lotExpiryStore struct {
	fakeStore
	called int
	at     time.Time
	limit  int
	err    error
}

func (s *lotExpiryStore) ExpireCreditLots(_ context.Context, now time.Time, limit int) (int64, error) {
	s.called++
	s.at = now
	s.limit = limit
	return 3, s.err
}

func TestCleanupOptionallyExpiresCreditLots(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	store := &lotExpiryStore{}
	svc := New(store, nil, Config{}, WithClock(func() time.Time { return now }))
	if err := svc.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.called != 1 || !store.at.Equal(now) || store.limit <= 0 || store.limit > 1000 {
		t.Fatalf("expiry calls=%d at=%v limit=%d", store.called, store.at, store.limit)
	}
}

func TestCleanupCreditLotExpiryFailurePropagates(t *testing.T) {
	sentinel := errors.New("expiry failed")
	store := &lotExpiryStore{err: sentinel}
	if err := New(store, nil, Config{}).Cleanup(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("expiry failure %v", err)
	}
}
