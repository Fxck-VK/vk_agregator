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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
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
	timeout, err := migrationTimeout(cfg)
	if err != nil {
		fatal("invalid configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

func migrationTimeout(cfg config.Config) (time.Duration, error) {
	if cfg.MigrationTimeout <= 0 {
		return 0, fmt.Errorf("MIGRATION_TIMEOUT must be positive")
	}
	return cfg.MigrationTimeout, nil
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

// checksum returns a platform-independent SHA-256 for a migration file.
func checksum(data []byte) string {
	return rawChecksum(normalizeLineEndings(data))
}

// checksumMatches accepts the canonical LF hash and the legacy CRLF hash while
// still rejecting SQL content changes.
func checksumMatches(recorded string, data []byte) bool {
	normalized := normalizeLineEndings(data)
	if recorded == rawChecksum(normalized) {
		return true
	}
	legacyCRLF := bytes.ReplaceAll(normalized, []byte("\n"), []byte("\r\n"))
	return recorded == rawChecksum(legacyCRLF)
}

func normalizeLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func rawChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

const nonTransactionalMigrationDirective = "-- migrate: no-transaction"

type migrationExecutionMode uint8

const (
	migrationExecutionTransactional migrationExecutionMode = iota
	migrationExecutionNonTransactional
)

// migrationExecutionModeForSQL returns the execution contract declared by a
// migration. Concurrent index DDL cannot run in a transaction, so it is only
// allowed when the first non-empty line explicitly opts into non-transactional
// execution. That makes the retry/idempotency requirement reviewable in the
// migration file itself instead of being inferred by the runner.
func migrationExecutionModeForSQL(sqlText []byte) (migrationExecutionMode, error) {
	lines := strings.Split(string(normalizeLineEndings(sqlText)), "\n")
	firstNonEmpty := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			firstNonEmpty = i
			break
		}
	}

	directiveAt := -1
	for i, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), nonTransactionalMigrationDirective) {
			if directiveAt >= 0 {
				return migrationExecutionTransactional, fmt.Errorf("%s may appear only once", nonTransactionalMigrationDirective)
			}
			directiveAt = i
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "-- migrate:") {
			return migrationExecutionTransactional, fmt.Errorf("unsupported migration directive on line %d: %s", i+1, strings.TrimSpace(line))
		}
	}

	if directiveAt < 0 {
		if containsConcurrentIndexDDL(sqlText) {
			return migrationExecutionTransactional, fmt.Errorf("concurrent index DDL requires %s as the first non-empty line", nonTransactionalMigrationDirective)
		}
		return migrationExecutionTransactional, nil
	}
	if directiveAt != firstNonEmpty {
		return migrationExecutionTransactional, fmt.Errorf("%s must be the first non-empty line", nonTransactionalMigrationDirective)
	}
	statements, err := splitMigrationSQLStatements(string(normalizeLineEndings(sqlText)))
	if err != nil {
		return migrationExecutionTransactional, fmt.Errorf("parse non-transactional migration: %w", err)
	}
	allowedStatements := 0
	for _, statement := range statements {
		statement = trimLeadingSQLComments(statement)
		if statement == "" {
			continue
		}
		if !isConcurrentIndexStatement(statement) {
			return migrationExecutionTransactional, fmt.Errorf("%s permits only CREATE INDEX CONCURRENTLY or DROP INDEX CONCURRENTLY statements", nonTransactionalMigrationDirective)
		}
		allowedStatements++
	}
	if allowedStatements == 0 {
		return migrationExecutionTransactional, fmt.Errorf("%s requires CREATE INDEX CONCURRENTLY or DROP INDEX CONCURRENTLY", nonTransactionalMigrationDirective)
	}
	return migrationExecutionNonTransactional, nil
}

func containsConcurrentIndexDDL(sqlText []byte) bool {
	return strings.Contains(strings.ToUpper(string(sqlText)), "INDEX CONCURRENTLY")
}

func isConcurrentIndexStatement(statement string) bool {
	statement = strings.ToUpper(strings.TrimSpace(statement))
	return strings.HasPrefix(statement, "CREATE INDEX CONCURRENTLY ") ||
		strings.HasPrefix(statement, "CREATE UNIQUE INDEX CONCURRENTLY ") ||
		strings.HasPrefix(statement, "DROP INDEX CONCURRENTLY ")
}

func trimLeadingSQLComments(statement string) string {
	for {
		statement = strings.TrimSpace(statement)
		switch {
		case strings.HasPrefix(statement, "--"):
			if end := strings.IndexByte(statement, '\n'); end >= 0 {
				statement = statement[end+1:]
				continue
			}
			return ""
		case strings.HasPrefix(statement, "/*"):
			end := strings.Index(statement[2:], "*/")
			if end < 0 {
				return statement
			}
			statement = statement[end+4:]
		default:
			return statement
		}
	}
}

