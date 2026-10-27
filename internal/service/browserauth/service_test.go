package browserauth

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountlink"
)

type testStore struct {
	sync.Mutex
	rows   map[string]accountlink.Challenge
	counts map[string]int64
}

func (s *testStore) SaveChallenge(_ context.Context, k string, v accountlink.Challenge, _ time.Duration) error {
	s.Lock()
	defer s.Unlock()
	s.rows[k] = v
	return nil
}
func (s *testStore) LoadChallenge(_ context.Context, k string) (accountlink.Challenge, error) {
	s.Lock()
	defer s.Unlock()
	v, ok := s.rows[k]
	if !ok {
		return v, accountlink.ErrInvalidCode
	}
	return v, nil
}
func (s *testStore) DeleteChallenge(_ context.Context, k string) error {
	s.Lock()
	defer s.Unlock()
	delete(s.rows, k)
	return nil
}
func (s *testStore) ConsumeChallenge(_ context.Context, k string, v accountlink.Challenge) error {
	s.Lock()
	defer s.Unlock()
	if old, ok := s.rows[k]; !ok || old != v {
		return accountlink.ErrInvalidCode
	}
	delete(s.rows, k)
	return nil
}
func (s *testStore) Increment(_ context.Context, k string, _ time.Duration) (int64, error) {
	s.Lock()
	defer s.Unlock()
	s.counts[k]++
	return s.counts[k], nil
}

type testSender struct {
	code, link string
	fail       bool
	sent       int
	onCode     func(string) error
}

func (s *testSender) SendEmailLinkCode(_ context.Context, _, code string, _ time.Time) error {
	s.code = code
	s.sent++
	if s.onCode != nil {
		return s.onCode(code)
	}
	if s.fail {
		return errors.New("offline")
	}
	return nil
}
func (s *testSender) SendPasswordResetLink(_ context.Context, _, link string, _ time.Time) error {
	s.link = link
	s.sent++
	return nil
}

type testAuth struct {
	proof                  accountauth.PasswordProof
	sessions, resets       int
	stale                  bool
	failSession, failReset bool
}

func (a *testAuth) CapturePasswordProof(_ context.Context, email, password string) (accountauth.PasswordProof, error) {
	if email != a.proof.EmailBinding.NormalizedID || password != "synthetic-password" {
		return accountauth.PasswordProof{}, accountauth.ErrInvalidPasswordLogin
	}
	p := a.proof
	p.PasswordVerified = true
	return p, nil
}
func (a *testAuth) CaptureRecoveryProof(_ context.Context, email string) (accountauth.PasswordProof, error) {
	if email != a.proof.EmailBinding.NormalizedID {
		return accountauth.PasswordProof{}, domain.ErrNotFound
	}
	return a.proof, nil
}
func (a *testAuth) ValidatePasswordProof(_ context.Context, p accountauth.PasswordProof) error {
	if a.stale || p.ExpectedHash != a.proof.ExpectedHash || p.EmailBinding.ID != a.proof.EmailBinding.ID || p.EmailBinding.AccountID != a.proof.EmailBinding.AccountID || p.EmailBinding.NormalizedID != a.proof.EmailBinding.NormalizedID || !p.EmailBinding.UpdatedAt.Equal(a.proof.EmailBinding.UpdatedAt) {
		return accountauth.ErrInvalidPasswordLogin
	}
	return nil
}
func (a *testAuth) CompletePasswordProof(ctx context.Context, p accountauth.PasswordProof, m accountauth.SessionMetadata) (accountauth.SessionTokens, error) {
	if a.failSession {
		a.failSession = false
		return accountauth.SessionTokens{}, errors.New("storage unavailable")
	}
	if err := a.ValidatePasswordProof(ctx, p); err != nil {
		return accountauth.SessionTokens{}, err
	}
	a.sessions++
	return accountauth.SessionTokens{Session: accountauth.AccountSessionSafe{AccountID: p.EmailBinding.AccountID}, Persistent: m.Persistent}, nil
}
func (a *testAuth) ResetPasswordProof(ctx context.Context, p accountauth.PasswordProof, _ string) error {
	if a.failReset {
		a.failReset = false
		return errors.New("storage unavailable")
	}
	if err := a.ValidatePasswordProof(ctx, p); err != nil {
		return err
	}
	a.resets++
	a.stale = true
	return nil
}

