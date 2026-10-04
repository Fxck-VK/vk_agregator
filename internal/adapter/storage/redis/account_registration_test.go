package redis_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"os"
	"sync"
	"testing"
	"time"
	storage "vk-ai-aggregator/internal/adapter/storage/redis"
	"vk-ai-aggregator/internal/service/accountregistration"
)

func TestRedisRegistrationCompareVerifyAndTakeAreAtomic(t *testing.T) {
	endpoint := os.Getenv("TEST_REDIS_URL")
	if endpoint == "" {
		t.Skip("TEST_REDIS_URL not set; isolated Redis integration skipped")
	}
	options, err := goredis.ParseURL(endpoint)
	if err != nil {
		t.Fatal("invalid Redis test configuration")
	}
	client := goredis.NewClient(options)
	defer client.Close()
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal("test Redis unavailable")
	}
	store := storage.NewAccountRegistrationStore(client)
	key := "registration-test:" + uuid.NewString()
	defer client.Del(ctx, key)
	row := accountregistration.Challenge{IdentityHash: "identity-test-hash", CodeHash: "code-test-hash", ExpiresAt: time.Now().Add(time.Minute).Unix()}
	if err := store.Save(ctx, key, row, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Take(ctx, key, row.IdentityHash, time.Now().Unix()); !errors.Is(err, accountregistration.ErrInvalidProof) {
		t.Fatal("unverified proof consumed")
	}
	if err := store.Verify(ctx, key, "different-identity", row.CodeHash, time.Now().Unix()); !errors.Is(err, accountregistration.ErrInvalidProof) {
		t.Fatal("foreign identity verified")
	}
	if err := store.Verify(ctx, key, row.IdentityHash, row.CodeHash, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	out := make(chan error, 2)
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := store.Take(ctx, key, row.IdentityHash, time.Now().Unix())
			out <- err
		}()
	}
	group.Wait()
	close(out)
	winners := 0
	for err := range out {
		if err == nil {
			winners++
		} else if !errors.Is(err, accountregistration.ErrInvalidProof) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatal("proof taken more than once")
	}
	row.Verified = true
	if err := store.Restore(ctx, key, row, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Take(ctx, key, row.IdentityHash, row.ExpiresAt); !errors.Is(err, accountregistration.ErrInvalidProof) {
		t.Fatal("expired proof consumed")
	}
}