// splitMigrationSQLStatements splits a SQL migration at top-level semicolons.
// It intentionally supports the quoting and comment forms used by PostgreSQL
// migrations so a non-transactional migration can run every concurrent DDL
// command in its own autocommit statement. A single multi-statement protocol
// query would instead be wrapped in an implicit transaction by PostgreSQL.
func splitMigrationSQLStatements(sqlText string) ([]string, error) {
	const (
		stateNormal = iota
		stateSingleQuote
		stateDoubleQuote
		stateLineComment
		stateBlockComment
		stateDollarQuote
	)

	state := stateNormal
	blockDepth := 0
	dollarDelimiter := ""
	start := 0
	statements := make([]string, 0, 2)
	for i := 0; i < len(sqlText); i++ {
		switch state {
		case stateNormal:
			switch sqlText[i] {
			case '\'':
				state = stateSingleQuote
			case '"':
				state = stateDoubleQuote
			case '-':
				if i+1 < len(sqlText) && sqlText[i+1] == '-' {
					i++
					state = stateLineComment
				}
			case '/':
				if i+1 < len(sqlText) && sqlText[i+1] == '*' {
					i++
					state = stateBlockComment
					blockDepth = 1
				}
			case '$':
				if delimiter, ok := sqlDollarQuoteDelimiter(sqlText[i:]); ok {
					dollarDelimiter = delimiter
					i += len(delimiter) - 1
					state = stateDollarQuote
				}
			case ';':
				if statement := strings.TrimSpace(sqlText[start:i]); statement != "" {
					statements = append(statements, statement)
				}
				start = i + 1
			}
		case stateSingleQuote:
			if sqlText[i] != '\'' {
				continue
			}
			if i+1 < len(sqlText) && sqlText[i+1] == '\'' {
				i++
				continue
			}
			state = stateNormal
		case stateDoubleQuote:
			if sqlText[i] != '"' {
				continue
			}
			if i+1 < len(sqlText) && sqlText[i+1] == '"' {
				i++
				continue
			}
			state = stateNormal
		case stateLineComment:
			if sqlText[i] == '\n' {
				state = stateNormal
			}
		case stateBlockComment:
			if sqlText[i] == '/' && i+1 < len(sqlText) && sqlText[i+1] == '*' {
				i++
				blockDepth++
				continue
			}
			if sqlText[i] == '*' && i+1 < len(sqlText) && sqlText[i+1] == '/' {
				i++
				blockDepth--
				if blockDepth == 0 {
					state = stateNormal
				}
			}
		case stateDollarQuote:
			if strings.HasPrefix(sqlText[i:], dollarDelimiter) {
				i += len(dollarDelimiter) - 1
				state = stateNormal
			}
		}
	}

	switch state {
	case stateSingleQuote:
		return nil, fmt.Errorf("unterminated single-quoted string")
	case stateDoubleQuote:
		return nil, fmt.Errorf("unterminated double-quoted identifier")
	case stateBlockComment:
		return nil, fmt.Errorf("unterminated block comment")
	case stateDollarQuote:
		return nil, fmt.Errorf("unterminated dollar-quoted string")
	}
	if statement := strings.TrimSpace(sqlText[start:]); statement != "" {
		statements = append(statements, statement)
	}
	return statements, nil
}

