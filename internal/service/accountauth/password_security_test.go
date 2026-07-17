package accountauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/pbkdf2"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/identityresolver"
)

const legacyPasswordIterations = 120000

func TestHashPasswordUsesArgon2id(t *testing.T) {
	encoded, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$") {
		t.Fatalf("password hash algorithm = %q, want argon2id", strings.SplitN(encoded, "$", 3)[0])
	}
	ok, err := verifyPassword("correct horse battery staple", encoded)
	if err != nil || !ok {
		t.Fatalf("verify new password hash: ok=%v err=%v", ok, err)
	}
}

func TestLegacyPasswordLoginRehashesOnce(t *testing.T) {
	ctx := context.Background()
	security := memory.NewAccountSecurityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil)
	service := New(resolver, WithCredentialRepository(security))

	account, err := service.ResolveVerifiedEmailPassword(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("resolve verified email: %v", err)
	}
	legacy := legacyPBKDF2Hash("correct horse battery staple")
	now := time.Now().UTC().Add(-time.Hour)
	if _, err := security.UpsertCredential(ctx, domain.AccountCredential{
		ID:             uuid.New(),
		AccountID:      account.AccountID,
		CredentialType: domain.AccountCredentialPassword,
		SecretHash:     legacy,
		ChangedAt:      &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("seed legacy credential: %v", err)
	}

	if _, err := service.AuthenticateEmailPassword(ctx, "owner@example.com", "correct horse battery staple"); err != nil {
		t.Fatalf("legacy password login: %v", err)
	}
	upgraded, err := security.FindCredential(ctx, account.AccountID, domain.AccountCredentialPassword)
	if err != nil {
		t.Fatalf("find upgraded credential: %v", err)
	}
	if upgraded.SecretHash == legacy || !strings.HasPrefix(upgraded.SecretHash, "$argon2id$") {
		t.Fatalf("legacy credential was not upgraded to argon2id")
	}

	firstUpgrade := upgraded.SecretHash
	if _, err := service.AuthenticateEmailPassword(ctx, "owner@example.com", "correct horse battery staple"); err != nil {
		t.Fatalf("second password login: %v", err)
	}
	stable, err := security.FindCredential(ctx, account.AccountID, domain.AccountCredentialPassword)
	if err != nil {
		t.Fatalf("find stable credential: %v", err)
	}
	if stable.SecretHash != firstUpgrade {
		t.Fatal("current password hash was rewritten on an idempotent login")
	}
}

func TestVerifyPasswordFailsSafely(t *testing.T) {
	validHash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tests := []struct {
		name     string
		password string
		encoded  string
	}{
		{name: "wrong password", password: "wrong password", encoded: validHash},
		{name: "malformed hash", password: "correct horse battery staple", encoded: "$argon2id$broken"},
		{
			name:     "oversized legacy work factor",
			password: "correct horse battery staple",
			encoded: strings.Join([]string{
				passwordHashAlgorithm,
				strconv.Itoa(1_000_000),
				base64.RawStdEncoding.EncodeToString(make([]byte, passwordSaltBytes)),
				base64.RawStdEncoding.EncodeToString(make([]byte, passwordHashBytes)),
			}, "$"),
		},
		{
			name:     "oversized argon memory",
			password: "correct horse battery staple",
			encoded:  "$argon2id$v=19$m=1048576,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
		{
			name:     "argon time does not fit uint32",
			password: "correct horse battery staple",
			encoded:  "$argon2id$v=19$m=19456,t=4294967296,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
		{
			name:     "argon threads do not fit uint8",
			password: "correct horse battery staple",
			encoded:  "$argon2id$v=19$m=19456,t=2,p=256$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			started := time.Now()
			ok, _ := verifyPassword(tc.password, tc.encoded)
			if ok {
				t.Fatal("unsafe password hash unexpectedly verified")
			}
			if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
				t.Fatalf("unsafe password hash consumed %s, want fail-fast", elapsed)
			}
		})
	}
}

func BenchmarkHashPassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := hashPassword("correct horse battery staple"); err != nil {
			b.Fatal(err)
		}
	}
}

func legacyPBKDF2Hash(password string) string {
	salt := []byte("legacy-salt-1234")
	sum := pbkdf2.Key([]byte(password), salt, legacyPasswordIterations, passwordHashBytes, sha256.New)
	return strings.Join([]string{
		passwordHashAlgorithm,
		strconv.Itoa(legacyPasswordIterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	}, "$")
}
