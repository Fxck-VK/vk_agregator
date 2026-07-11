package accountoauth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const defaultOAuthHTTPTimeout = 5 * time.Second

// OIDCClaims are the normalized claims account OAuth adapters need.
type OIDCClaims struct {
	Subject       string
	Issuer        string
	Audience      []string
	Email         string
	EmailVerified bool
	ExpiresAt     time.Time
	NotBefore     time.Time
	IssuedAt      time.Time
}

// OIDCVerifyConfig describes the trust policy for one OIDC provider.
type OIDCVerifyConfig struct {
	Issuers   []string
	Audiences []string
	JWKSURL   string
	Clock     func() time.Time
	Leeway    time.Duration
}

// OIDCVerifier verifies an ID token against provider trust material.
type OIDCVerifier interface {
	VerifyIDToken(ctx context.Context, token string, cfg OIDCVerifyConfig) (OIDCClaims, error)
}

// OIDCAdapter verifies Google/Apple/VK ID ID tokens and maps them to account
// login assertions by stable provider subject.
type OIDCAdapter struct {
	provider domain.IdentityProvider
	method   domain.AccountLoginMethod
	cfg      OIDCVerifyConfig
	verifier OIDCVerifier
}

// OIDCAdapterConfig configures a provider-specific OIDC adapter.
type OIDCAdapterConfig struct {
	Provider domain.IdentityProvider
	Method   domain.AccountLoginMethod
	Issuers  []string
	Audience []string
	JWKSURL  string
	Verifier OIDCVerifier
	Clock    func() time.Time
}

// NewOIDCAdapter creates a fail-closed adapter for one OIDC provider.
func NewOIDCAdapter(cfg OIDCAdapterConfig) *OIDCAdapter {
	return &OIDCAdapter{
		provider: domain.NormalizeIdentityProvider(cfg.Provider),
		method:   cfg.Method,
		cfg: OIDCVerifyConfig{
			Issuers:   normalizedNonEmpty(cfg.Issuers),
			Audiences: normalizedNonEmpty(cfg.Audience),
			JWKSURL:   strings.TrimSpace(cfg.JWKSURL),
			Clock:     cfg.Clock,
			Leeway:    time.Minute,
		},
		verifier: cfg.Verifier,
	}
}

// Provider returns the identity provider this adapter owns.
func (a *OIDCAdapter) Provider() domain.IdentityProvider {
	if a == nil {
		return ""
	}
	return a.provider
}

// Verify checks the OIDC token and returns only a stable verified subject.
func (a *OIDCAdapter) Verify(ctx context.Context, req VerifyRequest) (domain.VerifiedAccountLogin, error) {
	if a == nil || a.provider == "" || a.method == "" || a.verifier == nil {
		return domain.VerifiedAccountLogin{}, ErrUnavailable
	}
	if domain.NormalizeIdentityProvider(req.Provider) != a.provider {
		return domain.VerifiedAccountLogin{}, ErrUnsupportedProvider
	}
	token := strings.TrimSpace(req.IDToken)
	if token == "" {
		return domain.VerifiedAccountLogin{}, ErrInvalidAssertion
	}
	if len(a.cfg.Audiences) == 0 || len(a.cfg.Issuers) == 0 {
		return domain.VerifiedAccountLogin{}, ErrUnavailable
	}
	claims, err := a.verifier.VerifyIDToken(ctx, token, a.cfg)
	if err != nil {
		return domain.VerifiedAccountLogin{}, fmt.Errorf("%w: %v", ErrInvalidAssertion, err)
	}
	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		return domain.VerifiedAccountLogin{}, ErrInvalidAssertion
	}
	return domain.VerifiedAccountLogin{
		Method:     a.method,
		ExternalID: subject,
		Verified:   true,
	}, nil
}

// RemoteJWKSOIDCVerifier verifies RS256 ID tokens using provider JWKS.
type RemoteJWKSOIDCVerifier struct {
	client *http.Client
	mu     sync.Mutex
	keys   map[string]*rsa.PublicKey
	expiry time.Time
}

// NewRemoteJWKSOIDCVerifier builds a JWKS-backed verifier.
func NewRemoteJWKSOIDCVerifier(client *http.Client) *RemoteJWKSOIDCVerifier {
	if client == nil {
		client = &http.Client{Timeout: defaultOAuthHTTPTimeout}
	}
	return &RemoteJWKSOIDCVerifier{
		client: client,
		keys:   make(map[string]*rsa.PublicKey),
	}
}

// VerifyIDToken verifies signature, issuer, audience and token time bounds.
func (v *RemoteJWKSOIDCVerifier) VerifyIDToken(ctx context.Context, token string, cfg OIDCVerifyConfig) (OIDCClaims, error) {
	if v == nil || v.client == nil {
		return OIDCClaims{}, ErrUnavailable
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return OIDCClaims{}, errors.New("malformed jwt")
	}
	var header jwtHeader
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return OIDCClaims{}, err
	}
	if header.Algorithm != "RS256" || strings.TrimSpace(header.KeyID) == "" {
		return OIDCClaims{}, errors.New("unsupported jwt header")
	}
	key, err := v.keyFor(ctx, cfg, header.KeyID)
	if err != nil {
		return OIDCClaims{}, err
	}
	signed := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return OIDCClaims{}, errors.New("invalid jwt signature encoding")
	}
	sum := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], signature); err != nil {
		return OIDCClaims{}, errors.New("invalid jwt signature")
	}
	var raw rawOIDCClaims
	if err := decodeJWTPart(parts[1], &raw); err != nil {
		return OIDCClaims{}, err
	}
	claims := raw.toClaims()
	if err := validateOIDCClaims(claims, cfg); err != nil {
		return OIDCClaims{}, err
	}
	return claims, nil
}

