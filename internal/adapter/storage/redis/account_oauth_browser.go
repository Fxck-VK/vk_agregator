package redis

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"vk-ai-aggregator/internal/adapter/accountoauth"
)

var takeOAuthTransaction = goredis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value then return false end
local tx = cjson.decode(value)
if tx.binding_hash ~= ARGV[1] then return false end
redis.call("DEL", KEYS[1])
return value
`)

type AccountOAuthBrowserStore struct{ client goredis.Cmdable }

func NewAccountOAuthBrowserStore(client goredis.Cmdable) *AccountOAuthBrowserStore {
	return &AccountOAuthBrowserStore{client}
}
func (s *AccountOAuthBrowserStore) Save(ctx context.Context, key string, tx accountoauth.BrowserTransaction, ttl time.Duration) error {
	body, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, body, ttl).Err()
}
func (s *AccountOAuthBrowserStore) Take(ctx context.Context, key, binding string) (accountoauth.BrowserTransaction, error) {
	body, err := takeOAuthTransaction.Run(ctx, s.client, []string{key}, binding).Text()
	if err != nil {
		return accountoauth.BrowserTransaction{}, accountoauth.ErrInvalidAssertion
	}
	var tx accountoauth.BrowserTransaction
	if err := json.Unmarshal([]byte(body), &tx); err != nil {
		return tx, accountoauth.ErrInvalidAssertion
	}
	return tx, nil
}
