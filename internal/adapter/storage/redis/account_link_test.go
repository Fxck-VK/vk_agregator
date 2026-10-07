package redis_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	storage "vk-ai-aggregator/internal/adapter/storage/redis"
	"vk-ai-aggregator/internal/service/accountlink"
)

func TestRedisAccountLinkConsumptionIsAtomicAndExact(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal("test Redis unavailable")
	}
	store := storage.NewAccountLinkStore(client)
	consumer, ok := any(store).(interface {
		ConsumeChallenge(context.Context, string, accountlink.Challenge) error
	})
	if !ok {
		t.Fatal("atomic challenge consumption missing")
	}
	key := "accountlink-test:" + uuid.NewString()
	defer client.Del(context.Background(), key)
	row := accountlink.Challenge{AccountID: uuid.New(), IdentityHash: "test-identity-hash", CodeHash: "test-code-hash", BackupIdentityID: uuid.New(), BackupVersion: time.Now().UTC(), ExpiresAt: time.Now().Add(time.Minute).UTC()}
	for _, change := range []func(*accountlink.Challenge){
		func(c *accountlink.Challenge) { c.AccountID = uuid.New() },
		func(c *accountlink.Challenge) { c.IdentityHash += "different" },
		func(c *accountlink.Challenge) { c.CodeHash += "different" },
		func(c *accountlink.Challenge) { c.BackupIdentityID = uuid.New() },
		func(c *accountlink.Challenge) { c.BackupVersion = c.BackupVersion.Add(time.Nanosecond) },
		func(c *accountlink.Challenge) { c.ExpiresAt = c.ExpiresAt.Add(time.Nanosecond) },
	} {
		if err := store.SaveChallenge(ctx, key, row, time.Minute); err != nil {
			t.Fatal("save test challenge failed")
		}
		loaded, err := store.LoadChallenge(ctx, key)
		if err != nil {
			t.Fatal("load test challenge failed")
		}
		replacement := loaded
		change(&replacement)
		if err := store.SaveChallenge(ctx, key, replacement, time.Minute); err != nil {
			t.Fatal("replace test challenge failed")
		}
		if err := consumer.ConsumeChallenge(ctx, key, loaded); !errors.Is(err, accountlink.ErrInvalidCode) {
			t.Fatal("stale proof consumed replacement")
		}
		if err := consumer.ConsumeChallenge(ctx, key, replacement); err != nil {
			t.Fatal("stale proof deleted replacement")
		}
	}
	if err := store.SaveChallenge(ctx, key, row, time.Minute); err != nil {
		t.Fatal("save test challenge failed")
	}
	loaded, err := store.LoadChallenge(ctx, key)
	if err != nil {
		t.Fatal("load test challenge failed")
	}
	results := make(chan error, 16)
	start := make(chan struct{})
	for i := 0; i < cap(results); i++ {
		go func() { <-start; results <- consumer.ConsumeChallenge(ctx, key, loaded) }()
	}
	close(start)
	winners := 0
	for i := 0; i < cap(results); i++ {
		err := <-results
		if err == nil {
			winners++
		} else if !errors.Is(err, accountlink.ErrInvalidCode) {
			t.Fatal("consume test challenge failed")
		}
	}
	if winners != 1 {
		t.Fatalf("proof authorized %d callers; want one", winners)
	}
	row.ExpiresAt = time.Now().Add(-time.Second)
	if err := store.SaveChallenge(ctx, key, row, time.Minute); err != nil {
		t.Fatal("save expired test challenge failed")
	}
	if err := consumer.ConsumeChallenge(ctx, key, row); !errors.Is(err, accountlink.ErrExpiredCode) {
		t.Fatal("expired proof accepted")
	}
	if err := client.Close(); err != nil {
		t.Fatal("close test connection failed")
	}
	if err := consumer.ConsumeChallenge(ctx, key, row); err == nil || errors.Is(err, accountlink.ErrInvalidCode) || errors.Is(err, accountlink.ErrExpiredCode) {
		t.Fatal("store failure not propagated")
	}
}
