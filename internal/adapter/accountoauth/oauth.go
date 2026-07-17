// Package accountoauth verifies external OAuth/login assertions before they
// enter the shared account identity layer.
package accountoauth

import (
	"context"
	"errors"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
)

var (
	// ErrUnsupportedProvider is returned when no adapter exists for the
	// requested OAuth provider.
	ErrUnsupportedProvider = errors.New("accountoauth: unsupported provider")
	// ErrUnavailable is returned when an adapter is not configured.
	ErrUnavailable = errors.New("accountoauth: provider unavailable")
	// ErrInvalidAssertion is returned when an external assertion fails
	// provider-specific verification.
	ErrInvalidAssertion = errors.New("accountoauth: invalid assertion")
)

// VerifyRequest contains raw provider-specific proof. It must not be logged.
type VerifyRequest struct {
	Provider domain.IdentityProvider
	IDToken  string
	AuthData map[string]string
}

// Adapter verifies one provider-specific assertion and returns only the safe
// verified account-login boundary object.
type Adapter interface {
	Provider() domain.IdentityProvider
	Verify(ctx context.Context, req VerifyRequest) (domain.VerifiedAccountLogin, error)
}

// Registry routes verification to provider-specific adapters.
type Registry struct {
	adapters map[domain.IdentityProvider]Adapter
}

// NewRegistry builds a provider registry. Nil adapters are ignored.
func NewRegistry(adapters ...Adapter) *Registry {
	r := &Registry{adapters: make(map[domain.IdentityProvider]Adapter, len(adapters))}
	for _, adapter := range adapters {
		if adapter == nil {
			continue
		}
		provider := domain.NormalizeIdentityProvider(adapter.Provider())
		if provider == "" {
			continue
		}
		r.adapters[provider] = adapter
	}
	return r
}

// Verify verifies the raw provider proof and returns a verified login.
func (r *Registry) Verify(ctx context.Context, req VerifyRequest) (domain.VerifiedAccountLogin, error) {
	if r == nil {
		return domain.VerifiedAccountLogin{}, ErrUnavailable
	}
	provider := domain.NormalizeIdentityProvider(req.Provider)
	adapter := r.adapters[provider]
	if adapter == nil {
		return domain.VerifiedAccountLogin{}, ErrUnsupportedProvider
	}
	return adapter.Verify(ctx, req)
}

func containsToken(tokens []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, token := range tokens {
		if strings.TrimSpace(token) == target {
			return true
		}
	}
	return false
}

func clockNow(clock func() time.Time) time.Time {
	if clock != nil {
		return clock()
	}
	return time.Now()
}
