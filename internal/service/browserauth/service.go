// Package browserauth coordinates browser-owned email proofs. Receipts are
// authenticated/encrypted; Redis retains only hashes, account IDs and deadlines.
package browserauth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountlink"
)

const ChallengeTTL = 10 * time.Minute
const ResetTTL = 15 * time.Minute
const RememberTTL = 30 * 24 * time.Hour

var ErrProof = errors.New("browserauth: invalid or expired proof")

type Auth interface {
	CapturePasswordProof(context.Context, string, string) (accountauth.PasswordProof, error)
	CaptureRecoveryProof(context.Context, string) (accountauth.PasswordProof, error)
	ValidatePasswordProof(context.Context, accountauth.PasswordProof) error
	CompletePasswordProof(context.Context, accountauth.PasswordProof, accountauth.SessionMetadata) (accountauth.SessionTokens, error)
	ResetPasswordProof(context.Context, accountauth.PasswordProof, string) error
}
type Sender interface {
	SendEmailLinkCode(context.Context, string, string, time.Time) error
	SendPasswordResetLink(context.Context, string, string, time.Time) error
}
type challengeRestorer interface {
	RestoreChallenge(context.Context, string, accountlink.Challenge, time.Duration) error
}
type Service struct {
	store  accountlink.Store
	sender Sender
	auth   Auth
	box    cipher.AEAD
	secret []byte
	origin string
	now    func() time.Time
}
type receipt struct {
	Purpose    string                    `json:"purpose"`
	Nonce      string                    `json:"nonce"`
	Binding    string                    `json:"binding,omitempty"`
	Agent      string                    `json:"agent,omitempty"`
	Proof      accountauth.PasswordProof `json:"proof"`
	Email      string                    `json:"email,omitempty"`
	Provider   string                    `json:"provider,omitempty"`
	Label      string                    `json:"label,omitempty"`
	Persistent *bool                     `json:"persistent,omitempty"`
	Expires    time.Time                 `json:"expires"`
}
type Outcome struct {
	Tokens                         *accountauth.SessionTokens
	Challenge, Trusted, Remembered string
}

