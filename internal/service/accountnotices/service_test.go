package accountnotices

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"sync"
	"testing"
	"time"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

type fakeSender struct {
	mu   sync.Mutex
	fail bool
	sent []uuid.UUID
}

func (s *fakeSender) SendAccountSecurityNotice(ctx context.Context, n domain.AccountSecurityNotice) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("missing send deadline")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("fake SMTP failure with private details")
	}
	s.sent = append(s.sent, n.ID)
	return nil
}
func noticesFixture(t *testing.T) *memory.AccountIdentityRepo {
	t.Helper()
	r := memory.NewAccountIdentityRepo()
	id := uuid.New()
	for _, email := range []string{"first@example.test", "second@example.test"} {
		if _, err := r.LinkIdentity(context.Background(), id, domain.IdentityProviderEmail, email, email); err != nil {
			t.Fatal(err)
		}
	}
	return r
}
func TestDispatcherSMTPFailureRetriesIndependentOfMutation(t *testing.T) {
	repo := noticesFixture(t)
	sender := &fakeSender{fail: true}
	svc := New(repo, sender, nil)
	at := time.Now().Add(time.Second)
	svc.now = func() time.Time { return at }
	if err := svc.DispatchOnce(context.Background()); err != nil {
		t.Fatal("SMTP error escaped dispatcher")
	}
	if len(repo.SecurityNotices()) != 2 {
		t.Fatal("SMTP failure changed committed operation")
	}
	sender.fail = false
	if err := svc.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("unattempted recipient was not sent: sent=%d attempts=%v", len(sender.sent), noticeAttempts(repo.SecurityNotices()))
	}
	at = at.Add(time.Minute)
	if err := svc.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 2 {
		t.Fatal("retry did not deliver")
	}
	at = at.Add(domain.AccountSecurityNoticeSentRetention + time.Second)
	if err := svc.CleanupOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.SecurityNotices()) != 0 {
		t.Fatal("sent PII retention exceeded")
	}
}
func TestConcurrentDispatchersLeaseSeparateNotices(t *testing.T) {
	repo := noticesFixture(t)
	sender := &fakeSender{}
	a, b := New(repo, sender, nil), New(repo, sender, nil)
	at := time.Now().Add(time.Second)
	a.now = func() time.Time { return at }
	b.now = func() time.Time { return at }
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, svc := range []*Service{a, b} {
		wg.Add(1)
		go func(s *Service) { defer wg.Done(); errs <- s.DispatchOnce(context.Background()) }(svc)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(sender.sent) != 2 || sender.sent[0] == sender.sent[1] {
		t.Fatalf("duplicate/concurrent delivery: sent=%d attempts=%v", len(sender.sent), noticeAttempts(repo.SecurityNotices()))
	}
}
func noticeAttempts(notices []domain.AccountSecurityNotice) []int {
	out := make([]int, 0, len(notices))
	for _, notice := range notices {
		out = append(out, notice.Attempts)
	}
	return out
}

func TestDisabledDeliveryRetentionStillRemovesPrivateRecipients(t *testing.T) {
	repo := noticesFixture(t)
	svc := New(repo, nil, nil)
	at := time.Now().Add(domain.AccountSecurityNoticeRetention + time.Second)
	svc.now = func() time.Time { return at }
	if err := svc.CleanupOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.SecurityNotices()) != 0 {
		t.Fatal("disabled delivery retained expired addresses")
	}
}
