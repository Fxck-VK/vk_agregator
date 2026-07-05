package accountauth_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestResolveOrCreateSupportsAllLoginMethods(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), identities, nil)
	service := accountauth.New(resolver)

	tests := []struct {
		name       string
		login      domain.VerifiedAccountLogin
		provider   domain.IdentityProvider
		normalized string
	}{
		{
			name: "email password",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginEmailPassword,
				ExternalID: "User@Example.COM",
				Verified:   true,
			},
			provider:   domain.IdentityProviderEmail,
			normalized: "user@example.com",
		},
		{
			name: "phone otp",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginPhoneOTP,
				ExternalID: "+7 (999) 123-45-67",
				Verified:   true,
			},
			provider:   domain.IdentityProviderPhone,
			normalized: "+79991234567",
		},
		{
			name: "google",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginGoogle,
				ExternalID: "Google-Subject-123",
				Verified:   true,
			},
			provider:   domain.IdentityProviderGoogle,
			normalized: "google-subject-123",
		},
		{
			name: "apple",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginApple,
				ExternalID: "Apple-Subject-123",
				Verified:   true,
			},
			provider:   domain.IdentityProviderApple,
			normalized: "apple-subject-123",
		},
		{
			name: "telegram",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginTelegram,
				ExternalID: "+987654",
				Verified:   true,
			},
			provider:   domain.IdentityProviderTelegram,
			normalized: "987654",
		},
		{
			name: "vk id",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginVKID,
				ExternalID: "+777",
				Verified:   true,
			},
			provider:   domain.IdentityProviderVK,
			normalized: "777",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, err := service.ResolveOrCreate(ctx, tt.login)
			if err != nil {
				t.Fatalf("resolve first: %v", err)
			}
			if first.AccountID == uuid.Nil || first.Identity == nil {
				t.Fatalf("incomplete resolution: %+v", first)
			}
			if first.Identity.Provider != tt.provider || first.Identity.NormalizedID != tt.normalized {
				t.Fatalf("identity = %+v, want provider=%s normalized=%s", first.Identity, tt.provider, tt.normalized)
			}

			second, err := service.ResolveOrCreate(ctx, tt.login)
			if err != nil {
				t.Fatalf("resolve second: %v", err)
			}
			if second.AccountID != first.AccountID || second.Identity.ID != first.Identity.ID {
				t.Fatalf("resolve is not idempotent: first=%+v second=%+v", first, second)
			}
		})
	}
}

func TestLinkUnlinkAuditAndSafeRef(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), identities, nil)
	service := accountauth.New(resolver)

	account, err := service.ResolveVerifiedEmailPassword(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("resolve owner: %v", err)
	}
	linked, err := service.LinkVerifiedIdentity(ctx, account.AccountID, account.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginPhoneOTP,
		ExternalID: "+7 (999) 111-22-33",
		Verified:   true,
	})
	if err != nil {
		t.Fatalf("link identity: %v", err)
	}
	if linked.ExternalID == "" || linked.NormalizedID == "" {
		t.Fatalf("test setup expected raw identity in domain model")
	}

	safeJSON, err := json.Marshal(linked.SafeRef())
	if err != nil {
		t.Fatalf("marshal safe ref: %v", err)
	}
	for _, forbidden := range []string{linked.ExternalID, linked.NormalizedID} {
		if strings.Contains(string(safeJSON), forbidden) {
			t.Fatalf("safe identity ref leaked %q: %s", forbidden, safeJSON)
		}
	}

	if err := service.UnlinkIdentity(ctx, account.AccountID, account.AccountID, linked.ID); err != nil {
		t.Fatalf("unlink identity: %v", err)
	}
	audits := identities.AuditEntries()
	if len(audits) < 3 {
		t.Fatalf("expected create/link/unlink audit entries, got %+v", audits)
	}
	auditJSON, err := json.Marshal(audits)
	if err != nil {
		t.Fatalf("marshal audit entries: %v", err)
	}
	for _, forbidden := range []string{
		"owner@example.com",
		"+79991112233",
		"79991112233",
		"111-22-33",
	} {
		if strings.Contains(string(auditJSON), forbidden) {
			t.Fatalf("audit leaked %q: %s", forbidden, auditJSON)
		}
	}
	last := audits[len(audits)-1]
	if last.Action != domain.AccountLinkActionUnlinked ||
		last.AccountID != account.AccountID ||
		last.Provider != domain.IdentityProviderPhone ||
		last.IdentityID == nil ||
		*last.IdentityID != linked.ID {
		t.Fatalf("unexpected unlink audit: %+v", last)
	}
	if err := service.UnlinkIdentity(ctx, account.AccountID, account.AccountID, account.Identity.ID); !errors.Is(err, domain.ErrAccountLastIdentity) {
		t.Fatalf("unlink last identity error = %v, want %v", err, domain.ErrAccountLastIdentity)
	}
}