func sqlDollarQuoteDelimiter(sqlText string) (string, bool) {
	if len(sqlText) < 2 || sqlText[0] != '$' {
		return "", false
	}
	end := 1
	for end < len(sqlText) && (sqlText[end] == '_' || (sqlText[end] >= 'a' && sqlText[end] <= 'z') || (sqlText[end] >= 'A' && sqlText[end] <= 'Z') || (end > 1 && sqlText[end] >= '0' && sqlText[end] <= '9')) {
		end++
	}
	if end < len(sqlText) && sqlText[end] == '$' {
		return sqlText[:end+1], true
	}
	return "", false
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
		sqlText, err := readMigrationFile(dir, v, "up")
		if err != nil {
			return err
		}
		sum := checksum(sqlText)
		if rec, ok := applied[v]; ok {
			// Detect drift: a previously applied migration whose file changed.
			if rec != "" && !checksumMatches(rec, sqlText) {
				return fmt.Errorf("checksum mismatch for %s: applied %s, file %s", v, rec, sum)
			}
			continue
		}
		mode, err := migrationExecutionModeForSQL(sqlText)
		if err != nil {
			return fmt.Errorf("validate %s: %w", v, err)
		}
		if mode == migrationExecutionNonTransactional {
			// Concurrent index DDL must run outside a transaction. The migration is
			// explicitly directive-gated and each statement is run in its own
			// autocommit command. Recording happens only after all DDL succeeds;
			// if recording fails, the migration stays pending and its SQL must be
			// safe to run again.
			appliedNow, err := applyNonTransactionalMigration(ctx, pool, v, sum, sqlText)
			if err != nil {
				return err
			}
			if !appliedNow {
				// Another runner completed this migration while this runner waited
				// on the advisory lock. Its checksum was revalidated under the lock.
				continue
			}
			fmt.Printf("applied %s\n", v)
			count++
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

const migrationRunnerAdvisoryLockKey int64 = 0x4d494752415445 // "MIGRATE"

func applyNonTransactionalMigration(ctx context.Context, pool *pgxpool.Pool, version, sum string, sqlText []byte) (applied bool, err error) {
	err = withMigrationAdvisoryLock(ctx, pool, func(conn *pgxpool.Conn) error {
		recordedChecksum, recorded, err := migrationChecksum(ctx, conn, version)
		if err != nil {
			return err
		}
		if recorded {
			if recordedChecksum != "" && !checksumMatches(recordedChecksum, sqlText) {
				return fmt.Errorf("checksum mismatch for %s: applied %s, file %s", version, recordedChecksum, sum)
			}
			return nil
		}
		if err := executeNonTransactionalMigrationSQL(ctx, conn, sqlText); err != nil {
			return fmt.Errorf("apply %s: %w", version, err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)`, version, sum); err != nil {
			return fmt.Errorf("record %s after non-transactional SQL: %w", version, err)
		}
		applied = true
		return nil
	})
	return applied, err
}

func rollbackNonTransactionalMigration(ctx context.Context, pool *pgxpool.Pool, version string, sqlText []byte) (rolledBack bool, err error) {
	err = withMigrationAdvisoryLock(ctx, pool, func(conn *pgxpool.Conn) error {
		_, recorded, err := migrationChecksum(ctx, conn, version)
		if err != nil {
			return err
		}
		if !recorded {
			return nil
		}
		if err := executeNonTransactionalMigrationSQL(ctx, conn, sqlText); err != nil {
			return fmt.Errorf("rollback %s: %w", version, err)
		}
		if _, err := conn.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, version); err != nil {
			return fmt.Errorf("remove %s after non-transactional rollback: %w", version, err)
		}
		rolledBack = true
		return nil
	})
	return rolledBack, err
}

func withMigrationAdvisoryLock(ctx context.Context, pool *pgxpool.Pool, fn func(*pgxpool.Conn) error) (err error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration advisory-lock connection: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationRunnerAdvisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, unlockErr := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, migrationRunnerAdvisoryLockKey); unlockErr != nil && err == nil {
			err = fmt.Errorf("release migration advisory lock: %w", unlockErr)
		}
	}()
	return fn(conn)
}

func migrationChecksum(ctx context.Context, conn *pgxpool.Conn, version string) (checksum string, recorded bool, err error) {
	err = conn.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE version = $1`, version).Scan(&checksum)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return checksum, true, nil
}

func executeNonTransactionalMigrationSQL(ctx context.Context, conn *pgxpool.Conn, sqlText []byte) error {
	statements, err := splitMigrationSQLStatements(string(normalizeLineEndings(sqlText)))
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := conn.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
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
		sqlText, err := readMigrationFile(dir, v, "down")
		if err != nil {
			return err
		}
		mode, err := migrationExecutionModeForSQL(sqlText)
		if err != nil {
			return fmt.Errorf("validate rollback %s: %w", v, err)
		}
		if mode == migrationExecutionNonTransactional {
			rolledBack, err := rollbackNonTransactionalMigration(ctx, pool, v, sqlText)
			if err != nil {
				return err
			}
			if !rolledBack {
				// Another runner rolled this migration back while this runner waited
				// on the advisory lock. Do not continue and remove an older version.
				fmt.Println("down complete: migration already rolled back")
				return nil
			}
			fmt.Printf("rolled back %s\n", v)
			return nil
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

func readMigrationFile(dir, version, direction string) ([]byte, error) {
	if !validMigrationVersion(version) {
		return nil, fmt.Errorf("invalid migration version %q", version)
	}
	if direction != "up" && direction != "down" {
		return nil, fmt.Errorf("invalid migration direction %q", direction)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = root.Close()
	}()
	return root.ReadFile(version + "." + direction + ".sql")
}

func validMigrationVersion(version string) bool {
	if version == "" || strings.Contains(version, ".") || strings.ContainsAny(version, `/\`) {
		return false
	}
	for _, r := range version {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", args...)
	os.Exit(1)
}
