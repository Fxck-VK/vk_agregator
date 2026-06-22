package readiness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestLatestMigrationVersion(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"000002_second.up.sql",
		"000001_first.up.sql",
		"000001_first.down.sql",
		"README.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := LatestMigrationVersion(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "000002_second" {
		t.Fatalf("latest migration = %q, want %q", got, "000002_second")
	}
}

func TestCheckLatestMigrationApplied(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000001_first.up.sql"), []byte("-- test"), 0o600); err != nil {
		t.Fatal(err)
	}

	version, err := CheckLatestMigrationApplied(context.Background(), fakeSchemaQuerier{version: "000001_first"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if version != "000001_first" {
		t.Fatalf("version = %q, want %q", version, "000001_first")
	}

	_, err = CheckLatestMigrationApplied(context.Background(), fakeSchemaQuerier{err: pgx.ErrNoRows}, dir)
	if err == nil {
		t.Fatal("expected pending migration error")
	}
}

type fakeSchemaQuerier struct {
	version string
	err     error
}

func (q fakeSchemaQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakeRow(q)
}

type fakeRow fakeSchemaQuerier

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 0 {
		return errors.New("missing scan destination")
	}
	value, ok := dest[0].(*string)
	if !ok {
		return errors.New("scan destination must be *string")
	}
	*value = r.version
	return nil
}