func (v *RemoteJWKSOIDCVerifier) keyFor(ctx context.Context, cfg OIDCVerifyConfig, kid string) (*rsa.PublicKey, error) {
	now := clockNow(cfg.Clock)
	v.mu.Lock()
	if key := v.keys[kid]; key != nil && now.Before(v.expiry) {
		v.mu.Unlock()
		return key, nil
	}
	v.mu.Unlock()

	keys, expiry, err := v.fetchKeys(ctx, cfg.JWKSURL)
	if err != nil {
		return nil, err
	}
	v.mu.Lock()
	v.keys = keys
	v.expiry = expiry
	key := v.keys[kid]
	v.mu.Unlock()
	if key == nil {
		return nil, errors.New("jwt key not found")
	}
	return key, nil
}

func (v *RemoteJWKSOIDCVerifier) fetchKeys(ctx context.Context, jwksURL string) (map[string]*rsa.PublicKey, time.Time, error) {
	jwksURL = strings.TrimSpace(jwksURL)
	if jwksURL == "" {
		return nil, time.Time{}, ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, time.Time{}, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, time.Time{}, fmt.Errorf("jwks status %d", resp.StatusCode)
	}
	var jwks jwksResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&jwks); err != nil {
		return nil, time.Time{}, err
	}
	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" || key.Kid == "" || key.N == "" || key.E == "" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(key)
		if err != nil {
			continue
		}
		keys[key.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, time.Time{}, errors.New("jwks has no usable rsa keys")
	}
	expiry := time.Now().Add(time.Hour)
	if cc := resp.Header.Get("Cache-Control"); strings.Contains(cc, "max-age=") {
		if maxAge, ok := parseMaxAge(cc); ok {
			expiry = time.Now().Add(maxAge)
		}
	}
	return keys, expiry, nil
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Type      string `json:"typ"`
}

type rawOIDCClaims struct {
	Subject       string       `json:"sub"`
	Issuer        string       `json:"iss"`
	Audience      audienceList `json:"aud"`
	Email         string       `json:"email"`
	EmailVerified bool         `json:"email_verified"`
	ExpiresAt     int64        `json:"exp"`
	NotBefore     int64        `json:"nbf"`
	IssuedAt      int64        `json:"iat"`
}

func (c rawOIDCClaims) toClaims() OIDCClaims {
	claims := OIDCClaims{
		Subject:       c.Subject,
		Issuer:        c.Issuer,
		Audience:      []string(c.Audience),
		Email:         c.Email,
		EmailVerified: c.EmailVerified,
	}
	if c.ExpiresAt > 0 {
		claims.ExpiresAt = time.Unix(c.ExpiresAt, 0)
	}
	if c.NotBefore > 0 {
		claims.NotBefore = time.Unix(c.NotBefore, 0)
	}
	if c.IssuedAt > 0 {
		claims.IssuedAt = time.Unix(c.IssuedAt, 0)
	}
	return claims
}

type audienceList []string

func (a *audienceList) UnmarshalJSON(data []byte) error {
	var one string
	if err := json.Unmarshal(data, &one); err == nil {
		*a = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func decodeJWTPart(part string, dst any) error {
	data, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return errors.New("invalid jwt encoding")
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return errors.New("invalid jwt json")
	}
	return nil
}

func validateOIDCClaims(claims OIDCClaims, cfg OIDCVerifyConfig) error {
	now := clockNow(cfg.Clock)
	leeway := cfg.Leeway
	if leeway == 0 {
		leeway = time.Minute
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return errors.New("missing subject")
	}
	if !containsToken(cfg.Issuers, claims.Issuer) {
		return errors.New("invalid issuer")
	}
	audOK := false
	for _, aud := range claims.Audience {
		if containsToken(cfg.Audiences, aud) {
			audOK = true
			break
		}
	}
	if !audOK {
		return errors.New("invalid audience")
	}
	if claims.ExpiresAt.IsZero() || now.After(claims.ExpiresAt.Add(leeway)) {
		return errors.New("expired token")
	}
	if !claims.NotBefore.IsZero() && now.Add(leeway).Before(claims.NotBefore) {
		return errors.New("token not yet valid")
	}
	return nil
}

func rsaPublicKeyFromJWK(key jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, err
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, errors.New("invalid exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

func parseMaxAge(header string) (time.Duration, bool) {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "max-age=") {
			continue
		}
		raw := strings.TrimPrefix(part, "max-age=")
		var seconds int64
		if _, err := fmt.Sscanf(raw, "%d", &seconds); err != nil || seconds <= 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	return 0, false
}

func normalizedNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
