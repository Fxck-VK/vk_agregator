// Package accountregistration turns delivered email codes into browser-bound,
// one-use registration proofs. Account creation and sessions stay in accountauth.
package accountregistration

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"math/big"
	"net/mail"
	"strings"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

const TTL = 10 * time.Minute

var (
	ErrInvalidProof = errors.New("accountregistration: invalid or expired proof")
	ErrRateLimited  = errors.New("accountregistration: rate limited")
)

type Challenge struct {
	IdentityHash string `json:"identity_hash"`
	CodeHash     string `json:"code_hash"`
	ExpiresAt    int64  `json:"expires_at"`
	Verified     bool   `json:"verified"`
}

// Verify and Take must compare and mutate atomically. Restore is SET NX with
// the original remaining lifetime, never an extension of a consumed challenge.
type Store interface {
	Save(context.Context, string, Challenge, time.Duration) error
	Delete(context.Context, string) error
	Verify(context.Context, string, string, string, int64) error
	Take(context.Context, string, string, int64) (Challenge, error)
	Restore(context.Context, string, Challenge, time.Duration) error
	Increment(context.Context, string, time.Duration) (int64, error)
}
type Sender interface {
	SendEmailLinkCode(context.Context, string, string, time.Time) error
}
type AccountAuth interface {
	RegisterVerifiedEmailPassword(context.Context, uuid.UUID, string, string) (domain.IdentityResolution, error)
	IssueSession(context.Context, uuid.UUID, accountauth.SessionMetadata) (accountauth.SessionTokens, error)
}
type Service struct {
	store  Store
	sender Sender
	auth   AccountAuth
	secret []byte
	now    func() time.Time
}

func New(store Store, sender Sender, auth AccountAuth, secret string) (*Service, error) {
	if store == nil || sender == nil || auth == nil || strings.TrimSpace(secret) == "" {
		return nil, errors.New("accountregistration: missing dependency")
	}
	return &Service{store: store, sender: sender, auth: auth, secret: []byte(secret), now: time.Now}, nil
}

func (s *Service) Request(ctx context.Context, email, network string) (string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", err
	}
	if err := s.limit(ctx, "request-email", email, 3, 15*time.Minute); err != nil {
		return "", err
	}
	// Network means the trusted transport peer, not browser-supplied IP headers.
	// Shared BFF peers get a broad ceiling; per-email throttling is independent.
	if err := s.limit(ctx, "request-peer", network, 100, 15*time.Minute); err != nil {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	binding := base64.RawURLEncoding.EncodeToString(raw)
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", number.Int64())
	expires := s.now().Add(TTL)
	row := Challenge{IdentityHash: digest(email), CodeHash: s.codeHash(binding, email, code), ExpiresAt: expires.Unix()}
	key := challengeKey(binding)
	if err := s.store.Save(ctx, key, row, TTL); err != nil {
		return "", err
	}
	if err := s.sender.SendEmailLinkCode(ctx, email, code, expires); err != nil {
		_ = s.store.Delete(ctx, key)
		return "", err
	}
	return binding, nil
}
func (s *Service) Verify(ctx context.Context, binding, email, code string) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if err := s.limit(ctx, "verify-email", email, 5, 15*time.Minute); err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	if len(binding) != 43 || len(code) != 6 {
		return ErrInvalidProof
	}
	for _, n := range code {
		if n < '0' || n > '9' {
			return ErrInvalidProof
		}
	}
	return s.store.Verify(ctx, challengeKey(binding), digest(email), s.codeHash(binding, email, code), s.now().Unix())
}

// finalize prepares the caller's session result before proof consumption becomes
// permanent. A failure must leave no partially prepared response for the caller.
func (s *Service) Complete(ctx context.Context, binding, email, password string, meta accountauth.SessionMetadata, finalize func(accountauth.SessionTokens) error) (accountauth.SessionTokens, error) {
	if err := accountauth.ValidatePassword(password); err != nil {
		return accountauth.SessionTokens{}, err
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return accountauth.SessionTokens{}, err
	}
	if len(binding) != 43 {
		return accountauth.SessionTokens{}, ErrInvalidProof
	}
	if err := s.limit(ctx, "complete-email", email, 10, 15*time.Minute); err != nil {
		return accountauth.SessionTokens{}, err
	}
	key := challengeKey(binding)
	proof, err := s.store.Take(ctx, key, digest(email), s.now().Unix())
	if err != nil {
		return accountauth.SessionTokens{}, err
	}
	// The same issued proof maps to the same server account after a transient
	// database/session failure. Repository retries never update its password.
	sum := sha256.Sum256([]byte("accountregistration:account:" + binding))
	var accountID uuid.UUID
	copy(accountID[:], sum[:16])
	accountID[6] = (accountID[6] & 15) | 64
	accountID[8] = (accountID[8] & 63) | 128
	resolution, err := s.auth.RegisterVerifiedEmailPassword(ctx, accountID, email, password)
	var tokens accountauth.SessionTokens
	if err == nil {
		tokens, err = s.auth.IssueSession(ctx, resolution.AccountID, meta)
	}
	if err == nil && finalize != nil {
		err = finalize(tokens)
	}
	if err != nil && !errors.Is(err, domain.ErrConflict) {
		remaining := time.Unix(proof.ExpiresAt, 0).Sub(s.now())
		if remaining > 0 {
			restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			_ = s.store.Restore(restoreCtx, key, proof, remaining)
		}
	}
	return tokens, err
}
func (s *Service) limit(ctx context.Context, scope, value string, max int64, window time.Duration) error {
	count, err := s.store.Increment(ctx, "accountregistration:limit:"+scope+":"+digest(value), window)
	if err != nil {
		return err
	}
	if count > max {
		return ErrRateLimited
	}
	return nil
}
func (s *Service) codeHash(binding, email, code string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte("registration\x00" + binding + "\x00" + email + "\x00" + code))
	return hex.EncodeToString(mac.Sum(nil))
}
func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func challengeKey(binding string) string { return "accountregistration:challenge:" + digest(binding) }
func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if len(email) > 254 {
		return "", domain.ErrInvalidIdentity
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", domain.ErrInvalidIdentity
	}
	return domain.NormalizeExternalIdentity(domain.IdentityProviderEmail, email)
}
