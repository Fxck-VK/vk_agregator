package accountlink_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountlink"
	"vk-ai-aggregator/internal/service/accountservice"
)

type challengeConsumer interface {
	ConsumeChallenge(context.Context, string, accountlink.Challenge) error
}

// Loading is deliberately paused after reading: both requests must validate
// the same proof before either attempts consumption.
type controlledChallengeStore struct {
	*accountlink.MemoryStore
	loads      atomic.Int32
	ready      chan struct{}
	release    chan struct{}
	consumeErr error
	replace    func(accountlink.Challenge) accountlink.Challenge
}

func (s *controlledChallengeStore) LoadChallenge(ctx context.Context, key string) (accountlink.Challenge, error) {
	challenge, err := s.MemoryStore.LoadChallenge(ctx, key)
	if err != nil {
		return challenge, err
	}
	if s.ready != nil {
		if s.loads.Add(1) == 2 {
			close(s.ready)
		}
		<-s.release
	}
	if s.replace != nil {
		replacement := s.replace(challenge)
		if err := s.MemoryStore.SaveChallenge(ctx, key, replacement, time.Minute); err != nil {
			return challenge, err
		}
	}
	return challenge, nil
}

func (s *controlledChallengeStore) DeleteChallenge(ctx context.Context, key string) error {
	if s.consumeErr != nil {
		return s.consumeErr
	}
	return s.MemoryStore.DeleteChallenge(ctx, key)
}

func (s *controlledChallengeStore) ConsumeChallenge(ctx context.Context, key string, expected accountlink.Challenge) error {
	if s.consumeErr != nil {
		return s.consumeErr
	}
	consumer, ok := any(s.MemoryStore).(challengeConsumer)
	if !ok {
		return errors.New("atomic challenge consumption missing")
	}
	return consumer.ConsumeChallenge(ctx, key, expected)
}

type proofAccount struct {
	mutations atomic.Int32
	version   time.Time
}

type supersededSender struct {
	calls   atomic.Int32
	first   chan struct{}
	release chan struct{}
	newCode string
}

func (s *supersededSender) send(code string) error {
	if s.calls.Add(1) == 1 {
		close(s.first)
		<-s.release
		return accountlink.ErrDeliveryUnavailable
	}
	s.newCode = code
	return nil
}

func (s *supersededSender) SendEmailLinkCode(_ context.Context, _, code string, _ time.Time) error {
	return s.send(code)
}
func (s *supersededSender) SendPhoneLinkOTP(_ context.Context, _, code string, _ time.Time) error {
	return s.send(code)
}

func TestSupersededDeliveryFailurePreservesNewChallenge(t *testing.T) {
	for _, flow := range []string{"email", "phone", "backup"} {
		t.Run(flow, func(t *testing.T) {
			ctx := context.Background()
			owner, identity := uuid.New(), uuid.New()
			sender := &supersededSender{first: make(chan struct{}), release: make(chan struct{})}
			account := &proofAccount{version: time.Now().UTC()}
			var clockCalls atomic.Int32
			base := time.Now()
			linker, err := accountlink.New(accountlink.NewMemoryStore(), sender, account, accountlink.Config{HashSecret: "test-secret", Now: func() time.Time { return base.Add(time.Duration(clockCalls.Add(1)) * time.Millisecond) }})
			if err != nil {
				t.Fatal(err)
			}
			request := func() error {
				var err error
				switch flow {
				case "email":
					_, err = linker.RequestEmailCode(ctx, owner, "proof@example.test")
				case "phone":
					_, err = linker.RequestPhoneOTP(ctx, owner, "+79991234567")
				case "backup":
					_, err = linker.RequestBackupEmailCode(ctx, owner, identity, "proof@example.test")
				}
				return err
			}
			firstResult := make(chan error, 1)
			go func() { firstResult <- request() }()
			select {
			case <-sender.first:
			case <-time.After(5 * time.Second):
				close(sender.release)
				t.Fatal("first delivery did not start")
			}
			if err := request(); err != nil {
				close(sender.release)
				t.Fatal(err)
			}
			close(sender.release)
			if err := <-firstResult; !errors.Is(err, accountlink.ErrDeliveryUnavailable) {
				t.Fatal("original delivery failure changed")
			}
			switch flow {
			case "email":
				_, err = linker.VerifyEmailCode(ctx, owner, "proof@example.test", sender.newCode)
			case "phone":
				_, err = linker.VerifyPhoneOTP(ctx, owner, "+79991234567", sender.newCode)
			case "backup":
				_, err = linker.VerifyBackupEmailCode(ctx, owner, identity, "proof@example.test", sender.newCode)
			}
			if err != nil {
				t.Fatalf("stale delivery failure deleted new proof: %v", err)
			}
		})
	}
}

