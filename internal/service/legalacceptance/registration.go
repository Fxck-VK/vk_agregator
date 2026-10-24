package legalacceptance

import (
	"context"
	"github.com/google/uuid"
)

// RegistrationNotice identifies the small notice displayed before registration.
// It is not a client-supplied account identity or an advertising permission.
type RegistrationNotice struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
	Locale   string `json:"locale"`
}

func (s *Service) ValidateRegistration(in *RegistrationNotice) error {
	if !s.meta.Published {
		return nil
	}
	if in == nil || (in.Locale != "ru" && in.Locale != "en") {
		return ErrInvalid
	}
	if in.Version != s.meta.Version || in.Checksum != s.meta.Checksum || !digestPattern.MatchString(in.Checksum) {
		return ErrVersion
	}
	return nil
}

func (s *Service) AcceptRegistration(ctx context.Context, account uuid.UUID, binding string, in *RegistrationNotice) error {
	if err := s.ValidateRegistration(in); err != nil {
		return err
	}
	if !s.meta.Published {
		return nil
	}
	if len(binding) != 43 {
		return ErrInvalid
	}
	// Domain-separated, server-proof-bound identity survives a response/cookie
	// failure. The repository's unique key deduplicates identical retries.
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("epifera:registration-legal:"+binding))
	_, err := s.Accept(ctx, account, AcceptInput{RequestID: id, Version: in.Version, Checksum: in.Checksum, Locale: in.Locale, Accepted: true})
	return err
}