func New(store accountlink.Store, sender Sender, auth Auth, origin, secret string) (*Service, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) || len(secret) < 32 || store == nil || sender == nil || auth == nil {
		return nil, errors.New("browserauth: invalid configuration")
	}
	key := sha256.Sum256([]byte("epifera:browser-auth:v1:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	box, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Service{store: store, sender: sender, auth: auth, box: box, secret: []byte(secret), origin: strings.TrimRight(origin, "/"), now: time.Now}, nil
}
func digest(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
func secretToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func normalize(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	m, err := mail.ParseAddress(email)
	if err != nil || m.Address != email || len(email) > 254 {
		return "", domain.ErrInvalidIdentity
	}
	return email, nil
}
func (s *Service) seal(r receipt) (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, s.box.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(s.box.Seal(nonce, nonce, b, []byte("epifera:browser-auth:v1"))), nil
}
func (s *Service) open(token, purpose string) (receipt, error) {
	var r receipt
	if len(token) < 32 || len(token) > 4096 {
		return r, ErrProof
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) < s.box.NonceSize() {
		return r, ErrProof
	}
	plain, err := s.box.Open(nil, b[:s.box.NonceSize()], b[s.box.NonceSize():], []byte("epifera:browser-auth:v1"))
	if err != nil || json.Unmarshal(plain, &r) != nil || r.Purpose != purpose || !r.Expires.After(s.now()) {
		return receipt{}, ErrProof
	}
	return r, nil
}
func (s *Service) limit(ctx context.Context, scope, value string, max int64) error {
	n, err := s.store.Increment(ctx, "browserauth:limit:"+scope+":"+digest(value), 15*time.Minute)
	if err != nil {
		return err
	}
	if n > max {
		return accountlink.ErrRateLimited
	}
	return nil
}
func (s *Service) rowKey(purpose, nonce string) string {
	return "browserauth:" + purpose + ":" + digest(nonce)
}
func (s *Service) codeHash(token, binding, code string) string {
	h := hmac.New(sha256.New, s.secret)
	_, _ = h.Write([]byte("browser-login:v1:" + token + ":" + binding + ":" + code))
	return hex.EncodeToString(h.Sum(nil))
}
func (s *Service) Begin(ctx context.Context, email, password, binding, trust, agent string, meta accountauth.SessionMetadata) (Outcome, error) {
	email, err := normalize(email)
	if err != nil {
		return Outcome{}, accountauth.ErrInvalidPasswordLogin
	}
	p, err := s.auth.CapturePasswordProof(ctx, email, password)
	if err != nil {
		return Outcome{}, err
	}
	if r, e := s.open(trust, "trust"); e == nil && r.Binding == digest(binding) && r.Agent == digest(agent) &&
		r.Proof.EmailBinding.ID == p.EmailBinding.ID && r.Proof.EmailBinding.AccountID == p.EmailBinding.AccountID &&
		r.Proof.EmailBinding.NormalizedID == p.EmailBinding.NormalizedID && r.Proof.EmailBinding.UpdatedAt.Equal(p.EmailBinding.UpdatedAt) &&
		r.Proof.ExpectedHash == p.ExpectedHash {
		if err := s.auth.ValidatePasswordProof(ctx, r.Proof); err != nil {
			return Outcome{}, err
		}
		tokens, err := s.auth.CompletePasswordProof(ctx, p, meta)
		if err != nil {
			return Outcome{}, err
		}
		return s.finished(p, binding, agent, tokens)
	}
	nonce, err := secretToken()
	if err != nil {
		return Outcome{}, err
	}
	r := receipt{Purpose: "login", Nonce: nonce, Binding: digest(binding), Agent: digest(agent), Proof: p, Persistent: meta.Persistent, Expires: s.now().Add(ChallengeTTL)}
	token, err := s.seal(r)
	if err != nil {
		return Outcome{}, err
	}
	if err := s.sendCode(ctx, token, binding, r); err != nil {
		return Outcome{}, err
	}
	return Outcome{Challenge: token}, nil
}
func (s *Service) sendCode(ctx context.Context, token, binding string, r receipt) error {
	email := r.Proof.EmailBinding.ExternalID
	if err := s.limit(ctx, "login-code", email, 3); err != nil {
		return err
	}
	code, row, err := s.newCode(token, binding, r)
	if err != nil {
		return err
	}
	key := s.rowKey("login", r.Nonce)
	if err := s.store.SaveChallenge(ctx, key, row, r.Expires.Sub(s.now())); err != nil {
		return err
	}
	if err := s.sender.SendEmailLinkCode(ctx, email, code, r.Expires); err != nil {
		_ = s.store.ConsumeChallenge(ctx, key, row)
		return err
	}
	return nil
}
func (s *Service) newCode(token, binding string, r receipt) (string, accountlink.Challenge, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", accountlink.Challenge{}, err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	row := accountlink.Challenge{AccountID: r.Proof.EmailBinding.AccountID, IdentityHash: digest(token), CodeHash: s.codeHash(token, binding, code), ExpiresAt: r.Expires}
	return code, row, nil
}
func (s *Service) Resend(ctx context.Context, token, binding, agent string) error {
	r, err := s.loginReceipt(ctx, token, binding, agent)
	if err != nil {
		return err
	}
	key := s.rowKey("login", r.Nonce)
	row, err := s.store.LoadChallenge(ctx, key)
	if err != nil || row.IdentityHash != digest(token) || !row.ExpiresAt.After(s.now()) {
		return ErrProof
	}
	if _, ok := s.store.(challengeRestorer); !ok {
		return ErrProof
	}
	email := r.Proof.EmailBinding.ExternalID
	// Rejected limits and entropy failures must leave the current code usable.
	if err := s.limit(ctx, "login-code", email, 3); err != nil {
		return err
	}
	code, replacement, err := s.newCode(token, binding, r)
	if err != nil {
		return err
	}
	// Claim the live generation before replacing it. A concurrent successful
	// verification consumes the same row and makes this resend fail closed.
	if err := s.store.ConsumeChallenge(ctx, key, row); err != nil {
		return ErrProof
	}
	// Publish only after delivery acknowledges success. Otherwise verification
	// could consume the replacement while delivery is pending, and restoring the
	// previous row after a delivery error would resurrect an already used proof.
	if err := s.sender.SendEmailLinkCode(ctx, email, code, r.Expires); err != nil {
		_ = s.restoreChallenge(ctx, key, row)
		return err
	}
	if err := s.restoreChallenge(ctx, key, replacement); err != nil {
		// NX preserves a replacement that was stored despite an ambiguous error.
		_ = s.restoreChallenge(ctx, key, row)
		return err
	}
	return nil
}
func (s *Service) loginReceipt(ctx context.Context, token, binding, agent string) (receipt, error) {
	r, err := s.open(token, "login")
	if err != nil || r.Binding != digest(binding) || r.Agent != digest(agent) {
		return receipt{}, ErrProof
	}
	if err = s.auth.ValidatePasswordProof(ctx, r.Proof); err != nil {
		return receipt{}, ErrProof
	}
	return r, nil
}
func (s *Service) Verify(ctx context.Context, token, binding, code, agent string, meta accountauth.SessionMetadata) (Outcome, error) {
	r, err := s.loginReceipt(ctx, token, binding, agent)
	if err != nil {
		return Outcome{}, err
	}
	if err := s.limit(ctx, "login-verify", r.Nonce, 5); err != nil {
		return Outcome{}, err
	}
	row, err := s.store.LoadChallenge(ctx, s.rowKey("login", r.Nonce))
	if err != nil {
		return Outcome{}, ErrProof
	}
	expected := s.codeHash(token, binding, strings.TrimSpace(code))
	if !row.ExpiresAt.After(s.now()) || row.IdentityHash != digest(token) || !hmac.Equal([]byte(row.CodeHash), []byte(expected)) {
		return Outcome{}, ErrProof
	}
	if err := s.store.ConsumeChallenge(ctx, s.rowKey("login", r.Nonce), row); err != nil {
		return Outcome{}, ErrProof
	}
	meta.Persistent = r.Persistent
	tokens, err := s.auth.CompletePasswordProof(ctx, r.Proof, meta)
	if err != nil {
		s.restore(ctx, s.rowKey("login", r.Nonce), row, err)
		return Outcome{}, err
	}
	return s.finished(r.Proof, binding, agent, tokens)
}
func (s *Service) finished(p accountauth.PasswordProof, binding, agent string, tokens accountauth.SessionTokens) (Outcome, error) {
	if p.Upgrade != nil {
		p.ExpectedHash = p.Upgrade.SecretHash
		p.Upgrade = nil
	}
	trusted, err := s.seal(receipt{Purpose: "trust", Binding: digest(binding), Agent: digest(agent), Proof: p, Expires: s.now().Add(RememberTTL)})
	if err != nil {
		return Outcome{}, err
	}
	remembered, err := s.seal(receipt{Purpose: "hint", Email: p.EmailBinding.ExternalID, Expires: s.now().Add(RememberTTL)})
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{Tokens: &tokens, Trusted: trusted, Remembered: remembered}, nil
}
func (s *Service) RememberedEmail(token string) (string, error) {
	r, err := s.open(token, "hint")
	if err != nil {
		return "", err
	}
	return normalize(r.Email)
}
func (s *Service) Hint(token string) (string, error) {
	email, err := s.RememberedEmail(token)
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "", ErrProof
	}
	local := []rune(parts[0])
	return string(local[:1]) + "***@" + parts[1], nil
}

// ProviderHint remembers presentation only. It never authorizes a session or
// skips the external provider's own authentication.
func (s *Service) ProviderHint(provider, label string) (string, error) {
	switch provider {
	case "vk", "yandex", "apple", "google", "telegram":
	default:
		return "", ErrProof
	}
	if len(label) > 128 {
		label = ""
	}
	return s.seal(receipt{Purpose: "hint", Provider: provider, Label: strings.TrimSpace(label), Expires: s.now().Add(RememberTTL)})
}
func (s *Service) HintInfo(token string) (label, provider string, err error) {
	r, err := s.open(token, "hint")
	if err != nil {
		return "", "", err
	}
	if r.Provider != "" {
		return r.Label, r.Provider, nil
	}
	label, err = s.Hint(token)
	return label, "", err
}
func (s *Service) RequestReset(ctx context.Context, email, locale, network string) error {
	email, err := normalize(email)
	if err != nil {
		return err
	}
	if err = s.limit(ctx, "reset-email", email, 3); err != nil {
		return err
	}
	if err = s.limit(ctx, "reset-peer", network, 100); err != nil {
		return err
	}
	p, err := s.auth.CaptureRecoveryProof(ctx, email)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidIdentity) || errors.Is(err, accountauth.ErrInvalidPasswordLogin) {
		return nil
	}
	if err != nil {
		return err
	}
	nonce, err := secretToken()
	if err != nil {
		return err
	}
	r := receipt{Purpose: "reset", Nonce: nonce, Proof: p, Expires: s.now().Add(ResetTTL)}
	token, err := s.seal(r)
	if err != nil {
		return err
	}
	row := accountlink.Challenge{AccountID: p.EmailBinding.AccountID, IdentityHash: digest(token), CodeHash: digest(token), ExpiresAt: r.Expires}
	key := s.rowKey("reset", nonce)
	if err = s.store.SaveChallenge(ctx, key, row, ResetTTL); err != nil {
		return err
	}
	if locale != "en" {
		locale = "ru"
	}
	link := s.origin + "/" + locale + "/login/reset#token=" + url.QueryEscape(token)
	if err = s.sender.SendPasswordResetLink(ctx, email, link, r.Expires); err != nil {
		_ = s.store.ConsumeChallenge(ctx, key, row)
		return err
	}
	return nil
}
func (s *Service) InspectReset(ctx context.Context, token string) (time.Time, error) {
	r, err := s.open(token, "reset")
	if err != nil {
		return time.Time{}, err
	}
	row, err := s.store.LoadChallenge(ctx, s.rowKey("reset", r.Nonce))
	if err != nil || row.IdentityHash != digest(token) || !row.ExpiresAt.After(s.now()) {
		return time.Time{}, ErrProof
	}
	if err = s.auth.ValidatePasswordProof(ctx, r.Proof); err != nil {
		return time.Time{}, ErrProof
	}
	return r.Expires, nil
}
func (s *Service) Reset(ctx context.Context, token, password string) error {
	if err := accountauth.ValidatePassword(password); err != nil {
		return err
	}
	if _, err := s.InspectReset(ctx, token); err != nil {
		return err
	}
	r, err := s.open(token, "reset")
	if err != nil {
		return err
	}
	key := s.rowKey("reset", r.Nonce)
	row, err := s.store.LoadChallenge(ctx, key)
	if err != nil {
		return ErrProof
	}
	if err = s.store.ConsumeChallenge(ctx, key, row); err != nil {
		return ErrProof
	}
	err = s.auth.ResetPasswordProof(ctx, r.Proof, password)
	if err != nil {
		s.restore(ctx, key, row, err)
	}
	return err
}

func (s *Service) restore(ctx context.Context, key string, row accountlink.Challenge, failure error) {
	if errors.Is(failure, accountauth.ErrInvalidPasswordLogin) || errors.Is(failure, accountauth.ErrWeakPassword) || errors.Is(failure, domain.ErrConflict) || errors.Is(failure, domain.ErrNotFound) {
		return
	}
	_ = s.restoreChallenge(ctx, key, row)
}

func (s *Service) restoreChallenge(ctx context.Context, key string, row accountlink.Challenge) error {
	store, ok := s.store.(challengeRestorer)
	remaining := row.ExpiresAt.Sub(s.now())
	if !ok || remaining <= 0 {
		return ErrProof
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	return store.RestoreChallenge(bounded, key, row, remaining)
}

// RememberVerified is only called after successful server-owned registration.
func (s *Service) RememberVerified(ctx context.Context, email, binding, agent string) (Outcome, error) {
	p, err := s.auth.CaptureRecoveryProof(ctx, email)
	if err != nil {
		return Outcome{}, err
	}
	p.PasswordVerified = true
	return s.finished(p, binding, agent, accountauth.SessionTokens{})
}
