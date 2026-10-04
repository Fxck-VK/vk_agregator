package redis

import (
	"context"
	"encoding/json"
	goredis "github.com/redis/go-redis/v9"
	"time"
	"vk-ai-aggregator/internal/service/accountregistration"
)

type AccountRegistrationStore struct{ client goredis.Cmdable }

func NewAccountRegistrationStore(client goredis.Cmdable) *AccountRegistrationStore {
	return &AccountRegistrationStore{client}
}

var _ accountregistration.Store = (*AccountRegistrationStore)(nil)

var verifyRegistration = goredis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return 0 end
local row = cjson.decode(raw)
if row.identity_hash ~= ARGV[1] or row.code_hash ~= ARGV[2] or row.expires_at <= tonumber(ARGV[3]) then return 0 end
row.verified = true
redis.call('SET', KEYS[1], cjson.encode(row), 'KEEPTTL')
return 1
`)
var takeRegistration = goredis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return '' end
local row = cjson.decode(raw)
if row.identity_hash ~= ARGV[1] or not row.verified or row.expires_at <= tonumber(ARGV[2]) then return '' end
redis.call('DEL', KEYS[1])
return raw
`)

func (s *AccountRegistrationStore) Save(ctx context.Context, key string, row accountregistration.Challenge, ttl time.Duration) error {
	body, err := json.Marshal(row)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, body, ttl).Err()
}
func (s *AccountRegistrationStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
func (s *AccountRegistrationStore) Verify(ctx context.Context, key, identity, code string, now int64) error {
	result, err := verifyRegistration.Run(ctx, s.client, []string{key}, identity, code, now).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return accountregistration.ErrInvalidProof
	}
	return nil
}
func (s *AccountRegistrationStore) Take(ctx context.Context, key, identity string, now int64) (accountregistration.Challenge, error) {
	raw, err := takeRegistration.Run(ctx, s.client, []string{key}, identity, now).Text()
	if err != nil {
		return accountregistration.Challenge{}, err
	}
	if raw == "" {
		return accountregistration.Challenge{}, accountregistration.ErrInvalidProof
	}
	var row accountregistration.Challenge
	err = json.Unmarshal([]byte(raw), &row)
	return row, err
}
func (s *AccountRegistrationStore) Restore(ctx context.Context, key string, row accountregistration.Challenge, ttl time.Duration) error {
	if ttl <= 0 {
		return accountregistration.ErrInvalidProof
	}
	body, err := json.Marshal(row)
	if err != nil {
		return err
	}
	return s.client.SetNX(ctx, key, body, ttl).Err()
}
func (s *AccountRegistrationStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return incrementWithTTL.Run(ctx, s.client, []string{key}, ttl.Milliseconds()).Int64()
}