func (s *testStore) RestoreChallenge(_ context.Context, k string, v accountlink.Challenge, _ time.Duration) error {
	s.Lock()
	defer s.Unlock()
	if _, exists := s.rows[k]; !exists {
		s.rows[k] = v
	}
	return nil
}

func TestRetryStorageFailureWithoutResurrectingConsumedSuccess(t *testing.T) {
	s, a, sender := fixture(t)
	ctx := context.Background()
	r, err := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", "", "agent", accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	a.failSession = true
	if _, err = s.Verify(ctx, r.Challenge, "browser", sender.code, "agent", accountauth.SessionMetadata{}); err == nil {
		t.Fatal("session failure hidden")
	}
	if _, err = s.Verify(ctx, r.Challenge, "browser", sender.code, "agent", accountauth.SessionMetadata{}); err != nil {
		t.Fatal("transient failure consumed code", err)
	}
	if err = s.Resend(ctx, r.Challenge, "browser", "agent"); err == nil {
		t.Fatal("resend resurrected consumed login")
	}
	if err = s.RequestReset(ctx, "member@example.test", "ru", "peer"); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(sender.link)
	token := strings.TrimPrefix(u.Fragment, "token=")
	a.failReset = true
	if err = s.Reset(ctx, token, "replacement-password"); err == nil {
		t.Fatal("reset failure hidden")
	}
	if err = s.Reset(ctx, token, "replacement-password"); err != nil {
		t.Fatal("transient failure consumed reset", err)
	}
	if a.sessions != 1 || a.resets != 1 {
		t.Fatal("unexpected duplicate success")
	}
}
func fixture(t *testing.T) (*Service, *testAuth, *testSender) {
	t.Helper()
	a := &testAuth{proof: accountauth.PasswordProof{EmailBinding: domain.AccountIdentity{ID: uuid.New(), AccountID: uuid.New(), Provider: domain.IdentityProviderEmail, ExternalID: "member@example.test", NormalizedID: "member@example.test", VerifiedAt: time.Now(), UpdatedAt: time.Now()}, ExpectedHash: "synthetic-verifier"}}
	sender := &testSender{}
	s, err := New(&testStore{rows: map[string]accountlink.Challenge{}, counts: map[string]int64{}}, sender, a, "https://example.test", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return s, a, sender
}
func TestPasswordBeforeCodeAndNoSessionUntilVerification(t *testing.T) {
	s, a, sender := fixture(t)
	ctx := context.Background()
	if _, err := s.Begin(ctx, "member@example.test", "wrong", "browser", "", "agent", accountauth.SessionMetadata{}); err == nil || sender.sent != 0 {
		t.Fatal("invalid password caused delivery")
	}
	r, err := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", "", "agent", accountauth.SessionMetadata{})
	if err != nil || r.Challenge == "" || a.sessions != 0 {
		t.Fatal("challenge missing or session issued", err)
	}
	if _, err := s.Verify(ctx, r.Challenge, "foreign", sender.code, "agent", accountauth.SessionMetadata{}); err == nil {
		t.Fatal("foreign browser accepted")
	}
	if _, err := s.Verify(ctx, r.Challenge, "browser", "0000000", "agent", accountauth.SessionMetadata{}); err == nil {
		t.Fatal("wrong code accepted")
	}
	done, err := s.Verify(ctx, r.Challenge, "browser", sender.code, "agent", accountauth.SessionMetadata{})
	if err != nil || done.Tokens == nil || done.Trusted == "" || done.Remembered == "" || a.sessions != 1 {
		t.Fatal("verification failed", err)
	}
	if _, err := s.Verify(ctx, r.Challenge, "browser", sender.code, "agent", accountauth.SessionMetadata{}); err == nil {
		t.Fatal("proof replayed")
	}
	next, err := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", done.Trusted, "agent", accountauth.SessionMetadata{})
	if err != nil || next.Tokens == nil {
		t.Fatal("trusted browser prompted again", err)
	}
	a.stale = true
	if _, err := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", done.Trusted, "agent", accountauth.SessionMetadata{}); err == nil {
		t.Fatal("stale proof created session")
	}
}
func TestResetPurposeExpiryReplayAndUnknownAccount(t *testing.T) {
	s, a, sender := fixture(t)
	ctx := context.Background()
	if err := s.RequestReset(ctx, "missing@example.test", "ru", "peer"); err != nil || sender.sent != 0 {
		t.Fatal("unknown account leaked")
	}
	if err := s.RequestReset(ctx, "member@example.test", "ru", "peer"); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(sender.link)
	token := u.Fragment[len("token="):]
	if u.Host != "example.test" || u.Path != "/ru/login/reset" || u.RawQuery != "" {
		t.Fatal("unsafe reset URL")
	}
	if _, err := s.InspectReset(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify(ctx, token, "browser", "123456", "agent", accountauth.SessionMetadata{}); err == nil {
		t.Fatal("reset token used for login")
	}
	if err := s.Reset(ctx, token, "replacement-password"); err != nil || a.resets != 1 {
		t.Fatal("reset failed", err)
	}
	if err := s.Reset(ctx, token, "replacement-password"); err == nil {
		t.Fatal("reset replayed")
	}
}
func TestExpiryTamperingDeliveryAndHintCannotAuthenticate(t *testing.T) {
	s, a, sender := fixture(t)
	ctx := context.Background()
	r, _ := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", "", "agent", accountauth.SessionMetadata{})
	done, err := s.Verify(ctx, r.Challenge, "browser", sender.code, "agent", accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Begin(ctx, "member@example.test", "wrong", "browser", done.Remembered, "agent", accountauth.SessionMetadata{}); err == nil {
		t.Fatal("hint authenticated")
	}
	if hint, err := s.Hint(done.Remembered); err != nil || hint == "member@example.test" {
		t.Fatal("hint exposes full address")
	}
	if _, err := s.InspectReset(ctx, done.Trusted); err == nil {
		t.Fatal("wrong purpose accepted")
	}
	r, _ = s.Begin(ctx, "member@example.test", "synthetic-password", "other", "", "agent", accountauth.SessionMetadata{})
	s.now = func() time.Time { return time.Now().Add(time.Hour) }
	if _, err := s.Verify(ctx, r.Challenge, "other", sender.code, "agent", accountauth.SessionMetadata{}); err == nil {
		t.Fatal("expired challenge accepted")
	}
	s.now = time.Now
	sender.fail = true
	if _, err := s.Begin(ctx, "member@example.test", "synthetic-password", "third", "", "agent", accountauth.SessionMetadata{}); err == nil {
		t.Fatal("delivery failure accepted")
	}
	if a.sessions != 1 {
		t.Fatal("extra session issued")
	}
}

func TestResendRateLimitPreservesLiveCode(t *testing.T) {
	s, _, sender := fixture(t)
	ctx := context.Background()
	r, err := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", "", "agent", accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.Resend(ctx, r.Challenge, "browser", "agent"); err != nil {
			t.Fatal(err)
		}
	}
	receipt, _ := s.open(r.Challenge, "login")
	key := s.rowKey("login", receipt.Nonce)
	before, _ := s.store.LoadChallenge(ctx, key)
	code := sender.code
	if err := s.Resend(ctx, r.Challenge, "browser", "agent"); !errors.Is(err, accountlink.ErrRateLimited) {
		t.Fatal("resend limit was not enforced", err)
	}
	after, err := s.store.LoadChallenge(ctx, key)
	if err != nil || after != before || sender.sent != 3 {
		t.Fatal("rate limit replaced or consumed the live challenge", err)
	}
	if _, err := s.Verify(ctx, r.Challenge, "browser", code, "agent", accountauth.SessionMetadata{}); err != nil {
		t.Fatal("rate limit invalidated the last delivered code", err)
	}
}

