package accountauth

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/identityresolver"
)

type failingPasswordAudit struct{}

func (failingPasswordAudit) RecordAccountAudit(context.Context, domain.AccountLinkAuditEntry) error {
	return errors.New("audit unavailable")
}

type capturedPasswordStore struct {
	domain.AccountCredentialRepository
	atomic    domain.AccountPasswordSecurityRepository
	afterRead func()
}

func (r *capturedPasswordStore) FindCredential(ctx context.Context, id uuid.UUID, kind domain.AccountCredentialType) (*domain.AccountCredential, error) {
	credential, err := r.AccountCredentialRepository.FindCredential(ctx, id, kind)
	if err == nil && r.afterRead != nil {
		hook := r.afterRead
		r.afterRead = nil
		hook()
	}
	return credential, err
}
func (r *capturedPasswordStore) ExecutePasswordSecurity(ctx context.Context, op domain.PasswordSecurityOperation, deps domain.PasswordSecurityDependencies) error {
	return r.atomic.ExecutePasswordSecurity(ctx, op, deps)
}

func TestPasswordSessionRejectsCapturedCredentialAfterSecurityMutation(t *testing.T) {
	for _, mutation := range []string{"reset", "change", "unlink"} {
		t.Run(mutation, func(t *testing.T) {
			ctx := context.Background()
			identities := memory.NewAccountIdentityRepo()
			credentials := memory.NewAccountSecurityRepo()
			sessions := memory.NewAccountSessionRepo()
			resolver := identityresolver.New(nil, identities, nil)
			base := New(resolver, WithCredentialRepository(credentials), WithSessionRepository(sessions), WithAccountAuditRepository(credentials))
			owner, err := base.ResolveVerifiedEmailPassword(ctx, "member@example.test")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := base.LinkVerifiedIdentity(ctx, owner.AccountID, owner.AccountID, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "backup@example.test", Verified: true}); err != nil {
				t.Fatal(err)
			}
			if err := base.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password"); err != nil {
				t.Fatal(err)
			}
			store := &capturedPasswordStore{AccountCredentialRepository: credentials, atomic: credentials, afterRead: func() {
				var err error
				switch mutation {
				case "reset":
					err = base.ResetPasswordForVerifiedEmail(ctx, owner.AccountID, "member@example.test", "replacement-password")
				case "change":
					err = base.ChangePasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password", "replacement-password")
				case "unlink":
					err = base.UnlinkIdentity(ctx, owner.AccountID, owner.AccountID, owner.Identity.ID)
				}
				if err != nil {
					t.Fatal(err)
				}
			}}
			auth := New(resolver, WithCredentialRepository(store), WithSessionRepository(sessions), WithAccountAuditRepository(credentials))
			if _, err := auth.AuthenticateEmailPasswordSession(ctx, "member@example.test", "original-password", SessionMetadata{}); !errors.Is(err, ErrInvalidPasswordLogin) {
				t.Fatalf("stale password session result: %v", err)
			}
			active, err := base.ListActiveSessions(ctx, owner.AccountID, 100)
			if err != nil || len(active) != 0 {
				t.Fatal("stale password login created session")
			}
		})
	}
}

func TestPasswordChangePreservesOnlyActiveServerSession(t *testing.T) {
	ctx := context.Background()
	credentials := memory.NewAccountSecurityRepo()
	auth := New(identityresolver.New(nil, memory.NewAccountIdentityRepo(), nil), WithCredentialRepository(credentials), WithSessionRepository(memory.NewAccountSessionRepo()))
	owner, _ := auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	if err := auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password"); err != nil {
		t.Fatal(err)
	}
	current, _ := auth.IssueSession(ctx, owner.AccountID, SessionMetadata{})
	other, _ := auth.IssueSession(ctx, owner.AccountID, SessionMetadata{})
	if err := auth.ChangePasswordForVerifiedEmailSession(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password", "replacement-password", current.Session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.AuthenticateAccessToken(ctx, current.AccessToken); err != nil {
		t.Fatal("current server session revoked")
	}
	if _, err := auth.AuthenticateAccessToken(ctx, other.AccessToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("other session survived")
	}
	if err := auth.ChangePasswordForVerifiedEmailSession(ctx, owner.AccountID, owner.AccountID, "member@example.test", "replacement-password", "another-password", other.Session.ID); err == nil {
		t.Fatal("revoked principal session accepted")
	}
	if _, err := auth.AuthenticateEmailPasswordSession(ctx, "member@example.test", "replacement-password", SessionMetadata{}); err != nil {
		t.Fatal("rejected stale principal changed password")
	}
}

func TestPasswordSessionFailsClosedWithoutTransactionAdapter(t *testing.T) {
	ctx := context.Background()
	credentials := memory.NewAccountSecurityRepo()
	sessions := memory.NewAccountSessionRepo()
	resolver := identityresolver.New(nil, memory.NewAccountIdentityRepo(), nil)
	auth := New(resolver, WithCredentialRepository(credentials), WithSessionRepository(sessions))
	owner, _ := auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	if err := auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password"); err != nil {
		t.Fatal(err)
	}
	auth.credentials = struct {
		domain.AccountCredentialRepository
	}{credentials}
	if _, err := auth.AuthenticateEmailPasswordSession(ctx, "member@example.test", "original-password", SessionMetadata{}); !errors.Is(err, ErrPasswordStoreUnavailable) {
		t.Fatalf("unsupported transaction result: %v", err)
	}
	active, _ := auth.ListActiveSessions(ctx, owner.AccountID, 100)
	if len(active) != 0 {
		t.Fatal("unsupported adapter issued session")
	}
}

func TestPasswordMutationAuditFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	credentials := memory.NewAccountSecurityRepo()
	auth := New(identityresolver.New(nil, memory.NewAccountIdentityRepo(), nil), WithCredentialRepository(credentials))
	owner, _ := auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	auth.audit = failingPasswordAudit{}
	if err := auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password"); err == nil {
		t.Fatal("audit failure ignored")
	}
	if _, err := credentials.FindCredential(ctx, owner.AccountID, domain.AccountCredentialPassword); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("audit failure committed credential")
	}
}

func TestLegacyPasswordChangeRevokesAllSessions(t *testing.T) {
	ctx := context.Background()
	credentials := memory.NewAccountSecurityRepo()
	auth := New(identityresolver.New(nil, memory.NewAccountIdentityRepo(), nil), WithCredentialRepository(credentials), WithSessionRepository(memory.NewAccountSessionRepo()))
	owner, _ := auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	if err := auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password"); err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.IssueSession(ctx, owner.AccountID, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.ChangePasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password", "replacement-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.AuthenticateAccessToken(ctx, tokens.AccessToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("password change left old session active")
	}
}

func TestPasswordLoginRequiresAtomicSessionOperation(t *testing.T) {
	auth := New(nil)
	if _, ok := any(auth).(interface {
		AuthenticateEmailPasswordSession(context.Context, string, string, SessionMetadata) (SessionTokens, error)
	}); !ok {
		t.Fatal("combined password/session operation missing")
	}
}
