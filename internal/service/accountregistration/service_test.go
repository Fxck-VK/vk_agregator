package accountregistration

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	"sync"
	"testing"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

type testStore struct {
	mu     sync.Mutex
	rows   map[string]Challenge
	counts map[string]int64
}

func (s *testStore) Save(_ context.Context, key string, row Challenge, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[key] = row
	return nil
}
func (s *testStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, key)
	return nil
}
func (s *testStore) Increment(_ context.Context, key string, _ time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[key]++
	return s.counts[key], nil
}
func (s *testStore) Verify(_ context.Context, key, identity, code string, now int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[key]
	if !ok || row.IdentityHash != identity || row.CodeHash != code || row.ExpiresAt <= now {
		return ErrInvalidProof
	}
	row.Verified = true
	s.rows[key] = row
	return nil
}
func (s *testStore) Take(_ context.Context, key, identity string, now int64) (Challenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[key]
	if !ok || !row.Verified || row.IdentityHash != identity || row.ExpiresAt <= now {
		return Challenge{}, ErrInvalidProof
	}
	delete(s.rows, key)
	return row, nil
}
func (s *testStore) Restore(_ context.Context, key string, row Challenge, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rows[key]; !ok {
		s.rows[key] = row
	}
	return nil
}

type testSender struct {
	code string
	err  error
}

func (s *testSender) SendEmailLinkCode(_ context.Context, _ string, code string, _ time.Time) error {
	s.code = code
	return s.err
}

type testAuth struct {
	calls       int
	id          uuid.UUID
	fail        bool
	sessionFail bool
}

func (a *testAuth) RegisterVerifiedEmailPassword(_ context.Context, id uuid.UUID, _ string, _ string) (domain.IdentityResolution, error) {
	a.calls++
	if a.fail {
		return domain.IdentityResolution{}, errors.New("temporary storage failure")
	}
	a.id = id
	return domain.IdentityResolution{AccountID: id}, nil
}
func (a *testAuth) IssueSession(_ context.Context, id uuid.UUID, _ accountauth.SessionMetadata) (accountauth.SessionTokens, error) {
	if a.sessionFail {
		return accountauth.SessionTokens{}, errors.New("temporary session storage failure")
	}
	return accountauth.SessionTokens{Session: accountauth.AccountSessionSafe{AccountID: id}}, nil
}
func setup(t *testing.T) (*Service, *testStore, *testSender, *testAuth, *time.Time) {
	t.Helper()
	store := &testStore{rows: map[string]Challenge{}, counts: map[string]int64{}}
	sender := &testSender{}
	auth := &testAuth{}
	now := time.Now()
	service, err := New(store, sender, auth, "test-only-signup-secret-with-entropy")
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	return service, store, sender, auth, &now
}

func TestEmailCodeProofIsBoundToBrowserEmailAndOneUse(t *testing.T) {
	s, store, sender, auth, _ := setup(t)
	ctx := context.Background()
	binding, err := s.Request(ctx, "New@Example.test", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.code) != 6 || len(binding) != 43 {
		t.Fatal("invalid challenge")
	}
	for key, row := range store.rows {
		if strings.Contains(key, "new@example.test") || row.CodeHash == sender.code || row.IdentityHash == "new@example.test" {
			t.Fatal("raw code/email stored")
		}
	}
	if _, err := s.Complete(ctx, binding, "new@example.test", "strong-password", accountauth.SessionMetadata{}, nil); !errors.Is(err, ErrInvalidProof) {
		t.Fatal("unverified proof accepted")
	}
	if err := s.Verify(ctx, binding, "other@example.test", sender.code); !errors.Is(err, ErrInvalidProof) {
		t.Fatal("foreign email verified")
	}
	if err := s.Verify(ctx, strings.Repeat("x", 43), "new@example.test", sender.code); !errors.Is(err, ErrInvalidProof) {
		t.Fatal("foreign browser verified")
	}
	if err := s.Verify(ctx, binding, "new@example.test", sender.code); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Complete(ctx, binding, "new@example.test", "short", accountauth.SessionMetadata{}, nil); !errors.Is(err, accountauth.ErrWeakPassword) {
		t.Fatal("weak password accepted")
	}
	tokens, err := s.Complete(ctx, binding, "new@example.test", "strong-password", accountauth.SessionMetadata{}, nil)
	if err != nil || tokens.Session.AccountID == uuid.Nil || auth.calls != 1 {
		t.Fatalf("completion: %v", err)
	}
	if _, err := s.Complete(ctx, binding, "new@example.test", "strong-password", accountauth.SessionMetadata{}, nil); !errors.Is(err, ErrInvalidProof) {
		t.Fatal("proof replay accepted")
	}
}

