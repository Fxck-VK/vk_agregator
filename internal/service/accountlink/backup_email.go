package accountlink

import (
	"context"
	"crypto/hmac"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountservice"
)

type backupEmailAccount interface {
	BackupEmailVersion(context.Context, uuid.UUID, uuid.UUID) (time.Time, error)
	ReplaceVerifiedBackupEmail(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, domain.VerifiedAccountLogin, time.Time) (accountservice.AccountIdentitySafe, error)
}

func backupChallengeKey(accountID, identityID uuid.UUID, emailHash string) string {
	return "accountlink:backup-email:challenge:" + accountID.String() + ":" + identityID.String() + ":" + emailHash
}

func backupProofIdentity(identityID uuid.UUID, version time.Time, email string) string {
	return "backup-email:" + identityID.String() + ":" + version.UTC().Format(time.RFC3339Nano) + ":" + email
}

// RequestBackupEmailCode snapshots the existing backup version. It never
// changes a binding or reveals whether a foreign account owns the new address.
func (s *Service) RequestBackupEmailCode(ctx context.Context, accountID, identityID uuid.UUID, email string) (RequestResult, error) {
	if s == nil || s.store == nil || s.sender == nil {
		return RequestResult{}, ErrMissingDependency
	}
	account, ok := s.account.(backupEmailAccount)
	if !ok {
		return RequestResult{}, ErrMissingDependency
	}
	if accountID == uuid.Nil || identityID == uuid.Nil {
		return RequestResult{}, domain.ErrInvalidIdentity
	}
	normalized, err := normalizeEmail(email)
	if err != nil {
		return RequestResult{}, err
	}
	hash := hashIdentity(normalized)
	if err := s.checkLimit(ctx, requestRateKey(accountID, hash), s.cfg.RequestLimit, s.cfg.RequestWindow); err != nil {
		return RequestResult{}, err
	}
	version, err := account.BackupEmailVersion(ctx, accountID, identityID)
	if err != nil {
		return RequestResult{}, err
	}
	code, err := generateNumericCode(s.cfg.CodeDigits)
	if err != nil {
		return RequestResult{}, err
	}
	expires := s.cfg.Now().Add(s.cfg.CodeTTL)
	challenge := Challenge{AccountID: accountID, IdentityHash: hash, BackupIdentityID: identityID, BackupVersion: version, CodeHash: s.codeHash(accountID, backupProofIdentity(identityID, version, normalized), code), ExpiresAt: expires}
	key := backupChallengeKey(accountID, identityID, hash)
	if err := s.store.SaveChallenge(ctx, key, challenge, s.cfg.CodeTTL); err != nil {
		return RequestResult{}, err
	}
	if err := s.sender.SendEmailLinkCode(ctx, normalized, code, expires); err != nil {
		_ = s.store.ConsumeChallenge(ctx, key, challenge)
		return RequestResult{}, err
	}
	return RequestResult{Status: "verification_sent", ExpiresInSeconds: int64(s.cfg.CodeTTL.Seconds())}, nil
}

func (s *Service) VerifyBackupEmailCode(ctx context.Context, accountID, identityID uuid.UUID, email, code string) (accountservice.AccountIdentitySafe, error) {
	if s == nil || s.store == nil {
		return accountservice.AccountIdentitySafe{}, ErrMissingDependency
	}
	account, ok := s.account.(backupEmailAccount)
	if !ok {
		return accountservice.AccountIdentitySafe{}, ErrMissingDependency
	}
	if accountID == uuid.Nil || identityID == uuid.Nil {
		return accountservice.AccountIdentitySafe{}, domain.ErrInvalidIdentity
	}
	normalized, err := normalizeEmail(email)
	if err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	if strings.TrimSpace(code) == "" {
		return accountservice.AccountIdentitySafe{}, ErrInvalidCode
	}
	hash := hashIdentity(normalized)
	if err := s.checkLimit(ctx, verifyRateKey(accountID, hash), s.cfg.VerifyLimit, s.cfg.VerifyWindow); err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	key := backupChallengeKey(accountID, identityID, hash)
	challenge, err := s.store.LoadChallenge(ctx, key)
	if err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	if challenge.AccountID != accountID || challenge.IdentityHash != hash || challenge.BackupIdentityID != identityID || challenge.BackupVersion.IsZero() {
		return accountservice.AccountIdentitySafe{}, ErrInvalidCode
	}
	if !s.cfg.Now().Before(challenge.ExpiresAt) {
		return accountservice.AccountIdentitySafe{}, ErrExpiredCode
	}
	if !hmac.Equal([]byte(challenge.CodeHash), []byte(s.codeHash(accountID, backupProofIdentity(identityID, challenge.BackupVersion, normalized), code))) {
		return accountservice.AccountIdentitySafe{}, ErrInvalidCode
	}
	if err := s.store.ConsumeChallenge(ctx, key, challenge); err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	// Repository CAS also rejects proofs for a superseded backup binding.
	return account.ReplaceVerifiedBackupEmail(ctx, accountID, accountID, identityID, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: normalized, Verified: true}, challenge.BackupVersion)
}