func TestResendDeliveryFailurePreservesCodeAndDeadline(t *testing.T) {
	s, _, sender := fixture(t)
	ctx := context.Background()
	now := time.Now()
	s.now = func() time.Time { return now }
	r, err := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", "", "agent", accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	code := sender.code
	receipt, _ := s.open(r.Challenge, "login")
	key := s.rowKey("login", receipt.Nonce)
	before, _ := s.store.LoadChallenge(ctx, key)
	now = now.Add(time.Minute)
	sender.fail = true
	if err := s.Resend(ctx, r.Challenge, "browser", "agent"); err == nil {
		t.Fatal("delivery failure hidden")
	}
	after, err := s.store.LoadChallenge(ctx, key)
	if err != nil || after != before {
		t.Fatal("delivery failure lost the previous code or extended its deadline", err)
	}
	if _, err := s.Verify(ctx, r.Challenge, "browser", code, "agent", accountauth.SessionMetadata{}); err != nil {
		t.Fatal("delivery failure invalidated the previous code", err)
	}
}

type failedPublicationStore struct {
	*testStore
	publishBeforeFailure bool
	failed               bool
}

func (s *failedPublicationStore) RestoreChallenge(ctx context.Context, key string, row accountlink.Challenge, ttl time.Duration) error {
	if !s.failed {
		s.failed = true
		if s.publishBeforeFailure {
			if err := s.testStore.RestoreChallenge(ctx, key, row, ttl); err != nil {
				return err
			}
		}
		return errors.New("replacement storage unavailable")
	}
	return s.testStore.RestoreChallenge(ctx, key, row, ttl)
}