func TestLinkIdentityRequiresSameActorAndVerifiedTarget(t *testing.T) {
	ctx := context.Background()
	resolver := identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil)
	service := accountauth.New(resolver)
	account, err := service.ResolveVerifiedEmailPassword(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("resolve owner: %v", err)
	}

	_, err = service.LinkVerifiedIdentity(ctx, uuid.New(), account.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginPhoneOTP,
		ExternalID: "+79991112233",
		Verified:   true,
	})
	if !errors.Is(err, domain.ErrAccountIdentityOwnershipRequired) {
		t.Fatalf("link with another actor error = %v, want ownership required", err)
	}

	_, err = service.LinkVerifiedIdentity(ctx, account.AccountID, account.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginPhoneOTP,
		ExternalID: "+79991112233",
	})
	if !errors.Is(err, domain.ErrUnverifiedLogin) {
		t.Fatalf("link unverified error = %v, want unverified", err)
	}
}

func TestLinkIdentityConflictRequiresExplicitMerge(t *testing.T) {
	ctx := context.Background()
	resolver := identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil)
	service := accountauth.New(resolver)

	owner, err := service.ResolveVerifiedEmailPassword(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("resolve owner: %v", err)
	}
	other, err := service.ResolveVerifiedEmailPassword(ctx, "other@example.com")
	if err != nil {
		t.Fatalf("resolve other: %v", err)
	}

	_, err = service.LinkVerifiedIdentity(ctx, other.AccountID, other.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "owner@example.com",
		Verified:   true,
	})
	if !errors.Is(err, domain.ErrAccountMergeRequiresConfirmation) {
		t.Fatalf("conflicting link error = %v, want merge confirmation required", err)
	}
	resolved, err := service.ResolveVerifiedEmailPassword(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("resolve owner after conflict: %v", err)
	}
	if resolved.AccountID != owner.AccountID {
		t.Fatalf("conflict moved identity to account %s, want %s", resolved.AccountID, owner.AccountID)
	}
}

func TestRateLimitAppliesToLoginLinkAndUnlink(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), identities, nil)
	limiter := &fakeLimiter{allowed: true}
	service := accountauth.New(resolver, accountauth.WithLimiter(limiter))

	account, err := service.ResolveVerifiedEmailPassword(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("resolve owner: %v", err)
	}
	linked, err := service.LinkVerifiedIdentity(ctx, account.AccountID, account.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginPhoneOTP,
		ExternalID: "+79991112233",
		Verified:   true,
	})
	if err != nil {
		t.Fatalf("link identity: %v", err)
	}
	if err := service.UnlinkIdentity(ctx, account.AccountID, account.AccountID, linked.ID); err != nil {
		t.Fatalf("unlink identity: %v", err)
	}
	if limiter.calls != 3 {
		t.Fatalf("limiter calls = %d, want 3", limiter.calls)
	}
	for _, key := range limiter.keys {
		if strings.Contains(key, "owner@example.com") || strings.Contains(key, "79991112233") {
			t.Fatalf("rate limit key leaked raw identity: %q", key)
		}
	}

	limiter.allowed = false
	if _, err := service.ResolveVerifiedEmailPassword(ctx, "another@example.com"); !errors.Is(err, accountauth.ErrRateLimited) {
		t.Fatalf("rate limited login error = %v, want %v", err, accountauth.ErrRateLimited)
	}
	if _, err := service.LinkVerifiedIdentity(ctx, account.AccountID, account.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginPhoneOTP,
		ExternalID: "+79994445566",
		Verified:   true,
	}); !errors.Is(err, accountauth.ErrRateLimited) {
		t.Fatalf("rate limited link error = %v, want %v", err, accountauth.ErrRateLimited)
	}
	if err := service.UnlinkIdentity(ctx, account.AccountID, account.AccountID, account.Identity.ID); !errors.Is(err, accountauth.ErrRateLimited) {
		t.Fatalf("rate limited unlink error = %v, want %v", err, accountauth.ErrRateLimited)
	}
}

