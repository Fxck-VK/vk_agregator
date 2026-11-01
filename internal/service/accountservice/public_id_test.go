package accountservice_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountservice"
)

type failingPublicIDs struct{ err error }

func (f failingPublicIDs) GetAccountPublicNumber(context.Context, uuid.UUID) (int64, error) {
	return 0, f.err
}
func (f failingPublicIDs) GetAccountByPublicNumber(context.Context, int64) (*domain.Account, error) {
	return nil, f.err
}

func TestProfileIncludesStableServerPublicID(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	identity, err := identities.CreateAccountWithIdentity(ctx, domain.IdentityProviderYandex, "subject", "subject")
	if err != nil {
		t.Fatal(err)
	}
	svc := accountservice.New(identities, nil, accountservice.WithPublicIDRepository(identities))
	first, err := svc.Profile(ctx, identity.AccountID)
	if err != nil || first.PublicID != "EP-000001" {
		t.Fatalf("profile public ID = %q/%v", first.PublicID, err)
	}
	if _, err = identities.LinkIdentity(ctx, identity.AccountID, domain.IdentityProviderEmail, "backup@example.test", "backup@example.test"); err != nil {
		t.Fatal(err)
	}
	second, err := svc.Profile(ctx, identity.AccountID)
	if err != nil || second.PublicID != first.PublicID || second.AccountID != first.AccountID {
		t.Fatal("profile number changed after linking", err)
	}
}

func TestProfileDoesNotInventPublicIDOnRepositoryFailure(t *testing.T) {
	identities := memory.NewAccountIdentityRepo()
	failure := errors.New("synthetic storage failure")
	svc := accountservice.New(identities, nil, accountservice.WithPublicIDRepository(failingPublicIDs{failure}))
	if _, err := svc.Profile(context.Background(), uuid.New()); !errors.Is(err, failure) {
		t.Fatal("public ID failure hidden", err)
	}
}