func TestResendFailedPublicationRestoresWithoutOverwritingNewerCode(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-write", true: "ambiguous-write"}[published], func(t *testing.T) {
			s, _, sender := fixture(t)
			ctx := context.Background()
			r, err := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", "", "agent", accountauth.SessionMetadata{})
			if err != nil {
				t.Fatal(err)
			}
			oldCode := sender.code
			s.store = &failedPublicationStore{testStore: s.store.(*testStore), publishBeforeFailure: published}
			if err := s.Resend(ctx, r.Challenge, "browser", "agent"); err == nil {
				t.Fatal("replacement storage failure hidden")
			}
			want := oldCode
			if published {
				want = sender.code
			}
			if _, err := s.Verify(ctx, r.Challenge, "browser", want, "agent", accountauth.SessionMetadata{}); err != nil {
				t.Fatal("failed replacement lost the live generation", err)
			}
			if _, err := s.Verify(ctx, r.Challenge, "browser", want, "agent", accountauth.SessionMetadata{}); err == nil {
				t.Fatal("restored generation allowed replay")
			}
		})
	}
}

func TestResendPendingDeliveryCannotAuthenticateOrResurrectSuccess(t *testing.T) {
	s, a, sender := fixture(t)
	ctx := context.Background()
	r, err := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", "", "agent", accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	oldCode := sender.code
	delivered := make(chan string)
	release := make(chan struct{})
	sender.onCode = func(code string) error {
		delivered <- code
		<-release
		return errors.New("delivery acknowledgement failed")
	}
	done := make(chan error, 1)
	go func() { done <- s.Resend(ctx, r.Challenge, "browser", "agent") }()
	newCode := <-delivered
	for _, code := range []string{oldCode, newCode} {
		if _, err := s.Verify(ctx, r.Challenge, "browser", code, "agent", accountauth.SessionMetadata{}); err == nil {
			close(release)
			<-done
			t.Fatal("code authenticated while delivery result was pending")
		}
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("delivery acknowledgement failure hidden")
	}
	if _, err := s.Verify(ctx, r.Challenge, "browser", oldCode, "agent", accountauth.SessionMetadata{}); err != nil {
		t.Fatal("delivery failure lost the prior live code", err)
	}
	if err := s.Resend(ctx, r.Challenge, "browser", "agent"); err == nil {
		t.Fatal("resend resurrected a consumed success")
	}
	if a.sessions != 1 {
		t.Fatal("resend created duplicate sessions")
	}
}

func TestReplacedEmailBindingRequiresFreshCodeInsteadOfRejectingPassword(t *testing.T) {
	for _, email := range []string{"member@example.test", "replacement@example.test"} {
		t.Run(map[bool]string{false: "version-only", true: "replacement-email"}[email != "member@example.test"], func(t *testing.T) {
			s, a, sender := fixture(t)
			ctx := context.Background()
			r, err := s.Begin(ctx, "member@example.test", "synthetic-password", "browser", "", "agent", accountauth.SessionMetadata{})
			if err != nil {
				t.Fatal(err)
			}
			done, err := s.Verify(ctx, r.Challenge, "browser", sender.code, "agent", accountauth.SessionMetadata{})
			if err != nil {
				t.Fatal(err)
			}
			a.proof.EmailBinding.ExternalID = email
			a.proof.EmailBinding.NormalizedID = email
			a.proof.EmailBinding.UpdatedAt = a.proof.EmailBinding.UpdatedAt.Add(time.Second)
			next, err := s.Begin(ctx, email, "synthetic-password", "browser", done.Trusted, "agent", accountauth.SessionMetadata{})
			if err != nil || next.Challenge == "" || next.Tokens != nil || a.sessions != 1 {
				t.Fatal("stale trust rejected current credentials or bypassed fresh verification", err)
			}
			if _, err := s.Verify(ctx, next.Challenge, "browser", sender.code, "agent", accountauth.SessionMetadata{}); err != nil || a.sessions != 2 {
				t.Fatal("fresh code did not complete the replaced binding login", err)
			}
		})
	}
}