func TestPasswordLoginRequiresLinkedEmailAndStoresOnlyHash(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	security := memory.NewAccountSecurityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), identities, nil)
	service := accountauth.New(resolver,
		accountauth.WithCredentialRepository(security),
		accountauth.WithAccountAuditRepository(security),
	)

	account, err := service.ResolveVerifiedEmailPassword(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("resolve owner: %v", err)
	}
	if err := service.SetPasswordForVerifiedEmail(ctx, account.AccountID, account.AccountID, "owner@example.com", "correct horse battery staple"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	credential, err := security.FindCredential(ctx, account.AccountID, domain.AccountCredentialPassword)
	if err != nil {
		t.Fatalf("find credential: %v", err)
	}
	if credential.SecretHash == "correct horse battery staple" || strings.Contains(credential.SecretHash, "correct horse") {
		t.Fatalf("raw password leaked into credential hash: %q", credential.SecretHash)
	}

	loggedIn, err := service.AuthenticateEmailPassword(ctx, "owner@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if loggedIn.AccountID != account.AccountID {
		t.Fatalf("login account = %s, want %s", loggedIn.AccountID, account.AccountID)
	}
	if _, err := service.AuthenticateEmailPassword(ctx, "owner@example.com", "wrong password"); !errors.Is(err, accountauth.ErrInvalidPasswordLogin) {
		t.Fatalf("wrong password error = %v, want invalid password login", err)
	}
}

func TestPasswordSetRejectsUnlinkedEmailAndWeakPassword(t *testing.T) {
	ctx := context.Background()
	security := memory.NewAccountSecurityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil)
	service := accountauth.New(resolver, accountauth.WithCredentialRepository(security))

	account, err := service.ResolveVKID(ctx, 42)
	if err != nil {
		t.Fatalf("resolve vk: %v", err)
	}
	if err := service.SetPasswordForVerifiedEmail(ctx, account.AccountID, account.AccountID, "missing@example.com", "correct horse battery staple"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unlinked email error = %v, want not found", err)
	}
	if _, err := service.LinkVerifiedIdentity(ctx, account.AccountID, account.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "owner@example.com",
		Verified:   true,
	}); err != nil {
		t.Fatalf("link email: %v", err)
	}
	if err := service.SetPasswordForVerifiedEmail(ctx, account.AccountID, account.AccountID, "owner@example.com", "short"); !errors.Is(err, accountauth.ErrWeakPassword) {
		t.Fatalf("weak password error = %v, want weak password", err)
	}
}

func TestPasswordResetAuditsAndRotatesCredential(t *testing.T) {
	ctx := context.Background()
	security := memory.NewAccountSecurityRepo()
	sessionRepo := memory.NewAccountSessionRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil)
	service := accountauth.New(resolver,
		accountauth.WithSessionRepository(sessionRepo),
		accountauth.WithCredentialRepository(security),
		accountauth.WithAccountAuditRepository(security),
	)
	account, err := service.ResolveVerifiedEmailPassword(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("resolve owner: %v", err)
	}
	if err := service.SetPasswordForVerifiedEmail(ctx, account.AccountID, account.AccountID, "owner@example.com", "old password value"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	tokens, err := service.IssueSession(ctx, account.AccountID, accountauth.SessionMetadata{DeviceInfo: "before-reset"})
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	if err := service.ResetPasswordForVerifiedEmail(ctx, account.AccountID, "owner@example.com", "new password value"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if _, err := service.AuthenticateEmailPassword(ctx, "owner@example.com", "old password value"); !errors.Is(err, accountauth.ErrInvalidPasswordLogin) {
		t.Fatalf("old password error = %v, want invalid password login", err)
	}
	if _, err := service.AuthenticateEmailPassword(ctx, "owner@example.com", "new password value"); err != nil {
		t.Fatalf("new password login: %v", err)
	}
	if _, err := service.RefreshSession(ctx, tokens.RefreshToken, accountauth.SessionMetadata{}); !errors.Is(err, accountauth.ErrInvalidSession) {
		t.Fatalf("refresh after password reset error = %v, want invalid session", err)
	}
	var foundReset bool
	for _, audit := range security.AuditEntries() {
		if audit.Action == domain.AccountLinkActionPasswordReset {
			foundReset = true
		}
	}
	if !foundReset {
		t.Fatalf("password reset audit missing: %+v", security.AuditEntries())
	}
}

func TestMergeRequiresExplicitConfirmedFlow(t *testing.T) {
	service := accountauth.New(identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil))
	if err := service.MergeAccounts(context.Background(), false, uuid.New(), uuid.New()); !errors.Is(err, domain.ErrAccountMergeRequiresConfirmation) {
		t.Fatalf("unconfirmed merge error = %v, want confirmation required", err)
	}
	if err := service.MergeAccounts(context.Background(), true, uuid.New(), uuid.New()); err == nil {
		t.Fatal("confirmed merge unexpectedly succeeded before audited merge flow is implemented")
	}
}

