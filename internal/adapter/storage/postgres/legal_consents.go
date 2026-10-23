package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/legalacceptance"
)

type LegalConsentRepository struct{ db webReferralBeginQuerier }

func NewLegalConsentRepository(db webReferralBeginQuerier) *LegalConsentRepository {
	return &LegalConsentRepository{db}
}

var _ legalacceptance.Repository = (*LegalConsentRepository)(nil)

// This opaque version is returned only to the owning authenticated account.
// It binds a UI choice to its recipient without copying the address into events.
func legalRecipient(ctx context.Context, q Querier, id uuid.UUID) (string, error) {
	var raw string
	err := q.QueryRow(ctx, `SELECT normalized_id FROM account_identities WHERE account_id=$1 AND provider='email' AND verified_at IS NOT NULL ORDER BY created_at,id LIMIT 1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	raw = strings.ToLower(strings.TrimSpace(raw))
	address, err := mail.ParseAddress(raw)
	if err != nil || address.Address != raw {
		return "", nil
	}
	sum := sha256.Sum256([]byte("epifera:email-marketing-recipient:v1\n" + id.String() + "\n" + raw))
	return hex.EncodeToString(sum[:]), nil
}
func (r *LegalConsentRepository) State(ctx context.Context, id uuid.UUID, m legalacceptance.Metadata) (legalacceptance.State, error) {
	var out legalacceptance.State
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_legal_events WHERE account_id=$1 AND kind='accept' AND document_version=$2 AND document_checksum=$3)`, id, m.Version, m.Checksum).Scan(&out.Accepted); err != nil {
		return out, err
	}
	recipient, err := legalRecipient(ctx, r.db, id)
	if err != nil {
		return out, err
	}
	out.MarketingAvailable = recipient != ""
	out.RecipientVersion = recipient
	var granted bool
	var version, checksum, previousRecipient string
	err = r.db.QueryRow(ctx, `SELECT marketing_opt_in,marketing_version,marketing_checksum,recipient_hash FROM account_legal_events WHERE account_id=$1 AND (kind<>'accept' OR marketing_opt_in) ORDER BY sequence DESC LIMIT 1`, id).Scan(&granted, &version, &checksum, &previousRecipient)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.MarketingOptIn = granted && out.MarketingAvailable && recipient == previousRecipient && version == m.MarketingVersion && checksum == m.MarketingChecksum
	return out, nil
}
func (r *LegalConsentRepository) Record(ctx context.Context, v legalacceptance.Record) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var locked uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1 AND status='active' FOR UPDATE`, v.AccountID).Scan(&locked); err != nil {
		return mapError(err)
	}
	// The account lock is shared with identity replacement/unlink. Resolve the
	// recipient again here so a concurrent email change cannot enroll a new email.
	fingerprintInput := struct {
		Kind, Version, Checksum, MarketingVersion, MarketingChecksum, Locale, RecipientVersion string
		OptIn                                                                                  bool
	}{v.Kind, v.Metadata.Version, v.Metadata.Checksum, v.Metadata.MarketingVersion, v.Metadata.MarketingChecksum, v.Locale, v.RecipientVersion, v.MarketingOptIn}
	if v.Kind == "marketing_revoke" {
		// Withdrawal has no version/recipient precondition. Its retry remains the
		// same action even if a deployment or email replacement happened meanwhile.
		fingerprintInput.Version, fingerprintInput.Checksum = "", ""
		fingerprintInput.MarketingVersion, fingerprintInput.MarketingChecksum = "", ""
		fingerprintInput.RecipientVersion = ""
	}
	fingerprintBytes, _ := json.Marshal(fingerprintInput)
	digest := sha256.Sum256(fingerprintBytes)
	fingerprint := hex.EncodeToString(digest[:])
	var previous string
	err = tx.QueryRow(ctx, `SELECT request_fingerprint FROM account_legal_events WHERE account_id=$1 AND request_id=$2`, v.AccountID, v.RequestID).Scan(&previous)
	if err == nil {
		if previous != fingerprint {
			return legalacceptance.ErrReplay
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	recipient := ""
	if v.MarketingOptIn {
		recipient, err = legalRecipient(ctx, tx, v.AccountID)
		if err != nil {
			return err
		}
		if recipient == "" || recipient != v.RecipientVersion {
			return legalacceptance.ErrEmail
		}
	}
	if len(v.Metadata.Snapshot) == 0 {
		return domain.ErrInvalidIdentity
	}
	_, err = tx.Exec(ctx, `INSERT INTO account_legal_revisions(checksum,version,snapshot) VALUES($1,$2,$3::jsonb) ON CONFLICT(checksum) DO NOTHING`, v.Metadata.Checksum, v.Metadata.Version, string(v.Metadata.Snapshot))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO account_legal_events(account_id,request_id,kind,document_version,document_checksum,marketing_version,marketing_checksum,marketing_opt_in,recipient_hash,locale,request_fingerprint) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.AccountID, v.RequestID, v.Kind, v.Metadata.Version, v.Metadata.Checksum, v.Metadata.MarketingVersion, v.Metadata.MarketingChecksum, v.MarketingOptIn, recipient, v.Locale, fingerprint)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
