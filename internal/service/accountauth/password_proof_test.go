package accountauth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/identityresolver"
)

type passwordProofFixture struct {
	auth     *Service
	security *memory.AccountSecurityRepo
	owner    domain.IdentityResolution
}

func newPasswordProofFixture(t *testing.T, password bool) passwordProofFixture {
	t.Helper()
	ctx := context.Background()
	security := memory.NewAccountSecurityRepo()
	identities := memory.NewAccountIdentityRepo()
	auth := New(identityresolver.New(nil, identities, nil), WithCredentialRepository(security), WithSessionRepository(memory.NewAccountSessionRepo()), WithAccountAuditRepository(security))
	owner, err := auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.LinkVerifiedIdentity(ctx, owner.AccountID, owner.AccountID, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "backup@example.test", Verified: true}); err != nil {
		t.Fatal(err)
	}
	if password {
		if err := auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password"); err != nil {
			t.Fatal(err)
		}
	}
	return passwordProofFixture{auth: auth, security: security, owner: owner}
}

func TestCapturedPasswordProofRejectsSecurityMutationBeforeCompletion(t *testing.T) {
	for _, mutation := range []string{"reset", "change", "unlink"} {
		t.Run(mutation, func(t *testing.T) {
			f := newPasswordProofFixture(t, true)
			ctx := context.Background()
			proof, err := f.auth.CapturePasswordProof(ctx, "member@example.test", "original-password")
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "reset":
				err = f.auth.ResetPasswordForVerifiedEmail(ctx, f.owner.AccountID, "member@example.test", "replacement-password")
			case "change":
				err = f.auth.ChangePasswordForVerifiedEmail(ctx, f.owner.AccountID, f.owner.AccountID, "member@example.test", "original-password", "replacement-password")
			case "unlink":
				err = f.auth.UnlinkIdentity(ctx, f.owner.AccountID, f.owner.AccountID, f.owner.Identity.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = f.auth.ValidatePasswordProof(ctx, proof); !errors.Is(err, ErrInvalidPasswordLogin) {
				t.Fatalf("stale trust proof accepted: %v", err)
			}
			if _, err = f.auth.CompletePasswordProof(ctx, proof, SessionMetadata{}); !errors.Is(err, ErrInvalidPasswordLogin) {
				t.Fatalf("stale password proof completed: %v", err)
			}
			active, err := f.auth.ListActiveSessions(ctx, f.owner.AccountID, 100)
			if err != nil || len(active) != 0 {
				t.Fatal("stale proof issued session")
			}
		})
	}
}

func TestPasswordProofCaptureDoesNotIssueSessionOrAuditAndCompletionDoesOnce(t *testing.T) {
	f := newPasswordProofFixture(t, true)
	ctx := context.Background()
	before := len(f.security.AuditEntries())
	proof, err := f.auth.CapturePasswordProof(ctx, "member@example.test", "original-password")
	if err != nil {
		t.Fatal(err)
	}
	active, _ := f.auth.ListActiveSessions(ctx, f.owner.AccountID, 100)
	if len(active) != 0 || len(f.security.AuditEntries()) != before {
		t.Fatal("capture mutated sessions or audits")
	}
	if err := f.auth.ValidatePasswordProof(ctx, proof); err != nil {
		t.Fatal(err)
	}
	tokens, err := f.auth.CompletePasswordProof(ctx, proof, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.auth.AuthenticateAccessToken(ctx, tokens.AccessToken); err != nil {
		t.Fatal("completion did not issue usable session")
	}
	if len(f.security.AuditEntries()) != before+1 {
		t.Fatal("completion did not record exactly one audit")
	}
}

func TestRecoveryProofRejectsCredentialChangeAndCannotCompleteLogin(t *testing.T) {
	f := newPasswordProofFixture(t, true)
	ctx := context.Background()
	proof, err := f.auth.CaptureRecoveryProof(ctx, "member@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.auth.CompletePasswordProof(ctx, proof, SessionMetadata{}); !errors.Is(err, ErrInvalidPasswordLogin) {
		t.Fatal("recovery capture substituted for password verification")
	}
	if err := f.auth.ChangePasswordForVerifiedEmail(ctx, f.owner.AccountID, f.owner.AccountID, "member@example.test", "original-password", "replacement-password"); err != nil {
		t.Fatal(err)
	}
	if err := f.auth.ResetPasswordProof(ctx, proof, "reset-password"); !errors.Is(err, ErrInvalidPasswordLogin) {
		t.Fatalf("stale recovery token changed credential: %v", err)
	}
	if _, err := f.auth.AuthenticateEmailPasswordSession(ctx, "member@example.test", "replacement-password", SessionMetadata{}); err != nil {
		t.Fatal("stale recovery replaced current credential")
	}
}

func TestRecoveryProofResetIsSingleUseAndRevokesSessions(t *testing.T) {
	f := newPasswordProofFixture(t, true)
	ctx := context.Background()
	session, err := f.auth.IssueSession(ctx, f.owner.AccountID, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := f.auth.CaptureRecoveryProof(ctx, "member@example.test")
	if err != nil {
		t.Fatal(err)
	}
	before := len(f.security.AuditEntries())
	if err := f.auth.ResetPasswordProof(ctx, proof, "reset-password"); err != nil {
		t.Fatal(err)
	}
	if len(f.security.AuditEntries()) != before+1 {
		t.Fatal("reset audit count wrong")
	}
	if _, err := f.auth.AuthenticateAccessToken(ctx, session.AccessToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("reset kept old session")
	}
	if err := f.auth.ResetPasswordProof(ctx, proof, "another-password"); !errors.Is(err, ErrInvalidPasswordLogin) {
		t.Fatal("recovery proof could be reused")
	}
	if _, err := f.auth.AuthenticateEmailPasswordSession(ctx, "member@example.test", "reset-password", SessionMetadata{}); err != nil {
		t.Fatal("successful reset password unusable")
	}
}

func TestRecoveryProofSupportsNoCredentialButRejectsLaterSetup(t *testing.T) {
	ctx := context.Background()
	for _, setup := range []bool{false, true} {
		f := newPasswordProofFixture(t, false)
		proof, err := f.auth.CaptureRecoveryProof(ctx, "member@example.test")
		if err != nil || proof.ExpectedHash != "" {
			t.Fatal("missing credential recovery capture failed")
		}
		if setup {
			if err := f.auth.SetPasswordForVerifiedEmail(ctx, f.owner.AccountID, f.owner.AccountID, "member@example.test", "configured-password"); err != nil {
				t.Fatal(err)
			}
			if err := f.auth.ResetPasswordProof(ctx, proof, "reset-password"); !errors.Is(err, ErrInvalidPasswordLogin) {
				t.Fatal("pre-setup recovery proof overwrote configured password")
			}
		} else if err := f.auth.ResetPasswordProof(ctx, proof, "reset-password"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCapturedLegacyPasswordProofUpgradesOnlyAtCompletion(t *testing.T) {
	f := newPasswordProofFixture(t, true)
	ctx := context.Background()
	stored, err := f.security.FindCredential(ctx, f.owner.AccountID, domain.AccountCredentialPassword)
	if err != nil {
		t.Fatal(err)
	}
	stored.SecretHash = legacyPBKDF2Hash("original-password")
	if _, err := f.security.UpsertCredential(ctx, *stored); err != nil {
		t.Fatal(err)
	}
	proof, err := f.auth.CapturePasswordProof(ctx, "member@example.test", "original-password")
	if err != nil {
		t.Fatal(err)
	}
	if proof.Upgrade == nil || !strings.HasPrefix(proof.Upgrade.SecretHash, "$argon2id$") {
		t.Fatal("capture did not prepare legacy upgrade")
	}
	current, _ := f.security.FindCredential(ctx, f.owner.AccountID, domain.AccountCredentialPassword)
	if current.SecretHash != stored.SecretHash {
		t.Fatal("capture changed credential before additional verification")
	}
	if _, err := f.auth.CompletePasswordProof(ctx, proof, SessionMetadata{}); err != nil {
		t.Fatal(err)
	}
	current, _ = f.security.FindCredential(ctx, f.owner.AccountID, domain.AccountCredentialPassword)
	if !strings.HasPrefix(current.SecretHash, "$argon2id$") {
		t.Fatal("completion did not atomically upgrade credential")
	}
}

func TestPasswordProofRejectsMissingDependenciesAndUnknownCredentials(t *testing.T) {
	ctx := context.Background()
	var missing *Service
	if _, err := missing.CapturePasswordProof(ctx, "member@example.test", "original-password"); !errors.Is(err, ErrPasswordStoreUnavailable) {
		t.Fatal("nil capture did not fail closed")
	}
	if _, err := missing.CaptureRecoveryProof(ctx, "member@example.test"); !errors.Is(err, ErrPasswordStoreUnavailable) {
		t.Fatal("nil recovery did not fail closed")
	}
	if err := missing.ValidatePasswordProof(ctx, PasswordProof{}); !errors.Is(err, ErrPasswordStoreUnavailable) {
		t.Fatal("nil validation did not fail closed")
	}
	if err := missing.ResetPasswordProof(ctx, PasswordProof{}, "reset-password"); !errors.Is(err, ErrPasswordStoreUnavailable) {
		t.Fatal("nil reset did not fail closed")
	}
	if _, err := missing.CompletePasswordProof(ctx, PasswordProof{}, SessionMetadata{}); !errors.Is(err, ErrPasswordStoreUnavailable) {
		t.Fatal("nil completion did not fail closed")
	}
	f := newPasswordProofFixture(t, true)
	for _, email := range []string{"unknown@example.test", "bad-address"} {
		if _, err := f.auth.CapturePasswordProof(ctx, email, "original-password"); !errors.Is(err, ErrInvalidPasswordLogin) {
			t.Fatal("credential error leaks account existence")
		}
	}
	if _, err := f.auth.CapturePasswordProof(ctx, "member@example.test", "wrong-password"); !errors.Is(err, ErrInvalidPasswordLogin) {
		t.Fatal("incorrect password captured")
	}
}

type nilProofBindingResolver struct{ domain.IdentityResolver }

func (nilProofBindingResolver) PasswordEmailBinding(context.Context, string) (*domain.AccountIdentity, error) {
	return nil, nil
}

type deniedProofLimiter struct{}

func (deniedProofLimiter) Allow(context.Context, string) (bool, error) { return false, nil }

func TestPasswordProofCaptureFailsClosedOnMissingBindingAndHonorsRateLimit(t *testing.T) {
	ctx := context.Background()
	auth := New(nilProofBindingResolver{}, WithCredentialRepository(memory.NewAccountSecurityRepo()))
	if _, err := auth.CapturePasswordProof(ctx, "member@example.test", "original-password"); !errors.Is(err, ErrInvalidPasswordLogin) {
		t.Fatal("nil binding accepted")
	}
	f := newPasswordProofFixture(t, true)
	f.auth.limiter = deniedProofLimiter{}
	if _, err := f.auth.CapturePasswordProof(ctx, "member@example.test", "original-password"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("capture bypassed password rate limit")
	}
	if _, err := f.auth.CaptureRecoveryProof(ctx, "member@example.test"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("recovery capture bypassed rate limit")
	}
}

func TestPasswordProofSecurityOperationsFailClosedWithoutSessionsAndAudit(t *testing.T) {
	ctx := context.Background()
	f := newPasswordProofFixture(t, true)
	proof, err := f.auth.CapturePasswordProof(ctx, "member@example.test", "original-password")
	if err != nil {
		t.Fatal(err)
	}
	sessions := f.auth.sessions
	f.auth.sessions = nil
	if _, err := f.auth.CompletePasswordProof(ctx, proof, SessionMetadata{}); !errors.Is(err, ErrSessionStoreUnavailable) {
		t.Fatal("completion ignored missing sessions")
	}
	if err := f.auth.ResetPasswordProof(ctx, proof, "reset-password"); !errors.Is(err, ErrSessionStoreUnavailable) {
		t.Fatal("reset ignored missing session revocation store")
	}
	f.auth.sessions = sessions
	f.auth.audit = failingPasswordAudit{}
	if _, err := f.auth.CompletePasswordProof(ctx, proof, SessionMetadata{}); err == nil {
		t.Fatal("completion ignored audit failure")
	}
	if err := f.auth.ResetPasswordProof(ctx, proof, "reset-password"); err == nil {
		t.Fatal("reset ignored audit failure")
	}
	f.auth.audit = f.security
	if err := f.auth.ValidatePasswordProof(ctx, proof); err != nil {
		t.Fatal("failed security operation changed credential")
	}
	active, _ := f.auth.ListActiveSessions(ctx, f.owner.AccountID, 100)
	if len(active) != 0 {
		t.Fatal("failed security operation issued session")
	}
}

func TestRecoveryProofRejectsUnlinkedEmail(t *testing.T) {
	f := newPasswordProofFixture(t, true)
	ctx := context.Background()
	proof, err := f.auth.CaptureRecoveryProof(ctx, "member@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.auth.UnlinkIdentity(ctx, f.owner.AccountID, f.owner.AccountID, f.owner.Identity.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.auth.ResetPasswordProof(ctx, proof, "reset-password"); !errors.Is(err, ErrInvalidPasswordLogin) {
		t.Fatal("unlinked email still authorized recovery")
	}
	if _, err := f.auth.AuthenticateEmailPasswordSession(ctx, "backup@example.test", "original-password", SessionMetadata{}); err != nil {
		t.Fatal("rejected unlinked recovery changed credential")
	}
}
