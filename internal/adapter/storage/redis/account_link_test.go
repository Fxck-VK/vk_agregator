package redis_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	storage "vk-ai-aggregator/internal/adapter/storage/redis"
	"vk-ai-aggregator/internal/service/accountlink"
)

type restoredChallenge struct {
	body    string
	expires time.Time
}

// Embed Cmdable so an unexpected operation cannot silently pass this narrow
// command-contract test. Only SET NX is admitted, without a live Redis service.
type restoreClient struct {
	goredis.Cmdable
	rows  map[string]restoredChallenge
	calls int
}

func (c *restoreClient) SetNX(_ context.Context, key string, value interface{}, ttl time.Duration) *goredis.BoolCmd {
	c.calls++
	if _, exists := c.rows[key]; exists {
		return goredis.NewBoolResult(false, nil)
	}
	c.rows[key] = restoredChallenge{body: string(value.([]byte)), expires: time.Now().Add(ttl)}
	return goredis.NewBoolResult(true, nil)
}

func TestAccountLinkRestoreUsesNXAndNeverExtendsOriginalDeadline(t *testing.T) {
	ctx := context.Background()
	client := &restoreClient{rows: map[string]restoredChallenge{}}
	store := storage.NewAccountLinkStore(client)
	deadline := time.Now().Add(10 * time.Minute)
	challenge := accountlink.Challenge{AccountID: uuid.New(), IdentityHash: "synthetic-identity-hash", CodeHash: "synthetic-code-hash", ExpiresAt: deadline}
	newer := restoredChallenge{body: "newer-state", expires: time.Now().Add(time.Minute)}
	client.rows["proof"] = newer
	if err := store.RestoreChallenge(ctx, "proof", challenge, time.Hour); err != nil {
		t.Fatal(err)
	}
	if client.rows["proof"] != newer {
		t.Fatal("restore replaced a newer row or extended its expiry")
	}
	delete(client.rows, "proof")
	if err := store.RestoreChallenge(ctx, "proof", challenge, time.Hour); err != nil {
		t.Fatal(err)
	}
	stored := client.rows["proof"]
	if stored.expires.After(deadline.Add(time.Millisecond)) {
		t.Fatal("restore extended the original challenge deadline")
	}
	if !stored.expires.After(time.Now()) {
		t.Fatal("restored challenge already expired")
	}
	var decoded accountlink.Challenge
	if err := json.Unmarshal([]byte(stored.body), &decoded); err != nil || decoded.AccountID != challenge.AccountID || decoded.CodeHash != challenge.CodeHash {
		t.Fatal("restore changed the challenge")
	}
	if err := store.RestoreChallenge(ctx, "proof", challenge, time.Hour); err != nil {
		t.Fatal(err)
	}
	if client.rows["proof"] != stored {
		t.Fatal("repeated restore extended the live proof")
	}
	before := client.calls
	challenge.ExpiresAt = time.Now().Add(-time.Second)
	if err := store.RestoreChallenge(ctx, "expired", challenge, time.Minute); !errors.Is(err, accountlink.ErrExpiredCode) {
		t.Fatalf("expired restore error = %v", err)
	}
	challenge.ExpiresAt = deadline
	if err := store.RestoreChallenge(ctx, "zero", challenge, 0); !errors.Is(err, accountlink.ErrExpiredCode) {
		t.Fatalf("zero TTL restore error = %v", err)
	}
	if client.calls != before {
		t.Fatal("expired proof reached Redis")
	}
}

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
