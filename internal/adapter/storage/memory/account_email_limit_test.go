package memory_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestConcurrentEmailLinksAllowOnlyOneBackup(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountIdentityRepo()
	owner := uuid.New()
	primary, err := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, email := range []string{"backup-a@example.test", "backup-b@example.test"} {
		wg.Add(1)
		go func(email string) {
			defer wg.Done()
			<-start
			_, err := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, email, email)
			results <- err
		}(email)
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, domain.ErrAccountEmailLimit) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("expected one backup link, got %d", successes)
	}
	rows, err := repo.ListIdentitiesByAccount(ctx, owner, 100, 0)
	if err != nil || len(rows) != 2 {
		t.Fatalf("email count: %d, error: %v", len(rows), err)
	}
	same, err := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	if err != nil || same.ID != primary.ID {
		t.Fatal("existing email must remain idempotent at limit")
	}
}
