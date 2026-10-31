package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type renewalWorkerStub struct {
	calls int
	err   error
}

func (s *renewalWorkerStub) ProcessSubscriptionRenewals(context.Context, int) (int, error) {
	s.calls++
	return 0, s.err
}

type expiryWorkerStub struct {
	calls int
	err   error
}

func (s *expiryWorkerStub) ExpireCreditLots(context.Context, time.Time, int) (int64, error) {
	s.calls++
	return 0, s.err
}
func TestSubscriptionMaintenanceRunsIndependentExpiryAndRenewal(t *testing.T) {
	renew := &renewalWorkerStub{err: errors.New("renewal unavailable")}
	expire := &expiryWorkerStub{}
	if err := runSubscriptionMaintenanceOnce(context.Background(), renew, expire); err == nil {
		t.Fatal("renewal failure hidden")
	}
	if renew.calls != 1 || expire.calls != 1 {
		t.Fatal("expiry starved by failed renewal")
	}
	expire.err = errors.New("expiry unavailable")
	renew.err = nil
	if err := runSubscriptionMaintenanceOnce(context.Background(), renew, expire); err == nil {
		t.Fatal("expiry failure hidden")
	}
	if renew.calls != 2 || expire.calls != 2 {
		t.Fatal("renewal starved by failed expiry")
	}
}