func (a *proofAccount) LinkVerifiedIdentity(context.Context, uuid.UUID, uuid.UUID, domain.VerifiedAccountLogin) (accountservice.AccountIdentitySafe, error) {
	a.mutations.Add(1)
	return accountservice.AccountIdentitySafe{}, nil
}
func (a *proofAccount) BackupEmailVersion(context.Context, uuid.UUID, uuid.UUID) (time.Time, error) {
	return a.version, nil
}
func (a *proofAccount) ReplaceVerifiedBackupEmail(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, domain.VerifiedAccountLogin, time.Time) (accountservice.AccountIdentitySafe, error) {
	a.mutations.Add(1)
	return accountservice.AccountIdentitySafe{}, nil
}

func proofFixture(t *testing.T, flow string, store *controlledChallengeStore) (func(string) error, string, *proofAccount) {
	t.Helper()
	ctx := context.Background()
	owner, identity := uuid.New(), uuid.New()
	account := &proofAccount{version: time.Now().UTC()}
	sender := &capturingSender{}
	linker, err := accountlink.New(store, sender, account, accountlink.Config{HashSecret: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	var verify func(string) error
	switch flow {
	case "recovery", "email":
		_, err = linker.RequestEmailCode(ctx, owner, "proof@example.test")
		verify = func(code string) error {
			if flow == "recovery" {
				return linker.VerifyEmailRecoveryCode(ctx, owner, "proof@example.test", code)
			}
			_, err := linker.VerifyEmailCode(ctx, owner, "proof@example.test", code)
			return err
		}
	case "phone":
		_, err = linker.RequestPhoneOTP(ctx, owner, "+79991234567")
		verify = func(code string) error { _, err := linker.VerifyPhoneOTP(ctx, owner, "+79991234567", code); return err }
	case "backup":
		_, err = linker.RequestBackupEmailCode(ctx, owner, identity, "proof@example.test")
		verify = func(code string) error {
			_, err := linker.VerifyBackupEmailCode(ctx, owner, identity, "proof@example.test", code)
			return err
		}
	default:
		t.Fatal("unknown test flow")
	}
	if err != nil {
		t.Fatal(err)
	}
	code := sender.emailCode
	if flow == "phone" {
		code = sender.phoneCode
	}
	return verify, code, account
}

func TestConcurrentProofRedemptionHasOneWinner(t *testing.T) {
	for _, flow := range []string{"recovery", "email", "phone", "backup"} {
		t.Run(flow, func(t *testing.T) {
			store := &controlledChallengeStore{MemoryStore: accountlink.NewMemoryStore(), ready: make(chan struct{}), release: make(chan struct{})}
			verify, code, account := proofFixture(t, flow, store)
			results := make(chan error, 2)
			for i := 0; i < 2; i++ {
				go func() { results <- verify(code) }()
			}
			select {
			case <-store.ready:
			case <-time.After(5 * time.Second):
				close(store.release)
				t.Fatal("both verifiers did not read proof")
			}
			close(store.release)
			winners := 0
			for i := 0; i < 2; i++ {
				err := <-results
				if err == nil {
					winners++
				} else if !errors.Is(err, accountlink.ErrInvalidCode) {
					t.Fatal(err)
				}
			}
			if winners != 1 {
				t.Fatalf("proof authorized %d callers; want one", winners)
			}
			if flow != "recovery" && account.mutations.Load() != 1 {
				t.Fatal("proof authorized multiple identity mutations")
			}
		})
	}
}

func TestProofConsumptionFailureCannotAuthorize(t *testing.T) {
	for _, flow := range []string{"recovery", "email", "phone", "backup"} {
		t.Run(flow, func(t *testing.T) {
			failure := errors.New("test challenge store unavailable")
			store := &controlledChallengeStore{MemoryStore: accountlink.NewMemoryStore()}
			verify, code, account := proofFixture(t, flow, store)
			store.consumeErr = failure
			if err := verify(code); !errors.Is(err, failure) {
				t.Fatalf("store failure not propagated: %v", err)
			}
			if account.mutations.Load() != 0 {
				t.Fatal("store failure authorized mutation")
			}
			store.consumeErr = nil
			if err := verify(code); err != nil {
				t.Fatal("failed consumption destroyed good challenge")
			}
		})
	}
}

func TestInvalidProofDoesNotConsumeValidChallenge(t *testing.T) {
	for _, flow := range []string{"recovery", "email", "phone", "backup"} {
		t.Run(flow, func(t *testing.T) {
			verify, code, account := proofFixture(t, flow, &controlledChallengeStore{MemoryStore: accountlink.NewMemoryStore()})
			if err := verify("not-a-code"); !errors.Is(err, accountlink.ErrInvalidCode) {
				t.Fatal("invalid proof accepted")
			}
			if account.mutations.Load() != 0 {
				t.Fatal("invalid proof authorized mutation")
			}
			if err := verify(code); err != nil {
				t.Fatal("invalid proof consumed valid challenge")
			}
		})
	}
}

func TestReplacementBetweenReadAndConsumptionSurvives(t *testing.T) {
	for _, flow := range []string{"recovery", "email", "phone", "backup"} {
		t.Run(flow, func(t *testing.T) {
			store := &controlledChallengeStore{MemoryStore: accountlink.NewMemoryStore()}
			verify, code, account := proofFixture(t, flow, store)
			store.replace = func(c accountlink.Challenge) accountlink.Challenge {
				c.ExpiresAt = c.ExpiresAt.Add(time.Second)
				return c
			}
			if err := verify(code); !errors.Is(err, accountlink.ErrInvalidCode) {
				t.Fatalf("stale proof authorized: %v", err)
			}
			if account.mutations.Load() != 0 {
				t.Fatal("stale proof authorized mutation")
			}
			store.replace = nil
			if err := verify(code); err != nil {
				t.Fatal("stale proof deleted replacement")
			}
		})
	}
}

func TestMemoryConsumptionComparesWholeChallenge(t *testing.T) {
	store := accountlink.NewMemoryStore()
	consumer, ok := any(store).(challengeConsumer)
	if !ok {
		t.Fatal("atomic challenge consumption missing")
	}
	ctx := context.Background()
	challenge := accountlink.Challenge{AccountID: uuid.New(), IdentityHash: "identity-hash", CodeHash: "code-hash", BackupIdentityID: uuid.New(), BackupVersion: time.Now().UTC(), ExpiresAt: time.Now().Add(time.Minute)}
	for _, change := range []func(*accountlink.Challenge){
		func(c *accountlink.Challenge) { c.AccountID = uuid.New() },
		func(c *accountlink.Challenge) { c.IdentityHash += "different" },
		func(c *accountlink.Challenge) { c.CodeHash += "different" },
		func(c *accountlink.Challenge) { c.BackupIdentityID = uuid.New() },
		func(c *accountlink.Challenge) { c.BackupVersion = c.BackupVersion.Add(time.Nanosecond) },
		func(c *accountlink.Challenge) { c.ExpiresAt = c.ExpiresAt.Add(time.Nanosecond) },
	} {
		if err := store.SaveChallenge(ctx, "exact-proof", challenge, time.Minute); err != nil {
			t.Fatal(err)
		}
		expected := challenge
		change(&expected)
		if err := consumer.ConsumeChallenge(ctx, "exact-proof", expected); !errors.Is(err, accountlink.ErrInvalidCode) {
			t.Fatal("different challenge consumed")
		}
		if err := consumer.ConsumeChallenge(ctx, "exact-proof", challenge); err != nil {
			t.Fatal("mismatch destroyed challenge")
		}
		if err := consumer.ConsumeChallenge(ctx, "exact-proof", challenge); !errors.Is(err, accountlink.ErrInvalidCode) {
			t.Fatal("proof replay accepted")
		}
	}
}

func TestMemoryConsumptionRejectsExpiryAfterRead(t *testing.T) {
	store := accountlink.NewMemoryStore()
	consumer, ok := any(store).(challengeConsumer)
	if !ok {
		t.Fatal("atomic challenge consumption missing")
	}
	ctx := context.Background()
	now := time.Now()
	store.SetNow(func() time.Time { return now })
	for _, ttl := range []time.Duration{time.Second, time.Minute} {
		challenge := accountlink.Challenge{ExpiresAt: now.Add(time.Second)}
		if err := store.SaveChallenge(ctx, "expiring-proof", challenge, ttl); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
		if err := consumer.ConsumeChallenge(ctx, "expiring-proof", challenge); !errors.Is(err, accountlink.ErrExpiredCode) {
			t.Fatal("expired challenge consumed")
		}
	}
}
