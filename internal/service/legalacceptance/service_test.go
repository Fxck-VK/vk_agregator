package legalacceptance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type testRepository struct {
	state   State
	records []Record
	err     error
}

func (r *testRepository) State(context.Context, uuid.UUID, Metadata) (State, error) {
	return r.state, r.err
}
func (r *testRepository) Record(_ context.Context, v Record) error {
	if r.err != nil {
		return r.err
	}
	r.records = append(r.records, v)
	return nil
}
func testService() (*Service, *testRepository, Metadata) {
	m := Metadata{Published: true, Version: "2026-10-23.3", Checksum: strings.Repeat("a", 64), MarketingVersion: "2026-10-23.1", MarketingChecksum: strings.Repeat("b", 64)}
	r := &testRepository{state: State{MarketingAvailable: true, RecipientVersion: strings.Repeat("d", 64)}}
	return New(r, m), r, m
}
func TestAcceptanceRejectsWrongVersionAndDoesNotSubscribeByDefault(t *testing.T) {
	s, r, m := testService()
	ctx := context.Background()
	account := uuid.New()
	in := AcceptInput{RequestID: uuid.New(), Version: m.Version, Checksum: m.Checksum, Locale: "ru", Accepted: true}
	stale := in
	stale.Checksum = strings.Repeat("c", 64)
	if _, err := s.Accept(ctx, account, stale); !errors.Is(err, ErrVersion) {
		t.Fatalf("wrong checksum: %v", err)
	}
	if len(r.records) != 0 {
		t.Fatal("stale acceptance was persisted")
	}
	if _, err := s.Accept(ctx, account, in); err != nil {
		t.Fatal(err)
	}
	if len(r.records) != 1 || r.records[0].AccountID != account || r.records[0].MarketingOptIn {
		t.Fatal("acceptance must be owned and must not opt into marketing")
	}
	in.Accepted = false
	if _, err := s.Accept(ctx, account, in); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing affirmative acceptance: %v", err)
	}
}
func TestMarketingRequiresSeparateCurrentConsentAndCanAlwaysBeWithdrawn(t *testing.T) {
	s, r, m := testService()
	ctx := context.Background()
	id := uuid.New()
	in := MarketingInput{RequestID: uuid.New(), Locale: "en", OptIn: true, Version: "old", Checksum: m.MarketingChecksum, RecipientVersion: r.state.RecipientVersion}
	if _, err := s.UpdateMarketing(ctx, id, in); !errors.Is(err, ErrVersion) {
		t.Fatal(err)
	}
	in.Version = m.MarketingVersion
	r.state.MarketingAvailable = false
	if _, err := s.UpdateMarketing(ctx, id, in); !errors.Is(err, ErrEmail) {
		t.Fatalf("unverified recipient: %v", err)
	}
	r.state.MarketingAvailable = true
	if _, err := s.UpdateMarketing(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	if len(r.records) != 1 || r.records[0].Kind != "marketing_grant" {
		t.Fatal("separate grant missing")
	}
	s.meta.Published = false
	in.OptIn = false
	in.Version = "old"
	in.Checksum = "old"
	if _, err := s.UpdateMarketing(ctx, id, in); err != nil {
		t.Fatalf("withdrawal must survive unpublishing/version changes: %v", err)
	}
	if r.records[len(r.records)-1].Kind != "marketing_revoke" {
		t.Fatal("withdrawal missing")
	}
}
func TestAcceptanceFailsClosedOnStorageFailureOrUnpublishedDocuments(t *testing.T) {
	s, r, m := testService()
	in := AcceptInput{RequestID: uuid.New(), Version: m.Version, Checksum: m.Checksum, Accepted: true, Locale: "ru"}
	r.err = errors.New("unavailable")
	if _, err := s.Accept(context.Background(), uuid.New(), in); err == nil {
		t.Fatal("storage failure accepted")
	}
	s.meta.Published = false
	r.err = nil
	if _, err := s.Accept(context.Background(), uuid.New(), in); !errors.Is(err, ErrUnpublished) {
		t.Fatal(err)
	}
	if len(r.records) != 0 {
		t.Fatal("draft acceptance recorded")
	}
}