func TestExpiredCodeRateLimitsAndDeliveryFailure(t *testing.T) {
	ctx := context.Background()
	s, store, sender, auth, now := setup(t)
	binding, err := s.Request(ctx, "new@example.test", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(11 * time.Minute)
	if err := s.Verify(ctx, binding, "new@example.test", sender.code); !errors.Is(err, ErrInvalidProof) {
		t.Fatal("expired code verified")
	}
	if auth.calls != 0 {
		t.Fatal("account created before confirmation")
	}
	s, store, sender, _, _ = setup(t)
	sender.err = errors.New("delivery unavailable")
	if _, err := s.Request(ctx, "new@example.test", "192.0.2.1"); err == nil || len(store.rows) != 0 {
		t.Fatal("failed delivery left valid challenge")
	}
	s, _, sender, _, _ = setup(t)
	for i := 0; i < 3; i++ {
		if _, err := s.Request(ctx, "new@example.test", "192.0.2.1"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Request(ctx, "new@example.test", "192.0.2.1"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("requests not throttled")
	}
	s, _, sender, _, _ = setup(t)
	binding, _ = s.Request(ctx, "new@example.test", "192.0.2.1")
	for i := 0; i < 5; i++ {
		_ = s.Verify(ctx, binding, "new@example.test", "invalid")
	}
	if err := s.Verify(ctx, binding, "new@example.test", sender.code); !errors.Is(err, ErrRateLimited) {
		t.Fatal("guessing not throttled")
	}
}

func TestTransientFailureRestoresSameProofWithoutChangingAccountID(t *testing.T) {
	s, _, sender, auth, _ := setup(t)
	ctx := context.Background()
	binding, _ := s.Request(ctx, "new@example.test", "192.0.2.1")
	_ = s.Verify(ctx, binding, "new@example.test", sender.code)
	auth.fail = true
	if _, err := s.Complete(ctx, binding, "new@example.test", "strong-password", accountauth.SessionMetadata{}, nil); err == nil {
		t.Fatal("storage failure ignored")
	}
	auth.fail = false
	if _, err := s.Complete(ctx, binding, "new@example.test", "strong-password", accountauth.SessionMetadata{}, nil); err != nil {
		t.Fatal("proof not restored", err)
	}
}

func TestCookiePreparationFailureRestoresProofForSameAccount(t *testing.T) {
	s, _, sender, auth, _ := setup(t)
	ctx := context.Background()
	binding, _ := s.Request(ctx, "new@example.test", "192.0.2.1")
	_ = s.Verify(ctx, binding, "new@example.test", sender.code)
	if _, err := s.Complete(ctx, binding, "new@example.test", "strong-password", accountauth.SessionMetadata{}, func(accountauth.SessionTokens) error {
		return errors.New("cookie preparation unavailable")
	}); err == nil {
		t.Fatal("cookie preparation failure ignored")
	}
	firstID := auth.id
	finalized := false
	if _, err := s.Complete(ctx, binding, "new@example.test", "strong-password", accountauth.SessionMetadata{}, func(tokens accountauth.SessionTokens) error {
		finalized = tokens.Session.AccountID == firstID
		return nil
	}); err != nil || !finalized {
		t.Fatal("proof not restored for same account", err)
	}
}

func TestSessionFailureRetainsProofExpiryAndAccountID(t *testing.T) {
	s, store, sender, auth, now := setup(t)
	ctx := context.Background()
	binding, _ := s.Request(ctx, "new@example.test", "192.0.2.1")
	_ = s.Verify(ctx, binding, "new@example.test", sender.code)
	originalExpiry := store.rows[challengeKey(binding)].ExpiresAt
	auth.sessionFail = true
	if _, err := s.Complete(ctx, binding, "new@example.test", "strong-password", accountauth.SessionMetadata{}, nil); err == nil {
		t.Fatal("session failure ignored")
	}
	firstID := auth.id
	*now = now.Add(time.Minute)
	auth.sessionFail = false
	if row := store.rows[challengeKey(binding)]; row.ExpiresAt != originalExpiry || !row.Verified {
		t.Fatal("retry changed proof expiry or verification")
	}
	if _, err := s.Complete(ctx, binding, "new@example.test", "strong-password", accountauth.SessionMetadata{}, nil); err != nil || auth.id != firstID {
		t.Fatal("session retry changed account", err)
	}
}
