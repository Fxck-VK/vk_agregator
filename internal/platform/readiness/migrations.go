package readiness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// SchemaQuerier is the minimal database contract required to check whether the
// runtime schema has been migrated to the latest bundled migration.
type SchemaQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// LatestMigrationVersion returns the newest .up.sql migration version bundled
// with the running binary/container.
func LatestMigrationVersion(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read migrations dir: %w", err)
	}
	seen := map[string]struct{}{}
	var versions []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		version := strings.TrimSuffix(name, ".up.sql")
		if version == "" {
			continue
		}
		if _, ok := seen[version]; ok {
			continue
		}
		seen[version] = struct{}{}
		versions = append(versions, version)
	}
	if len(versions) == 0 {
		return "", errors.New("no up migrations found")
	}
	sort.Strings(versions)
	return versions[len(versions)-1], nil
}

// CheckLatestMigrationApplied fails until the newest bundled migration is
// present in schema_migrations. It intentionally does not compare checksums:
// readiness verifies deploy ordering, while migration checksum enforcement
// remains the migrate command's job.
func CheckLatestMigrationApplied(ctx context.Context, db SchemaQuerier, dir string) (string, error) {
	latest, err := LatestMigrationVersion(dir)
	if err != nil {
		return "", err
	}
	var applied string
	if err := db.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE version = $1`, latest).Scan(&applied); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return latest, fmt.Errorf("latest migration is not applied")
		}
		return latest, fmt.Errorf("query schema migrations: %w", err)
	}
	return latest, nil
}