type fakeLimiter struct {
	allowed bool
	calls   int
	keys    []string
}

func (f *fakeLimiter) Allow(_ context.Context, key string) (bool, error) {
	f.calls++
	f.keys = append(f.keys, key)
	return f.allowed, nil
}

func TestResolveOrCreateRejectsUnverifiedAndInvalidLogin(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), identities, nil)
	service := accountauth.New(resolver)

	tests := []struct {
		name  string
		login domain.VerifiedAccountLogin
		want  error
	}{
		{
			name: "unverified email",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginEmailPassword,
				ExternalID: "user@example.com",
			},
			want: domain.ErrUnverifiedLogin,
		},
		{
			name: "bad email",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginEmailPassword,
				ExternalID: "not-email",
				Verified:   true,
			},
			want: domain.ErrInvalidIdentity,
		},
		{
			name: "bad phone",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginPhoneOTP,
				ExternalID: "123",
				Verified:   true,
			},
			want: domain.ErrInvalidIdentity,
		},
		{
			name: "bad vk id",
			login: domain.VerifiedAccountLogin{
				Method:     domain.AccountLoginVKID,
				ExternalID: "abc",
				Verified:   true,
			},
			want: domain.ErrInvalidIdentity,
		},
		{
			name: "unknown method",
			login: domain.VerifiedAccountLogin{
				Method:     "saml",
				ExternalID: "subject",
				Verified:   true,
			},
			want: domain.ErrInvalidIdentity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.ResolveOrCreate(ctx, tt.login); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestConvenienceMethodsUseSharedIdentityLayer(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), identities, nil)
	service := accountauth.New(resolver)

	resolutions := []domain.IdentityResolution{}
	calls := []func(context.Context) (domain.IdentityResolution, error){
		func(ctx context.Context) (domain.IdentityResolution, error) {
			return service.ResolveVerifiedEmailPassword(ctx, "person@example.com")
		},
		func(ctx context.Context) (domain.IdentityResolution, error) {
			return service.ResolveVerifiedPhoneOTP(ctx, "+79991234567")
		},
		func(ctx context.Context) (domain.IdentityResolution, error) {
			return service.ResolveVerifiedGoogleSubject(ctx, "google-subject")
		},
		func(ctx context.Context) (domain.IdentityResolution, error) {
			return service.ResolveVerifiedAppleSubject(ctx, "apple-subject")
		},
		func(ctx context.Context) (domain.IdentityResolution, error) {
			return service.ResolveTelegramID(ctx, 100500)
		},
		func(ctx context.Context) (domain.IdentityResolution, error) {
			return service.ResolveVKID(ctx, 200600)
		},
	}

	for _, call := range calls {
		resolution, err := call(ctx)
		if err != nil {
			t.Fatalf("resolve convenience method: %v", err)
		}
		if resolution.AccountID == uuid.Nil {
			t.Fatalf("empty account id: %+v", resolution)
		}
		resolutions = append(resolutions, resolution)
	}

	seen := map[uuid.UUID]bool{}
	for _, resolution := range resolutions {
		if seen[resolution.AccountID] {
			t.Fatalf("different login methods should create separate accounts before explicit linking")
		}
		seen[resolution.AccountID] = true
	}
}
