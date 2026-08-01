package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"vk-ai-aggregator/internal/domain"
)

var errNullableRecorder = errors.New("nullable recorder")

type nullableRecorder struct {
	query string
	args  []any
}

func (r *nullableRecorder) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errNullableRecorder
}

func (r *nullableRecorder) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errNullableRecorder
}

func (r *nullableRecorder) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	r.query = query
	r.args = args
	return nullableErrorRow{}
}

type nullableErrorRow struct{}

func (nullableErrorRow) Scan(...any) error { return errNullableRecorder }

func TestAccountNativeNullableLegacyBindings(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()

	t.Run("job", func(t *testing.T) {
		recorder := &nullableRecorder{}
		err := NewJobRepository(recorder).Create(ctx, &domain.Job{AccountID: accountID})
		assertNullableRecorderError(t, err)
		assertNilArgument(t, recorder.args, 1, "job user_id")
		assertNilArgument(t, recorder.args, 11, "job vk_peer_id")
	})

	t.Run("artifact", func(t *testing.T) {
		recorder := &nullableRecorder{}
		err := NewArtifactRepository(recorder).Create(ctx, &domain.Artifact{OwnerAccountID: accountID})
		assertNullableRecorderError(t, err)
		assertNilArgument(t, recorder.args, 1, "artifact owner_user_id")
	})

	t.Run("credit account", func(t *testing.T) {
		recorder := &nullableRecorder{}
		err := NewBillingRepositoryTx(recorder).CreateAccount(ctx, &domain.CreditAccount{OwnerAccountID: accountID})
		assertNullableRecorderError(t, err)
		assertNilArgument(t, recorder.args, 1, "credit account user_id")
	})

	t.Run("payment intent", func(t *testing.T) {
		recorder := &nullableRecorder{}
		err := NewPaymentRepository(recorder).CreateIntent(ctx, &domain.PaymentIntent{AccountID: accountID})
		assertNullableRecorderError(t, err)
		assertNilArgument(t, recorder.args, 1, "payment intent user_id")
	})
}

func TestAccountNativeNullableLegacyScans(t *testing.T) {
	accountID := uuid.New()

	t.Run("job", func(t *testing.T) {
		var job domain.Job
		if err := scanJob(nullableLegacyScanRow{uuidPointers: map[int]uuid.UUID{2: accountID}, nullableUUIDs: map[int]bool{1: true}, nullableInt64s: map[int]bool{11: true}}, &job); err != nil {
			t.Fatalf("scan job: %v", err)
		}
		if job.AccountID != accountID || job.UserID != uuid.Nil || job.VKPeerID != 0 {
			t.Fatalf("job lost account or retained absent legacy provenance: %#v", job)
		}
	})

	t.Run("artifact", func(t *testing.T) {
		var artifact domain.Artifact
		if err := scanArtifact(nullableLegacyScanRow{uuidPointers: map[int]uuid.UUID{2: accountID}, nullableUUIDs: map[int]bool{1: true}}, &artifact); err != nil {
			t.Fatalf("scan artifact: %v", err)
		}
		if artifact.OwnerAccountID != accountID || artifact.OwnerUserID != uuid.Nil {
			t.Fatalf("artifact lost account or retained absent legacy provenance: %#v", artifact)
		}
	})

	t.Run("credit account", func(t *testing.T) {
		var creditAccount domain.CreditAccount
		if err := scanAccount(nullableLegacyScanRow{uuidPointers: map[int]uuid.UUID{2: accountID}, nullableUUIDs: map[int]bool{1: true}}, &creditAccount); err != nil {
			t.Fatalf("scan credit account: %v", err)
		}
		if creditAccount.OwnerAccountID != accountID || creditAccount.UserID != uuid.Nil {
			t.Fatalf("credit account lost account or retained absent legacy provenance: %#v", creditAccount)
		}
	})

	t.Run("payment intent", func(t *testing.T) {
		var intent domain.PaymentIntent
		if err := scanPaymentIntent(nullableLegacyScanRow{uuidPointers: map[int]uuid.UUID{2: accountID}, nullableUUIDs: map[int]bool{1: true}}, &intent); err != nil {
			t.Fatalf("scan payment intent: %v", err)
		}
		if intent.AccountID != accountID || intent.UserID != uuid.Nil {
			t.Fatalf("payment intent lost account or retained absent legacy provenance: %#v", intent)
		}
	})
}

type nullableLegacyScanRow struct {
	uuidPointers   map[int]uuid.UUID
	nullableUUIDs  map[int]bool
	nullableInt64s map[int]bool
}

func (r nullableLegacyScanRow) Scan(dest ...any) error {
	for index := range r.nullableUUIDs {
		if _, ok := dest[index].(**uuid.UUID); !ok {
			return fmt.Errorf("column %d must scan nullable UUID, got %T", index, dest[index])
		}
	}
	for index := range r.nullableInt64s {
		if _, ok := dest[index].(**int64); !ok {
			return fmt.Errorf("column %d must scan nullable int64, got %T", index, dest[index])
		}
	}
	for index, value := range r.uuidPointers {
		pointer, ok := dest[index].(**uuid.UUID)
		if !ok {
			return fmt.Errorf("column %d must scan UUID pointer, got %T", index, dest[index])
		}
		value := value
		*pointer = &value
	}
	return nil
}

func TestAccountNativeNullableMigration(t *testing.T) {
	root, err := nullableMigrationRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "migrations", "000043_account_native_legacy_nullable.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	migration := strings.ToLower(string(raw))
	for _, required := range []string{
		"alter table jobs alter column user_id drop not null;",
		"alter table jobs alter column vk_peer_id drop not null;",
		"alter table artifacts alter column owner_user_id drop not null;",
		"alter table credit_accounts alter column user_id drop not null;",
		"alter table payment_intents alter column user_id drop not null;",
		"raise exception",
		"owner_account_id",
		"group by owner_account_id, currency",
		"having count(*) > 1",
		"create unique index if not exists credit_accounts_owner_account_currency_unique",
		"on credit_accounts (owner_account_id, currency)",
		"where owner_account_id is not null",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"begin;", "commit;", "delete from", "update credit_accounts", "create index concurrently", "drop column"} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("migration must not contain %q", forbidden)
		}
	}
}

func nullableMigrationRepoRoot() (string, error) {
	return filepath.Abs(filepath.Join("..", "..", "..", ".."))
}

func assertNullableRecorderError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, errNullableRecorder) {
		t.Fatalf("expected recorder error, got %v", err)
	}
}

func assertNilArgument(t *testing.T, args []any, index int, label string) {
	t.Helper()
	if len(args) <= index {
		t.Fatalf("%s missing argument %d: %#v", label, index, args)
	}
	switch value := args[index].(type) {
	case nil:
		return
	case *uuid.UUID:
		if value == nil {
			return
		}
	case *int64:
		if value == nil {
			return
		}
	case *string:
		if value == nil {
			return
		}
	}
	t.Fatalf("%s must bind SQL NULL, got %#v", label, args[index])
}
