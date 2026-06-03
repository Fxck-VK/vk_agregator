// Command migrate applies or rolls back SQL migrations from the migrations
// directory against the configured PostgreSQL database.
//
// Usage:
//
//	go run ./cmd/migrate up      # apply all pending migrations
//	go run ./cmd/migrate down    # roll back the most recent migration
//	go run ./cmd/migrate status  # print applied/pending migrations
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/platform/config"
)

func main() {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("connect: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("ping: %v", err)
	}

	if err := ensureTable(ctx, pool); err != nil {
		fatal("ensure migrations table: %v", err)
	}

	switch cmd {
	case "up":
		err = up(ctx, pool, cfg.MigrationsDir)
	case "down":
		err = down(ctx, pool, cfg.MigrationsDir)
	case "status":
		err = status(ctx, pool, cfg.MigrationsDir)
	default:
		fatal("unknown command %q (use up|down|status)", cmd)
	}
	if err != nil {
		fatal("%s: %v", cmd, err)
	}
}

func ensureTable(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}
	// Track the checksum of each applied migration to detect drift (audit D1).
	_, err := pool.Exec(ctx, `ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum TEXT NOT NULL DEFAULT ''`)
	return err
}

// checksum returns the hex-encoded SHA-256 of a migration file's contents.
func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func up(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}
	versions, err := migrationVersions(dir)
	if err != nil {
		return err
	}
	count := 0
	for _, v := range versions {
		sqlText, err := os.ReadFile(filepath.Join(dir, v+".up.sql"))
		if err != nil {
			return err
		}
		sum := checksum(sqlText)
		if rec, ok := applied[v]; ok {
			// Detect drift: a previously applied migration whose file changed.
			if rec != "" && rec != sum {
				return fmt.Errorf("checksum mismatch for %s: applied %s, file %s", v, rec, sum)
			}
			continue
		}
		// Apply the migration and record it in a single transaction so a failed
		// migration never leaves a half-applied schema or a stale version row
		// (audit D1).
		if err := runTx(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sqlText)); err != nil {
				return fmt.Errorf("apply %s: %w", v, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)`, v, sum); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		fmt.Printf("applied %s\n", v)
		count++
	}
	fmt.Printf("up complete: %d migration(s) applied\n", count)
	return nil
}

// runTx runs fn inside a transaction, committing on success and rolling back on
// any error.
func runTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func down(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}
	versions, err := migrationVersions(dir)
	if err != nil {
		return err
	}
	for i := len(versions) - 1; i >= 0; i-- {
		v := versions[i]
		if _, ok := applied[v]; !ok {
			continue
		}
		sqlText, err := os.ReadFile(filepath.Join(dir, v+".down.sql"))
		if err != nil {
			return err
		}
		if err := runTx(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sqlText)); err != nil {
				return fmt.Errorf("rollback %s: %w", v, err)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, v); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		fmt.Printf("rolled back %s\n", v)
		return nil
	}
	fmt.Println("down complete: nothing to roll back")
	return nil
}

func status(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}
	versions, err := migrationVersions(dir)
	if err != nil {
		return err
	}
	for _, v := range versions {
		state := "pending"
		if _, ok := applied[v]; ok {
			state = "applied"
		}
		fmt.Printf("%-10s %s\n", state, v)
	}
	return nil
}

// appliedVersions returns the applied migration versions mapped to their
// recorded checksums.
func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]string, error) {
	rows, err := pool.Query(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var v, sum string
		if err := rows.Scan(&v, &sum); err != nil {
			return nil, err
		}
		out[v] = sum
	}
	return out, rows.Err()
}

// migrationVersions returns sorted migration version names (without the
// .up.sql/.down.sql suffix) found in dir.
func migrationVersions(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var versions []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		v := strings.TrimSuffix(name, ".up.sql")
		if !seen[v] {
			seen[v] = true
			versions = append(versions, v)
		}
	}
	sort.Strings(versions)
	return versions, nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", args...)
	os.Exit(1)
}
