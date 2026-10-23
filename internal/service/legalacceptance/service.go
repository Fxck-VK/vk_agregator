// Package legalacceptance records explicit legal choices for canonical accounts.
package legalacceptance

import (
	"context"
	"errors"
	"regexp"

	"github.com/google/uuid"
)

var (
	ErrInvalid     = errors.New("legalacceptance: invalid choice")
	ErrVersion     = errors.New("legalacceptance: documents changed")
	ErrUnpublished = errors.New("legalacceptance: documents not published")
	ErrEmail       = errors.New("legalacceptance: verified primary email required")
	ErrReplay      = errors.New("legalacceptance: idempotency conflict")
)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Metadata struct {
	Published         bool   `json:"published"`
	Version           string `json:"version"`
	Checksum          string `json:"checksum"`
	MarketingVersion  string `json:"marketing_version"`
	MarketingChecksum string `json:"marketing_checksum"`
	Snapshot          []byte `json:"-"`
}
type State struct {
	RecipientVersion   string `json:"recipient_version"`
	Accepted           bool   `json:"accepted"`
	MarketingOptIn     bool   `json:"marketing_opt_in"`
	MarketingAvailable bool   `json:"marketing_available"`
}
type Status struct {
	Metadata
	State
}
type Record struct {
	RecipientVersion string
	AccountID        uuid.UUID
	RequestID        uuid.UUID
	Kind             string
	Metadata         Metadata
	Locale           string
	MarketingOptIn   bool
}
type Repository interface {
	State(context.Context, uuid.UUID, Metadata) (State, error)
	Record(context.Context, Record) error
}
type Service struct {
	repo Repository
	meta Metadata
}

func New(repo Repository, meta Metadata) *Service { return &Service{repo: repo, meta: meta} }
func (s *Service) Status(ctx context.Context, id uuid.UUID) (Status, error) {
	if id == uuid.Nil {
		return Status{}, ErrInvalid
	}
	out := Status{Metadata: s.meta}
	if !s.meta.Published {
		return out, nil
	}
	state, err := s.repo.State(ctx, id, s.meta)
	out.State = state
	return out, err
}

type AcceptInput struct {
	RecipientVersion  string    `json:"recipient_version"`
	RequestID         uuid.UUID `json:"-"`
	Version           string    `json:"version"`
	Checksum          string    `json:"checksum"`
	MarketingVersion  string    `json:"marketing_version"`
	MarketingChecksum string    `json:"marketing_checksum"`
	Locale            string    `json:"locale"`
	Accepted          bool      `json:"accepted"`
	MarketingOptIn    bool      `json:"marketing_opt_in"`
}

func (s *Service) Accept(ctx context.Context, id uuid.UUID, in AcceptInput) (Status, error) {
	if !validActor(id, in.RequestID, in.Locale) || !in.Accepted {
		return Status{}, ErrInvalid
	}
	if !s.meta.Published {
		return Status{}, ErrUnpublished
	}
	if in.Version != s.meta.Version || in.Checksum != s.meta.Checksum || !digestPattern.MatchString(in.Checksum) {
		return Status{}, ErrVersion
	}
	if in.MarketingOptIn {
		if in.MarketingVersion != s.meta.MarketingVersion || in.MarketingChecksum != s.meta.MarketingChecksum {
			return Status{}, ErrVersion
		}
		state, err := s.repo.State(ctx, id, s.meta)
		if err != nil {
			return Status{}, err
		}
		if !state.MarketingAvailable || in.RecipientVersion == "" || in.RecipientVersion != state.RecipientVersion {
			return Status{}, ErrEmail
		}
	}
	if err := s.repo.Record(ctx, Record{AccountID: id, RequestID: in.RequestID, Kind: "accept", Metadata: s.meta, Locale: in.Locale, MarketingOptIn: in.MarketingOptIn, RecipientVersion: in.RecipientVersion}); err != nil {
		return Status{}, err
	}
	return s.Status(ctx, id)
}

type MarketingInput struct {
	RecipientVersion string    `json:"recipient_version"`
	RequestID        uuid.UUID `json:"-"`
	Version          string    `json:"version"`
	Checksum         string    `json:"checksum"`
	Locale           string    `json:"locale"`
	OptIn            bool      `json:"opt_in"`
}

func (s *Service) UpdateMarketing(ctx context.Context, id uuid.UUID, in MarketingInput) (Status, error) {
	if !validActor(id, in.RequestID, in.Locale) {
		return Status{}, ErrInvalid
	}
	kind := "marketing_revoke"
	if in.OptIn {
		if !s.meta.Published {
			return Status{}, ErrUnpublished
		}
		if in.Version != s.meta.MarketingVersion || in.Checksum != s.meta.MarketingChecksum {
			return Status{}, ErrVersion
		}
		state, err := s.repo.State(ctx, id, s.meta)
		if err != nil {
			return Status{}, err
		}
		if !state.MarketingAvailable || in.RecipientVersion == "" || in.RecipientVersion != state.RecipientVersion {
			return Status{}, ErrEmail
		}
		kind = "marketing_grant"
	}
	if err := s.repo.Record(ctx, Record{AccountID: id, RequestID: in.RequestID, Kind: kind, Metadata: s.meta, Locale: in.Locale, MarketingOptIn: in.OptIn, RecipientVersion: in.RecipientVersion}); err != nil {
		return Status{}, err
	}
	return s.Status(ctx, id)
}
func validActor(account, request uuid.UUID, locale string) bool {
	return account != uuid.Nil && request != uuid.Nil && (locale == "ru" || locale == "en")
}
